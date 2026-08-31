package terminal

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseClientMessageRejectsInvalidInput(t *testing.T) {
	_, err := ParseClientMessage([]byte(`{"type":"unknown"}`))
	require.Error(t, err)
	_, err = ParseClientMessage([]byte(`{"type":"read","data":"` + strings.Repeat("a", MaxInputBytes+1) + `"}`))
	require.Error(t, err)
	require.Error(t, ValidateSize(1, 24))
	require.Error(t, ValidateSize(80, 201))
	require.NoError(t, ValidateSize(80, 24))
}

func TestParseClientMessageAcceptsReadAndResize(t *testing.T) {
	read, err := ParseClientMessage([]byte(`{"type":"read","data":"ls\r"}`))
	require.NoError(t, err)
	require.Equal(t, "read", read.Type)
	require.Equal(t, "ls\r", read.Data)

	resize, err := ParseClientMessage([]byte(`{"type":"resize","cols":120,"rows":36}`))
	require.NoError(t, err)
	require.Equal(t, uint16(120), resize.Cols)
	require.Equal(t, uint16(36), resize.Rows)
	require.Equal(t, []string{
		"/bin/sh",
		"-c",
		"clear 2>/dev/null || true; exec bash 2>/dev/null || exec sh",
	}, ShellCommand())
}
