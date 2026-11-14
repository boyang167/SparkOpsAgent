package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/subscriber"
	"github.com/stretchr/testify/require"
)

func TestFileWorkerUsesStableSubscriberName(t *testing.T) {
	require.Equal(t, "filesystem.transfer.verify-commit", subscriber.TransferVerifyCommitSubscriber)
	require.Equal(t, "filesystem.revision.generate-preview", subscriber.PreviewGenerationSubscriber)
}

func TestRunWorkerStartsMessagingAfterRegisteringSubscribers(t *testing.T) {
	source, err := os.ReadFile("run.go")
	require.NoError(t, err)
	registerAt := bytes.Index(source, []byte("registerFilesystemPreviewWorker"))
	startAt := bytes.Index(source, []byte("factory.Start(ctx)"))
	require.NotEqual(t, -1, registerAt)
	require.NotEqual(t, -1, startAt)
	require.Greater(t, startAt, registerAt)
}

func TestRunWorkerClosesRuntimeCatalog(t *testing.T) {
	source, err := os.ReadFile("run.go")
	require.NoError(t, err)
	require.Contains(t, string(source), "catalog.Close()")
}
