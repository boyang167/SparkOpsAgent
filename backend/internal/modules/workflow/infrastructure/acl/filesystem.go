package acl

import (
	"context"
	"time"

	"github.com/google/uuid"

	fscommand "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	fsdomain "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	fsservice "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/service"
)

// RunFileRefs tracks file and artifact identities attached to a workflow run.
type RunFileRefs struct {
	FileIDs     []uuid.UUID
	ArtifactIDs []uuid.UUID
}

type issueCapabilityFunc func(context.Context, fscommand.IssueCapability) (fscommand.IssuedCapability, error)
type revokeCapabilityFunc func(context.Context, fscommand.RevokeCapability) error

// RunFileAccess issues and revokes run-scoped filesystem capabilities for workflow execution.
type RunFileAccess struct {
	issue  issueCapabilityFunc
	revoke revokeCapabilityFunc
}

func NewRunFileAccess(
	issue func(context.Context, fscommand.IssueCapability) (fscommand.IssuedCapability, error),
	revoke func(context.Context, fscommand.RevokeCapability) error,
) *RunFileAccess {
	return &RunFileAccess{issue: issue, revoke: revoke}
}

type IssueRunCapabilityRequest struct {
	CommandID      uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	ActorID        uuid.UUID
	RunID          uuid.UUID
	FileIDs        []uuid.UUID
	Actions        []string
	ExpiresAt      time.Time
}

func (a *RunFileAccess) IssueForRun(ctx context.Context, req IssueRunCapabilityRequest) (fscommand.IssuedCapability, error) {
	if a == nil || a.issue == nil {
		return fscommand.IssuedCapability{}, fsdomain.ErrCapabilityInvalid
	}
	return a.issue(ctx, fscommand.IssueCapability{
		CommandID:        req.CommandID,
		OrganizationID:   req.OrganizationID,
		WorkspaceID:      req.WorkspaceID,
		ActorID:          req.ActorID,
		RunID:            req.RunID,
		ResourceIDs:      append([]uuid.UUID(nil), req.FileIDs...),
		RunResourceIDs:   append([]uuid.UUID(nil), req.FileIDs...),
		Actions:          append([]string(nil), req.Actions...),
		RunActions:       append([]string(nil), req.Actions...),
		Audience:         fsservice.AgentCapabilityAudience,
		ExpiresAt:        req.ExpiresAt,
	})
}

func (a *RunFileAccess) RevokeForRun(ctx context.Context, capabilityID uuid.UUID) error {
	if a == nil || a.revoke == nil || capabilityID == uuid.Nil {
		return nil
	}
	return a.revoke(ctx, fscommand.RevokeCapability{ID: capabilityID})
}
