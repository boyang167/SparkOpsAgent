package query

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/repository"
	runtimek8s "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/infrastructure/k8s"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
	"k8s.io/client-go/kubernetes"
)

type CreateEnvironmentSyncPreviewInput struct {
	WorkflowID          kernel.ProductWorkflowID
	JobName             string
	SourceEnvironmentID kernel.EnvironmentID
	TargetEnvironmentID kernel.EnvironmentID
	Projects            []model.EnvironmentSyncProjectOption
}

type EnvironmentSyncPreviewProjectView struct {
	ProjectID   string                      `json:"project_id"`
	FirstSync   bool                        `json:"first_sync"`
	SyncImage   bool                        `json:"sync_image"`
	SyncYAML    bool                        `json:"sync_yaml"`
	SourceImage string                      `json:"source_image,omitempty"`
	TargetImage string                      `json:"target_image,omitempty"`
	Diffs       []model.EnvironmentSyncDiff `json:"diffs"`
}

type EnvironmentSyncPreviewView struct {
	PreviewID           string                            `json:"preview_id"`
	WorkflowID          string                            `json:"workflow_id,omitempty"`
	JobName             string                            `json:"job_name,omitempty"`
	SourceEnvironmentID string                            `json:"source_environment_id"`
	TargetEnvironmentID string                            `json:"target_environment_id"`
	Checksum            string                            `json:"checksum,omitempty"`
	Projects            []EnvironmentSyncPreviewProjectView `json:"projects"`
	Warnings            []string                          `json:"warnings,omitempty"`
}

type CreateEnvironmentSyncPreviewHandler struct {
	reader       deployport.ProjectDeploymentReader
	environments deployport.EnvironmentResolver
	snapshots    repository.EnvironmentSyncSnapshotRepository
	db           *gorm.DB
	clock        func() time.Time
	capture      func(context.Context, deployport.ProjectDeploymentContext) ([]model.EnvironmentSyncResource, error)
}

func NewCreateEnvironmentSyncPreviewHandler(
	reader deployport.ProjectDeploymentReader,
	environments deployport.EnvironmentResolver,
	snapshots repository.EnvironmentSyncSnapshotRepository,
	db *gorm.DB,
) *CreateEnvironmentSyncPreviewHandler {
	return &CreateEnvironmentSyncPreviewHandler{
		reader:       reader,
		environments: environments,
		snapshots:    snapshots,
		db:           db,
		clock:        func() time.Time { return time.Now().UTC() },
	}
}

func (h *CreateEnvironmentSyncPreviewHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	productID kernel.ProductID,
	actor kernel.MemberID,
	input CreateEnvironmentSyncPreviewInput,
) (*model.EnvironmentSyncSnapshot, error) {
	if input.SourceEnvironmentID == input.TargetEnvironmentID {
		return nil, domain.ErrEnvironmentSyncSameEnvironment
	}
	if err := h.validateEnvironmentsBelongToProduct(ctx, productID, input.SourceEnvironmentID, input.TargetEnvironmentID); err != nil {
		return nil, err
	}
	if len(input.Projects) == 0 {
		return nil, domain.ErrDeploymentSyncManifestsEmpty
	}

	projectSnapshots := make([]model.EnvironmentSyncProjectSnapshot, 0, len(input.Projects))
	for _, option := range input.Projects {
		if option.ProjectID == (kernel.ProjectID{}) {
			continue
		}
		sourceTarget, err := h.environments.Resolve(ctx, wsID, option.ProjectID, input.SourceEnvironmentID)
		if err != nil {
			return nil, err
		}
		targetTarget, err := h.environments.Resolve(ctx, wsID, option.ProjectID, input.TargetEnvironmentID)
		if err != nil {
			return nil, err
		}
		if sourceTarget.ProductID != productID || targetTarget.ProductID != productID {
			return nil, domain.ErrDeploymentEnvironmentProductMismatch
		}
		firstSync, err := h.isFirstSync(ctx, input.TargetEnvironmentID, option.ProjectID)
		if err != nil {
			return nil, err
		}
		option.FirstSync = firstSync
		model.NormalizeEnvironmentSyncOption(&option)

		sourceProject, err := h.reader.Load(ctx, wsID, option.ProjectID, sourceTarget.ClusterID)
		if err != nil {
			return nil, err
		}
		targetProject, err := h.reader.Load(ctx, wsID, option.ProjectID, targetTarget.ClusterID)
		if err != nil {
			return nil, err
		}
		sourceProject.Namespace = strings.TrimSpace(sourceTarget.Namespace)
		targetProject.Namespace = strings.TrimSpace(targetTarget.Namespace)

		sourceResources, err := h.loadEnvironmentResources(ctx, sourceProject)
		if err != nil {
			return nil, fmt.Errorf("capture source resources for %s: %w", option.ProjectID, err)
		}
		sourceResources = model.ApplyResourceOverrides(sourceResources, option.ResourceOverrides)
		targetResources, err := h.loadEnvironmentResources(ctx, targetProject)
		if err != nil {
			return nil, fmt.Errorf("capture target resources for %s: %w", option.ProjectID, err)
		}

		plan, err := service.PlanEnvironmentSync(
			sourceResources,
			targetResources,
			option,
			targetProject.Namespace,
		)
		if err != nil {
			return nil, err
		}
		projectSnapshots = append(projectSnapshots, model.EnvironmentSyncProjectSnapshot{
			ProjectID:       option.ProjectID,
			Option:          option,
			SourceResources: sourceResources,
			TargetResources: targetResources,
			Plan:            plan,
		})
	}
	if len(projectSnapshots) == 0 {
		return nil, domain.ErrDeploymentSyncManifestsEmpty
	}

	checksum, err := environmentSyncChecksum(projectSnapshots)
	if err != nil {
		return nil, err
	}
	snapshot := &model.EnvironmentSyncSnapshot{
		ID:                  kernel.NewEnvironmentSyncSnapshotID(),
		WorkspaceID:         wsID,
		ProductID:           productID,
		WorkflowID:          input.WorkflowID,
		JobName:             strings.TrimSpace(input.JobName),
		SourceEnvironmentID: input.SourceEnvironmentID,
		TargetEnvironmentID: input.TargetEnvironmentID,
		Projects:            projectSnapshots,
		Checksum:            checksum,
		CreatedBy:           actor,
		CreatedAt:           h.clock(),
	}
	if err := h.snapshots.Create(ctx, snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func ToEnvironmentSyncPreviewView(snapshot *model.EnvironmentSyncSnapshot) EnvironmentSyncPreviewView {
	if snapshot == nil {
		return EnvironmentSyncPreviewView{}
	}
	projects := make([]EnvironmentSyncPreviewProjectView, 0, len(snapshot.Projects))
	var warnings []string
	for _, project := range snapshot.Projects {
		view := EnvironmentSyncPreviewProjectView{
			ProjectID:   project.ProjectID.String(),
			FirstSync:   project.Option.FirstSync,
			SyncImage:   project.Option.SyncImage,
			SyncYAML:    project.Option.SyncYAML,
			Diffs:       project.Plan.Diffs,
			TargetImage: extractTargetImageBefore(project),
			SourceImage: joinImageRef(project.Plan.ImageRepository, project.Plan.ImageTag),
		}
		for _, diff := range project.Plan.Diffs {
			if diff.Note != "" {
				warnings = append(warnings, diff.Note)
			}
		}
		projects = append(projects, view)
	}
	return EnvironmentSyncPreviewView{
		PreviewID:           snapshot.ID.String(),
		WorkflowID:          snapshot.WorkflowID.String(),
		JobName:             snapshot.JobName,
		SourceEnvironmentID: snapshot.SourceEnvironmentID.String(),
		TargetEnvironmentID: snapshot.TargetEnvironmentID.String(),
		Checksum:            snapshot.Checksum,
		Projects:            projects,
		Warnings:            warnings,
	}
}

func (h *CreateEnvironmentSyncPreviewHandler) validateEnvironmentsBelongToProduct(
	ctx context.Context,
	productID kernel.ProductID,
	sourceID, targetID kernel.EnvironmentID,
) error {
	if h.db == nil {
		return nil
	}
	var count int64
	err := h.db.WithContext(ctx).Table("product.environments").
		Where("id IN ? AND product_id = ? AND deleted_at IS NULL", []string{sourceID.String(), targetID.String()}, productID.String()).
		Count(&count).Error
	if err != nil {
		return err
	}
	if count != 2 {
		return domain.ErrDeploymentEnvironmentProductMismatch
	}
	return nil
}

func (h *CreateEnvironmentSyncPreviewHandler) isFirstSync(
	ctx context.Context,
	environmentID kernel.EnvironmentID,
	projectID kernel.ProjectID,
) (bool, error) {
	if h.db == nil {
		return false, nil
	}
	var count int64
	err := h.db.WithContext(ctx).Table("product.environment_projects").
		Where("environment_id = ? AND project_id = ?", environmentID.String(), projectID.String()).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

func (h *CreateEnvironmentSyncPreviewHandler) loadEnvironmentResources(
	ctx context.Context,
	project deployport.ProjectDeploymentContext,
) ([]model.EnvironmentSyncResource, error) {
	if h.capture != nil {
		return h.capture(ctx, project)
	}
	client, err := runtimek8s.NewTypedClientFromKubeconfig(
		[]byte(project.Kubernetes.Credentials.ExtraConfig),
		project.Cluster.Server,
	)
	if err != nil {
		return nil, fmt.Errorf("open cluster: %w", err)
	}
	return captureEnvironmentSyncResources(ctx, client, project)
}

func captureEnvironmentSyncResources(
	ctx context.Context,
	client kubernetes.Interface,
	project deployport.ProjectDeploymentContext,
) ([]model.EnvironmentSyncResource, error) {
	items, err := buildSyncPreviewItems(ctx, client, project, "")
	if err != nil {
		return nil, err
	}
	return previewItemsToResources(items), nil
}

func previewItemsToResources(items []SyncPreviewItemView) []model.EnvironmentSyncResource {
	out := make([]model.EnvironmentSyncResource, 0, len(items))
	for _, item := range items {
		if item.Managed {
			out = append(out, model.EnvironmentSyncResource{
				Kind: item.Kind, Name: item.Name, ManagedPullSecret: true,
			})
			continue
		}
		if strings.TrimSpace(item.YAML) == "" {
			continue
		}
		out = append(out, model.EnvironmentSyncResource{
			Kind: item.Kind, Name: item.Name, YAML: item.YAML,
		})
	}
	return out
}

func environmentSyncChecksum(projects []model.EnvironmentSyncProjectSnapshot) (string, error) {
	raw, err := json.Marshal(projects)
	if err != nil {
		return "", fmt.Errorf("marshal environment sync projects: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func joinImageRef(repo, tag string) string {
	repo = strings.TrimSpace(repo)
	tag = strings.TrimSpace(tag)
	if repo == "" {
		return tag
	}
	if tag == "" {
		return repo
	}
	return repo + ":" + tag
}

func extractTargetImageBefore(project model.EnvironmentSyncProjectSnapshot) string {
	for _, resource := range project.TargetResources {
		if resource.Kind != model.SyncManifestKindDeployment || strings.TrimSpace(resource.YAML) == "" {
			continue
		}
		repo, tag := serviceExtractPrimaryImage([]model.SyncManifest{{
			Kind: resource.Kind, Name: resource.Name, YAML: resource.YAML,
		}})
		return joinImageRef(repo, tag)
	}
	return ""
}

func serviceExtractPrimaryImage(manifests []model.SyncManifest) (string, string) {
	plan := model.EnvironmentSyncPlan{Manifests: manifests}
	_ = plan
	for _, item := range manifests {
		if item.Kind != model.SyncManifestKindDeployment || strings.TrimSpace(item.YAML) == "" {
			continue
		}
		repo, tag := extractImageFromDeploymentYAML(item.YAML)
		if repo != "" || tag != "" {
			return repo, tag
		}
	}
	return "", ""
}

func extractImageFromDeploymentYAML(raw string) (repo, tag string) {
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return "", ""
	}
	containers, ok := nestedSlice(doc, "spec", "template", "spec", "containers")
	if !ok || len(containers) == 0 {
		return "", ""
	}
	container, ok := containers[0].(map[string]any)
	if !ok {
		return "", ""
	}
	image, _ := container["image"].(string)
	image = strings.TrimSpace(image)
	if image == "" {
		return "", ""
	}
	if idx := strings.LastIndex(image, ":"); idx > 0 && !strings.Contains(image[idx+1:], "/") {
		return image[:idx], image[idx+1:]
	}
	return image, ""
}
