package command

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

func TestReasonCodeDoesNotExposeUnderlyingError(t *testing.T) {
	secret := errors.New("fixture-content checksum object-key")
	require.Equal(t, "operation_failed", reasonCode(secret))
	require.NotContains(t, reasonCode(secret), "fixture-content")
	require.NotContains(t, reasonCode(secret), "object-key")
}

func TestRootDirectoryHasNilParent(t *testing.T) {
	root, err := model.NewRootDirectory(uuid.New(), uuid.New(), ".root")
	require.NoError(t, err)
	require.Nil(t, root.ParentID)
	require.NotEqual(t, uuid.Nil, root.ID)
}

func TestDirectoryCycleErrorIsStable(t *testing.T) {
	require.ErrorIs(t, domain.ErrDirectoryCycle, domain.ErrDirectoryCycle)
	require.Equal(t, "filesystem directory cycle", domain.ErrDirectoryCycle.Error())
}
