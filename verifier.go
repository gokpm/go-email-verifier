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
	// URL to fetch the latest list of disposable email domains
	disposableDomainsURL = "https://raw.githubusercontent.com/disposable/disposable-email-domains/master/domains.json"
	// Email address used as the sender when testing SMTP connections
	fromEmail = "user@example.com"
	// Standard SMTP port for mail server connections
	smtpPort = 25
)

// Config defines the validation options for email verification
type Config struct {
	ValidateMX      bool // Check if domain has MX (Mail Exchange) records
	ValidateSMTP    bool // Test actual SMTP connection to verify email deliverability
	ValidateDNS     bool // Verify domain has valid DNS records
	BlockDisposable bool // Reject emails from known disposable/temporary email providers
}

// Read-write mutex for thread-safe access to disposableDomains
var mu *sync.RWMutex

// Set of known disposable email domains
var disposableDomains map[string]struct{}

// Ticker for periodic updates of disposable domains list
var tk *time.Ticker

// DNS resolver for domain lookups
var resolver = &net.Resolver{}

// Network dialer for SMTP connections
var dialer = &net.Dialer{}

// Default timeout for network operations
var timeout = 30 * time.Second

var (
	// Email format is invalid
	ErrInvalidSyntax = errors.New("invalid syntax")
	// Email uses a disposable domain
	ErrDisposableEmail = errors.New("disposable domain")
	// Domain has no MX records
	ErrNoMXRecords = errors.New("mx record not found")
)

// init initializes the package by setting up the disposable domains list and starting
// a background goroutine to periodically refresh the list
func init() {
	mu = &sync.RWMutex{}
	// Set up ticker to refresh disposable domains list every hour
	tk = time.NewTicker(1 * time.Hour)
	// Load initial disposable domains list (ignore error on startup)
	disposableDomains, _ = getDisposableDomains()
	// Start background goroutine for periodic updates
	go loop()
}

// getDisposableDomains fetches the latest list of disposable email domains from GitHub
// Returns a map where keys are domain names and values are empty structs (for set behavior)
func getDisposableDomains() (map[string]struct{}, error) {
	// Create context with timeout to prevent hanging requests
	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()
	// Create HTTP request to fetch domains list
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, disposableDomainsURL, nil)
	if err != nil {
		return nil, err
	}
	// Execute the HTTP request
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	// Read the response body
	bytes, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	domains := []string{}
	// Parse JSON response into string slice
	err = json.Unmarshal(bytes, &domains)
	if err != nil {
		return nil, err
	}
	disposableDomains := map[string]struct{}{}
	// Convert slice to map for O(1) lookup performance
	for _, domain := range domains {
		disposableDomains[domain] = struct{}{}
	}
	return disposableDomains, nil
}

// loop runs indefinitely, refreshing the disposable domains list on each ticker interval
func loop() {
	for range tk.C {
		refresh()
	}
}

// refresh updates the global disposable domains list with the latest data
func refresh() error {
	// Fetch new domains list
	domains, err := getDisposableDomains()
	if err != nil {
		return err
	}
	// Thread-safe update of global disposableDomains map
	mu.Lock()
	disposableDomains = domains
	mu.Unlock()
	return nil
}

// Verify performs comprehensive email address validation based on the provided configuration
// Returns true if email is valid according to the specified checks, false otherwise
func Verify(ctx context.Context, input string, conf *Config) (bool, error) {
	// Parse email address to ensure basic RFC compliance
	email, err := mail.ParseAddress(input)
	if err != nil {
		return false, err
	}
	// Extract domain from email address
	i := strings.LastIndex(email.Address, "@")
	if i < 0 || i == len(email.Address)-1 {
		return false, ErrInvalidSyntax
	}
	domain := email.Address[i+1:]
	// Check if domain is in disposable domains list (if enabled)
	if conf.BlockDisposable {
		mu.RLock()
		_, ok := disposableDomains[domain]
		mu.RUnlock()
		if ok {
			return false, ErrDisposableEmail
		}
	}
	// Validate DNS records for the domain (if enabled)
	if conf.ValidateDNS {
		_, err = resolver.LookupNS(ctx, domain)
		if err != nil {
			return false, err
		}
	}
	// Validate MX records and optionally test SMTP connection
	if conf.ValidateMX || conf.ValidateSMTP {
		records, err := resolver.LookupMX(ctx, domain)
		if err != nil {
			return false, err
		}
		if len(records) < 1 {
			return false, ErrNoMXRecords
		}
		// If SMTP validation is enabled, test actual mail server connection
		if conf.ValidateSMTP {
			// Find MX record with lowest preference (highest priority)
			host := records[0].Host
			pref := records[0].Pref
			for _, record := range records {
				if record.Pref >= pref {
					continue
				}
				pref = record.Pref
				host = record.Host
			}
			// Establish TCP connection to mail server
			addr := fmt.Sprintf("%[1]s:%[2]d", host, smtpPort)
			conn, err := dialer.DialContext(ctx, "tcp", addr)
			if err != nil {
				return false, err
			}
			defer conn.Close()
			// Set connection deadline based on context or default timeout
			deadline, ok := ctx.Deadline()
			if !ok {
				deadline = time.Now().Add(timeout)
			}
			err = conn.SetDeadline(deadline)
			if err != nil {
				return false, err
			}
			// Create SMTP client and perform mail transaction test
			client, err := smtp.NewClient(conn, host)
			if err != nil {
				return false, err
			}
			defer func() {
				// Clean up SMTP connection
				err := client.Quit()
				if err != nil {
					client.Close()
				}
			}()
			// Test MAIL FROM command
			err = client.Mail(fromEmail)
			if err != nil {
				return false, err
			}
			// Test RCPT TO command with the target email address
			err = client.Rcpt(input)
			if err != nil {
				return false, err
			}
		}
	}
	// All validations passed
	return true, nil
}
