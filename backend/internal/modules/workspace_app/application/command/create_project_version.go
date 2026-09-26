package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type CreateProjectVersionHandler struct {
	projects  repository.ProjectRepository
	versions  repository.ProjectVersionRepository
	generate  *query.GenerateProjectTemplatesHandler
}

func NewCreateProjectVersionHandler(
	projects repository.ProjectRepository,
	versions repository.ProjectVersionRepository,
	generate *query.GenerateProjectTemplatesHandler,
) *CreateProjectVersionHandler {
	return &CreateProjectVersionHandler{projects: projects, versions: versions, generate: generate}
}

func (h *CreateProjectVersionHandler) Handle(ctx context.Context, req appctx.CreateVersionRequest) (kernel.ProjectVersionID, error) {
	versionTag := strings.TrimSpace(req.VersionTag)
	if versionTag == "" {
		return kernel.ProjectVersionID{}, fmt.Errorf("version_tag is required")
	}
	project, err := h.projects.FindByID(ctx, req.ProjectID)
	if err != nil {
		return kernel.ProjectVersionID{}, err
	}
	if project.WorkspaceID != req.WorkspaceID {
		return kernel.ProjectVersionID{}, fmt.Errorf("project workspace mismatch")
	}

	image := strings.TrimSpace(req.ImageRepository)
	if tag := strings.TrimSpace(req.ImageTag); tag != "" {
		if image != "" {
			image = strings.TrimSuffix(image, "/") + ":" + tag
		} else {
			image = tag
		}
	}
	artifacts, err := h.generate.Handle(ctx, req.WorkspaceID, req.CreatedBy, req.ProjectID)
	if err != nil {
		return kernel.ProjectVersionID{}, err
	}
	manifests := make([]model.VersionManifest, 0, 2)
	for _, artifact := range artifacts {
		if artifact.Kind != "Service" && artifact.Kind != "Deployment" {
			continue
		}
		content := artifact.Content
		if artifact.Kind == "Deployment" && image != "" {
			content, err = query.ReplaceDeploymentImage(content, image)
			if err != nil {
				return kernel.ProjectVersionID{}, err
			}
		}
		manifests = append(manifests, model.VersionManifest{
			Kind: artifact.Kind, Content: content, Hash: model.HashManifestContent(content),
		})
	}
	if len(manifests) == 0 {
		return kernel.ProjectVersionID{}, fmt.Errorf("no service/deployment manifests to snapshot")
	}

	images := []model.VersionImage{{
		ServiceName: project.Name.String(),
		Image:       strings.TrimSpace(req.ImageRepository),
		Tag:         strings.TrimSpace(req.ImageTag),
		BuildID:     strings.TrimSpace(req.BuildID),
	}}

	var productID *kernel.ProductID
	if req.ProductID.UUID != uuid.Nil {
		productID = &req.ProductID
	}
	version := model.NewProjectVersion(
		req.WorkspaceID, req.ProjectID, productID, versionTag,
		req.WorkflowRunID, req.WorkflowJobName,
		req.GitRefType, req.GitRefValue,
		manifests, images, req.CreatedBy,
	)
	if err := h.versions.Create(ctx, version); err != nil {
		return kernel.ProjectVersionID{}, err
	}
	return version.ID, nil
}
