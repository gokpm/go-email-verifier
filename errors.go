package email

import "errors"

var (
	ErrInvalidEmail    = errors.New("invalid syntax")
	ErrDisposableEmail = errors.New("disposable domain")
	ErrNoMXRecords     = errors.New("mx record not found")
)
