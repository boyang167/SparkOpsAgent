package command

import (
	"context"

	runtimek8s "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/infrastructure/k8s"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type RuntimeConfigHandler struct {
	checker      port.ProjectChecker
	configurator *runtimek8s.ProjectRuntimeConfigurator
}

func NewRuntimeConfigHandler(
	checker port.ProjectChecker,
	configurator *runtimek8s.ProjectRuntimeConfigurator,
) *RuntimeConfigHandler {
	return &RuntimeConfigHandler{checker: checker, configurator: configurator}
}

func (h *RuntimeConfigHandler) authorize(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) error {
	inWs, err := h.checker.ProjectInWorkspace(ctx, projectID, wsID)
	if err != nil {
		return err
	}
	if !inWs {
		return domain.ErrProjectAccessDenied
	}
	if h.configurator == nil {
		return domain.ErrProjectAccessDenied
	}
	return nil
}

func (h *RuntimeConfigHandler) UpsertLogCollection(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	input runtimek8s.LogCollectionInput,
) error {
	if err := h.authorize(ctx, wsID, projectID); err != nil {
		return err
	}
	return h.configurator.UpsertLogCollection(ctx, wsID, projectID, input)
}

func (h *RuntimeConfigHandler) UpsertHealthProbe(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	input runtimek8s.HealthProbeInput,
) error {
	if err := h.authorize(ctx, wsID, projectID); err != nil {
		return err
	}
	return h.configurator.UpsertHealthProbe(ctx, wsID, projectID, input)
}

func (h *RuntimeConfigHandler) UpdateServicePorts(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	ports []runtimek8s.ServicePortInput,
) error {
	if err := h.authorize(ctx, wsID, projectID); err != nil {
		return err
	}
	return h.configurator.UpdateServicePorts(ctx, wsID, projectID, ports)
}

func (h *RuntimeConfigHandler) CreateService(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) error {
	if err := h.authorize(ctx, wsID, projectID); err != nil {
		return err
	}
	return h.configurator.CreateService(ctx, wsID, projectID)
}

func (h *RuntimeConfigHandler) CreateIngress(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	input runtimek8s.IngressInput,
) error {
	if err := h.authorize(ctx, wsID, projectID); err != nil {
		return err
	}
	return h.configurator.CreateIngress(ctx, wsID, projectID, input)
}
