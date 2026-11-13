package main

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/service"
	"github.com/stretchr/testify/require"
)

func TestFileReconcilerUsesStableLeaseName(t *testing.T) {
	require.Equal(t, "filesystem-reconciler", service.ReconcilerLeaseName)
}
