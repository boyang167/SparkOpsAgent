package command_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/manifest"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

const enableManifestYAML = `id: example
version: "0.1.0"
display_name: Example Plugin
route_prefix: /api/v1/plugins/example
entitlement: plugin:example
menus:
  - title: Example
    path: /plugins/example
    component: plugins/example/page
apis:
  - method: GET
    path: /api/v1/plugins/example/ping
frontend:
  pages:
    - app/(platform)/plugins/example/page.tsx
`

type enableState struct {
	plugin      *model.PluginRecord
	binding     *model.WorkspaceBinding
	bound       bool
	bindCalls   int
	unbindCalls int
	binderErr   error
	saveErr     error
}

func (s *enableState) clone() *enableState {
	copy := *s
	if s.plugin != nil {
		plugin := *s.plugin
		copy.plugin = &plugin
	}
	if s.binding != nil {
		binding := *s.binding
		copy.binding = &binding
	}
	return &copy
}

type transactionalFake struct{ state *enableState }

func (u *transactionalFake) WithinTransaction(
	_ context.Context,
	fn func(command.BindingRepository, command.WorkspaceBinder) error,
) error {
	next := u.state.clone()
	if err := fn(&bindingRepoFake{state: next}, &binderFake{state: next}); err != nil {
		return err
	}
	*u.state = *next
	return nil
}

type bindingRepoFake struct{ state *enableState }

func (r *bindingRepoFake) FindPlugin(context.Context, string) (*model.PluginRecord, error) {
	if r.state.plugin == nil {
		return nil, domain.ErrPluginNotFound
	}
	copy := *r.state.plugin
	return &copy, nil
}

func (r *bindingRepoFake) SaveBinding(_ context.Context, binding *model.WorkspaceBinding) error {
	if r.state.saveErr != nil {
		return r.state.saveErr
	}
	copy := *binding
	r.state.binding = &copy
	return nil
}

type binderFake struct{ state *enableState }

func (b *binderFake) Bind(context.Context, kernel.WorkspaceID, *manifest.Manifest) error {
	b.state.bindCalls++
	if b.state.binderErr != nil {
		return b.state.binderErr
	}
	b.state.bound = true
	return nil
}

func (b *binderFake) Unbind(context.Context, kernel.WorkspaceID, string) error {
	b.state.unbindCalls++
	if b.state.binderErr != nil {
		return b.state.binderErr
	}
	b.state.bound = false
	return nil
}

func deployedPlugin() *model.PluginRecord {
	return &model.PluginRecord{
		ID:           "example",
		Status:       model.PluginStatusDeployed,
		ManifestYAML: enableManifestYAML,
	}
}

func TestEnablePersistsBindingAndBindsGovernance(t *testing.T) {
	state := &enableState{plugin: deployedPlugin()}
	handler := command.NewEnableHandler(&transactionalFake{state: state})
	wsID, actor := kernel.NewWorkspaceID(), kernel.NewMemberID()

	binding, err := handler.Handle(context.Background(), wsID, "example", actor)

	require.NoError(t, err)
	require.True(t, binding.Enabled)
	require.Equal(t, actor, binding.EnabledBy)
	require.False(t, binding.EnabledAt.IsZero())
	require.Equal(t, 1, state.bindCalls)
	require.True(t, state.bound)
}

func TestEnableRejectsPluginThatIsNotDeployed(t *testing.T) {
	state := &enableState{plugin: deployedPlugin()}
	state.plugin.Status = model.PluginStatusMerged
	handler := command.NewEnableHandler(&transactionalFake{state: state})

	_, err := handler.Handle(context.Background(), kernel.NewWorkspaceID(), "example", kernel.NewMemberID())

	require.ErrorIs(t, err, domain.ErrPluginNotDeployed)
	require.Zero(t, state.bindCalls)
	require.Nil(t, state.binding)
}

func TestEnableRollsBackBindingWhenBindFails(t *testing.T) {
	bindErr := errors.New("bind failed")
	state := &enableState{plugin: deployedPlugin(), binderErr: bindErr}
	handler := command.NewEnableHandler(&transactionalFake{state: state})

	_, err := handler.Handle(context.Background(), kernel.NewWorkspaceID(), "example", kernel.NewMemberID())

	require.ErrorIs(t, err, bindErr)
	require.Nil(t, state.binding)
	require.False(t, state.bound)
}

func TestDisableUnbindsAndPersistsDisabledBinding(t *testing.T) {
	wsID, actor := kernel.NewWorkspaceID(), kernel.NewMemberID()
	state := &enableState{
		plugin:  deployedPlugin(),
		bound:   true,
		binding: &model.WorkspaceBinding{WorkspaceID: wsID, PluginID: "example", Enabled: true},
	}
	handler := command.NewDisableHandler(&transactionalFake{state: state})

	binding, err := handler.Handle(context.Background(), wsID, "example", actor)

	require.NoError(t, err)
	require.False(t, binding.Enabled)
	require.Equal(t, actor, binding.EnabledBy)
	require.Equal(t, 1, state.unbindCalls)
	require.False(t, state.bound)
}

func TestDisableRollsBackUnbindWhenSavingBindingFails(t *testing.T) {
	saveErr := errors.New("save failed")
	wsID := kernel.NewWorkspaceID()
	enabledAt := time.Now().UTC()
	state := &enableState{
		plugin: deployedPlugin(),
		bound:  true,
		binding: &model.WorkspaceBinding{
			WorkspaceID: wsID, PluginID: "example", Enabled: true, EnabledAt: enabledAt,
		},
		saveErr: saveErr,
	}
	handler := command.NewDisableHandler(&transactionalFake{state: state})

	_, err := handler.Handle(context.Background(), wsID, "example", kernel.NewMemberID())

	require.ErrorIs(t, err, saveErr)
	require.True(t, state.bound)
	require.True(t, state.binding.Enabled)
	require.Equal(t, enabledAt, state.binding.EnabledAt)
}
