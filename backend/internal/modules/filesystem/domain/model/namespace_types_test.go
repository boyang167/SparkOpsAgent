package model

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNamespaceValidateAcceptsTypedScopes(t *testing.T) {
	orgID, workspaceID, userID := uuid.New(), uuid.New(), uuid.New()
	for _, namespace := range []Namespace{
		{NamespaceType: NamespaceTypeOrganization, OrganizationID: orgID},
		{NamespaceType: NamespaceTypeWorkspace, OrganizationID: orgID, WorkspaceID: workspaceID},
		{NamespaceType: NamespaceTypeUser, OrganizationID: orgID, OwnerUserID: userID},
	} {
		require.NoError(t, namespace.Validate())
	}
}

func TestNamespaceValidateRejectsInvalidScopeCombinations(t *testing.T) {
	orgID, workspaceID, userID := uuid.New(), uuid.New(), uuid.New()
	for _, namespace := range []Namespace{
		{},
		{NamespaceType: NamespaceTypeOrganization},
		{NamespaceType: NamespaceTypeOrganization, OrganizationID: orgID, WorkspaceID: workspaceID},
		{NamespaceType: NamespaceTypeWorkspace, OrganizationID: orgID},
		{NamespaceType: NamespaceTypeWorkspace, OrganizationID: orgID, WorkspaceID: workspaceID, OwnerUserID: userID},
		{NamespaceType: NamespaceTypeUser, OrganizationID: orgID},
		{NamespaceType: NamespaceTypeUser, OrganizationID: orgID, WorkspaceID: workspaceID, OwnerUserID: userID},
	} {
		require.Error(t, namespace.Validate())
	}
}
