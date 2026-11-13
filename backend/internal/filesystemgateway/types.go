package filesystemgateway

import (
	"errors"
)

const (
	ActionTransferWrite = "transfer:write"
	ActionTransferRead  = "transfer:read"
)

var (
	ErrForbidden           = errors.New("capability forbidden")
	ErrUnauthorized        = errors.New("capability unauthorized")
	ErrInvalidCapability   = errors.New("invalid capability token")
	ErrCapabilityReplay    = errors.New("capability nonce replay")
	ErrCapabilityExpired   = errors.New("capability expired")
	ErrPayloadTooLarge     = errors.New("payload too large")
	ErrGrantNotFound       = errors.New("transfer grant not found")
	ErrDownloadNotAudited  = errors.New("download audit not satisfied")
	ErrControlUnavailable  = errors.New("control plane unavailable")
	ErrInvalidGatewayConfig = errors.New("invalid gateway configuration")
)
