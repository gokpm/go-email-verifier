package verifier

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestVerifyAll(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()
	conf := &Config{
		ValidateMX:      true,
		ValidateSMTP:    true,
		ValidateDNS:     true,
		BlockDisposable: true,
	}
	ok, err := Verify(ctx, "mail.gokulpm@gmail.com", conf)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("Verify() returned false, expected true")
	}
}
func TestVerifyBlockDisposable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()
	conf := &Config{
		ValidateMX:      true,
		ValidateSMTP:    true,
		ValidateDNS:     true,
		BlockDisposable: true,
	}
	ok, err := Verify(ctx, "mail.gokulpm@0uxpgdvol9n.gq", conf)
	if err != ErrDisposableEmail {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("Verify() returned true, expected false")
	}
}

// Test valid email addresses
func TestVerifyValidEmails(t *testing.T) {
	tests := []struct {
		name  string
		email string
		conf  *Config
	}{
		{
			name:  "gmail with minimal config",
			email: "test@gmail.com",
			conf: &Config{
				ValidateDNS: true,
			},
		},
		{
			name:  "gmail with MX validation",
			email: "test@gmail.com",
			conf: &Config{
				ValidateDNS: true,
				ValidateMX:  true,
			},
		},
		{
			name:  "outlook with full validation except SMTP",
			email: "test@outlook.com",
			conf: &Config{
				ValidateDNS:     true,
				ValidateMX:      true,
				BlockDisposable: true,
			},
		},
		{
			name:  "yahoo with disposable blocking",
			email: "test@yahoo.com",
			conf: &Config{
				ValidateDNS:     true,
				BlockDisposable: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			ok, err := Verify(ctx, tt.email, tt.conf)
			if err != nil {
				t.Fatalf("Verify() failed: %v", err)
			}
			if !ok {
				t.Fatal("Verify() returned false, expected true")
			}
		})
	}
}

// Test invalid syntax
func TestVerifyInvalidSyntax(t *testing.T) {
	tests := []struct {
		name  string
		email string
	}{
		{"missing @", "invalid-email"},
		{"missing domain", "user@"},
		{"missing user", "@domain.com"},
		{"empty string", ""},
		{"just @", "@"},
		{"double @", "user@@domain.com"},
		{"spaces", "user @domain.com"},
		{"invalid characters", "user<>@domain.com"},
	}

	conf := &Config{ValidateDNS: true}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			ok, err := Verify(ctx, tt.email, conf)
			if err == nil {
				t.Fatal("Expected error for invalid syntax, got nil")
			}
			if ok {
				t.Fatal("Verify() returned true for invalid email, expected false")
			}
		})
	}
}

// Test disposable email detection
func TestVerifyDisposableEmails(t *testing.T) {
	// Common disposable domains (may need updating based on actual list)
	tests := []struct {
		name   string
		email  string
		domain string
	}{
		{"001216.xyz", "test@001216.xyz", "001216.xyz"},
		{"0039.ml", "test@0039.ml", "0039.ml"},
		{"zoparel.com", "test@zoparel.com", "zoparel.com"},
		{"zippymail.in", "test@zippymail.in", "zippymail.in"},
	}

	conf := &Config{
		BlockDisposable: true,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			ok, err := Verify(ctx, tt.email, conf)
			if err != ErrDisposableEmail {
				t.Fatalf("Expected ErrDisposableEmail, got: %v", err)
			}
			if ok {
				t.Fatal("Verify() returned true for disposable email, expected false")
			}
		})
	}
}

// Test disposable blocking disabled
func TestVerifyDisposableAllowed(t *testing.T) {
	conf := &Config{
		ValidateDNS:     true,
		BlockDisposable: false, // Allow disposable domains
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// This should pass even if it's a disposable domain
	ok, err := Verify(ctx, "test@zoparel.com", conf)
	// Note: This might still fail due to DNS issues, but shouldn't fail due to disposable check
	if err == ErrDisposableEmail {
		t.Fatal("Got ErrDisposableEmail when BlockDisposable is false")
	}
	// Don't check ok value since DNS/MX might legitimately fail
	_ = ok
}

// Test DNS validation
func TestVerifyDNSValidation(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid domain", "test@google.com", false},
		{"invalid domain", "test@nonexistentdomain12345.com", true},
		{"malformed domain", "test@invalid..domain", true},
	}

	conf := &Config{
		ValidateDNS: true,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			ok, err := Verify(ctx, tt.email, conf)

			if tt.wantErr {
				if err == nil {
					t.Fatal("Expected DNS error, got nil")
				}
				if ok {
					t.Fatal("Verify() returned true for invalid domain, expected false")
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if !ok {
					t.Fatal("Verify() returned false for valid domain, expected true")
				}
			}
		})
	}
}

// Test MX record validation
func TestVerifyMXValidation(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"gmail has MX", "test@gmail.com", false},
		{"google has MX", "test@google.com", false},
		// Note: It's hard to test domains without MX records reliably
		// as they might change over time
	}

	conf := &Config{
		ValidateMX: true,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			ok, err := Verify(ctx, tt.email, conf)

			if tt.wantErr {
				if err == nil {
					t.Fatal("Expected MX error, got nil")
				}
				if ok {
					t.Fatal("Verify() returned true for domain without MX, expected false")
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if !ok {
					t.Fatal("Verify() returned false for domain with MX, expected true")
				}
			}
		})
	}
}

// Test context timeout
func TestVerifyContextTimeout(t *testing.T) {
	// Very short timeout to trigger context cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Wait a bit to ensure context is expired
	time.Sleep(1 * time.Millisecond)

	conf := &Config{
		ValidateDNS: true,
		ValidateMX:  true,
	}

	ok, err := Verify(ctx, "test@gmail.com", conf)

	if err == nil {
		t.Fatal("Expected context timeout error, got nil")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Expected context.DeadlineExceeded, got: %v", err)
	}

	if ok {
		t.Fatal("Verify() returned true on timeout, expected false")
	}
}

// Test context cancellation
func TestVerifyContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	conf := &Config{
		ValidateDNS: true,
		ValidateMX:  true,
	}

	ok, err := Verify(ctx, "test@gmail.com", conf)

	if err == nil {
		t.Fatal("Expected context cancellation error, got nil")
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Expected context.Canceled, got: %v", err)
	}

	if ok {
		t.Fatal("Verify() returned true on cancellation, expected false")
	}
}

// Test empty configuration (all checks disabled)
func TestVerifyEmptyConfig(t *testing.T) {
	conf := &Config{} // All validation disabled

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ok, err := Verify(ctx, "test@gmail.com", conf)
	if err != nil {
		t.Fatalf("Unexpected error with empty config: %v", err)
	}
	if !ok {
		t.Fatal("Verify() returned false with empty config, expected true")
	}
}

// Test various email formats that should be valid
func TestVerifyValidEmailFormats(t *testing.T) {
	tests := []struct {
		name  string
		email string
	}{
		{"simple", "user@domain.com"},
		{"with dots", "user.name@domain.com"},
		{"with plus", "user+tag@domain.com"},
		{"with numbers", "user123@domain123.com"},
		{"with hyphens", "user-name@domain-name.com"},
		{"subdomain", "user@mail.domain.com"},
		{"long TLD", "user@domain.museum"},
	}

	conf := &Config{
		ValidateDNS: true, // Only basic DNS check
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			ok, err := Verify(ctx, tt.email, conf)
			// Note: Some of these might fail DNS lookup, but shouldn't fail parsing
			if err != nil {
				// If it's a DNS error, that's expected for test domains
				if dnsErr, ok := err.(*net.DNSError); ok {
					t.Logf("DNS error (expected for test domain): %v", dnsErr)
					return
				}
				// Any other error is unexpected
				t.Fatalf("Unexpected error: %v", err)
			}
			_ = ok // Don't check result since DNS might fail
		})
	}
}

// Test configuration combinations
func TestVerifyConfigCombinations(t *testing.T) {
	tests := []struct {
		name string
		conf *Config
	}{
		{
			name: "DNS only",
			conf: &Config{ValidateDNS: true},
		},
		{
			name: "MX only",
			conf: &Config{ValidateMX: true},
		},
		{
			name: "Disposable only",
			conf: &Config{BlockDisposable: true},
		},
		{
			name: "DNS + MX",
			conf: &Config{ValidateDNS: true, ValidateMX: true},
		},
		{
			name: "DNS + Disposable",
			conf: &Config{ValidateDNS: true, BlockDisposable: true},
		},
		{
			name: "MX + Disposable",
			conf: &Config{ValidateMX: true, BlockDisposable: true},
		},
		{
			name: "All except SMTP",
			conf: &Config{ValidateDNS: true, ValidateMX: true, BlockDisposable: true},
		},
	}

	email := "test@gmail.com"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			ok, err := Verify(ctx, email, tt.conf)
			if err != nil {
				t.Fatalf("Verify() failed with config %+v: %v", tt.conf, err)
			}
			if !ok {
				t.Fatalf("Verify() returned false with config %+v, expected true", tt.conf)
			}
		})
	}
}

// Benchmark tests
func BenchmarkVerifyDNSOnly(b *testing.B) {
	conf := &Config{ValidateDNS: true}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Verify(ctx, "test@gmail.com", conf)
	}
}

func BenchmarkVerifyFullValidation(b *testing.B) {
	conf := &Config{
		ValidateDNS:     true,
		ValidateMX:      true,
		BlockDisposable: true,
		// Skip SMTP for benchmark as it's slow
	}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Verify(ctx, "test@gmail.com", conf)
	}
}

// Test error types
func TestErrorTypes(t *testing.T) {
	t.Run("ErrInvalidSyntax", func(t *testing.T) {
		if ErrInvalidSyntax == nil {
			t.Fatal("ErrInvalidSyntax should not be nil")
		}
		if ErrInvalidSyntax.Error() != "invalid syntax" {
			t.Fatalf("Expected 'invalid syntax', got '%s'", ErrInvalidSyntax.Error())
		}
	})

	t.Run("ErrDisposableEmail", func(t *testing.T) {
		if ErrDisposableEmail == nil {
			t.Fatal("ErrDisposableEmail should not be nil")
		}
		if ErrDisposableEmail.Error() != "disposable domain" {
			t.Fatalf("Expected 'disposable domain', got '%s'", ErrDisposableEmail.Error())
		}
	})

	t.Run("ErrNoMXRecords", func(t *testing.T) {
		if ErrNoMXRecords == nil {
			t.Fatal("ErrNoMXRecords should not be nil")
		}
		if ErrNoMXRecords.Error() != "mx record not found" {
			t.Fatalf("Expected 'mx record not found', got '%s'", ErrNoMXRecords.Error())
		}
	})
}
