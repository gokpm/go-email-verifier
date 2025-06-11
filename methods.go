package email

import (
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
)

func (v *verifier) Verify(input string) (bool, error) {
	email, err := mail.ParseAddress(input)
	if err != nil {
		return false, err
	}
	i := strings.LastIndex(email.Address, "@")
	if i < 0 || i == len(email.Address)-1 {
		return false, ErrInvalidSyntax
	}
	domain := email.Address[i+1:]
	v.mu.RLock()
	_, ok := v.disposableDomains[domain]
	v.mu.RUnlock()
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

func (v *verifier) loop() {
	for {
		select {
		case <-v.ctx.Done():
			return
		case <-v.tk.C:
			v.refresh()
		}
	}
}

func (v *verifier) refresh() error {
	disposableDomains, err := getDisposableDomains()
	if err != nil {
		return err
	}
	v.mu.Lock()
	v.disposableDomains = disposableDomains
	v.mu.Unlock()
	return nil
}
