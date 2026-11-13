package sftpgateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/pkg/sftp"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
)

func TestSFTPRejectsEscapeFromVirtualRoot(t *testing.T) {
	root := t.TempDir()
	workspaceRoot := filepath.Join(root, "workspace-a")
	other := filepath.Join(root, "other-workspace", "secret.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(other), 0o755))
	require.NoError(t, os.WriteFile(other, []byte("secret"), 0o600))

	fs := newVirtualFS(t, workspaceRoot)
	_, err := fs.Open("../../other-workspace/secret.txt")
	require.ErrorIs(t, err, ErrPathEscape)
}

func TestVirtualFSAllowsFilesInsideRoot(t *testing.T) {
	root := t.TempDir()
	fs := newVirtualFS(t, root)
	path := filepath.Join(root, "upload.txt")
	require.NoError(t, os.WriteFile(path, []byte("ok"), 0o644))

	f, err := fs.Open("upload.txt")
	require.NoError(t, err)
	require.NoError(t, f.Close())
}

func TestSessionLandingDirIsIsolated(t *testing.T) {
	landing := t.TempDir()
	principal := Principal{
		OrganizationID: uuid.New(),
		WorkspaceID:    uuid.New(),
		ActorID:        uuid.New(),
	}
	sessionID := uuid.New()
	dir, err := SessionLandingDir(landing, principal, sessionID)
	require.NoError(t, err)
	require.Contains(t, dir, principal.WorkspaceID.String())
	require.Contains(t, dir, sessionID.String())
}

func newVirtualFS(t *testing.T, root string) *VirtualFS {
	t.Helper()
	require.NoError(t, os.MkdirAll(root, 0o755))
	fs, err := NewVirtualFS(root)
	require.NoError(t, err)
	return fs
}

func TestCloseCreatesAndCompletesTransferInLandingZone(t *testing.T) {
	landing := t.TempDir()
	production := t.TempDir()
	principal := Principal{
		OrganizationID: uuid.New(),
		WorkspaceID:    uuid.New(),
		ActorID:        uuid.New(),
	}
	sessionRoot, err := SessionLandingDir(landing, principal, uuid.New())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(sessionRoot, 0o755))

	control := &recordingControl{}
	handler := &sessionHandler{
		fs:       newVirtualFS(t, sessionRoot),
		importer: NewLandingImporter(control, principal, sessionRoot),
	}
	writer, err := handler.Filewrite(sftp.NewRequest("Open", "report.txt"))
	require.NoError(t, err)
	require.Empty(t, control.begins)
	require.Empty(t, control.completes)

	payload := []byte("hello")
	_, err = writer.WriteAt(payload, 0)
	require.NoError(t, err)
	require.NoError(t, writer.(io.Closer).Close())

	require.Equal(t, []string{"begin", "complete"}, control.order)
	require.Len(t, control.begins, 1)
	require.Len(t, control.completes, 1)
	require.Equal(t, "report.txt", control.begins[0].RelPath)
	require.Equal(t, sessionRoot, control.begins[0].SessionPath)
	require.Equal(t, principal, control.begins[0].Principal)
	require.Equal(t, int64(len(payload)), control.completes[0].Size)
	sum := sha256.Sum256(payload)
	require.Equal(t, hex.EncodeToString(sum[:]), control.completes[0].SHA256)
	require.Equal(t, control.completes[0].SHA256, control.begins[0].SHA256)

	landingFile := filepath.Join(sessionRoot, "report.txt")
	_, err = os.Stat(landingFile)
	require.NoError(t, err)
	entries, err := os.ReadDir(production)
	require.NoError(t, err)
	require.Empty(t, entries)

	rename := sftp.NewRequest("Rename", "report.txt")
	rename.Target = filepath.Join(production, "object.bin")
	require.ErrorIs(t, handler.Filecmd(rename), ErrRenameDenied)
	_, err = os.Stat(landingFile)
	require.NoError(t, err)
	entries, err = os.ReadDir(production)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestCloseLeavesLandingFileWhenCreateFails(t *testing.T) {
	landing := t.TempDir()
	principal := Principal{
		OrganizationID: uuid.New(),
		WorkspaceID:    uuid.New(),
		ActorID:        uuid.New(),
	}
	sessionRoot, err := SessionLandingDir(landing, principal, uuid.New())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(sessionRoot, 0o755))

	control := &recordingControl{beginErr: errors.New("control down")}
	handler := &sessionHandler{
		fs:       newVirtualFS(t, sessionRoot),
		importer: NewLandingImporter(control, principal, sessionRoot),
	}
	writer, err := handler.Filewrite(sftp.NewRequest("Open", "report.txt"))
	require.NoError(t, err)
	_, err = writer.WriteAt([]byte("hello"), 0)
	require.NoError(t, err)
	require.Error(t, writer.(io.Closer).Close())
	require.Empty(t, control.completes)
	_, err = os.Stat(filepath.Join(sessionRoot, "report.txt"))
	require.NoError(t, err)
}

func TestStaticTokenIgnoresUsernameWorkspaceOverride(t *testing.T) {
	workspaceID := uuid.New()
	validator := StaticTokenValidator{
		Token: "token",
		Principal: Principal{
			OrganizationID: uuid.New(),
			WorkspaceID:    workspaceID,
			ActorID:        uuid.New(),
		},
	}
	got, err := validator.ValidateSFTPPassword(context.Background(), uuid.NewString(), "token")
	require.NoError(t, err)
	require.Equal(t, workspaceID, got.WorkspaceID)
	require.Equal(t, validator.Principal, got)
}

func TestNewFromConfigRejectsLandingZoneOverlappingStorage(t *testing.T) {
	root := t.TempDir()
	storage := filepath.Join(root, "storage")
	landing := filepath.Join(storage, "sftp")
	cfg := &config.Config{}
	cfg.Filesystem.SFTP.Enabled = true
	cfg.Filesystem.SFTP.Listen = ":2022"
	cfg.Filesystem.SFTP.LandingZone = landing
	cfg.Filesystem.TransferGateway.StorageRoot = storage

	_, err := NewFromConfig(cfg)
	require.ErrorIs(t, err, ErrInvalidGatewayConfig)
}

type recordingControl struct {
	begins    []BeginUploadRequest
	completes []CompleteUploadRequest
	order     []string
	beginErr  error
}

func (r *recordingControl) BeginUpload(_ context.Context, req BeginUploadRequest) error {
	r.begins = append(r.begins, req)
	r.order = append(r.order, "begin")
	return r.beginErr
}

func (r *recordingControl) CompleteUpload(_ context.Context, req CompleteUploadRequest) error {
	r.completes = append(r.completes, req)
	r.order = append(r.order, "complete")
	return nil
}
