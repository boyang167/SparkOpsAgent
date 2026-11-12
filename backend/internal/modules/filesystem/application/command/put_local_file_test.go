package command

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/adapters/local"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type failingBus struct{}

func (failingBus) Publish(context.Context, ...events.Event) error {
	return errors.New("publish failed")
}

func (failingBus) Subscribe(string, string, messaging.Handler) error {
	return nil
}

type commitUnknownTransactor struct {
	inner postgres.Transactor
}

func (t commitUnknownTransactor) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	if err := t.inner.WithinTransaction(ctx, fn); err != nil {
		return err
	}
	return fmt.Errorf("%w: injected after commit", postgres.ErrCommitOutcomeUnknown)
}

type missingCommitUnknownTransactor struct{}

func (missingCommitUnknownTransactor) WithinTransaction(context.Context, func(context.Context) error) error {
	return fmt.Errorf("%w: injected before callback", postgres.ErrCommitOutcomeUnknown)
}

type commandReadOverrideRepository struct {
	repository.FilesystemRepository
	getCommand func(context.Context, uuid.UUID) (*repository.CommandDedup, error)
	calls      int
}

func (r *commandReadOverrideRepository) GetCommand(ctx context.Context, id uuid.UUID) (*repository.CommandDedup, error) {
	r.calls++
	if r.calls > 1 && r.getCommand != nil {
		return r.getCommand(ctx, id)
	}
	return r.FilesystemRepository.GetCommand(ctx, id)
}

type deleteTrackingProvider struct {
	storage.Provider
	deletes int
}

func (p *deleteTrackingProvider) Delete(ctx context.Context, key string) error {
	p.deletes++
	return p.Provider.Delete(ctx, key)
}

func TestPutLocalFileRollsBackWhenEventPublishFails(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	require.NoError(t, persistence.AutoMigrate(db))
	repo := persistence.NewRepository(db)
	ns := &model.Namespace{ID: uuid.New(), OrganizationID: uuid.New(), WorkspaceID: uuid.New(), Status: "ACTIVE"}
	require.NoError(t, repo.CreateNamespace(context.Background(), ns))
	seedPutNamespaceRoot(t, repo, ns)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	defer provider.Close()

	h := NewPutLocalFileHandler(repo, provider, postgres.NewTransactor(db), failingBus{}, nil)
	cmd := PutLocalFile{
		CommandID: uuid.New(), ActorID: uuid.New(), OrganizationID: ns.OrganizationID,
		WorkspaceID: ns.WorkspaceID, NamespaceID: ns.ID, Name: "fixture.txt",
		MIMEType: "text/plain", Size: int64(len("fixture-content")),
		SHA256: storage.SHA256([]byte("fixture-content")),
		Body:   bytes.NewBufferString("fixture-content"), Authorization: Authorization{Allowed: true},
		ReplicaProviderID: uuid.New(),
	}

	_, err = h.Handle(context.Background(), cmd)
	require.Error(t, err)
	_, err = repo.GetNode(context.Background(), ns.ID, uuid.Nil)
	require.ErrorIs(t, err, domain.ErrNodeNotFound)

	var nodes, revisions, blobs int64
	require.NoError(t, db.Table("filesystem.nodes").Where("namespace_id = ?", ns.ID).Count(&nodes).Error)
	require.NoError(t, db.Table("filesystem.file_revisions").Count(&revisions).Error)
	require.NoError(t, db.Table("filesystem.blobs").Count(&blobs).Error)
	require.Equal(t, int64(1), nodes) // namespace root remains
	require.Zero(t, revisions)
	require.Zero(t, blobs)
}

func TestPutLocalFileCommitOutcomeUnknownReturnsCommittedResultWithoutDeletingObject(t *testing.T) {
	db, repo, ns := newPutTestRepository(t)
	localProvider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = localProvider.Close() })
	provider := &deleteTrackingProvider{Provider: localProvider}
	h := NewPutLocalFileHandler(
		repo,
		provider,
		commitUnknownTransactor{inner: postgres.NewTransactor(db)},
		nil,
		nil,
	)
	cmd := putFixture(ns)

	got, err := h.Handle(context.Background(), cmd)

	require.NoError(t, err)
	require.NotNil(t, got)
	require.Zero(t, provider.deletes)
	_, err = provider.Stat(context.Background(), "sessions/"+cmd.CommandID.String()+"/content")
	require.NoError(t, err)
}

func TestPutLocalFileCommitOutcomeUnknownKeepsObjectForUncertainAuthorityStates(t *testing.T) {
	tests := []struct {
		name string
		read func(context.Context, uuid.UUID) (*repository.CommandDedup, error)
	}{
		{
			name: "authority query fails",
			read: func(context.Context, uuid.UUID) (*repository.CommandDedup, error) {
				return nil, domain.ErrPersistence
			},
		},
		{
			name: "command remains in progress",
			read: func(_ context.Context, id uuid.UUID) (*repository.CommandDedup, error) {
				return &repository.CommandDedup{CommandID: id, RequestHash: "same", State: "IN_PROGRESS"}, nil
			},
		},
		{
			name: "completed hash conflicts",
			read: func(_ context.Context, id uuid.UUID) (*repository.CommandDedup, error) {
				return &repository.CommandDedup{CommandID: id, RequestHash: "different", State: "COMPLETED"}, nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, baseRepo, ns := newPutTestRepository(t)
			localProvider, err := local.New(t.TempDir())
			require.NoError(t, err)
			t.Cleanup(func() { _ = localProvider.Close() })
			provider := &deleteTrackingProvider{Provider: localProvider}
			repo := &commandReadOverrideRepository{FilesystemRepository: baseRepo, getCommand: tt.read}
			h := NewPutLocalFileHandler(repo, provider, commitUnknownTransactor{inner: postgres.NewTransactor(db)}, nil, nil)
			cmd := putFixture(ns)

			_, err = h.Handle(context.Background(), cmd)

			require.ErrorIs(t, err, domain.ErrOutcomeUnknown)
			require.Zero(t, provider.deletes)
		})
	}
}

func TestPutLocalFileCommitOutcomeUnknownDeletesObjectOnlyWhenAuthorityIsMissing(t *testing.T) {
	_, repo, ns := newPutTestRepository(t)
	localProvider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = localProvider.Close() })
	provider := &deleteTrackingProvider{Provider: localProvider}
	h := NewPutLocalFileHandler(repo, provider, missingCommitUnknownTransactor{}, nil, nil)
	cmd := putFixture(ns)

	_, err = h.Handle(context.Background(), cmd)

	require.ErrorIs(t, err, domain.ErrOutcomeUnknown)
	require.Equal(t, 1, provider.deletes)
	_, err = provider.Stat(context.Background(), "sessions/"+cmd.CommandID.String()+"/content")
	require.ErrorIs(t, err, storage.ErrNotFound)
}

func TestPutLocalFileExistingNodeAllocatesNextRevisionInsideTransaction(t *testing.T) {
	db, repo, ns := newPutTestRepository(t)
	localProvider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = localProvider.Close() })
	h := NewPutLocalFileHandler(repo, localProvider, postgres.NewTransactor(db), nil, nil)
	nodeID := uuid.New()
	first := putFixture(ns)
	first.NodeID = &nodeID
	second := putFixture(ns)
	second.NodeID = &nodeID
	second.Name = first.Name

	_, err = h.Handle(context.Background(), first)
	require.NoError(t, err)
	_, err = h.Handle(context.Background(), second)
	require.NoError(t, err)

	revisions, _, err := repo.ListRevisions(context.Background(), nodeID, ns.ID, 10, "")
	require.NoError(t, err)
	require.Len(t, revisions, 2)
	require.Equal(t, 2, revisions[0].RevisionNo())
	require.Equal(t, 1, revisions[1].RevisionNo())
}

func TestPutLocalFileMissingDedupStrictlyValidatesDeterministicRevision(t *testing.T) {
	db, repo, ns := newPutTestRepository(t)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	h := NewPutLocalFileHandler(repo, provider, postgres.NewTransactor(db), nil, nil)
	cmd := putFixture(ns)
	_, err = h.Handle(context.Background(), cmd)
	require.NoError(t, err)
	require.NoError(t, db.Table("filesystem.command_dedup").Where("command_id = ?", cmd.CommandID).Delete(nil).Error)
	different := []byte("fixture-contens")
	cmd.SHA256 = storage.SHA256(different)
	cmd.Body = bytes.NewReader(different)

	_, err = h.Handle(context.Background(), cmd)

	require.ErrorIs(t, err, domain.ErrIdempotencyConflict)
}

func TestPutLocalFileMissingDedupRebuildsAuthorityForExactDeterministicRevision(t *testing.T) {
	db, repo, ns := newPutTestRepository(t)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	h := NewPutLocalFileHandler(repo, provider, postgres.NewTransactor(db), nil, nil)
	cmd := putFixture(ns)
	created, err := h.Handle(context.Background(), cmd)
	require.NoError(t, err)
	require.NoError(t, db.Table("filesystem.command_dedup").Where("command_id = ?", cmd.CommandID).Delete(nil).Error)
	cmd.Body = bytes.NewBufferString("fixture-content")

	got, err := h.Handle(context.Background(), cmd)

	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	authority, err := repo.GetCommand(context.Background(), cmd.CommandID)
	require.NoError(t, err)
	require.Equal(t, "COMPLETED", authority.State)
	require.Equal(t, created.ID, *authority.NodeID)
}

func TestPutLocalFileConcurrentDifferentCommandsAllocateContinuousUniqueRevisions(t *testing.T) {
	db, repo, ns := newPutTestRepository(t)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	h := NewPutLocalFileHandler(repo, provider, postgres.NewTransactor(db), nil, nil)
	nodeID := uuid.New()
	initial := putFixture(ns)
	initial.NodeID = &nodeID
	_, err = h.Handle(context.Background(), initial)
	require.NoError(t, err)

	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := putFixture(ns)
			cmd.NodeID = &nodeID
			cmd.Name = initial.Name
			<-start
			_, putErr := h.Handle(context.Background(), cmd)
			results <- putErr
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	for putErr := range results {
		require.NoError(t, putErr)
	}
	revisions, _, err := repo.ListRevisions(context.Background(), nodeID, ns.ID, 10, "")
	require.NoError(t, err)
	require.Len(t, revisions, 3)
	require.Equal(t, []int{3, 2, 1}, []int{
		revisions[0].RevisionNo(), revisions[1].RevisionNo(), revisions[2].RevisionNo(),
	})
}

func TestPutLocalFileCompletedCommandReturnsSavedResultWithoutDuplicatingUsage(t *testing.T) {
	db, repo, ns := newPutTestRepository(t)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	h := NewPutLocalFileHandler(repo, provider, postgres.NewTransactor(db), nil, nil)
	cmd := putFixture(ns)
	first, err := h.Handle(context.Background(), cmd)
	require.NoError(t, err)
	cmd.Body = bytes.NewBufferString("fixture-content")

	second, err := h.Handle(context.Background(), cmd)

	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	require.NotNil(t, first.CurrentRevisionID)
	require.Equal(t, first.CurrentRevisionID, second.CurrentRevisionID)
	var quota struct {
		UsedBytes int64
		UsedFiles int64
	}
	require.NoError(t, db.Table("filesystem.quota_accounts").Where("workspace_id = ?", ns.WorkspaceID).Take(&quota).Error)
	require.Equal(t, int64(15), quota.UsedBytes)
	require.Equal(t, int64(1), quota.UsedFiles)
}

func newPutTestRepository(t *testing.T) (*gorm.DB, repository.FilesystemRepository, *model.Namespace) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	require.NoError(t, persistence.AutoMigrate(db))
	repo := persistence.NewRepository(db)
	ns := &model.Namespace{ID: uuid.New(), OrganizationID: uuid.New(), WorkspaceID: uuid.New(), Status: "ACTIVE"}
	require.NoError(t, repo.CreateNamespace(context.Background(), ns))
	seedPutNamespaceRoot(t, repo, ns)
	return db, repo, ns
}

func seedPutNamespaceRoot(t *testing.T, repo repository.FilesystemRepository, ns *model.Namespace) {
	t.Helper()
	root, err := model.NewRootDirectory(ns.ID, uuid.New(), ".root")
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(context.Background(), root))
	require.NoError(t, repo.SetNamespaceRoot(context.Background(), ns.WorkspaceID, root.ID))
	ns.RootNodeID = &root.ID
}

func putFixture(ns *model.Namespace) PutLocalFile {
	body := []byte("fixture-content")
	return PutLocalFile{
		CommandID: uuid.New(), ActorID: uuid.New(), OrganizationID: ns.OrganizationID,
		WorkspaceID: ns.WorkspaceID, NamespaceID: ns.ID, Name: "fixture.txt",
		MIMEType: "text/plain", Size: int64(len(body)), SHA256: storage.SHA256(body),
		Body: bytes.NewReader(body), Authorization: Authorization{Allowed: true},
		ReplicaProviderID: uuid.New(),
	}
}
