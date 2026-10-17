package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	deployrepo "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/repository"
	deployacl "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/acl"
	buildmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/model"
	deliveryport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	runtimeport "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

var errAssociationObsolete = errors.New("deployment build association is obsolete")

type DeploymentPreparer interface {
	Run(context.Context, deployport.ProjectDeploymentContext, PrepareInput) error
}

type BuildReader interface {
	FindByID(context.Context, kernel.BuildID) (*buildmodel.Build, error)
}

type DeploymentTrigger interface {
	TriggerDeploymentAssociated(
		context.Context,
		kernel.WorkspaceID,
		kernel.MemberID,
		kernel.ProjectID,
		deliveryport.BuildRequest,
		func(context.Context, *buildmodel.Build) error,
	) (*buildmodel.Build, error)
}

type ManifestRenderer interface {
	Render(context.Context, deployport.ProjectDeploymentContext, string) ([]runtimeport.Manifest, error)
}

type RunPublisher interface {
	PublishRun(context.Context, *model.DeploymentRun) error
}

type ExecutionRegistry interface {
	Register(string, context.CancelFunc)
	Done(string)
}

type Orchestrator struct {
	repo           deployrepo.DeploymentRunRepository
	builds         BuildReader
	projects       deployport.ProjectDeploymentReader
	prepare        DeploymentPreparer
	trigger        DeploymentTrigger
	renderer       ManifestRenderer
	deployer       runtimeport.ManifestDeployer
	publisher      RunPublisher
	rolloutTimeout time.Duration
	jenkinsTimeout time.Duration
	clock          func() time.Time
	owner          string
	operations     ExecutionRegistry
	versions       appctx.ProjectVersionReader
	remover        runtimeport.ManifestRemover
}

func (o *Orchestrator) SetProjectVersionReader(reader appctx.ProjectVersionReader) {
	if o != nil {
		o.versions = reader
	}
}

func (o *Orchestrator) SetManifestRemover(remover runtimeport.ManifestRemover) {
	if o != nil {
		o.remover = remover
	}
}

func NewOrchestrator(
	repo deployrepo.DeploymentRunRepository,
	builds BuildReader,
	projects deployport.ProjectDeploymentReader,
	prepare DeploymentPreparer,
	trigger DeploymentTrigger,
	renderer ManifestRenderer,
	deployer runtimeport.ManifestDeployer,
	publisher RunPublisher,
	rolloutTimeout time.Duration,
	jenkinsTimeout time.Duration,
	operations ...ExecutionRegistry,
) *Orchestrator {
	orchestrator := &Orchestrator{
		repo: repo, builds: builds, projects: projects, prepare: prepare, trigger: trigger,
		renderer: renderer, deployer: deployer, publisher: publisher,
		rolloutTimeout: rolloutTimeout, jenkinsTimeout: jenkinsTimeout,
		clock: func() time.Time { return time.Now().UTC() },
		owner: kernel.NewDeploymentRunID().String(),
	}
	if len(operations) > 0 {
		orchestrator.operations = operations[0]
	}
	return orchestrator
}

func (o *Orchestrator) Step(ctx context.Context, runID kernel.DeploymentRunID, workspaceID kernel.WorkspaceID) error {
	run, err := o.repo.FindByID(ctx, runID)
	if errors.Is(err, domain.ErrDeploymentRunNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if run.WorkspaceID != workspaceID {
		return nil
	}

	switch run.Status {
	case model.StatusQueued:
		if _, err := o.projects.Load(ctx, workspaceID, run.ProjectID, run.ClusterID); err != nil {
			return o.handleProjectError(ctx, run, err)
		}
		nextStatus := model.StatusPreparing
		if run.SkipsBuild() {
			nextStatus = model.StatusDeploying
		}
		if err := run.Transition(nextStatus, o.clock()); err != nil {
			return err
		}
		if err := o.repo.Update(ctx, run); err != nil {
			return o.resolveConcurrent(ctx, run.ID, err)
		}
		return o.publishRun(ctx, run)

	case model.StatusPreparing:
		operationCtx, cancel := context.WithTimeout(ctx, o.jenkinsTimeout)
		defer cancel()
		o.registerOperation(run.ID, cancel)
		defer o.doneOperation(run.ID)
		acquired, err := o.acquireLease(operationCtx, run, operationLeaseTTL(operationCtx, o.jenkinsTimeout))
		if err != nil || !acquired {
			return err
		}
		project, err := o.projects.Load(operationCtx, workspaceID, run.ProjectID, run.ClusterID)
		if err != nil {
			if operationCtx.Err() != nil {
				return operationCtx.Err()
			}
			return o.handleProjectError(ctx, run, err)
		}
		deployacl.OverrideDeployNamespace(&project, run.DeployNamespace)
		applyRunImageRepository(&project, run)
		if err := o.prepare.Run(operationCtx, project, PrepareInput{
			GitRefType: run.GitRefType, GitRefValue: run.GitRefValue,
			ImageTag: run.ImageTag, ImageRepository: project.ImageRepository,
		}); err != nil {
			if operationCtx.Err() != nil {
				return operationCtx.Err()
			}
			return o.handleDependencyError(ctx, run, err)
		}

		k8sErrCh := make(chan error, 1)
		go func() {
			k8sErrCh <- o.bootstrapKubernetes(operationCtx, run, project)
		}()

		_, err = o.trigger.TriggerDeploymentAssociated(operationCtx, workspaceID, run.CreatedBy, run.ProjectID, deliveryport.BuildRequest{
			Namespace: project.Namespace, ProjectName: project.Name,
			Folder: project.Namespace, Job: project.Name,
			GitRefType: run.GitRefType, GitRefValue: run.GitRefValue,
			ImageTag: run.ImageTag, ImageRepository: project.ImageRepository, Timeout: o.jenkinsTimeout,
			Credentials: BuildJenkinsCredentials(workspaceID, project),
		}, func(callbackCtx context.Context, build *buildmodel.Build) error {
			run.BuildID = &build.ID
			if err := run.Transition(model.StatusBuilding, o.clock()); err != nil {
				return err
			}
			o.clearLease(run)
			if err := o.repo.Update(callbackCtx, run); err != nil {
				if !errors.Is(err, domain.ErrDeploymentRunConcurrentUpdate) {
					return err
				}
				latest, findErr := o.repo.FindByID(callbackCtx, run.ID)
				if findErr != nil {
					return findErr
				}
				if latest.Status == model.StatusBuilding && latest.BuildID != nil && *latest.BuildID == build.ID {
					return nil
				}
				return errAssociationObsolete
			}
			return nil
		})
		k8sErr := <-k8sErrCh
		if k8sErr != nil {
			if operationCtx.Err() != nil {
				return operationCtx.Err()
			}
			return o.handleDependencyError(ctx, run, k8sErr)
		}
		if err != nil {
			if errors.Is(err, errAssociationObsolete) {
				return nil
			}
			if operationCtx.Err() != nil {
				return operationCtx.Err()
			}
			return o.handleDependencyError(ctx, run, err)
		}
		return nil

	case model.StatusBuilding:
		if run.BuildID == nil {
			return o.failBusiness(ctx, run, domain.ErrDeploymentBindingInvalid)
		}
		build, err := o.builds.FindByID(ctx, *run.BuildID)
		if err != nil {
			return o.handleDependencyError(ctx, run, err)
		}
		switch build.Status {
		case buildmodel.BuildStatusRunning:
			if build.ExecutionStartedAt == nil {
				return o.failBusiness(ctx, run, domain.ErrDeploymentBuildStalled)
			}
			if o.jenkinsTimeout > 0 && build.ExecutionStartedAt.Add(o.jenkinsTimeout).Before(o.clock()) {
				return o.failBusiness(ctx, run, domain.ErrDeploymentBuildStalled)
			}
			return nil
		case buildmodel.BuildStatusSuccess:
			if err := run.Transition(model.StatusDeploying, o.clock()); err != nil {
				return err
			}
			if err := o.repo.Update(ctx, run); err != nil {
				return o.resolveConcurrent(ctx, run.ID, err)
			}
			return o.publishRun(ctx, run)
		case buildmodel.BuildStatusFailed, buildmodel.BuildStatusAborted:
			return o.failBusiness(ctx, run, fmt.Errorf("build completed with status %s: %w", build.Status, domain.ErrDeploymentBindingInvalid))
		default:
			return nil
		}

	case model.StatusDeploying:
		operationCtx, cancel := context.WithTimeout(ctx, o.rolloutTimeout)
		defer cancel()
		o.registerOperation(run.ID, cancel)
		defer o.doneOperation(run.ID)
		acquired, err := o.acquireLease(operationCtx, run, operationLeaseTTL(operationCtx, o.rolloutTimeout))
		if err != nil || !acquired {
			return err
		}
		project, err := o.projects.Load(operationCtx, workspaceID, run.ProjectID, run.ClusterID)
		if err != nil {
			if operationCtx.Err() != nil {
				return operationCtx.Err()
			}
			return o.handleProjectError(ctx, run, err)
		}
		deployacl.OverrideDeployNamespace(&project, run.DeployNamespace)
		applyRunImageRepository(&project, run)
		if run.IsUndeploy() {
			if err := o.removeKubernetes(operationCtx, run, project); err != nil {
				if operationCtx.Err() != nil {
					return operationCtx.Err()
				}
				return o.handleDependencyError(ctx, run, err)
			}
			if err := run.Transition(model.StatusSucceeded, o.clock()); err != nil {
				return err
			}
			o.clearLease(run)
			if err := o.repo.Update(ctx, run); err != nil {
				return o.resolveConcurrent(ctx, run.ID, err)
			}
			return nil
		}
		if !(run.IsSyncOnly() && len(run.SyncManifests) > 0) {
			if err := o.finalizeKubernetes(operationCtx, run, project); err != nil {
				if operationCtx.Err() != nil {
					return operationCtx.Err()
				}
				return o.handleDependencyError(ctx, run, err)
			}
		}
		if operationCtx.Err() != nil {
			return operationCtx.Err()
		}
		if err := o.syncCrossClusterTargets(operationCtx, ctx, workspaceID, run); err != nil {
			if operationCtx.Err() != nil {
				return operationCtx.Err()
			}
			return o.failBusiness(ctx, run, err)
		}
		if err := run.Transition(model.StatusSucceeded, o.clock()); err != nil {
			return err
		}
		o.clearLease(run)
		if err := o.repo.Update(ctx, run); err != nil {
			return o.resolveConcurrent(ctx, run.ID, err)
		}
		return nil

	default:
		return nil
	}
}

func (o *Orchestrator) registerOperation(runID kernel.DeploymentRunID, cancel context.CancelFunc) {
	if o.operations != nil {
		o.operations.Register(runID.String(), cancel)
	}
}

func (o *Orchestrator) doneOperation(runID kernel.DeploymentRunID) {
	if o.operations != nil {
		o.operations.Done(runID.String())
	}
}

func (o *Orchestrator) handleProjectError(ctx context.Context, run *model.DeploymentRun, err error) error {
	return o.handleDependencyError(ctx, run, err)
}

func (o *Orchestrator) handleDependencyError(ctx context.Context, run *model.DeploymentRun, err error) error {
	if errors.Is(err, domain.ErrDeploymentRunConcurrentUpdate) {
		return o.resolveConcurrent(ctx, run.ID, err)
	}
	var domainErr *kernel.DomainError
	if errors.As(err, &domainErr) {
		return o.failBusiness(ctx, run, err)
	}
	return o.recordInfrastructureFailure(ctx, run, err)
}

func (o *Orchestrator) failBusiness(ctx context.Context, run *model.DeploymentRun, cause error) error {
	run.Fail(cause.Error(), o.clock())
	if run.Status != model.StatusFailed {
		return fmt.Errorf("fail deployment run: %w", domain.ErrInvalidDeploymentTransition)
	}
	o.clearLease(run)
	if err := o.repo.Update(ctx, run); err != nil {
		return o.resolveConcurrent(ctx, run.ID, err)
	}
	return nil
}

func (o *Orchestrator) acquireLease(ctx context.Context, run *model.DeploymentRun, ttl time.Duration) (bool, error) {
	acquired, err := o.repo.TryAcquireLease(ctx, run.ID, run.Version, o.owner, ttl)
	if err != nil || !acquired {
		return acquired, err
	}
	run.Version++
	run.LeaseOwner = o.owner
	expires := o.clock().Add(ttl)
	run.LeaseExpiresAt = &expires
	return true, nil
}

func operationLeaseTTL(ctx context.Context, fallback time.Duration) time.Duration {
	operationTTL := fallback
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining < operationTTL {
			operationTTL = remaining
		}
	}
	if operationTTL < 0 {
		operationTTL = 0
	}
	return operationTTL + time.Minute
}

func (o *Orchestrator) clearLease(run *model.DeploymentRun) {
	run.LeaseOwner = ""
	run.LeaseExpiresAt = nil
}

func (o *Orchestrator) recordInfrastructureFailure(ctx context.Context, run *model.DeploymentRun, cause error) error {
	message := "infrastructure retry budget exhausted"
	if cause != nil {
		message = cause.Error()
	}
	updated, err := o.repo.RecordInfrastructureFailure(ctx, run.ID, run.Version, message)
	if err != nil {
		return err
	}
	if updated.Status == model.StatusFailed {
		return nil
	}
	return cause
}

func (o *Orchestrator) publishRun(ctx context.Context, run *model.DeploymentRun) error {
	if err := o.publisher.PublishRun(ctx, run); err != nil {
		return o.recordInfrastructureFailure(ctx, run, err)
	}
	return nil
}

func (o *Orchestrator) bootstrapKubernetes(
	ctx context.Context,
	run *model.DeploymentRun,
	project deployport.ProjectDeploymentContext,
) error {
	if err := o.applyConfigManifests(ctx, run, project); err != nil {
		return err
	}
	manifests, deploymentName, serviceName, err := o.renderManifests(ctx, project, run, run.ImageTag)
	if err != nil {
		return err
	}
	target := o.buildDeploymentTarget(project, run, deploymentName, serviceName)
	target.SkipRolloutWait = true
	return o.deployer.Deploy(ctx, target, manifests)
}

func (o *Orchestrator) finalizeKubernetes(
	ctx context.Context,
	run *model.DeploymentRun,
	project deployport.ProjectDeploymentContext,
) error {
	if err := o.applyConfigManifests(ctx, run, project); err != nil {
		return err
	}
	manifests, deploymentName, serviceName, err := o.renderManifests(ctx, project, run, run.ImageTag)
	if err != nil {
		return err
	}
	target := o.buildDeploymentTarget(project, run, deploymentName, serviceName)
	target.SkipRolloutWait = true
	if err := o.deployer.Deploy(ctx, target, manifests); err != nil {
		return err
	}
	return o.deployer.RolloutRestart(ctx, target)
}

func (o *Orchestrator) removeKubernetes(
	ctx context.Context,
	run *model.DeploymentRun,
	project deployport.ProjectDeploymentContext,
) error {
	if o.remover == nil {
		return domain.ErrDeploymentRemoverUnavailable
	}
	spec := runtimeport.RemoveSpec{AppLabel: project.Name}
	if run.UndeployOptions != nil {
		spec.DeleteConfig = run.UndeployOptions.DeleteConfig
		spec.DeleteStorage = run.UndeployOptions.DeleteStorage
	}
	manifests, deploymentName, serviceName, err := o.renderManifests(ctx, project, run, run.ImageTag)
	if err == nil {
		spec.Manifests = manifests
	} else {
		_ = o.repo.AppendEvent(ctx, run, "render manifests failed, falling back to label selector: "+err.Error())
	}
	target := o.buildDeploymentTarget(project, run, deploymentName, serviceName)
	_ = o.repo.AppendEvent(ctx, run, fmt.Sprintf("removing workloads from %s/%s", project.Cluster.Name, project.Namespace))
	return o.remover.Remove(ctx, target, spec)
}

func (o *Orchestrator) applyConfigManifests(
	ctx context.Context,
	run *model.DeploymentRun,
	project deployport.ProjectDeploymentContext,
) error {
	if run.IsSyncOnly() || len(run.SyncManifests) == 0 {
		return nil
	}
	_, err := o.applySyncManifests(ctx, run, project)
	return err
}

func (o *Orchestrator) syncCrossClusterTargets(
	operationCtx context.Context,
	parentCtx context.Context,
	workspaceID kernel.WorkspaceID,
	run *model.DeploymentRun,
) error {
	if !run.CrossCluster {
		return nil
	}
	clusters, err := o.projects.ListAuthorizedClusters(operationCtx, workspaceID)
	if err != nil {
		return err
	}
	useSyncManifests := run.IsSyncOnly() && len(run.SyncManifests) > 0
	for _, cluster := range clusters {
		if cluster.ID == run.ClusterID {
			continue
		}
		_ = o.repo.AppendEvent(parentCtx, run, fmt.Sprintf("cross-cluster sync started: %s", cluster.Name))
		project, err := o.projects.Load(operationCtx, workspaceID, run.ProjectID, cluster.ID)
		if err != nil {
			_ = o.repo.AppendEvent(parentCtx, run, fmt.Sprintf("cross-cluster sync failed: %s (%v)", cluster.Name, err))
			return fmt.Errorf("sync cluster %s: %w", cluster.Name, err)
		}
		applyRunImageRepository(&project, run)
		if useSyncManifests {
			applied, err := o.applySyncManifests(operationCtx, run, project)
			if err != nil {
				_ = o.repo.AppendEvent(parentCtx, run, fmt.Sprintf("cross-cluster sync failed: %s (%v)", cluster.Name, err))
				return fmt.Errorf("sync cluster %s: %w", cluster.Name, err)
			}
			if applied != "" {
				_ = o.repo.AppendEvent(parentCtx, run, fmt.Sprintf("applied: %s", applied))
			}
		} else if err := o.finalizeKubernetes(operationCtx, run, project); err != nil {
			_ = o.repo.AppendEvent(parentCtx, run, fmt.Sprintf("cross-cluster sync failed: %s (%v)", cluster.Name, err))
			return fmt.Errorf("sync cluster %s: %w", cluster.Name, err)
		}
		_ = o.repo.AppendEvent(parentCtx, run, fmt.Sprintf("cross-cluster sync succeeded: %s", cluster.Name))
	}
	return nil
}

func (o *Orchestrator) applySyncManifests(
	ctx context.Context,
	run *model.DeploymentRun,
	project deployport.ProjectDeploymentContext,
) (string, error) {
	ordered := model.SyncApplyOrder(run.SyncManifests)
	manifests := make([]runtimeport.Manifest, 0, len(ordered))
	pullSecrets := make([]string, 0)
	applied := make([]string, 0, len(ordered))
	deploymentName := ""
	serviceName := ""
	hasDeployment := false
	for _, item := range ordered {
		applied = append(applied, item.Kind+"/"+item.Name)
		if item.ManagedPullSecret {
			pullSecrets = append(pullSecrets, item.Name)
			continue
		}
		manifests = append(manifests, runtimeport.Manifest{YAML: item.YAML})
		switch item.Kind {
		case model.SyncManifestKindDeployment:
			deploymentName = item.Name
			hasDeployment = true
		case model.SyncManifestKindService:
			serviceName = item.Name
		}
	}
	if len(manifests) == 0 && len(pullSecrets) == 0 {
		return "", domain.ErrDeploymentSyncManifestsEmpty
	}
	target := o.buildDeploymentTarget(project, run, deploymentName, serviceName)
	target.AllowPartialWorkloads = true
	target.SkipRolloutWait = true
	target.ExtraImagePullSecrets = pullSecrets
	if !hasDeployment {
		target.ImageTag = ""
	}
	if err := o.deployer.Deploy(ctx, target, manifests); err != nil {
		return "", err
	}
	if hasDeployment {
		if err := o.deployer.RolloutRestart(ctx, target); err != nil {
			return "", err
		}
	}
	return strings.Join(applied, ", "), nil
}

func (o *Orchestrator) renderManifests(
	ctx context.Context,
	project deployport.ProjectDeploymentContext,
	run *model.DeploymentRun,
	imageTag string,
) ([]runtimeport.Manifest, string, string, error) {
	if run != nil && run.ProjectVersionID != nil && o.versions != nil {
		snapshot, err := o.versions.LoadSnapshot(ctx, *run.ProjectVersionID)
		if err != nil {
			return nil, "", "", err
		}
		for _, item := range snapshot.Manifests {
			switch item.Kind {
			case "Service":
				project.ServiceYAML = item.Content
			case "Deployment":
				project.DeploymentYAML = item.Content
			}
		}
		if imageTag == "" && len(snapshot.Images) > 0 {
			imageTag = snapshot.Images[0].Tag
		}
	}
	manifests, err := o.renderer.Render(ctx, project, imageTag)
	if err != nil {
		return nil, "", "", err
	}
	deploymentName, serviceName, err := extractManifestResourceNames(manifests)
	if err != nil {
		return nil, "", "", err
	}
	return manifests, deploymentName, serviceName, nil
}

func (o *Orchestrator) buildDeploymentTarget(
	project deployport.ProjectDeploymentContext,
	run *model.DeploymentRun,
	deploymentName, serviceName string,
) runtimeport.DeploymentTarget {
	return runtimeport.DeploymentTarget{
		ClusterID: project.Cluster.ID.String(), Server: project.Cluster.Server,
		Kubeconfig: project.Kubernetes.Credentials.ExtraConfig,
		Namespace:  project.Namespace, DeploymentName: deploymentName,
		ServiceName: serviceName, ImageTag: run.ImageTag,
		RolloutTimeout: o.rolloutTimeout,
		RegistryPull:   registryPullCredentialFromHarbor(project.Harbor),
	}
}

func (o *Orchestrator) resolveConcurrent(ctx context.Context, runID kernel.DeploymentRunID, updateErr error) error {
	if !errors.Is(updateErr, domain.ErrDeploymentRunConcurrentUpdate) {
		return updateErr
	}
	latest, err := o.repo.FindByID(ctx, runID)
	if err != nil {
		return err
	}
	switch latest.Status {
	case model.StatusPreparing, model.StatusDeploying:
		return o.publishRun(ctx, latest)
	case model.StatusBuilding, model.StatusSucceeded, model.StatusFailed, model.StatusAborted:
		return nil
	default:
		return updateErr
	}
}

func applyRunImageRepository(project *deployport.ProjectDeploymentContext, run *model.DeploymentRun) {
	if project == nil || run == nil {
		return
	}
	if repo := strings.TrimSpace(run.ImageRepository); repo != "" {
		project.ImageRepository = repo
	}
}
