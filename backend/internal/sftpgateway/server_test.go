package sftpgateway

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
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
