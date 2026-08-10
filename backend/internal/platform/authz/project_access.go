package authz

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/policy"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectAccessChecker interface {
	HasProjectAccess(ctx context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID, projectID kernel.ProjectID, action string) (bool, error)
}

type projectAccessImpl struct {
	governance PermissionChecker
	engine     policy.Engine
	members    ProjectMemberReader
}

type ProjectMemberReader interface {
	HasAccess(ctx context.Context, projectID kernel.ProjectID, memberID kernel.MemberID) (bool, string, error)
}

func NewProjectAccessChecker(governance PermissionChecker, engine policy.Engine, members ProjectMemberReader) ProjectAccessChecker {
	return &projectAccessImpl{governance: governance, engine: engine, members: members}
}

func (p *projectAccessImpl) HasProjectAccess(
	ctx context.Context,
	workspaceID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
	action string,
) (bool, error) {
	if p.governance != nil {
		isAdmin, err := p.governance.HasPermission(ctx, workspaceID, memberID, PermGovernanceAdmin)
		if err != nil {
			return false, err
		}
		if isAdmin {
			return true, nil
		}
	}
	if p.engine == nil {
		if p.members != nil {
			has, role, err := p.members.HasAccess(ctx, projectID, memberID)
			if err != nil {
				return false, err
			}
			return has && roleAllowsProjectAction(role, action), nil
		}
		return false, nil
	}
	allowed, err := p.engine.Enforce(ctx, memberID.String(), workspaceID.String(), policy.ProjectObject(projectID.String()), action)
	if err != nil {
		return false, err
	}
	if allowed {
		return true, nil
	}
	if p.members != nil {
		has, role, err := p.members.HasAccess(ctx, projectID, memberID)
		if err != nil {
			return false, err
		}
		return has && roleAllowsProjectAction(role, action), nil
	}
	return false, nil
}

func roleAllowsProjectAction(role, action string) bool {
	switch action {
	case policy.ActionAdmin:
		return role == "lead"
	case policy.ActionWrite:
		return role == "lead" || role == "member"
	default:
		return role == "lead" || role == "member" || role == "viewer"
	}
}

func RequireProjectAccess(checker ProjectAccessChecker, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if checker == nil {
			httpx.Fail(c, "PROJECT_ACCESS_DENIED", "project access denied", http.StatusForbidden)
			c.Abort()
			return
		}
		memberID, ok := authn.MemberIDFromContext(c)
		if !ok {
			httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
			c.Abort()
			return
		}
		wsID, ok := httpx.WorkspaceIDFromContext(c)
		if !ok {
			httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
			c.Abort()
			return
		}
		projectID, err := kernel.ParseProjectID(c.Param("projectId"))
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid project id", http.StatusBadRequest)
			c.Abort()
			return
		}
		allowed, err := checker.HasProjectAccess(c.Request.Context(), wsID, memberID, projectID, action)
		if err != nil {
			httpx.HandleError(c, err)
			c.Abort()
			return
		}
		if !allowed {
			httpx.Fail(c, "PROJECT_ACCESS_DENIED", "project access denied", http.StatusForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}
