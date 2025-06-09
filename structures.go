package email

const (
	disposableDomainsURL = "https://raw.githubusercontent.com/disposable/disposable-email-domains/master/domains.json"
	fromEmail            = "user@example.com"
	smtpPort             = 25
)

type Conf struct {
	DisposableCheck bool
}

type verifier struct {
	disposableDomains map[string]struct{}
}
