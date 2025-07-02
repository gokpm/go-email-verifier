package verifier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

const (
	disposableDomainsURL = "https://raw.githubusercontent.com/disposable/disposable-email-domains/master/domains.json"
	fromEmail            = "user@example.com"
	smtpPort             = 25
)

type Config struct {
	ValidateMX      bool
	ValidateSMTP    bool
	ValidateDNS     bool
	BlockDisposable bool
}

var mu *sync.RWMutex
var disposableDomains map[string]struct{}
var tk *time.Ticker
var resolver = &net.Resolver{}
var dialer = &net.Dialer{}
var timeout = 30 * time.Second

var (
	ErrInvalidSyntax   = errors.New("invalid syntax")
	ErrDisposableEmail = errors.New("disposable domain")
	ErrNoMXRecords     = errors.New("mx record not found")
)

func init() {
	mu = &sync.RWMutex{}
	tk = time.NewTicker(1 * time.Hour)
	disposableDomains, _ = getDisposableDomains()
	go loop()
}

func getDisposableDomains() (map[string]struct{}, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, disposableDomainsURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	bytes, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	domains := []string{}
	err = json.Unmarshal(bytes, &domains)
	if err != nil {
		return nil, err
	}
	disposableDomains := map[string]struct{}{}
	for _, domain := range domains {
		disposableDomains[domain] = struct{}{}
	}
	return disposableDomains, nil
}

func loop() {
	for range tk.C {
		refresh()
	}
}

func refresh() error {
	domains, err := getDisposableDomains()
	if err != nil {
		return err
	}
	mu.Lock()
	disposableDomains = domains
	mu.Unlock()
	return nil
}

func Verify(ctx context.Context, input string, conf *Config) (bool, error) {
	email, err := mail.ParseAddress(input)
	if err != nil {
		return false, err
	}
	i := strings.LastIndex(email.Address, "@")
	if i < 0 || i == len(email.Address)-1 {
		return false, ErrInvalidSyntax
	}
	domain := email.Address[i+1:]
	if conf.BlockDisposable {
		mu.RLock()
		_, ok := disposableDomains[domain]
		mu.RUnlock()
		if ok {
			return false, ErrDisposableEmail
		}
	}
	if conf.ValidateDNS {
		_, err = resolver.LookupNS(ctx, domain)
		if err != nil {
			return false, err
		}
	}
	if conf.ValidateMX || conf.ValidateSMTP {
		records, err := resolver.LookupMX(ctx, domain)
		if err != nil {
			return false, err
		}
		if len(records) < 1 {
			return false, ErrNoMXRecords
		}
		if conf.ValidateSMTP {
			host := records[0].Host
			pref := records[0].Pref
			for _, record := range records {
				if record.Pref >= pref {
					continue
				}
				pref = record.Pref
				host = record.Host
			}
			addr := fmt.Sprintf("%[1]s:%[2]d", host, smtpPort)
			conn, err := dialer.DialContext(ctx, "tcp", addr)
			if err != nil {
				return false, err
			}
			defer conn.Close()
			deadline, ok := ctx.Deadline()
			if !ok {
				deadline = time.Now().Add(timeout)
			}
			err = conn.SetDeadline(deadline)
			if err != nil {
				return false, err
			}
			client, err := smtp.NewClient(conn, host)
			if err != nil {
				return false, err
			}
			defer func() {
				err := client.Quit()
				if err != nil {
					client.Close()
				}
			}()
			err = client.Mail(fromEmail)
			if err != nil {
				return false, err
			}
			err = client.Rcpt(input)
			if err != nil {
				return false, err
			}
		}
	}
	return true, nil
}
