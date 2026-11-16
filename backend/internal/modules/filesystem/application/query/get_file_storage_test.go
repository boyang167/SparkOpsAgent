package query

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

type storageLookupRepo struct {
	node     *model.Node
	revision *model.FileRevision
	blob     *model.Blob
	replica  *model.StorageReplica
	provider *model.StorageProvider
}

func (r storageLookupRepo) GetNode(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error) {
	return r.node, nil
}
func (r storageLookupRepo) GetRevision(context.Context, uuid.UUID, uuid.UUID) (*model.FileRevision, error) {
	return r.revision, nil
}
func (r storageLookupRepo) GetBlob(context.Context, uuid.UUID) (*model.Blob, error) {
	return r.blob, nil
}
func (r storageLookupRepo) GetPreferredReplica(context.Context, uuid.UUID) (*model.StorageReplica, error) {
	return r.replica, nil
}
func (r storageLookupRepo) GetStorageProvider(context.Context, uuid.UUID) (*model.StorageProvider, error) {
	return r.provider, nil
}

func TestGetFileStorageResolvesLocalObjectPath(t *testing.T) {
	nsID := uuid.New()
	nodeID := uuid.New()
	revID := uuid.New()
	blobID := uuid.New()
	providerID := uuid.New()
	node, err := model.NewFile(nsID, uuid.New(), "kubeconfig.yaml", uuid.New())
	require.NoError(t, err)
	node.ID = nodeID
	revision, err := model.NewRevision(nodeID, 1, blobID, 42, "application/yaml", uuid.New())
	require.NoError(t, err)
	_ = revID
	blob := &model.Blob{ID: blobID, SHA256: "abc123"}
	replica := &model.StorageReplica{
		ID: uuid.New(), BlobID: blobID, ProviderID: providerID,
		ObjectKey: "transfers/demo/content", State: model.ReplicaStateAvailable,
	}
	provider, err := model.NewStorageProvider(model.StorageProviderInput{
		ID: providerID, OrganizationID: uuid.New(), Name: "local-dev",
		ProviderType: model.ProviderTypeLocal, Endpoint: `C:\data\filesystem`, SecretRef: "ref",
	})
	require.NoError(t, err)

	view, err := (&GetFileStorageHandler{repo: storageLookupRepo{
		node:     node,
		revision: revision,
		blob:     blob,
		replica:  replica,
		provider: provider,
	}}).Handle(context.Background(), GetFileStorage{
		NamespaceID: nsID, NodeID: nodeID, RevisionID: &revID,
	})

	require.NoError(t, err)
	require.Equal(t, "transfers/demo/content", view.ObjectKey)
	require.Equal(t, model.ProviderTypeLocal, view.ProviderType)
	require.Contains(t, view.LocalObjectFile, `C:\data\filesystem`)
	require.Contains(t, view.LocalObjectFile, ".object")
}

func TestGetFileStorageRejectsDirectoryNodes(t *testing.T) {
	nsID := uuid.New()
	dir, err := model.NewDirectory(nsID, uuid.New(), "folder", uuid.New())
	require.NoError(t, err)
	_, err = (&GetFileStorageHandler{repo: storageLookupRepo{node: dir}}).Handle(context.Background(), GetFileStorage{
		NamespaceID: nsID, NodeID: dir.ID,
	})
	require.ErrorIs(t, err, domain.ErrUnsupportedNodeType)
}