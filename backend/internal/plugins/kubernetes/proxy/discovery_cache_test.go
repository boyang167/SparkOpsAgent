package proxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseMinorAcceptsLeadingDigits(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"18+", "18.1", "18"} {
		minor, ok := parseMinor(raw)
		require.True(t, ok)
		require.Equal(t, 18, minor)
	}
}
