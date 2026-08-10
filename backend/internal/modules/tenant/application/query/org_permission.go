package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func (a *AccessChecker) IsOrgMember(ctx context.Context, orgID kernel.OrgID, memberID kernel.MemberID) (bool, error) {
	return a.orgRepo.IsMember(ctx, orgID, memberID)
}

func (a *AccessChecker) HasOrgPermission(ctx context.Context, orgID kernel.OrgID, memberID kernel.MemberID, permission string) (bool, error) {
	role, err := a.orgRepo.GetMemberRole(ctx, orgID, memberID)
	if err != nil {
		return false, err
	}
	return authz.OrgRoleHasPermission(role, permission), nil
}

func (a *AccessChecker) CanCreateWorkspace(ctx context.Context, orgID kernel.OrgID, memberID kernel.MemberID) (bool, error) {
	return a.HasOrgPermission(ctx, orgID, memberID, authz.PermOrgWorkspaceCreate)
}
