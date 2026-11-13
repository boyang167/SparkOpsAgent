//go:build windows

package local

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestSameWindowsIdentityUsesVolumeAndFileIndex(t *testing.T) {
	original := windows.ByHandleFileInformation{VolumeSerialNumber: 7, FileIndexHigh: 11, FileIndexLow: 13}
	require.True(t, sameWindowsIdentity(original, original))

	changedVolume := original
	changedVolume.VolumeSerialNumber++
	require.False(t, sameWindowsIdentity(original, changedVolume))

	changedIndex := original
	changedIndex.FileIndexLow++
	require.False(t, sameWindowsIdentity(original, changedIndex))
}
