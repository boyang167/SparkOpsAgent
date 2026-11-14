package command

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type artifactRepoStub struct {
	namespace *model.Namespace
	node      *model.Node
	created   bool
	artifact  model.Artifact
	link      model.FileLink
}

type artifactTransactor struct{}

func (artifactTransactor) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type recordingBus struct {
	published []events.Event
}

func (b *recordingBus) Publish(_ context.Context, ev ...events.Event) error {
	b.published = append(b.published, ev...)
	return nil
}

func (b *recordingBus) Subscribe(string, string, messaging.Handler) error { return nil }

func (s *artifactRepoStub) GetNamespaceByWorkspace(_ context.Context, _ uuid.UUID) (*model.Namespace, error) {
	return s.namespace, nil
}

func (s *artifactRepoStub) GetNodeByID(_ context.Context, _ uuid.UUID) (*model.Node, error) {
	return s.node, nil
}

func (s *artifactRepoStub) CreateArtifact(_ context.Context, artifact model.Artifact, link model.FileLink) error {
	s.created = true
	s.artifact = artifact
	s.link = link
	return nil
}

func TestArtifactCannotReferenceFileFromAnotherWorkspace(t *testing.T) {
	workspaceA := uuid.New()
	org := uuid.New()
	namespaceA := uuid.New()
	namespaceB := uuid.New()
	fileID := uuid.New()
	stub := &artifactRepoStub{
		namespace: &model.Namespace{ID: namespaceA, NamespaceType: model.NamespaceTypeWorkspace, WorkspaceID: workspaceA, OrganizationID: org, Status: "ACTIVE"},
		node:      &model.Node{ID: fileID, NamespaceID: namespaceB, Type: model.NodeTypeFile, State: model.NodeStateActive},
	}
	handler := NewCreateArtifactHandlerWithRepo(stub, nil, nil)
	_, err := handler.Handle(context.Background(), CreateArtifact{
		CommandID:      uuid.New(),
		ActorID:        uuid.New(),
		OrganizationID: org,
		WorkspaceID:    workspaceA,
		ArtifactType:   "output",
		ProducerType:   "agent_run",
		ProducerID:     uuid.New(),
		RunID:          uuid.New(),
		FileID:         fileID,
	})
	require.ErrorIs(t, err, domain.ErrCrossWorkspaceReference)
	require.False(t, stub.created)
}

func TestCreateArtifactWritesFileLinkAndVersionedEvent(t *testing.T) {
	organizationID := uuid.New()
	workspaceID := uuid.New()
	namespaceID := uuid.New()
	fileID := uuid.New()
	revisionID := uuid.New()
	runID := uuid.New()
	stub := &artifactRepoStub{
		namespace: &model.Namespace{ID: namespaceID, NamespaceType: model.NamespaceTypeWorkspace, OrganizationID: organizationID, WorkspaceID: workspaceID, Status: "ACTIVE"},
		node: &model.Node{
			ID: fileID, NamespaceID: namespaceID, Type: model.NodeTypeFile,
			CurrentRevisionID: &revisionID, State: model.NodeStateActive,
		},
	}
	bus := &recordingBus{}
	handler := NewCreateArtifactHandlerWithRepo(stub, artifactTransactor{}, bus)
	artifact, err := handler.Handle(context.Background(), CreateArtifact{
		CommandID: uuid.New(), ActorID: uuid.New(), OrganizationID: organizationID,
		WorkspaceID: workspaceID, ArtifactType: "output", ProducerType: "agent_run",
		ProducerID: uuid.New(), RunID: runID, FileID: fileID,
	})
	require.NoError(t, err)
	require.Equal(t, fileID, artifact.FileID)
	require.Equal(t, model.FileLinkTypeArtifact, stub.link.LinkType)
	require.Equal(t, artifact.ID, stub.link.LinkID)
	require.Equal(t, fileID, stub.link.NodeID)
	require.Equal(t, &revisionID, stub.link.RevisionID)
	require.Len(t, bus.published, 1)
	created, ok := bus.published[0].(event.ArtifactCreated)
	require.True(t, ok)
	require.Equal(t, event.ArtifactCreatedName, created.EventName())
	require.Equal(t, "filesystem.artifact.created.v1", created.EventName())
	require.Equal(t, artifact.ID, created.ArtifactID)
	require.Equal(t, workspaceID, created.WorkspaceID)
	require.Equal(t, fileID, created.FileID)
	require.Equal(t, runID, created.RunID)
}
