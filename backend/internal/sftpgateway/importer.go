package sftpgateway

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/google/uuid"
)

var ErrControlUnavailable = errors.New("transfer control unavailable")

// BeginUploadRequest starts a landing-zone upload for one relative path.
type BeginUploadRequest struct {
	Principal   Principal
	SessionPath string
	RelPath     string
}

// CompleteUploadRequest completes a landing-zone upload after the file is closed.
type CompleteUploadRequest struct {
	Principal   Principal
	SessionPath string
	RelPath     string
	Size        int64
	SHA256      string
}

// TransferControlPort connects SFTP uploads to filesystem transfer commands.
type TransferControlPort interface {
	BeginUpload(ctx context.Context, req BeginUploadRequest) error
	CompleteUpload(ctx context.Context, req CompleteUploadRequest) error
}

// UnconfiguredTransferControl fails until the control plane adapter is wired.
type UnconfiguredTransferControl struct{}

func (UnconfiguredTransferControl) BeginUpload(context.Context, BeginUploadRequest) error {
	return fmt.Errorf("%w", ErrControlUnavailable)
}

func (UnconfiguredTransferControl) CompleteUpload(context.Context, CompleteUploadRequest) error {
	return fmt.Errorf("%w", ErrControlUnavailable)
}

// LandingImporter notifies the control port when landing-zone files are closed.
type LandingImporter struct {
	control     TransferControlPort
	principal   Principal
	sessionRoot string
}

func NewLandingImporter(control TransferControlPort, principal Principal, sessionRoot string) *LandingImporter {
	if control == nil {
		control = UnconfiguredTransferControl{}
	}
	return &LandingImporter{
		control:     control,
		principal:   principal,
		sessionRoot: sessionRoot,
	}
}

func (i *LandingImporter) OnFileClosed(ctx context.Context, relPath string, size int64) error {
	if i == nil || i.control == nil {
		return nil
	}
	req := CompleteUploadRequest{
		Principal:   i.principal,
		SessionPath: i.sessionRoot,
		RelPath:     relPath,
		Size:        size,
	}
	return i.control.CompleteUpload(ctx, req)
}

func (i *LandingImporter) OnFileOpen(ctx context.Context, relPath string) error {
	if i == nil || i.control == nil {
		return nil
	}
	req := BeginUploadRequest{
		Principal:   i.principal,
		SessionPath: i.sessionRoot,
		RelPath:     relPath,
	}
	return i.control.BeginUpload(ctx, req)
}

// SessionLandingDir builds an isolated directory for one SFTP connection.
func SessionLandingDir(landingZone string, principal Principal, sessionID uuid.UUID) (string, error) {
	if landingZone == "" {
		return "", fmt.Errorf("landing zone is required")
	}
	if sessionID == uuid.Nil {
		return "", fmt.Errorf("session id is required")
	}
	return filepath.Join(
		landingZone,
		principal.OrganizationID.String(),
		principal.WorkspaceID.String(),
		principal.ActorID.String(),
		sessionID.String(),
	), nil
}
