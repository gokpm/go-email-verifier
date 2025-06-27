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

	"github.com/gokpm/go-sig"
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
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()
	log := sig.Start(ctx)
	defer log.End()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, disposableDomainsURL, nil)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	defer response.Body.Close()
	bytes, err := io.ReadAll(response.Body)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	domains := []string{}
	err = json.Unmarshal(bytes, &domains)
	if err != nil {
		log.Error(err)
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
	log := sig.Start(context.TODO())
	defer log.End()
	domains, err := getDisposableDomains()
	if err != nil {
		log.Error(err)
		return err
	}
	mu.Lock()
	disposableDomains = domains
	mu.Unlock()
	return nil
}

func Verify(ctx context.Context, input string, conf *Config) (bool, error) {
	log := sig.Start(ctx)
	defer log.End()
	select {
	case <-ctx.Done():
		err := ctx.Err()
		log.Error(err)
		return false, err
	case err := <-verifyWithTimeout(log.Ctx(), input, conf):
		if err != nil {
			log.Error(err)
			return false, err
		}
	}
	return true, nil
}

func verifyWithTimeout(ctx context.Context, input string, conf *Config) chan error {
	log := sig.Start(ctx)
	defer log.End()
	ch := make(chan error, 1)
	go verifyWithChannel(log.Ctx(), ch, input, conf)
	return ch
}

func verifyWithChannel(ctx context.Context, ch chan error, input string, conf *Config) {
	log := sig.Start(ctx)
	defer log.End()
	defer close(ch)
	email, err := mail.ParseAddress(input)
	if err != nil {
		log.Error(err)
		ch <- err
		return
	}
	i := strings.LastIndex(email.Address, "@")
	if i < 0 || i == len(email.Address)-1 {
		log.Error(ErrInvalidSyntax)
		ch <- ErrInvalidSyntax
		return
	}
	domain := email.Address[i+1:]
	if conf.BlockDisposable {
		mu.RLock()
		_, ok := disposableDomains[domain]
		mu.RUnlock()
		if ok {
			log.Error(ErrDisposableEmail)
			ch <- ErrDisposableEmail
			return
		}
	}
	if conf.ValidateDNS {
		_, err = net.LookupNS(domain)
		if err != nil {
			log.Error(err)
			ch <- err
			return
		}
	}
	if conf.ValidateMX || conf.ValidateSMTP {
		records, err := net.LookupMX(domain)
		if err != nil {
			log.Error(err)
			ch <- err
			return
		}
		if len(records) < 1 {
			log.Error(ErrNoMXRecords)
			ch <- ErrNoMXRecords
			return
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
			client, err := smtp.Dial(addr)
			if err != nil {
				log.Error(err)
				ch <- err
				return
			}
			defer client.Close()
			err = client.Mail(fromEmail)
			if err != nil {
				log.Error(err)
				ch <- err
				return
			}
			err = client.Rcpt(input)
			if err != nil {
				log.Error(err)
				ch <- err
				return
			}
		}
	}
}
