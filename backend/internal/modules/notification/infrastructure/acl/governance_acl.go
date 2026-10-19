package acl

import (
	"context"

	governanceacl "github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type GovernanceACL struct {
	members   *governanceacl.MemberACL
	permCheck authz.PermissionChecker
}

func NewGovernanceACL(members *governanceacl.MemberACL, permCheck authz.PermissionChecker) *GovernanceACL {
	return &GovernanceACL{members: members, permCheck: permCheck}
}

func (g *GovernanceACL) ListProjectReviewRecipients(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	excludeMemberID kernel.MemberID,
) ([]kernel.MemberID, error) {
	members, err := g.members.ListWorkspaceMembers(ctx, wsID)
	if err != nil {
		return nil, err
	}
	out := make([]kernel.MemberID, 0, len(members))
	seen := make(map[string]struct{}, len(members))
	for _, member := range members {
		if member.MemberID == excludeMemberID {
			continue
		}
		key := member.MemberID.String()
		if _, ok := seen[key]; ok {
			continue
		}
		canReview, err := g.permCheck.HasPermission(ctx, wsID, member.MemberID, authz.PermProjectReview)
		if err != nil {
			return nil, err
		}
		canAdmin, err := g.permCheck.HasPermission(ctx, wsID, member.MemberID, authz.PermGovernanceAdmin)
		if err != nil {
			return nil, err
		}
		if !canReview && !canAdmin {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, member.MemberID)
	}
	return out, nil
}
