package command

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

type artifactRepoStub struct {
	namespace *model.Namespace
	node      *model.Node
	created   bool
}

func (s *artifactRepoStub) GetNamespaceByWorkspace(_ context.Context, _ uuid.UUID) (*model.Namespace, error) {
	return s.namespace, nil
}

func (s *artifactRepoStub) GetNodeByID(_ context.Context, _ uuid.UUID) (*model.Node, error) {
	return s.node, nil
}

func (s *artifactRepoStub) CreateArtifact(_ context.Context, _ model.Artifact, _ model.FileLink) error {
	s.created = true
	return nil
}

func TestArtifactCannotReferenceFileFromAnotherWorkspace(t *testing.T) {
	workspaceA := uuid.New()
	org := uuid.New()
	namespaceA := uuid.New()
	namespaceB := uuid.New()
	fileID := uuid.New()
	stub := &artifactRepoStub{
		namespace: &model.Namespace{ID: namespaceA, WorkspaceID: workspaceA, OrganizationID: org, Status: "ACTIVE"},
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
