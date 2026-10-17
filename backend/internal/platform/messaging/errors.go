package messaging

import (
	"errors"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

func Permanent(err error) error {
	if err == nil || IsPermanent(err) {
		return err
	}
	return &events.PermanentError{Err: err}
}

func IsPermanent(err error) bool {
	var permanent *events.PermanentError
	return errors.As(err, &permanent)
}
