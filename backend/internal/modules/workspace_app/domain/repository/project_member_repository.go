package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectMemberRecord struct {
	MemberID    kernel.MemberID
	Email       string
	DisplayName string
	Role        string
	JoinedAt    string
}

type ProjectMemberRepository interface {
	Save(ctx context.Context, member *model.ProjectMember) error
	Find(ctx context.Context, projectID kernel.ProjectID, memberID kernel.MemberID) (*model.ProjectMember, error)
	ListByProject(ctx context.Context, projectID kernel.ProjectID) ([]ProjectMemberRecord, error)
	ListProjectIDsForMember(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID) ([]kernel.ProjectID, error)
	UpdateRole(ctx context.Context, projectID kernel.ProjectID, memberID kernel.MemberID, role string) error
	Remove(ctx context.Context, projectID kernel.ProjectID, memberID kernel.MemberID) error
	HasAccess(ctx context.Context, projectID kernel.ProjectID, memberID kernel.MemberID) (bool, string, error)
}
