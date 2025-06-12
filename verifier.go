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

type Conf struct {
	CheckDisposableDomains bool
	CheckNS                bool
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

func Verify(ctx context.Context, input string, conf *Conf) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case err := <-verifyWithTimeout(input, conf):
		if err != nil {
			return false, err
		}
	}
	return true, nil
}

func verifyWithTimeout(input string, conf *Conf) chan error {
	ch := make(chan error, 1)
	go verifyWithChannel(ch, input, conf)
	return ch
}

func verifyWithChannel(ch chan error, input string, conf *Conf) {
	defer close(ch)
	email, err := mail.ParseAddress(input)
	if err != nil {
		ch <- err
		return
	}
	i := strings.LastIndex(email.Address, "@")
	if i < 0 || i == len(email.Address)-1 {
		ch <- ErrInvalidSyntax
		return
	}
	domain := email.Address[i+1:]
	if conf.CheckDisposableDomains {
		mu.RLock()
		_, ok := disposableDomains[domain]
		mu.RUnlock()
		if ok {
			ch <- ErrDisposableEmail
			return
		}
	}
	if conf.CheckNS {
		_, err = net.LookupNS(domain)
		if err != nil {
			ch <- err
			return
		}
	}
	records, err := net.LookupMX(domain)
	if err != nil {
		ch <- err
		return
	}
	if len(records) < 1 {
		ch <- ErrNoMXRecords
		return
	}
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
		ch <- err
		return
	}
	defer client.Close()
	err = client.Mail(fromEmail)
	if err != nil {
		ch <- err
		return
	}
	err = client.Rcpt(input)
	if err != nil {
		ch <- err
		return
	}
}
