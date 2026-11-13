package sftpgateway

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var (
	ErrAuthFailed           = errors.New("sftp authentication failed")
	ErrInvalidPrincipal     = errors.New("invalid sftp principal")
	ErrUnconfiguredIdentity = errors.New("sftp identity validator not configured")
)

// Principal binds an SFTP session to tenancy and actor identity.
type Principal struct {
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	ActorID        uuid.UUID
}

// TokenValidator validates SFTP password credentials (API token).
type TokenValidator interface {
	ValidateSFTPPassword(ctx context.Context, username, password string) (Principal, error)
}

// UnconfiguredTokenValidator fails until a control-plane adapter is wired.
type UnconfiguredTokenValidator struct{}

func (UnconfiguredTokenValidator) ValidateSFTPPassword(context.Context, string, string) (Principal, error) {
	return Principal{}, fmt.Errorf("%w", ErrUnconfiguredIdentity)
}

// StaticTokenValidator maps a fixed token to a principal (tests and local dev).
type StaticTokenValidator struct {
	Token     string
	Principal Principal
}

func (p Principal) bound() bool {
	return p.OrganizationID != uuid.Nil && p.WorkspaceID != uuid.Nil && p.ActorID != uuid.Nil
}

func (v StaticTokenValidator) ValidateSFTPPassword(_ context.Context, _, password string) (Principal, error) {
	if password == "" || password != v.Token {
		return Principal{}, ErrAuthFailed
	}
	if !v.Principal.bound() {
		return Principal{}, ErrInvalidPrincipal
	}
	return v.Principal, nil
}
