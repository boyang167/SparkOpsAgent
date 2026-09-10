package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	runtimek8s "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/infrastructure/k8s"
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

func (h *RuntimeConfigHandler) UpdateDeploymentResources(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	input runtimek8s.DeploymentResourcesInput,
) error {
	if err := h.authorize(ctx, wsID, projectID); err != nil {
		return err
	}
	return h.configurator.UpdateDeploymentResources(ctx, wsID, projectID, input)
}

func (h *RuntimeConfigHandler) ScaleDeployment(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	replicas int32,
) error {
	if err := h.authorize(ctx, wsID, projectID); err != nil {
		return err
	}
	return h.configurator.ScaleDeployment(ctx, wsID, projectID, replicas)
}

func (h *RuntimeConfigHandler) ListConfigMaps(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) ([]runtimek8s.ProjectConfigMapVM, error) {
	if err := h.authorize(ctx, wsID, projectID); err != nil {
		return nil, err
	}
	return h.configurator.ListConfigMaps(ctx, wsID, projectID)
}

func (h *RuntimeConfigHandler) CreateConfigMap(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	input runtimek8s.CreateConfigMapInput,
) error {
	if err := h.authorize(ctx, wsID, projectID); err != nil {
		return err
	}
	return h.configurator.CreateConfigMap(ctx, wsID, projectID, input)
}

func (h *RuntimeConfigHandler) UpsertConfigMapEntry(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	name string,
	input runtimek8s.UpsertConfigMapEntryInput,
) error {
	if err := h.authorize(ctx, wsID, projectID); err != nil {
		return err
	}
	return h.configurator.UpsertConfigMapEntry(ctx, wsID, projectID, name, input)
}

func (h *RuntimeConfigHandler) ReplaceConfigMapData(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	name string,
	input runtimek8s.ReplaceConfigMapDataInput,
) error {
	if err := h.authorize(ctx, wsID, projectID); err != nil {
		return err
	}
	return h.configurator.ReplaceConfigMapData(ctx, wsID, projectID, name, input)
}

func (h *RuntimeConfigHandler) DeleteConfigMapEntry(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	name string,
	key string,
) error {
	if err := h.authorize(ctx, wsID, projectID); err != nil {
		return err
	}
	return h.configurator.DeleteConfigMapEntry(ctx, wsID, projectID, name, key)
}

func (h *RuntimeConfigHandler) ListPVCs(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) ([]runtimek8s.ProjectPVCVM, error) {
	if err := h.authorize(ctx, wsID, projectID); err != nil {
		return nil, err
	}
	return h.configurator.ListPVCs(ctx, wsID, projectID)
}

func (h *RuntimeConfigHandler) ListStorageClasses(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) ([]runtimek8s.StorageClassVM, error) {
	if err := h.authorize(ctx, wsID, projectID); err != nil {
		return nil, err
	}
	return h.configurator.ListStorageClasses(ctx, wsID, projectID)
}

func (h *RuntimeConfigHandler) CreatePVC(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	input runtimek8s.CreatePVCInput,
) error {
	if err := h.authorize(ctx, wsID, projectID); err != nil {
		return err
	}
	return h.configurator.CreatePVC(ctx, wsID, projectID, input)
}

func (h *RuntimeConfigHandler) BindPVC(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	name string,
	input runtimek8s.BindPVCInput,
) error {
	if err := h.authorize(ctx, wsID, projectID); err != nil {
		return err
	}
	return h.configurator.BindPVC(ctx, wsID, projectID, name, input)
}

func (h *RuntimeConfigHandler) UnbindPVC(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	name string,
) error {
	if err := h.authorize(ctx, wsID, projectID); err != nil {
		return err
	}
	return h.configurator.UnbindPVC(ctx, wsID, projectID, name)
}

func (h *RuntimeConfigHandler) DeletePVC(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	name string,
) error {
	if err := h.authorize(ctx, wsID, projectID); err != nil {
		return err
	}
	return h.configurator.DeletePVC(ctx, wsID, projectID, name)
}
