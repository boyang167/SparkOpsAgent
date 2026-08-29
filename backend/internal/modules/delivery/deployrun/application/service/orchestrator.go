package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	deployrepo "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/repository"
	buildmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/model"
	deliveryport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	runtimeport "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
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
		if err := run.Transition(model.StatusPreparing, o.clock()); err != nil {
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
		if err := o.prepare.Run(operationCtx, project, PrepareInput{
			GitRefValue: run.GitRefValue, ImageTag: run.ImageTag, ImageRepository: project.ImageRepository,
		}); err != nil {
			if operationCtx.Err() != nil {
				return operationCtx.Err()
			}
			return o.handleDependencyError(ctx, run, err)
		}
		_, err = o.trigger.TriggerDeploymentAssociated(operationCtx, workspaceID, run.CreatedBy, run.ProjectID, deliveryport.BuildRequest{
			Namespace: project.Namespace, ProjectName: project.Name,
			Folder: project.Namespace, Job: project.Name, GitRefValue: run.GitRefValue,
			ImageTag: run.ImageTag, ImageRepository: project.ImageRepository, Timeout: o.jenkinsTimeout,
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
		manifests, err := o.renderer.Render(operationCtx, project, run.ImageTag)
		if err != nil {
			if operationCtx.Err() != nil {
				return operationCtx.Err()
			}
			return o.failBusiness(ctx, run, err)
		}
		target := runtimeport.DeploymentTarget{
			ClusterID: project.Cluster.ID.String(), Server: project.Cluster.Server,
			Kubeconfig: project.Kubernetes.Credentials.ExtraConfig,
			Namespace:  project.Namespace, DeploymentName: project.Name + "-deployment",
			ServiceName: project.Name + "-service", ImageTag: run.ImageTag,
			RolloutTimeout: o.rolloutTimeout,
		}
		if err := o.deployer.Deploy(operationCtx, target, manifests); err != nil {
			if operationCtx.Err() != nil {
				return operationCtx.Err()
			}
			return o.handleDependencyError(ctx, run, err)
		}
		if operationCtx.Err() != nil {
			return operationCtx.Err()
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
	updated, err := o.repo.RecordInfrastructureFailure(
		ctx, run.ID, run.Version, "infrastructure retry budget exhausted",
	)
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
