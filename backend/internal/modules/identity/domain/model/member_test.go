package model_test

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestMember_Authenticate_Success(t *testing.T) {
	hasher := model.BcryptHasher{}
	hash, err := hasher.Hash("secret")
	require.NoError(t, err)

	m := model.NewMember("a@b.com", "Alice", hash)
	require.NoError(t, m.Authenticate("secret", hasher))
}

func TestMember_Authenticate_WrongPassword(t *testing.T) {
	hasher := model.BcryptHasher{}
	hash, err := hasher.Hash("secret")
	require.NoError(t, err)

	m := model.NewMember("a@b.com", "Alice", hash)
	err = m.Authenticate("wrong", hasher)
	require.Error(t, err)
	de, ok := kernel.IsDomainError(err)
	require.True(t, ok)
	require.Equal(t, domain.ErrInvalidCredentials.Code(), de.Code())
}

func TestMember_Authenticate_Disabled(t *testing.T) {
	hasher := model.BcryptHasher{}
	hash, err := hasher.Hash("secret")
	require.NoError(t, err)

	m := model.NewMember("a@b.com", "Alice", hash)
	m.Disable()
	err = m.Authenticate("secret", hasher)
	require.Error(t, err)
	de, ok := kernel.IsDomainError(err)
	require.True(t, ok)
	require.Equal(t, domain.ErrMemberDisabled.Code(), de.Code())
}
