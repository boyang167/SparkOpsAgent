package command

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	filesystemevent "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

type persistentAdoptionAuthorizer struct{ allowed bool }

func (a persistentAdoptionAuthorizer) Authorize(context.Context, AdoptionAuthorizationRequest) bool {
	return a.allowed
}

type failingAdoptionAudit struct{}

func (failingAdoptionAudit) Log(context.Context, audit.Entry) error {
	return errors.New("audit unavailable")
}

func TestAdoptFilePersistsVisibleChildQuotaLineageAuditAndOutboxAtomically(t *testing.T) {
	db, repo, sourceNamespace := newPutTestRepository(t)
	bus := newFilesystemOutboxBus(t, db)
	require.NoError(t, audit.NewGormStore(db).AutoMigrate())
	auditService := audit.NewService(audit.NewGormStore(db))
	targetNamespace, targetRoot, source := seedPersistentAdoption(t, repo.(*persistence.Repository), sourceNamespace)
	cmd := AdoptFile{
		CommandID: uuid.New(), ActorID: uuid.New(), OrganizationID: sourceNamespace.OrganizationID,
		WorkspaceID: sourceNamespace.WorkspaceID, SourceNodeID: source.ID,
		TargetParentID: targetRoot.ID, Name: "adopted.txt", TraceID: "trace-persistent-adoption",
	}
	handler := NewAdoptFileHandler(repo.(*persistence.Repository), postgres.NewTransactor(db), persistentAdoptionAuthorizer{allowed: true}, auditService).
		WithBus(bus)

	adopted, err := handler.Handle(context.Background(), cmd)
	require.NoError(t, err)
	replayed, err := NewAdoptFileHandler(repo.(*persistence.Repository), postgres.NewTransactor(db), persistentAdoptionAuthorizer{}, auditService).
		WithBus(bus).Handle(context.Background(), cmd)
	require.NoError(t, err)
	require.Equal(t, adopted.ID, replayed.ID)

	children, _, err := repo.ListChildren(context.Background(), targetNamespace.ID, &targetRoot.ID, nil, 100, "")
	require.NoError(t, err)
	require.Len(t, children, 1)
	require.Equal(t, adopted.ID, children[0].ID)

	var quota struct {
		UsedBytes int64
		UsedFiles int64
	}
	require.NoError(t, db.Table("filesystem.quota_accounts").Where("namespace_id = ?", targetNamespace.ID).Take(&quota).Error)
	require.Equal(t, int64(17), quota.UsedBytes)
	require.Equal(t, int64(1), quota.UsedFiles)
	for table, want := range map[string]int64{
		"filesystem.file_lineage": 1,
		"platform.audit_logs":     1,
	} {
		var count int64
		require.NoError(t, db.Table(table).Count(&count).Error)
		require.Equal(t, want, count, table)
	}
	for _, eventType := range []string{filesystemevent.FileCreatedName, filesystemevent.FileRevisionCreatedName} {
		var count int64
		require.NoError(t, db.Table("platform.event_outbox").Where("event_type = ?", eventType).Count(&count).Error)
		require.Equal(t, int64(1), count, eventType)
	}
	var grantCount int64
	require.NoError(t, db.Table("filesystem.resource_grants").Where("node_id = ?", adopted.ID).Count(&grantCount).Error)
	require.Zero(t, grantCount)
}

func TestAdoptFileRollsBackNodeQuotaLineageCommandAndOutboxWhenAuditFails(t *testing.T) {
	db, repo, sourceNamespace := newPutTestRepository(t)
	bus := newFilesystemOutboxBus(t, db)
	targetNamespace, targetRoot, source := seedPersistentAdoption(t, repo.(*persistence.Repository), sourceNamespace)
	cmd := AdoptFile{
		CommandID: uuid.New(), ActorID: uuid.New(), OrganizationID: sourceNamespace.OrganizationID,
		WorkspaceID: sourceNamespace.WorkspaceID, SourceNodeID: source.ID,
		TargetParentID: targetRoot.ID, Name: "rolled-back.txt",
	}
	_, err := NewAdoptFileHandler(repo.(*persistence.Repository), postgres.NewTransactor(db), persistentAdoptionAuthorizer{allowed: true}, failingAdoptionAudit{}).
		WithBus(bus).Handle(context.Background(), cmd)
	require.ErrorContains(t, err, "audit unavailable")

	var quota struct {
		UsedBytes int64
		UsedFiles int64
	}
	require.NoError(t, db.Table("filesystem.quota_accounts").Where("namespace_id = ?", targetNamespace.ID).Take(&quota).Error)
	require.Zero(t, quota.UsedBytes)
	require.Zero(t, quota.UsedFiles)
	for _, table := range []string{"filesystem.file_lineage", "filesystem.command_dedup", "platform.event_outbox"} {
		var count int64
		require.NoError(t, db.Table(table).Count(&count).Error)
		require.Zero(t, count, table)
	}
	var nodeCount int64
	require.NoError(t, db.Table("filesystem.nodes").Where("namespace_id = ? AND normalized_name = ?", targetNamespace.ID, "rolled-back.txt").Count(&nodeCount).Error)
	require.Zero(t, nodeCount)
}

func TestAdoptFileConcurrentSameCommandClaimsOnce(t *testing.T) {
	db, repo, sourceNamespace := newPutTestRepository(t)
	bus := newFilesystemOutboxBus(t, db)
	targetNamespace, targetRoot, source := seedPersistentAdoption(t, repo.(*persistence.Repository), sourceNamespace)
	cmd := AdoptFile{
		CommandID: uuid.New(), ActorID: uuid.New(), OrganizationID: sourceNamespace.OrganizationID,
		WorkspaceID: sourceNamespace.WorkspaceID, SourceNodeID: source.ID,
		TargetParentID: targetRoot.ID, Name: "concurrent.txt",
	}
	handler := NewAdoptFileHandler(repo.(*persistence.Repository), postgres.NewTransactor(db), persistentAdoptionAuthorizer{allowed: true}, nil).
		WithBus(bus)
	start := make(chan struct{})
	results := make(chan *model.Node, 2)
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			node, err := handler.Handle(context.Background(), cmd)
			results <- node
			errs <- err
		}()
	}
	close(start)
	first, second := <-results, <-results
	require.NoError(t, <-errs)
	require.NoError(t, <-errs)
	require.NotNil(t, first)
	require.NotNil(t, second)
	require.Equal(t, first.ID, second.ID)

	for table, want := range map[string]int64{
		"filesystem.file_lineage": 1,
		"filesystem.command_dedup": 1,
	} {
		var count int64
		require.NoError(t, db.Table(table).Count(&count).Error)
		require.Equal(t, want, count, table)
	}
	var quota struct {
		UsedBytes int64
		UsedFiles int64
	}
	require.NoError(t, db.Table("filesystem.quota_accounts").Where("namespace_id = ?", targetNamespace.ID).Take(&quota).Error)
	require.Equal(t, int64(17), quota.UsedBytes)
	require.Equal(t, int64(1), quota.UsedFiles)
}

func seedPersistentAdoption(
	t *testing.T,
	repo *persistence.Repository,
	sourceNamespace *model.Namespace,
) (*model.Namespace, *model.Node, *model.Node) {
	t.Helper()
	sourceRoot, err := repo.GetNode(context.Background(), sourceNamespace.ID, *sourceNamespace.RootNodeID)
	require.NoError(t, err)
	source, err := model.NewFile(sourceNamespace.ID, uuid.New(), "source.txt", sourceRoot.ID)
	require.NoError(t, err)
	source.Classification = model.TransferClassificationConfidential
	require.NoError(t, repo.CreateNode(context.Background(), source))
	blob := &model.Blob{
		ID: uuid.New(), DedupDomain: sourceNamespace.OrganizationID,
		SHA256: strings.Repeat("a", 64), Size: 17, RefCount: 1, State: model.BlobStateAvailable,
	}
	require.NoError(t, repo.InsertBlob(context.Background(), blob))
	verifiedAt := time.Now().UTC()
	replica := &model.StorageReplica{
		ID: uuid.New(), BlobID: blob.ID, ProviderID: uuid.New(), ObjectKey: "source/object",
		State: model.ReplicaStateAvailable, VerifiedAt: &verifiedAt,
	}
	require.NoError(t, repo.InsertReplica(context.Background(), replica))
	revision, err := model.NewRevisionWithID(uuid.New(), source.ID, 1, blob.ID, 17, "text/plain", source.OwnerID)
	require.NoError(t, err)
	require.NoError(t, repo.SaveRevision(context.Background(), revision, blob, replica))
	require.NoError(t, repo.SetCurrentRevision(context.Background(), source.ID, revision.ID()))

	targetNamespace := &model.Namespace{
		ID: uuid.New(), NamespaceType: model.NamespaceTypeOrganization,
		OrganizationID: sourceNamespace.OrganizationID, Status: "ACTIVE",
	}
	require.NoError(t, repo.CreateNamespace(context.Background(), targetNamespace))
	targetRoot, err := model.NewRootDirectory(targetNamespace.ID, uuid.New(), ".root")
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(context.Background(), targetRoot))
	require.NoError(t, repo.SetNamespaceRootByID(context.Background(), targetNamespace.ID, targetRoot.ID))
	targetNamespace.RootNodeID = &targetRoot.ID
	return targetNamespace, targetRoot, source
}
