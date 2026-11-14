package command

import (
	"context"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	filesystemevent "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	sharedevents "github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type shareStoreStub struct {
	namespace      *model.Namespace
	node           *model.Node
	classification model.TransferClassification
	subtree        model.TransferClassification
	requests       map[uuid.UUID]model.ShareRequest
	grants         []model.ResourceGrant
	commands       map[uuid.UUID]repository.CommandDedup
}

func (s *shareStoreStub) GetNodeByID(context.Context, uuid.UUID) (*model.Node, error) {
	return s.node, nil
}
func (s *shareStoreStub) GetNamespace(context.Context, uuid.UUID) (*model.Namespace, error) {
	return s.namespace, nil
}
func (s *shareStoreStub) GetNodeClassification(context.Context, uuid.UUID) (model.TransferClassification, error) {
	return s.classification, nil
}
func (s *shareStoreStub) GetSubtreeHighestClassification(context.Context, uuid.UUID, uuid.UUID) (model.TransferClassification, error) {
	return s.subtree, nil
}
func (s *shareStoreStub) CreateShareRequest(_ context.Context, request model.ShareRequest) error {
	if s.requests == nil {
		s.requests = map[uuid.UUID]model.ShareRequest{}
	}
	s.requests[request.ID] = request
	return nil
}
func (s *shareStoreStub) GetShareRequest(_ context.Context, id uuid.UUID) (model.ShareRequest, error) {
	request, ok := s.requests[id]
	if !ok {
		return model.ShareRequest{}, domain.ErrShareRequestNotFound
	}
	return request, nil
}
func (s *shareStoreStub) ReviewShareRequestCAS(_ context.Context, request model.ShareRequest, from model.ShareRequestStatus) error {
	current := s.requests[request.ID]
	if current.Status != from {
		return domain.ErrShareRequestConflict
	}
	s.requests[request.ID] = request
	return nil
}
func (s *shareStoreStub) CreateResourceGrants(_ context.Context, grants []model.ResourceGrant) error {
	s.grants = append(s.grants, grants...)
	return nil
}
func (s *shareStoreStub) ClaimCommand(_ context.Context, command repository.CommandDedup) (*repository.CommandDedup, error) {
	if s.commands == nil {
		s.commands = map[uuid.UUID]repository.CommandDedup{}
	}
	if existing, ok := s.commands[command.CommandID]; ok {
		if existing.RequestHash != command.RequestHash {
			return nil, repository.ErrIdempotencyConflict
		}
		return &existing, nil
	}
	command.State = "PENDING"
	s.commands[command.CommandID] = command
	return &command, nil
}
func (s *shareStoreStub) CompleteCommand(_ context.Context, commandID uuid.UUID, nodeID, revisionID *uuid.UUID) error {
	command := s.commands[commandID]
	command.State = "COMPLETED"
	command.NodeID, command.RevisionID = nodeID, revisionID
	s.commands[commandID] = command
	return nil
}

type principalScopeStub struct {
	scopes map[uuid.UUID]PrincipalScope
}

func (s principalScopeStub) Resolve(_ context.Context, _ model.PrincipalType, id, _ uuid.UUID) (PrincipalScope, error) {
	scope, ok := s.scopes[id]
	if !ok {
		return PrincipalScope{}, domain.ErrUnsupportedPrincipal
	}
	return scope, nil
}

type shareAuthorityStub struct {
	workspaceAdmin bool
	orgAdmin       bool
}

type shareSourceAuthorizerStub struct {
	allowed bool
	err     error
	called  int
}

func (s *shareSourceAuthorizerStub) AuthorizeShare(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (bool, error) {
	s.called++
	return s.allowed, s.err
}

func (s shareAuthorityStub) HasWorkspaceFileAdmin(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return s.workspaceAdmin, nil
}
func (s shareAuthorityStub) HasOrganizationFileAdmin(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return s.orgAdmin, nil
}

func TestShareRequestPolicyMatrix(t *testing.T) {
	organizationID, workspaceID := uuid.New(), uuid.New()
	actorID, userID := uuid.New(), uuid.New()
	tests := []struct {
		name           string
		namespaceType  model.NamespaceType
		principalType  model.PrincipalType
		actions        []model.ResourcePermission
		effect         model.GrantEffect
		classification model.TransferClassification
		authority      shareAuthorityStub
		wantStatus     model.ShareRequestStatus
		wantErr        error
		wantGrants     int
	}{
		{name: "selected same organization user read is immediate", namespaceType: model.NamespaceTypeUser, principalType: model.PrincipalUser, actions: []model.ResourcePermission{model.ResourcePermissionRead, model.ResourcePermissionDownload}, classification: model.TransferClassificationInternal, wantStatus: model.ShareRequestApproved, wantGrants: 2},
		{name: "personal to workspace is pending without admin", namespaceType: model.NamespaceTypeUser, principalType: model.PrincipalWorkspace, actions: []model.ResourcePermission{model.ResourcePermissionRead}, classification: model.TransferClassificationInternal, wantStatus: model.ShareRequestPending},
		{name: "personal to workspace is immediate for workspace admin", namespaceType: model.NamespaceTypeUser, principalType: model.PrincipalWorkspace, actions: []model.ResourcePermission{model.ResourcePermissionRead}, classification: model.TransferClassificationInternal, authority: shareAuthorityStub{workspaceAdmin: true}, wantStatus: model.ShareRequestApproved, wantGrants: 1},
		{name: "workspace to organization is pending", namespaceType: model.NamespaceTypeWorkspace, principalType: model.PrincipalOrganization, actions: []model.ResourcePermission{model.ResourcePermissionRead}, classification: model.TransferClassificationInternal, wantStatus: model.ShareRequestPending},
		{name: "organization sensitive grant is rejected", namespaceType: model.NamespaceTypeWorkspace, principalType: model.PrincipalOrganization, actions: []model.ResourcePermission{model.ResourcePermissionDelete}, classification: model.TransferClassificationInternal, wantErr: domain.ErrSharePolicyDenied},
		{name: "restricted to organization is rejected", namespaceType: model.NamespaceTypeUser, principalType: model.PrincipalOrganization, actions: []model.ResourcePermission{model.ResourcePermissionRead}, classification: model.TransferClassificationRestricted, wantErr: domain.ErrSharePolicyDenied},
		{name: "confidential expansion remains pending for admin", namespaceType: model.NamespaceTypeUser, principalType: model.PrincipalWorkspace, actions: []model.ResourcePermission{model.ResourcePermissionRead}, classification: model.TransferClassificationConfidential, authority: shareAuthorityStub{workspaceAdmin: true, orgAdmin: true}, wantStatus: model.ShareRequestPending},
		{name: "organization manager publishes to workspace immediately", namespaceType: model.NamespaceTypeOrganization, principalType: model.PrincipalWorkspace, actions: []model.ResourcePermission{model.ResourcePermissionRead}, classification: model.TransferClassificationInternal, authority: shareAuthorityStub{orgAdmin: true}, wantStatus: model.ShareRequestApproved, wantGrants: 1},
		{name: "organization ordinary user cannot publish", namespaceType: model.NamespaceTypeOrganization, principalType: model.PrincipalWorkspace, actions: []model.ResourcePermission{model.ResourcePermissionRead}, classification: model.TransferClassificationInternal, wantErr: domain.ErrResourceDenied},
		{name: "deny write is immediate without expansion approval", namespaceType: model.NamespaceTypeUser, principalType: model.PrincipalWorkspace, actions: []model.ResourcePermission{model.ResourcePermissionWrite}, effect: model.GrantEffectDeny, classification: model.TransferClassificationConfidential, wantStatus: model.ShareRequestApproved, wantGrants: 1},
		{name: "deny organization admin action bypasses allow restrictions", namespaceType: model.NamespaceTypeUser, principalType: model.PrincipalOrganization, actions: []model.ResourcePermission{model.ResourcePermissionAdmin}, effect: model.GrantEffectDeny, classification: model.TransferClassificationRestricted, wantStatus: model.ShareRequestApproved, wantGrants: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespace := &model.Namespace{ID: uuid.New(), NamespaceType: tt.namespaceType, OrganizationID: organizationID, WorkspaceID: workspaceID, OwnerUserID: actorID}
			node := &model.Node{ID: uuid.New(), NamespaceID: namespace.ID}
			store := &shareStoreStub{namespace: namespace, node: node, classification: tt.classification}
			principalID := userID
			scope := PrincipalScope{OrganizationID: organizationID}
			if tt.principalType == model.PrincipalWorkspace {
				principalID = workspaceID
				scope.WorkspaceID = workspaceID
			} else if tt.principalType == model.PrincipalOrganization {
				principalID = organizationID
			}
			handler := NewShareRequestHandler(store, postgres.NewDirectTransactor(), nil, principalScopeStub{
				scopes: map[uuid.UUID]PrincipalScope{principalID: scope},
			}, tt.authority, &shareSourceAuthorizerStub{allowed: true})

			result, err := handler.Submit(context.Background(), SubmitShare{
				CommandID: uuid.New(), ActorID: actorID, OrganizationID: organizationID, WorkspaceID: workspaceID,
				NodeID: node.ID, PrincipalType: tt.principalType, PrincipalIDs: []uuid.UUID{principalID},
				Actions: tt.actions, Effect: tt.effect, InheritToChildren: true,
			})

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Empty(t, store.grants)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantStatus, result.Request.Status)
			require.Len(t, store.grants, tt.wantGrants)
			if tt.effect == model.GrantEffectDeny {
				require.Equal(t, model.GrantEffectDeny, result.Request.Effect)
				require.Equal(t, model.GrantEffectDeny, store.grants[0].Effect)
			}
		})
	}
}

func TestSharePreviewMatchesSubmitPolicy(t *testing.T) {
	organizationID, workspaceID, actorID, userID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	tests := []struct {
		name           string
		principalType  model.PrincipalType
		actions        []model.ResourcePermission
		classification model.TransferClassification
		authority      shareAuthorityStub
		wantOutcome    SharePreviewOutcome
	}{
		{
			name: "confidential user read requires approval", principalType: model.PrincipalUser,
			actions:        []model.ResourcePermission{model.ResourcePermissionRead},
			classification: model.TransferClassificationConfidential, wantOutcome: SharePreviewPendingApproval,
		},
		{
			name: "ordinary user write requires approval", principalType: model.PrincipalUser,
			actions:        []model.ResourcePermission{model.ResourcePermissionWrite},
			classification: model.TransferClassificationInternal, wantOutcome: SharePreviewPendingApproval,
		},
		{
			name: "workspace admin internal share is active", principalType: model.PrincipalWorkspace,
			actions:        []model.ResourcePermission{model.ResourcePermissionRead},
			classification: model.TransferClassificationInternal,
			authority:      shareAuthorityStub{workspaceAdmin: true}, wantOutcome: SharePreviewActive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespace := &model.Namespace{
				ID: uuid.New(), NamespaceType: model.NamespaceTypeUser,
				OrganizationID: organizationID, OwnerUserID: actorID,
			}
			node := &model.Node{ID: uuid.New(), NamespaceID: namespace.ID, Type: model.NodeTypeFile}
			store := &shareStoreStub{namespace: namespace, node: node, classification: tt.classification}
			principalID := userID
			scope := PrincipalScope{OrganizationID: organizationID}
			if tt.principalType == model.PrincipalWorkspace {
				principalID = workspaceID
				scope.WorkspaceID = workspaceID
			}
			handler := NewShareRequestHandler(
				store, postgres.NewDirectTransactor(), nil,
				principalScopeStub{scopes: map[uuid.UUID]PrincipalScope{principalID: scope}},
				tt.authority, &shareSourceAuthorizerStub{allowed: true},
			)
			input := SubmitShare{
				CommandID: uuid.New(), ActorID: actorID, OrganizationID: organizationID,
				WorkspaceID: workspaceID, NodeID: node.ID, PrincipalType: tt.principalType,
				PrincipalIDs: []uuid.UUID{principalID}, Actions: tt.actions,
			}

			preview, err := handler.Preview(context.Background(), input)
			require.NoError(t, err)
			require.Equal(t, tt.wantOutcome, preview.Outcome)
			require.Empty(t, store.requests)
			require.Empty(t, store.grants)
			require.Empty(t, store.commands)

			input.CommandID = uuid.New()
			result, err := handler.Submit(context.Background(), input)
			require.NoError(t, err)
			if tt.wantOutcome == SharePreviewActive {
				require.Equal(t, model.ShareRequestApproved, result.Request.Status)
			} else {
				require.Equal(t, model.ShareRequestPending, result.Request.Status)
			}
		})
	}
}

func TestSharePreviewRequiresAuthoritativeShareAccess(t *testing.T) {
	organizationID, workspaceID, actorID := uuid.New(), uuid.New(), uuid.New()
	namespace := &model.Namespace{
		ID: uuid.New(), NamespaceType: model.NamespaceTypeWorkspace,
		OrganizationID: organizationID, WorkspaceID: workspaceID,
	}
	node := &model.Node{ID: uuid.New(), NamespaceID: namespace.ID, Type: model.NodeTypeFile}
	store := &shareStoreStub{
		namespace: namespace, node: node,
		classification: model.TransferClassificationInternal,
	}
	handler := NewShareRequestHandler(
		store, postgres.NewDirectTransactor(), nil,
		principalScopeStub{}, shareAuthorityStub{}, &shareSourceAuthorizerStub{allowed: false},
	)

	_, err := handler.Preview(context.Background(), SubmitShare{
		CommandID: uuid.New(), ActorID: actorID, OrganizationID: organizationID,
		WorkspaceID: workspaceID, NodeID: node.ID, PrincipalType: model.PrincipalUser,
		PrincipalIDs: []uuid.UUID{actorID}, Actions: []model.ResourcePermission{model.ResourcePermissionRead},
	})

	require.ErrorIs(t, err, domain.ErrResourceDenied)
	require.Empty(t, store.requests)
	require.Empty(t, store.grants)
}

func TestSubmitShareRequiresAuthoritativeSourceSharePermission(t *testing.T) {
	organizationID, workspaceID := uuid.New(), uuid.New()
	namespace := &model.Namespace{
		ID: uuid.New(), NamespaceType: model.NamespaceTypeWorkspace,
		OrganizationID: organizationID, WorkspaceID: workspaceID,
	}
	node := &model.Node{ID: uuid.New(), NamespaceID: namespace.ID, Type: model.NodeTypeFile}
	store := &shareStoreStub{namespace: namespace, node: node, classification: model.TransferClassificationInternal}
	source := &shareSourceAuthorizerStub{}
	target := uuid.New()
	handler := NewShareRequestHandler(store, postgres.NewDirectTransactor(), nil, principalScopeStub{
		scopes: map[uuid.UUID]PrincipalScope{target: {OrganizationID: organizationID}},
	}, shareAuthorityStub{}, source)

	_, err := handler.Submit(context.Background(), SubmitShare{
		CommandID: uuid.New(), ActorID: uuid.New(), OrganizationID: organizationID, WorkspaceID: workspaceID,
		NodeID: node.ID, PrincipalType: model.PrincipalUser, PrincipalIDs: []uuid.UUID{target},
		Actions: []model.ResourcePermission{model.ResourcePermissionRead},
	})
	require.ErrorIs(t, err, domain.ErrResourceDenied)
	require.Equal(t, 1, source.called)
	require.Empty(t, store.requests)
	require.Empty(t, store.grants)
}

func TestSubmitShareAllowsOwnerAndNamespaceAdminWhenAuthorizerAllowsShare(t *testing.T) {
	for _, name := range []string{"personal owner", "namespace admin"} {
		t.Run(name, func(t *testing.T) {
			organizationID, actorID, targetID := uuid.New(), uuid.New(), uuid.New()
			namespace := &model.Namespace{
				ID: uuid.New(), NamespaceType: model.NamespaceTypeUser,
				OrganizationID: organizationID, OwnerUserID: actorID,
			}
			store := &shareStoreStub{
				namespace:      namespace,
				node:           &model.Node{ID: uuid.New(), NamespaceID: namespace.ID, Type: model.NodeTypeFile},
				classification: model.TransferClassificationInternal,
			}
			handler := NewShareRequestHandler(store, postgres.NewDirectTransactor(), nil, principalScopeStub{
				scopes: map[uuid.UUID]PrincipalScope{targetID: {OrganizationID: organizationID}},
			}, shareAuthorityStub{}, &shareSourceAuthorizerStub{allowed: true})
			result, err := handler.Submit(context.Background(), SubmitShare{
				CommandID: uuid.New(), ActorID: actorID, OrganizationID: organizationID, WorkspaceID: uuid.New(),
				NodeID: store.node.ID, PrincipalType: model.PrincipalUser, PrincipalIDs: []uuid.UUID{targetID},
				Actions: []model.ResourcePermission{model.ResourcePermissionRead},
			})
			require.NoError(t, err)
			require.Equal(t, model.ShareRequestApproved, result.Request.Status)
		})
	}
}

func TestDirectoryInheritedClassificationFailsClosed(t *testing.T) {
	organizationID, actorID := uuid.New(), uuid.New()
	namespace := &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeUser, OrganizationID: organizationID, OwnerUserID: actorID}
	principalID := organizationID
	tests := []struct {
		name       string
		highest    model.TransferClassification
		wantStatus model.ShareRequestStatus
		wantErr    error
	}{
		{name: "restricted child forbids organization audience", highest: model.TransferClassificationRestricted, wantErr: domain.ErrSharePolicyDenied},
		{name: "confidential child requires approval", highest: model.TransferClassificationConfidential, wantStatus: model.ShareRequestPending},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &shareStoreStub{
				namespace:      namespace,
				node:           &model.Node{ID: uuid.New(), NamespaceID: namespace.ID, Type: model.NodeTypeDirectory},
				classification: model.TransferClassificationInternal, subtree: tt.highest,
			}
			handler := NewShareRequestHandler(store, postgres.NewDirectTransactor(), nil, principalScopeStub{
				scopes: map[uuid.UUID]PrincipalScope{principalID: {OrganizationID: organizationID}},
			}, shareAuthorityStub{orgAdmin: true}, &shareSourceAuthorizerStub{allowed: true})
			result, err := handler.Submit(context.Background(), SubmitShare{
				CommandID: uuid.New(), ActorID: actorID, OrganizationID: organizationID, WorkspaceID: uuid.New(),
				NodeID: store.node.ID, PrincipalType: model.PrincipalOrganization, PrincipalIDs: []uuid.UUID{principalID},
				Actions: []model.ResourcePermission{model.ResourcePermissionRead}, InheritToChildren: true,
			})
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantStatus, result.Request.Status)
		})
	}
}

func TestApproveAndRejectShareRequestUseCASAndSeparationOfDuties(t *testing.T) {
	organizationID, workspaceID := uuid.New(), uuid.New()
	requester, approver, principalID := uuid.New(), uuid.New(), uuid.New()
	store := &shareStoreStub{requests: map[uuid.UUID]model.ShareRequest{}}
	request := model.ShareRequest{
		ID: uuid.New(), OrganizationID: organizationID, NamespaceID: uuid.New(), NodeID: uuid.New(),
		RequestedBy: requester, PrincipalType: model.PrincipalWorkspace, PrincipalIDs: []uuid.UUID{principalID},
		Actions: []model.ResourcePermission{model.ResourcePermissionRead}, Status: model.ShareRequestPending,
		Classification: model.TransferClassificationConfidential,
	}
	store.requests[request.ID] = request
	handler := NewShareRequestHandler(store, postgres.NewDirectTransactor(), nil, principalScopeStub{
		scopes: map[uuid.UUID]PrincipalScope{principalID: {OrganizationID: organizationID, WorkspaceID: workspaceID}},
	}, shareAuthorityStub{workspaceAdmin: true}, &shareSourceAuthorizerStub{allowed: true})

	_, err := handler.Approve(context.Background(), ReviewShareRequest{CommandID: uuid.New(), ActorID: requester, RequestID: request.ID})
	require.ErrorIs(t, err, domain.ErrShareSelfApproval)
	require.Empty(t, store.grants)

	writeRequest := request
	writeRequest.ID = uuid.New()
	writeRequest.Classification = model.TransferClassificationInternal
	writeRequest.Actions = []model.ResourcePermission{model.ResourcePermissionWrite}
	store.requests[writeRequest.ID] = writeRequest
	_, err = handler.Approve(context.Background(), ReviewShareRequest{CommandID: uuid.New(), ActorID: requester, RequestID: writeRequest.ID})
	require.ErrorIs(t, err, domain.ErrShareSelfApproval)

	personalWorkspace := request
	personalWorkspace.ID = uuid.New()
	personalWorkspace.Classification = model.TransferClassificationInternal
	store.requests[personalWorkspace.ID] = personalWorkspace
	_, err = handler.Approve(context.Background(), ReviewShareRequest{CommandID: uuid.New(), ActorID: requester, RequestID: personalWorkspace.ID})
	require.ErrorIs(t, err, domain.ErrShareSelfApproval)

	commandID := uuid.New()
	approved, err := handler.Approve(context.Background(), ReviewShareRequest{CommandID: commandID, ActorID: approver, RequestID: request.ID})
	require.NoError(t, err)
	require.Equal(t, model.ShareRequestApproved, approved.Status)
	require.Len(t, store.grants, 1)

	replayed, err := handler.Approve(context.Background(), ReviewShareRequest{CommandID: commandID, ActorID: approver, RequestID: request.ID})
	require.NoError(t, err)
	require.Equal(t, approved, replayed)
	require.Len(t, store.grants, 1)

	_, err = handler.Approve(context.Background(), ReviewShareRequest{CommandID: uuid.New(), ActorID: approver, RequestID: request.ID})
	require.ErrorIs(t, err, domain.ErrShareRequestConflict)

	rejectRequest := request
	rejectRequest.ID = uuid.New()
	rejectRequest.Classification = model.TransferClassificationInternal
	store.requests[rejectRequest.ID] = rejectRequest
	for _, reason := range []string{"   ", strings.Repeat("x", 1001), strings.Repeat("界", 1001)} {
		_, err = handler.Reject(context.Background(), ReviewShareRequest{
			CommandID: uuid.New(), ActorID: approver, RequestID: rejectRequest.ID, RejectionReason: reason,
		})
		require.ErrorIs(t, err, domain.ErrSharePolicyDenied)
	}
	unicodeRequest := rejectRequest
	unicodeRequest.ID = uuid.New()
	store.requests[unicodeRequest.ID] = unicodeRequest
	unicodeRejected, err := handler.Reject(context.Background(), ReviewShareRequest{
		CommandID: uuid.New(), ActorID: approver, RequestID: unicodeRequest.ID, RejectionReason: strings.Repeat("界", 1000),
	})
	require.NoError(t, err)
	require.Equal(t, strings.Repeat("界", 1000), unicodeRejected.RejectionReason)
	rejected, err := handler.Reject(context.Background(), ReviewShareRequest{
		CommandID: uuid.New(), ActorID: approver, RequestID: rejectRequest.ID, RejectionReason: "  not appropriate  ",
	})
	require.NoError(t, err)
	require.Equal(t, model.ShareRequestRejected, rejected.Status)
	require.Equal(t, "not appropriate", rejected.RejectionReason)
	require.Len(t, store.grants, 1, "rejection must not create grants")
}

func TestSubmitShareReplaysSameCommandAndRejectsDifferentPayload(t *testing.T) {
	organizationID, actorID, userID := uuid.New(), uuid.New(), uuid.New()
	namespace := &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeUser, OrganizationID: organizationID, OwnerUserID: actorID}
	node := &model.Node{ID: uuid.New(), NamespaceID: namespace.ID}
	store := &shareStoreStub{namespace: namespace, node: node, classification: model.TransferClassificationInternal}
	handler := NewShareRequestHandler(store, postgres.NewDirectTransactor(), nil, principalScopeStub{
		scopes: map[uuid.UUID]PrincipalScope{userID: {OrganizationID: organizationID}},
	}, shareAuthorityStub{}, &shareSourceAuthorizerStub{allowed: true})
	cmd := SubmitShare{
		CommandID: uuid.New(), ActorID: actorID, OrganizationID: organizationID, WorkspaceID: uuid.New(),
		NodeID: node.ID, PrincipalType: model.PrincipalUser, PrincipalIDs: []uuid.UUID{userID},
		Actions: []model.ResourcePermission{model.ResourcePermissionRead}, InheritToChildren: true,
	}
	first, err := handler.Submit(context.Background(), cmd)
	require.NoError(t, err)
	second, err := handler.Submit(context.Background(), cmd)
	require.NoError(t, err)
	require.Equal(t, first.Request.ID, second.Request.ID)
	require.Len(t, store.grants, 1)

	cmd.Actions = []model.ResourcePermission{model.ResourcePermissionDownload}
	_, err = handler.Submit(context.Background(), cmd)
	require.ErrorIs(t, err, repository.ErrIdempotencyConflict)
}

func TestApproveShareRequestCreatesGrantsStatusAndOutboxTransactionally(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	require.NoError(t, persistence.AutoMigrate(db))
	require.NoError(t, messagingpersistence.AutoMigrate(db))
	store := persistence.NewRepository(db)
	requester, approver := uuid.New(), uuid.New()
	namespace := &model.Namespace{
		ID: uuid.New(), NamespaceType: model.NamespaceTypeUser, OrganizationID: uuid.New(),
		OwnerUserID: requester, Status: "ACTIVE",
	}
	require.NoError(t, store.CreateNamespace(context.Background(), namespace))
	node, err := model.NewRootDirectory(namespace.ID, requester, ".root")
	require.NoError(t, err)
	require.NoError(t, store.CreateNode(context.Background(), node))
	workspaceID := uuid.New()
	scopes := principalScopeStub{scopes: map[uuid.UUID]PrincipalScope{
		workspaceID: {OrganizationID: namespace.OrganizationID, WorkspaceID: workspaceID},
	}}
	registry := sharedevents.NewRegistry()
	require.NoError(t, filesystemevent.Register(registry))
	bus := messaging.NewReliableBus(registry, messagingpersistence.NewOutboxStore(db), nil, nil, nil)
	tx := postgres.NewTransactor(db)
	submitter := NewShareRequestHandler(store, tx, bus, scopes, shareAuthorityStub{}, &shareSourceAuthorizerStub{allowed: true})
	result, err := submitter.Submit(context.Background(), SubmitShare{
		CommandID: uuid.New(), ActorID: requester, OrganizationID: namespace.OrganizationID, WorkspaceID: workspaceID,
		NodeID: node.ID, PrincipalType: model.PrincipalWorkspace, PrincipalIDs: []uuid.UUID{workspaceID},
		Actions: []model.ResourcePermission{model.ResourcePermissionRead}, InheritToChildren: true,
	})
	require.NoError(t, err)
	require.Equal(t, model.ShareRequestPending, result.Request.Status)

	approverHandler := NewShareRequestHandler(store, tx, bus, scopes, shareAuthorityStub{workspaceAdmin: true}, &shareSourceAuthorizerStub{allowed: true})
	approved, err := approverHandler.Approve(context.Background(), ReviewShareRequest{
		CommandID: uuid.New(), ActorID: approver, OrganizationID: namespace.OrganizationID, RequestID: result.Request.ID,
	})
	require.NoError(t, err)
	require.Equal(t, model.ShareRequestApproved, approved.Status)
	grants, err := store.ListNodeResourceGrants(context.Background(), node.ID)
	require.NoError(t, err)
	require.Len(t, grants, 1)
	var outboxCount int64
	require.NoError(t, db.Table("platform.event_outbox").Where("event_type = ?", filesystemevent.PermissionChangedName).Count(&outboxCount).Error)
	require.EqualValues(t, 1, outboxCount)
}
