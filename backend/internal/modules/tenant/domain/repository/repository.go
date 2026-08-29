package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type OrgMember struct {
	MemberID    kernel.MemberID
	Email       string
	DisplayName string
	Role        string
	JoinedAt    string
}

type OrgOwner struct {
	OrgID       kernel.OrgID
	MemberID    kernel.MemberID
	Email       string
	DisplayName string
}

type OrgUsage struct {
	Workspaces int
	Agents     int
	Projects   int
}

type OrganizationRepository interface {
	Save(ctx context.Context, org *model.Organization) error
	FindByID(ctx context.Context, id kernel.OrgID) (*model.Organization, error)
	UpdateQuota(ctx context.Context, orgID kernel.OrgID, quota model.OrgQuota) error
	GetUsage(ctx context.Context, orgID kernel.OrgID) (*OrgUsage, error)
	ExistsBySlug(ctx context.Context, slug string) (bool, error)
	ListByMember(ctx context.Context, memberID kernel.MemberID) ([]*model.Organization, error)
	ListAll(ctx context.Context) ([]*model.Organization, error)
	ListAllIncludingDeleted(ctx context.Context) ([]*model.Organization, error)
	UpdateStatus(ctx context.Context, orgID kernel.OrgID, status string) error
	SoftDelete(ctx context.Context, orgID kernel.OrgID) error
	AddMember(ctx context.Context, orgID kernel.OrgID, memberID kernel.MemberID, role string) error
	IsMember(ctx context.Context, orgID kernel.OrgID, memberID kernel.MemberID) (bool, error)
	GetMemberRole(ctx context.Context, orgID kernel.OrgID, memberID kernel.MemberID) (string, error)
	ListMembers(ctx context.Context, orgID kernel.OrgID) ([]OrgMember, error)
	ListOwners(ctx context.Context) ([]OrgOwner, error)
	UpdateMemberRole(ctx context.Context, orgID kernel.OrgID, memberID kernel.MemberID, role string) error
	RemoveMember(ctx context.Context, orgID kernel.OrgID, memberID kernel.MemberID) error
	CountMembersByRole(ctx context.Context, orgID kernel.OrgID, role string) (int, error)
	TransferOwner(ctx context.Context, orgID kernel.OrgID, fromID, toID kernel.MemberID) error
}

type WorkspaceWithMembership struct {
	Workspace *model.Workspace
	IsMember  bool
}

type WorkspaceRepository interface {
	Save(ctx context.Context, ws *model.Workspace) error
	FindByID(ctx context.Context, id kernel.WorkspaceID) (*model.Workspace, error)
	ListByMember(ctx context.Context, memberID kernel.MemberID) ([]*model.Workspace, error)
	ListAllWithMembership(ctx context.Context, memberID kernel.MemberID) ([]WorkspaceWithMembership, error)
	ExistsByOrgSlug(ctx context.Context, orgID kernel.OrgID, slug string) (bool, error)
	AddMember(ctx context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID, role string) error
}
