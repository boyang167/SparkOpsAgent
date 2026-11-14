package command_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
)

type adoptionTransactor struct{}

func (adoptionTransactor) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type adoptionAuthorizer struct {
	read  bool
	write bool
}

func (a adoptionAuthorizer) Authorize(_ context.Context, request command.AdoptionAuthorizationRequest) bool {
	if request.Permission == model.ResourcePermissionRead {
		return a.read
	}
	return a.write
}

type adoptionAudit struct{ entries []audit.Entry }

func (a *adoptionAudit) Log(_ context.Context, entry audit.Entry) error {
	a.entries = append(a.entries, entry)
	return nil
}

type adoptionStore struct {
	source          *model.Node
	sourceNamespace *model.Namespace
	targetNamespace *model.Namespace
	parent          *model.Node
	revision        *model.FileRevision
	blob            *model.Blob
	replica         *model.StorageReplica
	commands        map[uuid.UUID]repository.CommandDedup
	nodes           map[uuid.UUID]*model.Node
	lineages        []model.FileLineage
	quotaBytes      int64
	quotaFiles      int64
	blobRefAdds     int
}

func newAdoptionStore(t *testing.T) *adoptionStore {
	t.Helper()
	organizationID := uuid.New()
	sourceNamespaceID := uuid.New()
	targetNamespaceID := uuid.New()
	source, err := model.NewFile(sourceNamespaceID, uuid.New(), "源文件.txt", uuid.New())
	require.NoError(t, err)
	source.Classification = model.TransferClassificationConfidential
	revisionID, blobID := uuid.New(), uuid.New()
	source.CurrentRevisionID = &revisionID
	revision, err := model.NewRevisionWithID(revisionID, source.ID, 1, blobID, 17, "text/plain", source.OwnerID)
	require.NoError(t, err)
	parent, err := model.NewDirectory(targetNamespaceID, uuid.New(), "制度", uuid.New())
	require.NoError(t, err)
	return &adoptionStore{
		source:          source,
		sourceNamespace: &model.Namespace{ID: sourceNamespaceID, NamespaceType: model.NamespaceTypeUser, OrganizationID: organizationID},
		targetNamespace: &model.Namespace{ID: targetNamespaceID, NamespaceType: model.NamespaceTypeOrganization, OrganizationID: organizationID},
		parent:          parent,
		revision:        revision,
		blob:            &model.Blob{ID: blobID, DedupDomain: organizationID, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Size: 17, RefCount: 1, State: model.BlobStateAvailable},
		replica:         &model.StorageReplica{ID: uuid.New(), BlobID: blobID, ProviderID: uuid.New(), State: model.ReplicaStateAvailable},
		commands:        map[uuid.UUID]repository.CommandDedup{},
		nodes:           map[uuid.UUID]*model.Node{},
	}
}

func (s *adoptionStore) GetNodeByID(_ context.Context, id uuid.UUID) (*model.Node, error) {
	if id == s.source.ID {
		return s.source, nil
	}
	if node := s.nodes[id]; node != nil {
		return node, nil
	}
	return nil, domain.ErrNodeNotFound
}
func (s *adoptionStore) GetNamespace(_ context.Context, id uuid.UUID) (*model.Namespace, error) {
	if id == s.sourceNamespace.ID {
		return s.sourceNamespace, nil
	}
	if id == s.targetNamespace.ID {
		return s.targetNamespace, nil
	}
	return nil, domain.ErrNamespaceNotFound
}
func (s *adoptionStore) GetNamespaceByOrganization(_ context.Context, id uuid.UUID) (*model.Namespace, error) {
	if id == s.targetNamespace.OrganizationID {
		return s.targetNamespace, nil
	}
	return nil, domain.ErrNamespaceNotFound
}
func (s *adoptionStore) GetNode(_ context.Context, namespaceID, nodeID uuid.UUID) (*model.Node, error) {
	if namespaceID == s.targetNamespace.ID && nodeID == s.parent.ID {
		return s.parent, nil
	}
	if node := s.nodes[nodeID]; node != nil && node.NamespaceID == namespaceID {
		return node, nil
	}
	return nil, domain.ErrNodeNotFound
}
func (s *adoptionStore) GetRevision(context.Context, uuid.UUID, uuid.UUID) (*model.FileRevision, error) {
	return s.revision, nil
}
func (s *adoptionStore) GetBlob(context.Context, uuid.UUID) (*model.Blob, error) { return s.blob, nil }
func (s *adoptionStore) GetAvailableReplica(context.Context, uuid.UUID, uuid.UUID) (*model.StorageReplica, error) {
	return s.replica, nil
}
func (s *adoptionStore) CreateNode(_ context.Context, node *model.Node) error {
	for _, existing := range s.nodes {
		if existing.NamespaceID == node.NamespaceID && existing.ParentID != nil && node.ParentID != nil &&
			*existing.ParentID == *node.ParentID && existing.NormalizedName == node.NormalizedName {
			return domain.ErrNodeNameConflict
		}
	}
	copyNode := *node
	s.nodes[node.ID] = &copyNode
	return nil
}
func (s *adoptionStore) SaveRevision(context.Context, *model.FileRevision, *model.Blob, *model.StorageReplica) error {
	return nil
}
func (s *adoptionStore) SetCurrentRevision(_ context.Context, nodeID, revisionID uuid.UUID) error {
	s.nodes[nodeID].CurrentRevisionID = &revisionID
	return nil
}
func (s *adoptionStore) IncrementBlobRef(context.Context, uuid.UUID) error {
	s.blobRefAdds++
	return nil
}
func (s *adoptionStore) ApplyNamespaceQuotaUsage(_ context.Context, _ uuid.UUID, bytes, files int64) error {
	s.quotaBytes += bytes
	s.quotaFiles += files
	return nil
}
func (s *adoptionStore) CreateFileLineage(_ context.Context, lineage model.FileLineage) error {
	for _, existing := range s.lineages {
		if existing.CommandID == lineage.CommandID {
			return nil
		}
	}
	s.lineages = append(s.lineages, lineage)
	return nil
}
func (s *adoptionStore) ClaimCommand(_ context.Context, input repository.CommandDedup) (*repository.CommandDedup, error) {
	if existing, ok := s.commands[input.CommandID]; ok {
		if existing.RequestHash != input.RequestHash {
			return nil, domain.ErrIdempotencyConflict
		}
		copyExisting := existing
		return &copyExisting, nil
	}
	input.State = "IN_PROGRESS"
	s.commands[input.CommandID] = input
	copyInput := input
	return &copyInput, nil
}
func (s *adoptionStore) CompleteCommand(_ context.Context, id uuid.UUID, nodeID, revisionID *uuid.UUID) error {
	item := s.commands[id]
	item.State, item.NodeID, item.RevisionID = "COMPLETED", nodeID, revisionID
	s.commands[id] = item
	return nil
}

func adoptionCommand(store *adoptionStore) command.AdoptFile {
	return command.AdoptFile{
		CommandID: uuid.New(), ActorID: uuid.New(), OrganizationID: store.targetNamespace.OrganizationID,
		WorkspaceID: uuid.New(), SourceNodeID: store.source.ID, TargetParentID: store.parent.ID,
		Name: "租户制度.txt", TraceID: "trace-adopt-1",
	}
}

func TestAdoptFileCreatesIndependentOrganizationFileAndLineageExactlyOnce(t *testing.T) {
	store := newAdoptionStore(t)
	logger := &adoptionAudit{}
	handler := command.NewAdoptFileHandler(store, adoptionTransactor{}, adoptionAuthorizer{read: true, write: true}, logger)
	cmd := adoptionCommand(store)

	first, err := handler.Handle(context.Background(), cmd)
	require.NoError(t, err)
	second, err := handler.Handle(context.Background(), cmd)
	require.NoError(t, err)

	require.Equal(t, first.ID, second.ID)
	require.Equal(t, store.targetNamespace.ID, first.NamespaceID)
	require.Equal(t, store.source.Classification, first.Classification)
	require.Equal(t, cmd.ActorID, first.OwnerID)
	require.Equal(t, int64(17), store.quotaBytes)
	require.Equal(t, int64(1), store.quotaFiles)
	require.Equal(t, 1, store.blobRefAdds)
	require.Len(t, store.lineages, 1)
	require.Equal(t, store.source.ID, store.lineages[0].SourceNodeID)
	require.Equal(t, first.ID, store.lineages[0].TargetNodeID)
	require.Len(t, logger.entries, 1)
	require.Equal(t, "filesystem.file.adopted", logger.entries[0].Action)
	require.Equal(t, store.targetNamespace.ID.String(), logger.entries[0].Detail["namespace_id"])
	require.Equal(t, first.ID.String(), logger.entries[0].Detail["node_id"])
	require.Equal(t, "trace-adopt-1", logger.entries[0].Detail["trace_id"])
}

func TestAdoptFileEnforcesFileBoundaryPermissionsAndOrganization(t *testing.T) {
	tests := []struct {
		name      string
		prepare   func(*adoptionStore)
		authority adoptionAuthorizer
		want      error
	}{
		{name: "source read required", authority: adoptionAuthorizer{write: true}, want: domain.ErrResourceDenied},
		{name: "target write required", authority: adoptionAuthorizer{read: true}, want: domain.ErrResourceDenied},
		{name: "cross organization rejected", authority: adoptionAuthorizer{read: true, write: true}, prepare: func(s *adoptionStore) {
			s.sourceNamespace.OrganizationID = uuid.New()
		}, want: domain.ErrCrossOrganizationGrant},
		{name: "directories rejected", authority: adoptionAuthorizer{read: true, write: true}, prepare: func(s *adoptionStore) {
			s.source.Type = model.NodeTypeDirectory
		}, want: domain.ErrUnsupportedNodeType},
		{name: "target parent must be active directory", authority: adoptionAuthorizer{read: true, write: true}, prepare: func(s *adoptionStore) {
			s.parent.State = model.NodeStateTrashed
		}, want: domain.ErrParentNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newAdoptionStore(t)
			if tt.prepare != nil {
				tt.prepare(store)
			}
			logger := &adoptionAudit{}
			_, err := command.NewAdoptFileHandler(store, adoptionTransactor{}, tt.authority, logger).
				Handle(context.Background(), adoptionCommand(store))
			require.ErrorIs(t, err, tt.want)
			require.Empty(t, logger.entries)
			require.Empty(t, store.lineages)
			require.Zero(t, store.quotaFiles)
		})
	}
}

func TestAdoptFileRejectsNameConflictWithoutCopyingSourceGrants(t *testing.T) {
	store := newAdoptionStore(t)
	cmd := adoptionCommand(store)
	existing, err := model.NewFile(store.targetNamespace.ID, uuid.New(), cmd.Name, store.parent.ID)
	require.NoError(t, err)
	store.nodes[existing.ID] = existing
	_, err = command.NewAdoptFileHandler(store, adoptionTransactor{}, adoptionAuthorizer{read: true, write: true}, &adoptionAudit{}).
		Handle(context.Background(), cmd)
	require.True(t, errors.Is(err, domain.ErrNodeNameConflict))
	require.Zero(t, store.blobRefAdds)
	require.Empty(t, store.lineages)
}
