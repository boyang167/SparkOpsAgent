package model_test

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestPluginRecordStatusMachine(t *testing.T) {
	tests := []struct {
		name       string
		start      model.PluginStatus
		transition func(*model.PluginRecord) error
		want       model.PluginStatus
	}{
		{"uploaded to pr_open", model.PluginStatusUploaded, func(p *model.PluginRecord) error {
			return p.MarkPROpen("https://example.test/pr/1")
		}, model.PluginStatusPROpen},
		{"uploaded to pr_failed", model.PluginStatusUploaded, func(p *model.PluginRecord) error {
			return p.MarkPRFailed()
		}, model.PluginStatusPRFailed},
		{"pr_open to merged", model.PluginStatusPROpen, func(p *model.PluginRecord) error {
			return p.MarkMerged()
		}, model.PluginStatusMerged},
		{"pr_open to pr_failed", model.PluginStatusPROpen, func(p *model.PluginRecord) error {
			return p.MarkPRFailed()
		}, model.PluginStatusPRFailed},
		{"merged to deployed", model.PluginStatusMerged, func(p *model.PluginRecord) error {
			return p.MarkDeployed()
		}, model.PluginStatusDeployed},
		{"merged to ci_failed", model.PluginStatusMerged, func(p *model.PluginRecord) error {
			return p.MarkCIFailed()
		}, model.PluginStatusCIFailed},
		{"pr_failed to pr_open", model.PluginStatusPRFailed, func(p *model.PluginRecord) error {
			return p.MarkPROpen("https://example.test/pr/2")
		}, model.PluginStatusPROpen},
		{"ci_failed to pr_open", model.PluginStatusCIFailed, func(p *model.PluginRecord) error {
			return p.MarkPROpen("https://example.test/pr/3")
		}, model.PluginStatusPROpen},
		{"deployed to uninstalling", model.PluginStatusDeployed, func(p *model.PluginRecord) error {
			return p.MarkUninstalling("https://example.test/pr/uninstall")
		}, model.PluginStatusUninstalling},
		{"uninstalling to uninstall_merged", model.PluginStatusUninstalling, func(p *model.PluginRecord) error {
			return p.MarkUninstallMerged()
		}, model.PluginStatusUninstallMerged},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := &model.PluginRecord{ID: "example", Status: tt.start}

			require.NoError(t, tt.transition(plugin))
			require.Equal(t, tt.want, plugin.Status)
		})
	}
}

func TestPluginRecordRejectsInvalidTransitions(t *testing.T) {
	tests := []struct {
		name       string
		start      model.PluginStatus
		transition func(*model.PluginRecord) error
	}{
		{"uploaded cannot deploy", model.PluginStatusUploaded, func(p *model.PluginRecord) error {
			return p.MarkDeployed()
		}},
		{"pr_open cannot deploy", model.PluginStatusPROpen, func(p *model.PluginRecord) error {
			return p.MarkDeployed()
		}},
		{"merged cannot reopen pr", model.PluginStatusMerged, func(p *model.PluginRecord) error {
			return p.MarkPROpen("https://example.test/pr/4")
		}},
		{"deployed cannot fail ci", model.PluginStatusDeployed, func(p *model.PluginRecord) error {
			return p.MarkCIFailed()
		}},
		{"uploaded cannot uninstall", model.PluginStatusUploaded, func(p *model.PluginRecord) error {
			return p.MarkUninstalling("https://example.test/pr/uninstall")
		}},
		{"uninstall merged cannot deploy", model.PluginStatusUninstallMerged, func(p *model.PluginRecord) error {
			return p.MarkDeployed()
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := &model.PluginRecord{ID: "example", Status: tt.start}

			err := tt.transition(plugin)

			require.ErrorIs(t, err, model.ErrInvalidPluginStatusTransition)
			require.Equal(t, tt.start, plugin.Status)
		})
	}
}

func TestPluginRecordCanEnableOnlyWhenDeployed(t *testing.T) {
	for _, status := range []model.PluginStatus{
		model.PluginStatusUploaded,
		model.PluginStatusPROpen,
		model.PluginStatusMerged,
		model.PluginStatusPRFailed,
		model.PluginStatusCIFailed,
		model.PluginStatusUninstalling,
		model.PluginStatusUninstallMerged,
	} {
		plugin := &model.PluginRecord{ID: "example", Status: status}
		require.False(t, plugin.CanEnable(), "status %s must not be enableable", status)
	}

	plugin := &model.PluginRecord{ID: "example", Status: model.PluginStatusDeployed}
	require.True(t, plugin.CanEnable())
}

func TestWorkspaceBindingShape(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()

	binding := model.WorkspaceBinding{
		WorkspaceID: wsID,
		PluginID:    "example",
		Enabled:     true,
		EnabledBy:   memberID,
	}

	require.Equal(t, wsID, binding.WorkspaceID)
	require.Equal(t, "example", binding.PluginID)
	require.True(t, binding.Enabled)
	require.Equal(t, memberID, binding.EnabledBy)
}
