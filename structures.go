package email

import (
	"context"
	"sync"
	"time"
)

const (
	disposableDomainsURL = "https://raw.githubusercontent.com/disposable/disposable-email-domains/master/domains.json"
	fromEmail            = "user@example.com"
	smtpPort             = 25
)

type ConfVerifier struct {
	CheckDisposableDomains bool
}

type verifier struct {
	conf              *ConfVerifier
	mu                *sync.RWMutex
	tk                *time.Ticker
	ctx               context.Context
	cancel            context.CancelFunc
	disposableDomains map[string]struct{}
}
