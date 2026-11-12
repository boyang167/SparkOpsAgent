package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeEnvironmentSyncOptionAllowsExplicitSecretSelection(t *testing.T) {
	option := EnvironmentSyncProjectOption{
		SyncYAML:          true,
		SyncResourceKinds: []string{SyncManifestKindSecret},
	}

	NormalizeEnvironmentSyncOption(&option)

	require.Equal(t, []string{SyncManifestKindSecret}, option.SyncResourceKinds)
	syncImage, syncYAML := ResourceSyncModes(option, EnvironmentSyncResource{
		Kind: SyncManifestKindSecret,
		YAML: "apiVersion: v1\nkind: Secret\n",
	})
	require.False(t, syncImage)
	require.True(t, syncYAML)
}
