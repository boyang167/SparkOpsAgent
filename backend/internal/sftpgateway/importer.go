package sftpgateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

var ErrControlUnavailable = errors.New("transfer control unavailable")

// BeginUploadRequest starts a landing-zone upload for one relative path.
type BeginUploadRequest struct {
	Principal   Principal
	SessionPath string
	RelPath     string
	Size        int64
	SHA256      string
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

func (i *LandingImporter) OnFileClosed(ctx context.Context, relPath string) error {
	if i == nil || i.control == nil {
		return ErrControlUnavailable
	}
	rel := cleanSFTPPath(relPath)
	if rel == "" || rel == ".." || strings.HasPrefix(rel, "../") {
		return ErrPathEscape
	}
	size, sum, err := i.hashRel(rel)
	if err != nil {
		return err
	}
	req := BeginUploadRequest{
		Principal:   i.principal,
		SessionPath: i.sessionRoot,
		RelPath:     rel,
		Size:        size,
		SHA256:      sum,
	}
	if err := i.control.BeginUpload(ctx, req); err != nil {
		return err
	}
	return i.control.CompleteUpload(ctx, CompleteUploadRequest{
		Principal:   req.Principal,
		SessionPath: req.SessionPath,
		RelPath:     req.RelPath,
		Size:        req.Size,
		SHA256:      req.SHA256,
	})
}

func (i *LandingImporter) hashRel(rel string) (int64, string, error) {
	vfs, err := NewVirtualFS(i.sessionRoot)
	if err != nil {
		return 0, "", err
	}
	file, err := vfs.Open(rel)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()
	digest := sha256.New()
	size, err := io.Copy(digest, file)
	if err != nil {
		return 0, "", err
	}
	return size, hex.EncodeToString(digest.Sum(nil)), nil
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
