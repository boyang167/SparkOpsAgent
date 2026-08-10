package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/infrastructure/k8s"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type K8sResources struct {
	checker port.ProjectChecker
	svc     *k8s.ResourceService
}

func NewK8sResources(checker port.ProjectChecker, svc *k8s.ResourceService) *K8sResources {
	return &K8sResources{checker: checker, svc: svc}
}

func (h *K8sResources) projectAccess(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) (string, error) {
	inWs, err := h.checker.ProjectInWorkspace(ctx, projectID, wsID)
	if err != nil {
		return "", err
	}
	if !inWs {
		return "", domain.ErrProjectAccessDenied
	}
	return h.checker.ProjectName(ctx, projectID)
}

func (h *K8sResources) ListDeployments(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) ([]map[string]any, error) {
	projectName, err := h.projectAccess(ctx, wsID, projectID)
	if err != nil {
		return nil, err
	}
	return h.svc.ListDeployments(ctx, projectID, projectName)
}

func (h *K8sResources) GetDeployment(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID, name string) (map[string]any, error) {
	projectName, err := h.projectAccess(ctx, wsID, projectID)
	if err != nil {
		return nil, err
	}
	return h.svc.GetDeployment(ctx, projectID, projectName, name)
}

func (h *K8sResources) ScaleDeployment(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID, name string, replicas int32) error {
	projectName, err := h.projectAccess(ctx, wsID, projectID)
	if err != nil {
		return err
	}
	return h.svc.ScaleDeployment(ctx, projectID, projectName, name, replicas)
}

func (h *K8sResources) ListIngresses(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) ([]map[string]any, error) {
	projectName, err := h.projectAccess(ctx, wsID, projectID)
	if err != nil {
		return nil, err
	}
	return h.svc.ListIngresses(ctx, projectID, projectName)
}

func (h *K8sResources) GetIngress(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID, name string) (map[string]any, error) {
	projectName, err := h.projectAccess(ctx, wsID, projectID)
	if err != nil {
		return nil, err
	}
	return h.svc.GetIngress(ctx, projectID, projectName, name)
}

func (h *K8sResources) ListConfigMaps(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) ([]map[string]any, error) {
	projectName, err := h.projectAccess(ctx, wsID, projectID)
	if err != nil {
		return nil, err
	}
	return h.svc.ListConfigMaps(ctx, projectID, projectName)
}

func (h *K8sResources) GetConfigMap(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID, name string) (map[string]any, error) {
	projectName, err := h.projectAccess(ctx, wsID, projectID)
	if err != nil {
		return nil, err
	}
	return h.svc.GetConfigMap(ctx, projectID, projectName, name)
}

func (h *K8sResources) ListServices(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) ([]map[string]any, error) {
	projectName, err := h.projectAccess(ctx, wsID, projectID)
	if err != nil {
		return nil, err
	}
	return h.svc.ListServices(ctx, projectID, projectName)
}

func (h *K8sResources) GetService(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID, name string) (map[string]any, error) {
	projectName, err := h.projectAccess(ctx, wsID, projectID)
	if err != nil {
		return nil, err
	}
	return h.svc.GetService(ctx, projectID, projectName, name)
}

func (h *K8sResources) ListPVCs(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) ([]map[string]any, error) {
	projectName, err := h.projectAccess(ctx, wsID, projectID)
	if err != nil {
		return nil, err
	}
	return h.svc.ListPVCs(ctx, projectID, projectName)
}

func (h *K8sResources) GetPVC(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID, name string) (map[string]any, error) {
	projectName, err := h.projectAccess(ctx, wsID, projectID)
	if err != nil {
		return nil, err
	}
	return h.svc.GetPVC(ctx, projectID, projectName, name)
}

func (h *K8sResources) ListStorageClasses(ctx context.Context) ([]map[string]any, error) {
	return h.svc.ListStorageClasses(ctx)
}

func (h *K8sResources) GetStorageClass(ctx context.Context, name string) (map[string]any, error) {
	return h.svc.GetStorageClass(ctx, name)
}

func (h *K8sResources) ListVirtualServices(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) ([]map[string]any, error) {
	projectName, err := h.projectAccess(ctx, wsID, projectID)
	if err != nil {
		return nil, err
	}
	return h.svc.ListVirtualServices(ctx, projectID, projectName)
}

func (h *K8sResources) GetVirtualService(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID, name string) (map[string]any, error) {
	projectName, err := h.projectAccess(ctx, wsID, projectID)
	if err != nil {
		return nil, err
	}
	return h.svc.GetVirtualService(ctx, projectID, projectName, name)
}
