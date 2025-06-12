package email

import (
	"sync"
	"time"
)

const (
	disposableDomainsURL = "https://raw.githubusercontent.com/disposable/disposable-email-domains/master/domains.json"
	fromEmail            = "user@example.com"
	smtpPort             = 25
)

var mu *sync.RWMutex
var disposableDomains map[string]struct{}
var tk *time.Ticker
