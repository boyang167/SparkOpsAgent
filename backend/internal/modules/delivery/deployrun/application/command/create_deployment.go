package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/consumer"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gopkg.in/yaml.v3"
)

type CreateInput struct {
	ClusterID        string               `json:"cluster_id"`
	EnvironmentID    string               `json:"environment_id"`
	ProjectVersionID string               `json:"project_version_id"`
	GitRefType       string               `json:"git_ref_type"`
	GitRefValue      string               `json:"git_ref_value"`
	ImageTag         string               `json:"image_tag"`
	ImageRepository  string               `json:"image_repository"`
	CrossCluster     bool                 `json:"cross_cluster"`
	SyncOnly         bool                 `json:"sync_only"`
	Manifests        []model.SyncManifest `json:"manifests"`
}

type CreateDeploymentHandler struct {
	repo         repository.DeploymentRunRepository
	reader       port.ProjectDeploymentReader
	environments port.EnvironmentResolver
	queue        messaging.Queue
	clock        func() time.Time
}

func NewCreateDeploymentHandler(
	repo repository.DeploymentRunRepository,
	reader port.ProjectDeploymentReader,
	queue messaging.Queue,
	clock func() time.Time,
	resolvers ...port.EnvironmentResolver,
) *CreateDeploymentHandler {
	if clock == nil {
		clock = time.Now
	}
	handler := &CreateDeploymentHandler{repo: repo, reader: reader, queue: queue, clock: clock}
	if len(resolvers) > 0 {
		handler.environments = resolvers[0]
	}
	return handler
}

func (h *CreateDeploymentHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
	input CreateInput,
) (*model.DeploymentRun, error) {
	var selected kernel.ClusterInfoID
	var deployNamespace string
	var environmentID *kernel.EnvironmentID
	var err error
	if strings.TrimSpace(input.EnvironmentID) != "" {
		if h.environments == nil {
			return nil, domain.ErrDeploymentEnvironmentNotFound
		}
		envID, parseErr := kernel.ParseEnvironmentID(input.EnvironmentID)
		if parseErr != nil {
			return nil, domain.ErrDeploymentEnvironmentNotFound
		}
		target, resolveErr := h.environments.Resolve(ctx, wsID, projectID, envID)
		if resolveErr != nil {
			return nil, resolveErr
		}
		selected = target.ClusterID
		deployNamespace = target.Namespace
		environmentID = &target.EnvironmentID
	}
	needsAuthorizedClusters := input.CrossCluster || input.SyncOnly
	if needsAuthorizedClusters {
		clusters, listErr := h.reader.ListAuthorizedClusters(ctx, wsID)
		if listErr != nil {
			return nil, listErr
		}
		if len(clusters) < 2 {
			return nil, domain.ErrCrossClusterSyncTargetsMissing
		}
		// Force build cluster (priority first); ignore client-selected cluster.
		selected = kernel.ClusterInfoID{}
	} else if environmentID == nil && strings.TrimSpace(input.ClusterID) != "" {
		selected, err = kernel.ParseClusterInfoID(input.ClusterID)
		if err != nil {
			return nil, domain.ErrDeploymentClusterNotFound
		}
	}
	deploymentContext, err := h.reader.Load(ctx, wsID, projectID, selected)
	if err != nil {
		return nil, err
	}
	active, err := h.repo.HasActiveByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if active {
		return nil, domain.ErrDeploymentAlreadyRunning
	}

	gitRefType := input.GitRefType
	gitRefValue := input.GitRefValue
	imageTag := input.ImageTag
	if input.SyncOnly {
		if strings.TrimSpace(imageTag) == "" || strings.TrimSpace(gitRefValue) == "" {
			latest, listErr := h.repo.ListByProject(ctx, projectID, 20)
			if listErr != nil {
				return nil, listErr
			}
			var source *model.DeploymentRun
			for _, item := range latest {
				if item.Status == model.StatusSucceeded && strings.TrimSpace(item.ImageTag) != "" {
					source = item
					break
				}
			}
			if source == nil {
				return nil, domain.ErrDeploymentNotSyncable
			}
			if strings.TrimSpace(gitRefType) == "" {
				gitRefType = source.GitRefType
			}
			if strings.TrimSpace(gitRefValue) == "" {
				gitRefValue = source.GitRefValue
			}
			if strings.TrimSpace(imageTag) == "" {
				imageTag = source.ImageTag
			}
		}
	}

	var run *model.DeploymentRun
	if input.SyncOnly {
		if err := model.ValidateSyncManifests(input.Manifests, deploymentContext.Namespace); err != nil {
			return nil, err
		}
		run = model.NewSyncDeploymentRun(wsID, projectID, deploymentContext.Cluster.ID, memberID,
			gitRefType, gitRefValue, imageTag)
		run.SyncManifests = append([]model.SyncManifest(nil), input.Manifests...)
	} else {
		run = model.NewDeploymentRun(wsID, projectID, deploymentContext.Cluster.ID, memberID,
			gitRefType, gitRefValue, imageTag)
		run.CrossCluster = input.CrossCluster
		if len(input.Manifests) > 0 {
			ns := firstNonEmpty(deployNamespace, deploymentContext.Namespace)
			manifests := rewriteManifestNamespaces(input.Manifests, ns)
			if err := model.ValidateSyncManifests(manifests, ns); err != nil {
				return nil, err
			}
			run.SyncManifests = manifests
		}
	}
	run.EnvironmentID = environmentID
	run.DeployNamespace = deployNamespace
	run.ImageRepository = strings.TrimSpace(input.ImageRepository)
	if strings.TrimSpace(input.ProjectVersionID) != "" {
		versionID, parseErr := kernel.ParseProjectVersionID(input.ProjectVersionID)
		if parseErr != nil {
			return nil, parseErr
		}
		run.ProjectVersionID = &versionID
	}
	run.CreatedAt = h.clock().UTC()
	if err := h.repo.Create(ctx, run); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(consumer.Message{RunID: run.ID.String(), WorkspaceID: wsID.String()})
	if err != nil {
		return nil, fmt.Errorf("marshal deployment message: %w", err)
	}
	if err := h.queue.Publish(ctx, messaging.TopicDeploy, payload); err != nil {
		run.Fail("publish deploy message: "+err.Error(), h.clock())
		if updateErr := h.repo.Update(ctx, run); updateErr != nil {
			return nil, errors.Join(err, fmt.Errorf("mark deployment failed: %w", updateErr))
		}
		return nil, err
	}
	return run, nil
}

type AbortDeploymentHandler struct {
	repo         repository.DeploymentRunRepository
	buildAborter BuildAborter
	runCanceler  RunCanceler
	clock        func() time.Time
}

type BuildAborter interface {
	Handle(context.Context, kernel.WorkspaceID, kernel.BuildID) error
}

type RunCanceler interface {
	Cancel(string) bool
}

type AbortDependencies struct {
	Builds BuildAborter
	Runs   RunCanceler
}

func NewAbortDeploymentHandler(
	repo repository.DeploymentRunRepository,
	clock func() time.Time,
	dependencies ...AbortDependencies,
) *AbortDeploymentHandler {
	if clock == nil {
		clock = time.Now
	}
	handler := &AbortDeploymentHandler{repo: repo, clock: clock}
	if len(dependencies) > 0 {
		handler.buildAborter = dependencies[0].Builds
		handler.runCanceler = dependencies[0].Runs
	}
	return handler
}

func (h *AbortDeploymentHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	runID kernel.DeploymentRunID,
) (*model.DeploymentRun, error) {
	run, err := h.repo.FindByID(ctx, runID)
	if err != nil {
		return nil, err
	}
	if run.WorkspaceID != wsID {
		return nil, domain.ErrDeploymentRunNotFound
	}
	if run.Status == model.StatusAborted || run.Status == model.StatusFailed || run.Status == model.StatusSucceeded {
		return run, nil
	}
	switch run.Status {
	case model.StatusBuilding:
		if run.BuildID == nil {
			return nil, domain.ErrDeploymentBindingInvalid
		}
		if h.buildAborter == nil {
			return nil, errors.New("deployment build aborter is unavailable")
		}
		if err := h.buildAborter.Handle(ctx, wsID, *run.BuildID); err != nil {
			return nil, err
		}
	case model.StatusPreparing, model.StatusDeploying:
		if h.runCanceler != nil {
			h.runCanceler.Cancel(run.ID.String())
		}
	}
	if err := run.Transition(model.StatusAborted, h.clock()); err != nil {
		return nil, err
	}
	run.LeaseOwner = ""
	run.LeaseExpiresAt = nil
	if err := h.repo.Update(ctx, run); err != nil {
		if errors.Is(err, domain.ErrDeploymentRunConcurrentUpdate) {
			latest, findErr := h.repo.FindByID(ctx, runID)
			if findErr != nil {
				return nil, findErr
			}
			if latest.WorkspaceID == wsID &&
				(latest.Status == model.StatusAborted || latest.Status == model.StatusFailed || latest.Status == model.StatusSucceeded) {
				return latest, nil
			}
		}
		return nil, err
	}
	return run, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func rewriteManifestNamespaces(items []model.SyncManifest, namespace string) []model.SyncManifest {
	ns := strings.TrimSpace(namespace)
	out := make([]model.SyncManifest, 0, len(items))
	for _, item := range items {
		next := item
		if ns != "" && strings.TrimSpace(item.YAML) != "" {
			if rewritten, err := setManifestNamespace(item.YAML, ns); err == nil {
				next.YAML = rewritten
			}
		}
		out = append(out, next)
	}
	return out
}

func setManifestNamespace(raw, namespace string) (string, error) {
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return "", err
	}
	metadata, _ := doc["metadata"].(map[string]any)
	if metadata == nil {
		metadata = map[string]any{}
		doc["metadata"] = metadata
	}
	metadata["namespace"] = namespace
	encoded, err := yaml.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
