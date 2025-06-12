package email

import (
	"context"
	"encoding/json"
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

func Verify(input string, conf *Conf) (bool, error) {
	email, err := mail.ParseAddress(input)
	if err != nil {
		return false, err
	}
	i := strings.LastIndex(email.Address, "@")
	if i < 0 || i == len(email.Address)-1 {
		return false, ErrInvalidSyntax
	}
	domain := email.Address[i+1:]
	if conf.CheckDisposableDomains {
		mu.RLock()
		_, ok := disposableDomains[domain]
		mu.RUnlock()
		if ok {
			return false, ErrDisposableEmail
		}
	}
	if conf.CheckNS {
		_, err = net.LookupNS(domain)
		if err != nil {
			return false, err
		}
	}
	records, err := net.LookupMX(domain)
	if err != nil {
		return false, err
	}
	if len(records) < 1 {
		return false, ErrNoMXRecords
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
		return false, err
	}
	defer client.Close()
	err = client.Mail(fromEmail)
	if err != nil {
		return false, err
	}
	err = client.Rcpt(input)
	if err != nil {
		return false, err
	}
	return true, nil
}
