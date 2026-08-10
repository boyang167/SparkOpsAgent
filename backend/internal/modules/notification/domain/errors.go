package domain

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

var (
	ErrNotificationNotFound = kernel.NewDomainError("NOTIFICATION_NOT_FOUND", "notification not found")
	ErrInvalidTitle         = kernel.NewDomainError("NOTIFICATION_INVALID_TITLE", "notification title required")
)
