package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

func TestExplicitDenyOverridesInheritedAllow(t *testing.T) {
	org, workspace := uuid.New(), uuid.New()
	parentDirectory := uuid.New()
	childFile := Resource{
		ID: uuid.New(), OrganizationID: org, WorkspaceID: workspace, ParentID: &parentDirectory,
	}
	subject := Subject{
		ActorID: uuid.New(), OrganizationID: org, WorkspaceID: workspace,
		Authenticated: true, Entitled: true, RBACAllowed: true, TraceID: "trace-deny",
	}

	allowed := Decide(subject, actionRead, childFile, inheritedAllow(parentDirectory))
	require.Equal(t, DecisionAllow, allowed.Result)
	require.Equal(t, "grant_allow", allowed.ReasonCode)

	decision := Decide(subject, actionRead, childFile, inheritedAllow(parentDirectory), explicitDeny(childFile))
	require.Equal(t, DecisionDeny, decision.Result)
	require.Equal(t, "explicit_deny", decision.ReasonCode)
	require.Equal(t, "deny-child", decision.PolicyID)
	require.Equal(t, "trace-deny", decision.TraceID)
}

func TestCapabilityIsIntersectionNotDelegatorUnion(t *testing.T) {
	fileA, fileB := uuid.New(), uuid.New()
	issuer := NewCapabilityIssuer(nil)

	token, err := issuer.Issue(context.Background(), requestForResources(fileA, fileB), runAllows(fileA))
	require.NoError(t, err)
	require.ElementsMatch(t, []uuid.UUID{fileA}, token.ResourceIDs)

	narrower, err := issuer.Issue(context.Background(), requestForResources(fileA), runAllows(fileA, fileB))
	require.NoError(t, err)
	require.ElementsMatch(t, []uuid.UUID{fileA}, narrower.ResourceIDs)

	delegated := requestForResources(fileA, fileB)
	delegated.DelegatorResourceIDs = []uuid.UUID{fileB}
	_, err = issuer.Issue(context.Background(), delegated, runAllows(fileA))
	require.ErrorIs(t, err, domain.ErrCapabilityScopeEmpty)
}

var actionRead = model.ResourcePermissionRead

func inheritedAllow(parentDirectory uuid.UUID) Relation {
	return Relation{
		ResourceID: parentDirectory, Effect: model.GrantEffectAllow, Inherited: true,
		Permission: model.ResourcePermissionRead, Source: SourceGrant,
		PolicyID: "allow-parent", PolicyVersion: "1",
	}
}

func explicitDeny(childFile Resource) Relation {
	return Relation{
		ResourceID: childFile.ID, Effect: model.GrantEffectDeny,
		Permission: model.ResourcePermissionRead, Source: SourceGrant,
		PolicyID: "deny-child", PolicyVersion: "1",
	}
}

func requestForResources(ids ...uuid.UUID) CapabilityRequest {
	return CapabilityRequest{
		OrganizationID: uuid.New(), WorkspaceID: uuid.New(), ActorID: uuid.New(), RunID: uuid.New(),
		ResourceIDs: ids, Actions: []string{"file:read"}, Audience: "sparkops-agent-capability",
	}
}

func runAllows(ids ...uuid.UUID) RunScope {
	return RunScope{ResourceIDs: ids, Actions: []string{"file:read"}}
}
