package main

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/subscriber"
	"github.com/stretchr/testify/require"
)

func TestFileWorkerUsesStableSubscriberName(t *testing.T) {
	require.Equal(t, "filesystem.transfer.verify-commit", subscriber.TransferVerifyCommitSubscriber)
}
