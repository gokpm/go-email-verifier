package email

const (
	disposableDomainsURL = "https://raw.githubusercontent.com/disposable/disposable-email-domains/master/domains.json"
	fromEmail            = "user@example.com"
	smtpPort             = 25
)

var disposableDomains = map[string]struct{}{}
