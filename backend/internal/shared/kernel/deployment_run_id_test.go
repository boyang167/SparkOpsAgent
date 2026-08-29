package kernel

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeploymentRunIDParseAndStringRoundTrip(t *testing.T) {
	id, err := ParseDeploymentRunID("550e8400-e29b-41d4-a716-446655440000")
	require.NoError(t, err)
	require.Equal(t, "550e8400-e29b-41d4-a716-446655440000", id.String())
}
