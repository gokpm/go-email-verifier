package email

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

func init() {
	err := getDisposableDomainsMap()
	if err != nil {
		log.Fatalln(err)
	}
}

func getDisposableDomainsMap() error {
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, disposableDomainsURL, nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	bytes, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	domains := []string{}
	err = json.Unmarshal(bytes, &domains)
	if err != nil {
		return err
	}
	disposableDomains = map[string]struct{}{}
	for _, domain := range domains {
		disposableDomains[domain] = struct{}{}
	}
	return nil
}

func Verify(input string) (bool, error) {
	email, err := mail.ParseAddress(input)
	if err != nil {
		return false, err
	}
	i := strings.LastIndex(email.Address, "@")
	if i < 0 || i == len(email.Address)-1 {
		return false, ErrInvalidSyntax
	}
	domain := email.Address[i+1:]
	_, ok := disposableDomains[domain]
	if ok {
		return false, ErrDisposableEmail
	}
	_, err = net.LookupNS(domain)
	if err != nil {
		return false, err
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
