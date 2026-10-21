package diagnosis

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseJps(t *testing.T) {
	items := parseJps("1234 com.example.Main\n5678 sun.tools.jps.Jps\n")
	require.Len(t, items, 2)
	require.Equal(t, "1234", items[0].Value)
	require.Equal(t, "com.example.Main", items[0].Name)
}

func TestParseProcessID(t *testing.T) {
	pid, ok := parseProcessID(" 42 ")
	require.True(t, ok)
	require.Equal(t, 42, pid)
	_, ok = parseProcessID("")
	require.False(t, ok)
}
