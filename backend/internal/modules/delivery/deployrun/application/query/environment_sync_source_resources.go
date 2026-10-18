package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ListEnvironmentSyncSourceResourcesInput struct {
	SourceEnvironmentID kernel.EnvironmentID
	Projects            []model.EnvironmentSyncProjectOption
}

type EnvironmentSyncSourceResourceView struct {
	Kind              string `json:"kind"`
	Name              string `json:"name"`
	YAML              string `json:"yaml,omitempty"`
	ManagedPullSecret bool   `json:"managed_pull_secret,omitempty"`
	Note              string `json:"note,omitempty"`
}

type EnvironmentSyncSourceProjectResourcesView struct {
	ProjectID string                            `json:"project_id"`
	Resources []EnvironmentSyncSourceResourceView `json:"resources"`
}

type ListEnvironmentSyncSourceResourcesHandler struct {
	reader       deployport.ProjectDeploymentReader
	environments deployport.EnvironmentResolver
	capture      func(context.Context, deployport.ProjectDeploymentContext) ([]model.EnvironmentSyncResource, error)
}

func NewListEnvironmentSyncSourceResourcesHandler(
	reader deployport.ProjectDeploymentReader,
	environments deployport.EnvironmentResolver,
) *ListEnvironmentSyncSourceResourcesHandler {
	return &ListEnvironmentSyncSourceResourcesHandler{
		reader:       reader,
		environments: environments,
	}
}

func (h *ListEnvironmentSyncSourceResourcesHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	productID kernel.ProductID,
	input ListEnvironmentSyncSourceResourcesInput,
) ([]EnvironmentSyncSourceProjectResourcesView, error) {
	if input.SourceEnvironmentID == (kernel.EnvironmentID{}) {
		return nil, domain.ErrDeploymentSyncManifestsEmpty
	}
	if len(input.Projects) == 0 {
		return nil, domain.ErrDeploymentSyncManifestsEmpty
	}
	out := make([]EnvironmentSyncSourceProjectResourcesView, 0, len(input.Projects))
	for _, option := range input.Projects {
		if option.ProjectID == (kernel.ProjectID{}) {
			continue
		}
		model.NormalizeEnvironmentSyncOption(&option)
		if !option.SyncYAML {
			out = append(out, EnvironmentSyncSourceProjectResourcesView{
				ProjectID: option.ProjectID.String(),
				Resources: nil,
			})
			continue
		}
		sourceTarget, err := h.environments.Resolve(ctx, wsID, option.ProjectID, input.SourceEnvironmentID)
		if err != nil {
			return nil, err
		}
		if sourceTarget.ProductID != productID {
			return nil, domain.ErrDeploymentEnvironmentProductMismatch
		}
		sourceProject, err := h.reader.Load(ctx, wsID, option.ProjectID, sourceTarget.ClusterID)
		if err != nil {
			return nil, fmt.Errorf("load source project %s: %w", option.ProjectID, err)
		}
		sourceProject.Namespace = strings.TrimSpace(sourceTarget.Namespace)
		sourceResources, err := h.loadEnvironmentResources(ctx, sourceProject)
		if err != nil {
			return nil, fmt.Errorf("capture source resources for %s: %w", option.ProjectID, err)
		}
		sourceResources = model.ApplyResourceOverrides(sourceResources, option.ResourceOverrides)
		views := make([]EnvironmentSyncSourceResourceView, 0, len(sourceResources))
		for _, resource := range sourceResources {
			syncImage, syncYAML := model.ResourceSyncModes(option, resource)
			if !syncImage && !syncYAML {
				continue
			}
			view := EnvironmentSyncSourceResourceView{
				Kind: resource.Kind, Name: resource.Name, YAML: resource.YAML,
				ManagedPullSecret: resource.ManagedPullSecret,
			}
			if resource.ManagedPullSecret {
				view.Note = "目标环境将使用 Harbor 凭据 ensure，不会复制 Secret 密文"
			}
			views = append(views, view)
		}
		out = append(out, EnvironmentSyncSourceProjectResourcesView{
			ProjectID: option.ProjectID.String(),
			Resources: views,
		})
	}
	if len(out) == 0 {
		return nil, domain.ErrDeploymentSyncManifestsEmpty
	}
	return out, nil
}

func (h *ListEnvironmentSyncSourceResourcesHandler) loadEnvironmentResources(
	ctx context.Context,
	project deployport.ProjectDeploymentContext,
) ([]model.EnvironmentSyncResource, error) {
	if h.capture != nil {
		return h.capture(ctx, project)
	}
	previewHandler := NewCreateEnvironmentSyncPreviewHandler(h.reader, h.environments, nil, nil)
	return previewHandler.loadEnvironmentResources(ctx, project)
}
