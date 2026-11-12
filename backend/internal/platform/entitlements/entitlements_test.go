package entitlements

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilesystemFeaturePrecedence(t *testing.T) {
	tests := []struct {
		name      string
		platform  map[string]bool
		workspace map[string]bool
		want      bool
	}{
		{name: "default", want: true},
		{name: "platform disabled", platform: map[string]bool{"filesystem": false}, want: false},
		{name: "platform enabled", platform: map[string]bool{"filesystem": true}, want: true},
		{name: "workspace disables platform enabled", platform: map[string]bool{"filesystem": true}, workspace: map[string]bool{"filesystem": false}, want: false},
		{name: "workspace enables platform disabled", platform: map[string]bool{"filesystem": false}, workspace: map[string]bool{"filesystem": true}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ents := Default()
			applyFeatureMap(&ents, tt.platform)
			applyWorkspaceOverrides(&ents, tt.workspace)
			require.Equal(t, tt.want, ents.Filesystem)
			require.Equal(t, tt.want, ents.AllowsModule("filesystem"))
		})
	}
}

func TestWorkspaceEntitlementsUnmarshalMergesIntoExistingDefaults(t *testing.T) {
	ents := Default()
	ents.Agent = false
	ents.Plugins = []string{"existing"}
	require.NoError(t, json.Unmarshal([]byte(`{"filesystem":false}`), &ents))
	require.False(t, ents.Filesystem)
	require.False(t, ents.Agent)
	require.True(t, ents.Knowledge)
	require.Equal(t, []string{"existing"}, ents.Plugins)

	legacy := Default()
	legacy.Filesystem = false
	require.NoError(t, json.Unmarshal([]byte(`{"agent":false}`), &legacy))
	require.False(t, legacy.Agent)
	require.False(t, legacy.Filesystem)
	require.True(t, legacy.Knowledge)
}
