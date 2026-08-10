package kernel_test

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestDomainError(t *testing.T) {
	err := kernel.NewDomainError("IDENTITY_INVALID_CREDENTIALS", "invalid credentials")
	require.Equal(t, "IDENTITY_INVALID_CREDENTIALS", err.Code())
	require.Contains(t, err.Error(), "invalid credentials")
}
