package command

import (
	"context"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/manifest"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/entitlements"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type BindingRepository interface {
	FindPlugin(ctx context.Context, id string) (*model.PluginRecord, error)
	SaveBinding(ctx context.Context, binding *model.WorkspaceBinding) error
}

type WorkspaceBinder interface {
	Bind(ctx context.Context, wsID kernel.WorkspaceID, m *manifest.Manifest) error
	Unbind(ctx context.Context, wsID kernel.WorkspaceID, pluginID string) error
}

type WorkspaceTransaction interface {
	WithinTransaction(
		ctx context.Context,
		fn func(repo BindingRepository, binder WorkspaceBinder) error,
	) error
}

type EnableHandler struct {
	tx WorkspaceTransaction
}

func NewEnableHandler(tx WorkspaceTransaction) *EnableHandler {
	return &EnableHandler{tx: tx}
}

func (h *EnableHandler) Handle(
	ctx context.Context,
	workspaceID kernel.WorkspaceID,
	pluginID string,
	actor kernel.MemberID,
) (*model.WorkspaceBinding, error) {
	var binding *model.WorkspaceBinding
	err := h.tx.WithinTransaction(ctx, func(repo BindingRepository, binder WorkspaceBinder) error {
		record, err := repo.FindPlugin(ctx, pluginID)
		if err != nil {
			return err
		}
		if !record.CanEnable() {
			return domain.ErrPluginNotDeployed
		}
		spec, err := manifest.Parse([]byte(record.ManifestYAML))
		if err != nil {
			return err
		}
		binding = &model.WorkspaceBinding{
			WorkspaceID: workspaceID,
			PluginID:    pluginID,
			Enabled:     true,
			EnabledBy:   actor,
			EnabledAt:   time.Now().UTC(),
		}
		if err := repo.SaveBinding(ctx, binding); err != nil {
			return err
		}
		return binder.Bind(ctx, workspaceID, spec)
	})
	if err != nil {
		return nil, err
	}
	entitlements.InvalidateCache()
	return binding, nil
}

type DisableHandler struct {
	tx WorkspaceTransaction
}

func NewDisableHandler(tx WorkspaceTransaction) *DisableHandler {
	return &DisableHandler{tx: tx}
}

func (h *DisableHandler) Handle(
	ctx context.Context,
	workspaceID kernel.WorkspaceID,
	pluginID string,
	actor kernel.MemberID,
) (*model.WorkspaceBinding, error) {
	var binding *model.WorkspaceBinding
	err := h.tx.WithinTransaction(ctx, func(repo BindingRepository, binder WorkspaceBinder) error {
		if _, err := repo.FindPlugin(ctx, pluginID); err != nil {
			return err
		}
		if err := binder.Unbind(ctx, workspaceID, pluginID); err != nil {
			return err
		}
		binding = &model.WorkspaceBinding{
			WorkspaceID: workspaceID,
			PluginID:    pluginID,
			Enabled:     false,
			EnabledBy:   actor,
			EnabledAt:   time.Now().UTC(),
		}
		return repo.SaveBinding(ctx, binding)
	})
	if err != nil {
		return nil, err
	}
	entitlements.InvalidateCache()
	return binding, nil
}
