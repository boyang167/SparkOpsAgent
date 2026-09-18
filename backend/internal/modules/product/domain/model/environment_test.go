package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseEnvironmentType(t *testing.T) {
	dev, ok := ParseEnvironmentType("development")
	require.True(t, ok)
	require.Equal(t, EnvironmentTypeDev, dev)

	staging, ok := ParseEnvironmentType("uat")
	require.True(t, ok)
	require.Equal(t, EnvironmentTypeStaging, staging)

	got, ok := ParseEnvironmentType("")
	require.True(t, ok)
	require.Equal(t, EnvironmentTypeTest, got)

	_, ok = ParseEnvironmentType("lab")
	require.False(t, ok)
}

func TestEnvironmentTypeRequiresApproval(t *testing.T) {
	require.False(t, EnvironmentTypeDev.RequiresApproval())
	require.False(t, EnvironmentTypeTest.RequiresApproval())
	require.True(t, EnvironmentTypeStaging.RequiresApproval())
	require.True(t, EnvironmentTypeProduction.RequiresApproval())
}
