**# Email Verifier**

Go package for email address verification with disposable domain detection, DNS validation, and SMTP verification.

**## Installation**
```bash
go get github.com/gokpm/go-email-verifier
```

**## Usage**
```go
package main

import (
    "context"
    "fmt"
    "time"
    "github.com/gokpm/go-email-verifier"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    conf := Config{
        ValidateDNS:      true,
        ValidateMX:       true,
        ValidateSMTP:     true,
        BlockDisposable:  true,
    }
    
    isValid, err := verifier.Verify(ctx, "user@example.com", conf)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    fmt.Printf("Valid: %t\n", isValid)
}
```

**## Configuration**
```go
type Config struct {
    ValidateMX       bool // Check MX records exist
    ValidateSMTP     bool // Test SMTP server connectivity
    ValidateDNS      bool // Verify domain NS records
    BlockDisposable  bool // Check against disposable email domains
}
```

**## Errors**
- `ErrInvalidSyntax`: Invalid email format
- `ErrDisposableEmail`: Disposable domain detected  
- `ErrNoMXRecords`: No MX records found

**## Features**
- Syntax validation
- Disposable domain checking (auto-updated hourly)
- DNS/MX record validation
- SMTP verification
- Context-aware with timeout support