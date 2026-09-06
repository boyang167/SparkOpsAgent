package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	deployacl "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/acl"
	deliveryport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"go.uber.org/zap"
)

type jenkinsCredentialProjectLister interface {
	ListProjectsUsingSecret(context.Context, kernel.WorkspaceID, string) ([]deployacl.ProjectCredentialSyncTarget, error)
	LoadJenkinsCredentialContext(context.Context, kernel.WorkspaceID, kernel.ProjectID) (deployport.ProjectDeploymentContext, error)
}

type CredentialSyncService struct {
	projects jenkinsCredentialProjectLister
	jenkins  deliveryport.JenkinsExecutorProvider
	timeout  time.Duration
	logger   *zap.Logger
}

func NewCredentialSyncService(
	projects jenkinsCredentialProjectLister,
	jenkins deliveryport.JenkinsExecutorProvider,
	timeout time.Duration,
	logger *zap.Logger,
) *CredentialSyncService {
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	return &CredentialSyncService{projects: projects, jenkins: jenkins, timeout: timeout, logger: logger}
}

func (s *CredentialSyncService) SyncForSecret(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	secretID kernel.SecretID,
	serviceType string,
) error {
	if !IsJenkinsManagedSecretType(serviceType) {
		return nil
	}
	targets, err := s.projects.ListProjectsUsingSecret(ctx, wsID, secretID.String())
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return nil
	}
	var syncErrs []error
	for _, target := range targets {
		if err := s.syncProject(ctx, wsID, target); err != nil {
			syncErrs = append(syncErrs, fmt.Errorf("project %s: %w", target.ProjectID.String(), err))
		}
	}
	return errors.Join(syncErrs...)
}

func (s *CredentialSyncService) syncProject(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	target deployacl.ProjectCredentialSyncTarget,
) error {
	syncCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	project, err := s.projects.LoadJenkinsCredentialContext(syncCtx, wsID, target.ProjectID)
	if err != nil {
		return err
	}
	folder := strings.TrimSpace(project.Namespace)
	if folder == "" {
		folder = strings.TrimSpace(target.Namespace)
	}
	if folder == "" {
		return fmt.Errorf("project namespace is required")
	}
	executor, err := s.jenkins.ForProject(syncCtx, wsID, target.ProjectID)
	if err != nil {
		return err
	}
	credentials := BuildJenkinsCredentials(wsID, project)
	if len(credentials) == 0 {
		return nil
	}
	if err := executor.SyncCredentials(syncCtx, folder, credentials); err != nil {
		return err
	}
	if s.logger != nil {
		s.logger.Info("jenkins credentials synced",
			zap.String("workspace_id", wsID.String()),
			zap.String("project_id", target.ProjectID.String()),
			zap.String("folder", folder),
			zap.Int("credential_count", len(credentials)),
		)
	}
	return nil
}
