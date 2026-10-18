package httperror

import (
	"errors"
	"net/url"
)

type sanitizedNetworkError struct {
	cause     error
	timeout   bool
	temporary bool
}

func (e *sanitizedNetworkError) Error() string {
	if e.timeout {
		return "HTTP request timed out"
	}
	return "HTTP request failed"
}

func (e *sanitizedNetworkError) Unwrap() error { return e.cause }
func (e *sanitizedNetworkError) Timeout() bool { return e.timeout }
func (e *sanitizedNetworkError) Temporary() bool {
	return e.temporary
}

// SanitizeNetwork removes request URLs from client errors while retaining
// timeout/temporary classification and the underlying non-URL cause.
func SanitizeNetwork(err error) error {
	if err == nil {
		return nil
	}
	var requestErr *url.Error
	if errors.As(err, &requestErr) {
		return &sanitizedNetworkError{
			cause:     requestErr.Err,
			timeout:   requestErr.Timeout(),
			temporary: requestErr.Temporary(),
		}
	}
	var networkErr interface {
		Timeout() bool
		Temporary() bool
	}
	if errors.As(err, &networkErr) {
		return &sanitizedNetworkError{
			cause:     err,
			timeout:   networkErr.Timeout(),
			temporary: networkErr.Temporary(),
		}
	}
	return &sanitizedNetworkError{cause: err}
}
