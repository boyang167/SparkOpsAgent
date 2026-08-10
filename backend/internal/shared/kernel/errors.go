package kernel

type DomainError struct {
	code    string
	message string
}

func NewDomainError(code, message string) *DomainError {
	return &DomainError{code: code, message: message}
}

func (e *DomainError) Code() string  { return e.code }
func (e *DomainError) Error() string { return e.message }

func IsDomainError(err error) (*DomainError, bool) {
	if err == nil {
		return nil, false
	}
	if de, ok := err.(*DomainError); ok {
		return de, true
	}
	return nil, false
}
