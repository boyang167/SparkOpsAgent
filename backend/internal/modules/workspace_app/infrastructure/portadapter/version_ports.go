package portadapter

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type VersionCreatorPort struct {
	create *command.CreateProjectVersionHandler
}

func NewVersionCreatorPort(create *command.CreateProjectVersionHandler) *VersionCreatorPort {
	return &VersionCreatorPort{create: create}
}

func (p *VersionCreatorPort) CreateFromWorkflow(ctx context.Context, req appctx.CreateVersionRequest) (kernel.ProjectVersionID, error) {
	return p.create.Handle(ctx, req)
}

type ProjectVersionReaderPort struct {
	versions repository.ProjectVersionRepository
}

func NewProjectVersionReaderPort(versions repository.ProjectVersionRepository) *ProjectVersionReaderPort {
	return &ProjectVersionReaderPort{versions: versions}
}

func (p *ProjectVersionReaderPort) LoadSnapshot(ctx context.Context, versionID kernel.ProjectVersionID) (appctx.VersionSnapshot, error) {
	version, err := p.versions.FindByVersionID(ctx, versionID)
	if err != nil {
		return appctx.VersionSnapshot{}, err
	}
	manifests := make([]appctx.VersionManifest, 0, len(version.Manifests))
	for _, item := range version.Manifests {
		manifests = append(manifests, appctx.VersionManifest{Kind: item.Kind, Content: item.Content})
	}
	images := make([]appctx.VersionImage, 0, len(version.Images))
	for _, item := range version.Images {
		images = append(images, appctx.VersionImage{
			ServiceName: item.ServiceName, Image: item.Image, Tag: item.Tag, BuildID: item.BuildID,
		})
	}
	return appctx.VersionSnapshot{
		Manifests: manifests, Images: images,
		GitRefType: version.GitRefType, GitRefValue: version.GitRefValue,
	}, nil
}
