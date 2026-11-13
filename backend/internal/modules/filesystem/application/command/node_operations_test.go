package command

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/adapters/local"
)

type recordingAudit struct {
	mu      sync.Mutex
	entries []audit.Entry
}

type failingAudit struct {
	calls int
	err   error
}

func (a *failingAudit) Log(context.Context, audit.Entry) error {
	a.calls++
	return a.err
}

type namespaceReadErrorRepository struct {
	repository.FilesystemRepository
}

func (r namespaceReadErrorRepository) GetNamespaceByWorkspace(context.Context, uuid.UUID) (*model.Namespace, error) {
	return nil, domain.ErrPersistence
}

type countingTransactor struct {
	calls int
}

func (t *countingTransactor) WithinTransaction(context.Context, func(context.Context) error) error {
	t.calls++
	return errors.New("transaction unavailable")
}

type failingPutProvider struct {
	storage.Provider
}

func (failingPutProvider) Put(context.Context, storage.PutRequest) (storage.ObjectInfo, error) {
	return storage.ObjectInfo{}, errors.New("fixture-content object-key secret")
}

type retryMoveRepository struct {
	repository.FilesystemRepository
	calls int
}

type retryOnceTransactor struct {
	inner postgres.Transactor
	calls int
}

func (t *retryOnceTransactor) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	t.calls++
	if t.calls == 1 {
		return domain.ErrTransactionRetry
	}
	return t.inner.WithinTransaction(ctx, fn)
}

func (r *retryMoveRepository) MoveNodeSafely(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID, string) error {
	r.calls++
	if r.calls == 1 {
		return domain.ErrTransactionRetry
	}
	return nil
}

func (a *recordingAudit) Log(_ context.Context, entry audit.Entry) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = append(a.entries, entry)
	return nil
}

func (a *recordingAudit) decisions() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]string, len(a.entries))
	for index := range a.entries {
		out[index], _ = a.entries[index].Detail["decision"].(string)
	}
	return out
}

func TestReasonCodeDoesNotExposeUnderlyingError(t *testing.T) {
	secret := errors.New("fixture-content checksum object-key")
	require.Equal(t, "operation_failed", reasonCode(secret))
	require.NotContains(t, reasonCode(secret), "fixture-content")
	require.NotContains(t, reasonCode(secret), "object-key")
}

func TestRootDirectoryHasNilParent(t *testing.T) {
	root, err := model.NewRootDirectory(uuid.New(), uuid.New(), ".root")
	require.NoError(t, err)
	require.Nil(t, root.ParentID)
	require.NotEqual(t, uuid.Nil, root.ID)
}

func TestDirectoryCycleErrorIsStable(t *testing.T) {
	require.ErrorIs(t, domain.ErrDirectoryCycle, domain.ErrDirectoryCycle)
	require.Equal(t, "filesystem directory cycle", domain.ErrDirectoryCycle.Error())
}

func TestCopyNilParentResolvesNamespaceRoot(t *testing.T) {
	db, repo, ns := newPutTestRepository(t)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	tx := postgres.NewTransactor(db)
	put := NewPutLocalFileHandler(repo, provider, tx, nil, nil)
	source, err := put.Handle(context.Background(), putFixture(ns))
	require.NoError(t, err)
	secondBlob := &model.Blob{ID: uuid.New(), DedupDomain: ns.OrganizationID, SHA256: strings.Repeat("b", 64), Size: 15, RefCount: 1, State: model.BlobStateAvailable}
	secondRevision, err := model.NewRevision(source.ID, 2, secondBlob.ID, 15, "text/plain", source.OwnerID)
	require.NoError(t, err)
	verifiedAt := time.Now().UTC()
	secondReplica := &model.StorageReplica{ID: uuid.New(), BlobID: secondBlob.ID, ProviderID: uuid.New(), ObjectKey: "second", Checksum: secondBlob.SHA256, State: model.ReplicaStateAvailable, VerifiedAt: &verifiedAt}
	require.NoError(t, repo.SaveRevision(context.Background(), secondRevision, secondBlob, secondReplica))
	require.NoError(t, repo.SetCurrentRevision(context.Background(), source.ID, secondRevision.ID()))
	copyHandler := NewCopyNodeHandler(repo, tx, nil, nil)

	copied, err := copyHandler.Handle(context.Background(), CopyNode{
		ActorID: source.OwnerID, OrganizationID: ns.OrganizationID, WorkspaceID: ns.WorkspaceID,
		NamespaceID: ns.ID, SourceNodeID: source.ID, Name: "copied.txt",
		Authorization: Authorization{Allowed: true},
	})

	require.NoError(t, err)
	require.NotNil(t, copied.ParentID)
	require.Equal(t, *ns.RootNodeID, *copied.ParentID)
	persistedSource, err := repo.GetNode(context.Background(), ns.ID, source.ID)
	require.NoError(t, err)
	require.NotNil(t, persistedSource.CurrentRevisionID)
	sourceRevision, err := repo.GetRevision(context.Background(), source.ID, *persistedSource.CurrentRevisionID)
	require.NoError(t, err)
	persistedCopy, err := repo.GetNode(context.Background(), ns.ID, copied.ID)
	require.NoError(t, err)
	copiedRevision, err := repo.GetRevision(context.Background(), copied.ID, *persistedCopy.CurrentRevisionID)
	require.NoError(t, err)
	require.Equal(t, 1, copiedRevision.RevisionNo())
	blob, err := repo.GetBlob(context.Background(), sourceRevision.BlobID())
	require.NoError(t, err)
	require.Equal(t, int64(2), blob.RefCount)
	var replicaCount int64
	require.NoError(t, db.Table("filesystem.storage_replicas").Where("blob_id = ?", blob.ID).Count(&replicaCount).Error)
	require.Equal(t, int64(1), replicaCount)
	var quota struct {
		UsedBytes int64
		UsedFiles int64
	}
	require.NoError(t, db.Table("filesystem.quota_accounts").Where("workspace_id = ?", ns.WorkspaceID).Take(&quota).Error)
	require.Equal(t, int64(30), quota.UsedBytes)
	require.Equal(t, int64(2), quota.UsedFiles)
}

func TestEveryMutationAuthorizationRejectionIsAudited(t *testing.T) {
	db, repo, ns := newPutTestRepository(t)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	tx := postgres.NewTransactor(db)
	logs := &recordingAudit{}
	auth := Authorization{Allowed: false, Reason: "policy denied"}
	_, _ = NewCreateNamespaceHandler(repo, tx, nil, logs).Handle(context.Background(), CreateNamespace{
		ActorID: uuid.New(), OrganizationID: ns.OrganizationID, WorkspaceID: uuid.New(), Authorization: auth,
	})
	_, _ = NewCreateDirectoryHandler(repo, tx, nil, logs).Handle(context.Background(), CreateDirectory{
		ActorID: uuid.New(), OrganizationID: ns.OrganizationID, WorkspaceID: ns.WorkspaceID, NamespaceID: ns.ID, Authorization: auth,
	})
	put := putFixture(ns)
	put.Authorization = auth
	_, _ = NewPutLocalFileHandler(repo, provider, tx, nil, logs).Handle(context.Background(), put)
	_ = NewRenameNodeHandler(repo, tx, nil, logs).Handle(context.Background(), RenameNode{WorkspaceID: ns.WorkspaceID, Authorization: auth})
	_ = NewMoveNodeHandler(repo, tx, nil, logs).Handle(context.Background(), MoveNode{WorkspaceID: ns.WorkspaceID, Authorization: auth})
	_, _ = NewCopyNodeHandler(repo, tx, nil, logs).Handle(context.Background(), CopyNode{WorkspaceID: ns.WorkspaceID, Authorization: auth})
	_ = NewTrashNodeHandler(repo, tx, nil, logs).Handle(context.Background(), TrashNode{WorkspaceID: ns.WorkspaceID, Authorization: auth})
	_ = NewRestoreRevisionHandler(repo, tx, nil, logs).Handle(context.Background(), RestoreRevision{WorkspaceID: ns.WorkspaceID, Authorization: auth})

	require.Equal(t, []string{"DENY", "DENY", "DENY", "DENY", "DENY", "DENY", "DENY", "DENY"}, logs.decisions())
}

func TestCreateDirectoryValidationFailureIsAudited(t *testing.T) {
	db, repo, ns := newPutTestRepository(t)
	logs := &recordingAudit{}
	_, err := NewCreateDirectoryHandler(repo, postgres.NewTransactor(db), nil, logs).Handle(context.Background(), CreateDirectory{
		ActorID: uuid.New(), OrganizationID: ns.OrganizationID, WorkspaceID: ns.WorkspaceID,
		NamespaceID: ns.ID, Name: "../invalid", Authorization: Authorization{Allowed: true},
	})

	require.ErrorIs(t, err, model.ErrInvalidNodeName)
	require.Equal(t, []string{"DENY"}, logs.decisions())
}

func TestCreateDirectoryRollsBackWhenOutboxPublishFails(t *testing.T) {
	db, repo, ns := newPutTestRepository(t)
	command := CreateDirectory{
		CommandID: uuid.New(), ActorID: uuid.New(), OrganizationID: ns.OrganizationID,
		WorkspaceID: ns.WorkspaceID, NamespaceID: ns.ID, Name: "outbox-rollback",
		Authorization: Authorization{Allowed: true},
	}
	_, err := NewCreateDirectoryHandler(repo, postgres.NewTransactor(db), failingBus{}, nil).Handle(context.Background(), command)
	require.Error(t, err)

	children, _, listErr := repo.ListChildren(context.Background(), ns.ID, ns.RootNodeID, 100, "")
	require.NoError(t, listErr)
	for _, child := range children {
		require.NotEqual(t, "outbox-rollback", child.Name)
	}
	_, claimErr := repo.GetCommand(context.Background(), command.CommandID)
	require.ErrorIs(t, claimErr, domain.ErrCommandNotFound)
}

func TestCreateNamespaceIdempotentSuccessIsAuditedAsAllow(t *testing.T) {
	db, repo, _ := newPutTestRepository(t)
	logs := &recordingAudit{}
	handler := NewCreateNamespaceHandler(repo, postgres.NewTransactor(db), nil, logs)
	cmd := CreateNamespace{
		ActorID: uuid.New(), OrganizationID: uuid.New(), WorkspaceID: uuid.New(),
		Authorization: Authorization{Allowed: true},
	}
	_, err := handler.Handle(context.Background(), cmd)
	require.NoError(t, err)
	_, err = handler.Handle(context.Background(), cmd)
	require.NoError(t, err)
	require.Equal(t, []string{"ALLOW", "ALLOW"}, logs.decisions())
}

func TestCreateNamespaceConcurrentSameWorkspaceReturnsSingleNamespace(t *testing.T) {
	db, repo, _ := newPutTestRepository(t)
	handler := NewCreateNamespaceHandler(repo, postgres.NewTransactor(db), nil, nil)
	cmd := CreateNamespace{
		ActorID: uuid.New(), OrganizationID: uuid.New(), WorkspaceID: uuid.New(),
		Authorization: Authorization{Allowed: true},
	}
	start := make(chan struct{})
	results := make(chan *model.Namespace, 2)
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			result, err := handler.Handle(context.Background(), cmd)
			results <- result
			errs <- err
		}()
	}
	close(start)
	first, second := <-results, <-results
	require.NoError(t, <-errs)
	require.NoError(t, <-errs)
	require.Equal(t, first.ID, second.ID)
	var count int64
	require.NoError(t, db.Table("filesystem.namespaces").Where("workspace_id = ?", cmd.WorkspaceID).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestPutLocalFileConcurrentQuotaAllowsOnlyOneWinner(t *testing.T) {
	db, _, ns, mainDSN, filesystemDSN := newConcurrentPutTestRepository(t)
	require.NoError(t, db.Table("filesystem.quota_accounts").Where("workspace_id = ?", ns.WorkspaceID).
		Updates(map[string]any{"max_bytes": 15, "max_files": 1}).Error)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	start := make(chan struct{})
	ready := make(chan error, 2)
	errs := make(chan error, 2)
	claimBarrier := newConcurrentBarrier()
	for range 2 {
		go func() {
			workerDB, openErr := openPutWorkerDB(mainDSN, filesystemDSN)
			ready <- openErr
			if openErr != nil {
				errs <- openErr
				return
			}
			workerSQLDB, _ := workerDB.DB()
			defer workerSQLDB.Close()
			cmd := putFixture(ns)
			cmd.Name = cmd.CommandID.String() + ".txt"
			<-start
			workerRepo := claimBarrierRepository{FilesystemRepository: persistence.NewRepository(workerDB), barrier: claimBarrier}
			handler := NewPutLocalFileHandler(workerRepo, provider, postgres.NewTransactor(workerDB), nil, nil)
			_, putErr := handler.Handle(context.Background(), cmd)
			errs <- putErr
		}()
	}
	require.NoError(t, <-ready)
	require.NoError(t, <-ready)
	close(start)
	first, second := <-errs, <-errs
	require.True(t,
		(first == nil && errors.Is(second, domain.ErrQuotaExceeded)) ||
			(second == nil && errors.Is(first, domain.ErrQuotaExceeded)),
		"unexpected results: first=%v second=%v", first, second,
	)
	var quota struct {
		UsedBytes int64
		UsedFiles int64
	}
	require.NoError(t, db.Table("filesystem.quota_accounts").Where("workspace_id = ?", ns.WorkspaceID).Take(&quota).Error)
	require.Equal(t, int64(15), quota.UsedBytes)
	require.Equal(t, int64(1), quota.UsedFiles)
}

func TestMoveConcurrentOppositeMovesCannotCreateCycle(t *testing.T) {
	_, repo, ns, mainDSN, filesystemDSN := newConcurrentPutTestRepository(t)
	first, err := model.NewDirectory(ns.ID, uuid.New(), "first", *ns.RootNodeID)
	require.NoError(t, err)
	second, err := model.NewDirectory(ns.ID, uuid.New(), "second", *ns.RootNodeID)
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(context.Background(), first))
	require.NoError(t, repo.CreateNode(context.Background(), second))
	start := make(chan struct{})
	ready := make(chan error, 2)
	errs := make(chan error, 2)
	namespaceBarrier := newConcurrentBarrier()
	retries := make(chan struct{}, 10)
	logs := &recordingAudit{}
	moves := []MoveNode{
		{
			ActorID: first.OwnerID, OrganizationID: ns.OrganizationID, WorkspaceID: ns.WorkspaceID,
			NamespaceID: ns.ID, NodeID: first.ID, NewParentID: &second.ID, Authorization: Authorization{Allowed: true},
		},
		{
			ActorID: second.OwnerID, OrganizationID: ns.OrganizationID, WorkspaceID: ns.WorkspaceID,
			NamespaceID: ns.ID, NodeID: second.ID, NewParentID: &first.ID, Authorization: Authorization{Allowed: true},
		},
	}
	for _, move := range moves {
		move := move
		go func() {
			workerDB, openErr := openPutWorkerDB(mainDSN, filesystemDSN)
			ready <- openErr
			if openErr != nil {
				errs <- openErr
				return
			}
			workerSQLDB, _ := workerDB.DB()
			defer workerSQLDB.Close()
			<-start
			workerRepo := namespaceBarrierRepository{FilesystemRepository: persistence.NewRepository(workerDB), barrier: namespaceBarrier}
			handler := NewMoveNodeHandler(workerRepo, postgres.NewTransactor(workerDB), nil, logs)
			defaultWait := handler.retryWait
			handler.retryWait = func(ctx context.Context, delay time.Duration) error {
				retries <- struct{}{}
				return defaultWait(ctx, delay)
			}
			errs <- handler.Handle(context.Background(), move)
		}()
	}
	require.NoError(t, <-ready)
	require.NoError(t, <-ready)
	close(start)
	left, right := <-errs, <-errs
	close(retries)
	retryCount := 0
	for range retries {
		retryCount++
	}
	require.GreaterOrEqual(t, retryCount, 1)
	require.True(t,
		(left == nil && errors.Is(right, domain.ErrDirectoryCycle)) ||
			(right == nil && errors.Is(left, domain.ErrDirectoryCycle)),
		"unexpected results: left=%v right=%v", left, right,
	)
	allowCount, denyCount := 0, 0
	for _, decision := range logs.decisions() {
		switch decision {
		case "ALLOW":
			allowCount++
		case "DENY":
			denyCount++
		}
	}
	require.Equal(t, 1, allowCount)
	require.Equal(t, 1, denyCount)
	persistedFirst, err := repo.GetNode(context.Background(), ns.ID, first.ID)
	require.NoError(t, err)
	persistedSecond, err := repo.GetNode(context.Background(), ns.ID, second.ID)
	require.NoError(t, err)
	require.False(t,
		persistedFirst.ParentID != nil && persistedSecond.ParentID != nil &&
			*persistedFirst.ParentID == second.ID && *persistedSecond.ParentID == first.ID,
	)
}

func TestMoveRetriesWholeTransactionWithInjectedBackoffAndAuditsOnce(t *testing.T) {
	_, baseRepo, ns := newPutTestRepository(t)
	node, err := model.NewDirectory(ns.ID, uuid.New(), "moving", *ns.RootNodeID)
	require.NoError(t, err)
	require.NoError(t, baseRepo.CreateNode(context.Background(), node))
	repo := &retryMoveRepository{FilesystemRepository: baseRepo}
	logs := &recordingAudit{}
	handler := NewMoveNodeHandler(repo, postgres.NewDirectTransactor(), nil, logs)
	var waits []time.Duration
	handler.retryBackoff = func(attempt int) time.Duration {
		return time.Duration(attempt+1) * 7 * time.Millisecond
	}
	handler.retryWait = func(_ context.Context, delay time.Duration) error {
		waits = append(waits, delay)
		return nil
	}

	err = handler.Handle(context.Background(), MoveNode{
		ActorID: node.OwnerID, OrganizationID: ns.OrganizationID, WorkspaceID: ns.WorkspaceID,
		NamespaceID: ns.ID, NodeID: node.ID, Authorization: Authorization{Allowed: true},
	})

	require.NoError(t, err)
	require.Equal(t, 2, repo.calls)
	require.Equal(t, []time.Duration{7 * time.Millisecond}, waits)
	require.Equal(t, []string{"ALLOW"}, logs.decisions())
}

func TestCreateNamespaceRepositoryFailureIsAuditedAndDoesNotStartTransaction(t *testing.T) {
	_, baseRepo, _ := newPutTestRepository(t)
	repo := namespaceReadErrorRepository{FilesystemRepository: baseRepo}
	tx := &countingTransactor{}
	logs := &recordingAudit{}
	_, err := NewCreateNamespaceHandler(repo, tx, nil, logs).Handle(context.Background(), CreateNamespace{
		ActorID: uuid.New(), OrganizationID: uuid.New(), WorkspaceID: uuid.New(),
		Authorization: Authorization{Allowed: true},
	})

	require.ErrorIs(t, err, domain.ErrPersistence)
	require.Zero(t, tx.calls)
	require.Equal(t, []string{"DENY"}, logs.decisions())
}

func TestCreateNamespaceTransactionFailureIsAudited(t *testing.T) {
	_, repo, _ := newPutTestRepository(t)
	tx := &countingTransactor{}
	logs := &recordingAudit{}
	_, err := NewCreateNamespaceHandler(repo, tx, nil, logs).Handle(context.Background(), CreateNamespace{
		ActorID: uuid.New(), OrganizationID: uuid.New(), WorkspaceID: uuid.New(),
		Authorization: Authorization{Allowed: true},
	})

	require.ErrorContains(t, err, "transaction unavailable")
	require.Equal(t, 1, tx.calls)
	require.Equal(t, []string{"DENY"}, logs.decisions())
}

func TestPutStorageFailureIsAuditedWithoutLeakingProviderError(t *testing.T) {
	db, repo, ns := newPutTestRepository(t)
	baseProvider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = baseProvider.Close() })
	logs := &recordingAudit{}
	handler := NewPutLocalFileHandler(repo, failingPutProvider{Provider: baseProvider}, postgres.NewTransactor(db), nil, logs)

	_, err = handler.Handle(context.Background(), putFixture(ns))

	require.Error(t, err)
	require.Equal(t, []string{"DENY"}, logs.decisions())
	require.Equal(t, "operation_failed", logs.entries[0].Detail["reason"])
	require.NotContains(t, logs.entries[0].Detail, "fixture-content")
}

func TestMutationConstructorsRejectMissingRequiredDependencies(t *testing.T) {
	require.Panics(t, func() {
		NewCreateNamespaceHandler(nil, postgres.NewDirectTransactor(), nil, nil)
	})
	_, repo, _ := newPutTestRepository(t)
	require.Panics(t, func() {
		NewPutLocalFileHandler(repo, nil, postgres.NewDirectTransactor(), nil, nil)
	})
	require.Panics(t, func() {
		NewMoveNodeHandler(repo, nil, nil, nil)
	})
}

func TestDeniedWithAuditFailureAttemptsAuditOnceAndPreservesBusinessError(t *testing.T) {
	db, repo, ns := newPutTestRepository(t)
	auditErr := errors.New("audit unavailable")
	logs := &failingAudit{err: auditErr}
	_, err := NewCreateDirectoryHandler(repo, postgres.NewTransactor(db), nil, logs).Handle(context.Background(), CreateDirectory{
		ActorID: uuid.New(), OrganizationID: uuid.New(), WorkspaceID: ns.WorkspaceID,
		NamespaceID: ns.ID, Name: "child", Authorization: Authorization{Allowed: true},
	})

	require.ErrorIs(t, err, domain.ErrNamespaceMismatch)
	require.ErrorIs(t, err, auditErr)
	require.Equal(t, 1, logs.calls)
}

func TestMetadataMutationsUseCommandDedup(t *testing.T) {
	db, repo, ns := newPutTestRepository(t)
	tx := postgres.NewTransactor(db)
	actor := uuid.New()

	t.Run("rename", func(t *testing.T) {
		node, err := model.NewDirectory(ns.ID, actor, "before", *ns.RootNodeID)
		require.NoError(t, err)
		require.NoError(t, repo.CreateNode(context.Background(), node))
		logs := &recordingAudit{}
		handler := NewRenameNodeHandler(repo, tx, nil, logs)
		cmd := RenameNode{CommandID: uuid.New(), ActorID: actor, OrganizationID: ns.OrganizationID, WorkspaceID: ns.WorkspaceID, NamespaceID: ns.ID, NodeID: node.ID, Name: "after", Authorization: Authorization{Allowed: true}}
		require.NoError(t, handler.Handle(context.Background(), cmd))
		require.NoError(t, handler.Handle(context.Background(), cmd))
		require.Equal(t, []string{"ALLOW"}, logs.decisions())
		cmd.Name = "different"
		require.ErrorIs(t, handler.Handle(context.Background(), cmd), domain.ErrIdempotencyConflict)
	})

	t.Run("move", func(t *testing.T) {
		left, err := model.NewDirectory(ns.ID, actor, "move-source", *ns.RootNodeID)
		require.NoError(t, err)
		right, err := model.NewDirectory(ns.ID, actor, "move-target", *ns.RootNodeID)
		require.NoError(t, err)
		require.NoError(t, repo.CreateNode(context.Background(), left))
		require.NoError(t, repo.CreateNode(context.Background(), right))
		logs := &recordingAudit{}
		handler := NewMoveNodeHandler(repo, tx, nil, logs)
		cmd := MoveNode{CommandID: uuid.New(), ActorID: actor, OrganizationID: ns.OrganizationID, WorkspaceID: ns.WorkspaceID, NamespaceID: ns.ID, NodeID: left.ID, NewParentID: &right.ID, Authorization: Authorization{Allowed: true}}
		require.NoError(t, handler.Handle(context.Background(), cmd))
		require.NoError(t, handler.Handle(context.Background(), cmd))
		require.Equal(t, []string{"ALLOW"}, logs.decisions())
		cmd.NewName = "different"
		require.ErrorIs(t, handler.Handle(context.Background(), cmd), domain.ErrIdempotencyConflict)
	})

	t.Run("trash", func(t *testing.T) {
		node, err := model.NewDirectory(ns.ID, actor, "trash-source", *ns.RootNodeID)
		require.NoError(t, err)
		require.NoError(t, repo.CreateNode(context.Background(), node))
		logs := &recordingAudit{}
		handler := NewTrashNodeHandler(repo, tx, nil, logs)
		cmd := TrashNode{CommandID: uuid.New(), ActorID: actor, OrganizationID: ns.OrganizationID, WorkspaceID: ns.WorkspaceID, NamespaceID: ns.ID, NodeID: node.ID, Authorization: Authorization{Allowed: true}}
		require.NoError(t, handler.Handle(context.Background(), cmd))
		require.NoError(t, handler.Handle(context.Background(), cmd))
		require.Equal(t, []string{"ALLOW"}, logs.decisions())
		cmd.NodeID = uuid.New()
		require.ErrorIs(t, handler.Handle(context.Background(), cmd), domain.ErrIdempotencyConflict)
	})

	t.Run("restore", func(t *testing.T) {
		provider, err := local.New(t.TempDir())
		require.NoError(t, err)
		t.Cleanup(func() { _ = provider.Close() })
		source, err := NewPutLocalFileHandler(repo, provider, tx, nil, nil).Handle(context.Background(), putFixture(ns))
		require.NoError(t, err)
		logs := &recordingAudit{}
		handler := NewRestoreRevisionHandler(repo, tx, nil, logs)
		cmd := RestoreRevision{CommandID: uuid.New(), ActorID: actor, OrganizationID: ns.OrganizationID, WorkspaceID: ns.WorkspaceID, NamespaceID: ns.ID, NodeID: source.ID, RevisionID: *source.CurrentRevisionID, Authorization: Authorization{Allowed: true}}
		require.NoError(t, handler.Handle(context.Background(), cmd))
		require.NoError(t, handler.Handle(context.Background(), cmd))
		require.Equal(t, []string{"ALLOW"}, logs.decisions())
		cmd.RevisionID = uuid.New()
		require.ErrorIs(t, handler.Handle(context.Background(), cmd), domain.ErrIdempotencyConflict)
	})
}

func TestCopyCommandDedupDoesNotRepeatQuotaBlobOrAudit(t *testing.T) {
	db, repo, ns := newPutTestRepository(t)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	tx := postgres.NewTransactor(db)
	source, err := NewPutLocalFileHandler(repo, provider, tx, nil, nil).Handle(context.Background(), putFixture(ns))
	require.NoError(t, err)
	logs := &recordingAudit{}
	handler := NewCopyNodeHandler(repo, tx, nil, logs)
	cmd := CopyNode{CommandID: uuid.New(), ActorID: source.OwnerID, OrganizationID: ns.OrganizationID, WorkspaceID: ns.WorkspaceID, NamespaceID: ns.ID, SourceNodeID: source.ID, Name: "dedup-copy.txt", Authorization: Authorization{Allowed: true}}
	first, err := handler.Handle(context.Background(), cmd)
	require.NoError(t, err)
	second, err := handler.Handle(context.Background(), cmd)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	require.Equal(t, []string{"ALLOW"}, logs.decisions())

	revision, err := repo.GetRevision(context.Background(), source.ID, *source.CurrentRevisionID)
	require.NoError(t, err)
	blob, err := repo.GetBlob(context.Background(), revision.BlobID())
	require.NoError(t, err)
	require.Equal(t, int64(2), blob.RefCount)
	var quota struct{ UsedBytes, UsedFiles int64 }
	require.NoError(t, db.Table("filesystem.quota_accounts").Where("workspace_id = ?", ns.WorkspaceID).Take(&quota).Error)
	require.Equal(t, int64(30), quota.UsedBytes)
	require.Equal(t, int64(2), quota.UsedFiles)

	cmd.Name = "different.txt"
	_, err = handler.Handle(context.Background(), cmd)
	require.ErrorIs(t, err, domain.ErrIdempotencyConflict)
}

func TestCopyRetriesWholeTransactionAfterTransactionRetry(t *testing.T) {
	db, repo, ns := newPutTestRepository(t)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	source, err := NewPutLocalFileHandler(repo, provider, postgres.NewTransactor(db), nil, nil).Handle(context.Background(), putFixture(ns))
	require.NoError(t, err)
	logs := &recordingAudit{}
	tx := &retryOnceTransactor{inner: postgres.NewTransactor(db)}
	handler := NewCopyNodeHandler(repo, tx, nil, logs)
	var waits []time.Duration
	handler.retryBackoff = func(attempt int) time.Duration {
		return time.Duration(attempt+1) * 7 * time.Millisecond
	}
	handler.retryWait = func(_ context.Context, delay time.Duration) error {
		waits = append(waits, delay)
		return nil
	}
	cmd := CopyNode{
		CommandID: uuid.New(), ActorID: source.OwnerID, OrganizationID: ns.OrganizationID,
		WorkspaceID: ns.WorkspaceID, NamespaceID: ns.ID, SourceNodeID: source.ID,
		Name: "retry-copy.txt", Authorization: Authorization{Allowed: true},
	}

	copied, err := handler.Handle(context.Background(), cmd)

	require.NoError(t, err)
	require.NotNil(t, copied)
	require.Equal(t, 2, tx.calls)
	require.Equal(t, []time.Duration{7 * time.Millisecond}, waits)
	require.Equal(t, []string{"ALLOW"}, logs.decisions())
	revision, err := repo.GetRevision(context.Background(), source.ID, *source.CurrentRevisionID)
	require.NoError(t, err)
	blob, err := repo.GetBlob(context.Background(), revision.BlobID())
	require.NoError(t, err)
	require.Equal(t, int64(2), blob.RefCount)
	var quota struct{ UsedBytes, UsedFiles int64 }
	require.NoError(t, db.Table("filesystem.quota_accounts").Where("workspace_id = ?", ns.WorkspaceID).Take(&quota).Error)
	require.Equal(t, int64(30), quota.UsedBytes)
	require.Equal(t, int64(2), quota.UsedFiles)
}

func TestCopyCommandDedupConcurrentReturnsAuthoritativeResult(t *testing.T) {
	db, repo, ns, mainDSN, filesystemDSN := newConcurrentPutTestRepository(t)
	provider, err := local.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	source, err := NewPutLocalFileHandler(repo, provider, postgres.NewTransactor(db), nil, nil).Handle(context.Background(), putFixture(ns))
	require.NoError(t, err)
	logs := &recordingAudit{}
	cmd := CopyNode{CommandID: uuid.New(), ActorID: source.OwnerID, OrganizationID: ns.OrganizationID, WorkspaceID: ns.WorkspaceID, NamespaceID: ns.ID, SourceNodeID: source.ID, Name: "concurrent-copy.txt", Authorization: Authorization{Allowed: true}}

	start := make(chan struct{})
	ready := make(chan error, 2)
	results := make(chan *model.Node, 2)
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			workerDB, openErr := openPutWorkerDB(mainDSN, filesystemDSN)
			ready <- openErr
			if openErr != nil {
				results <- nil
				errs <- openErr
				return
			}
			sqlDB, _ := workerDB.DB()
			defer sqlDB.Close()
			<-start
			result, copyErr := NewCopyNodeHandler(persistence.NewRepository(workerDB), postgres.NewTransactor(workerDB), nil, logs).Handle(context.Background(), cmd)
			results <- result
			errs <- copyErr
		}()
	}
	require.NoError(t, <-ready)
	require.NoError(t, <-ready)
	close(start)
	first, second := <-results, <-results
	require.NoError(t, <-errs)
	require.NoError(t, <-errs)
	require.NotNil(t, first)
	require.NotNil(t, second)
	require.Equal(t, first.ID, second.ID)
	require.Equal(t, []string{"ALLOW"}, logs.decisions())

	revision, err := repo.GetRevision(context.Background(), source.ID, *source.CurrentRevisionID)
	require.NoError(t, err)
	blob, err := repo.GetBlob(context.Background(), revision.BlobID())
	require.NoError(t, err)
	require.Equal(t, int64(2), blob.RefCount)
	var quota struct{ UsedBytes, UsedFiles int64 }
	require.NoError(t, db.Table("filesystem.quota_accounts").Where("workspace_id = ?", ns.WorkspaceID).Take(&quota).Error)
	require.Equal(t, int64(30), quota.UsedBytes)
	require.Equal(t, int64(2), quota.UsedFiles)
}

func TestRenameCommandDedupConcurrentAuditsOnce(t *testing.T) {
	_, repo, ns, mainDSN, filesystemDSN := newConcurrentPutTestRepository(t)
	node, err := model.NewDirectory(ns.ID, uuid.New(), "rename-concurrent-before", *ns.RootNodeID)
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(context.Background(), node))
	logs := &recordingAudit{}
	cmd := RenameNode{CommandID: uuid.New(), ActorID: node.OwnerID, OrganizationID: ns.OrganizationID, WorkspaceID: ns.WorkspaceID, NamespaceID: ns.ID, NodeID: node.ID, Name: "rename-concurrent-after", Authorization: Authorization{Allowed: true}}

	start := make(chan struct{})
	ready := make(chan error, 2)
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			workerDB, openErr := openPutWorkerDB(mainDSN, filesystemDSN)
			ready <- openErr
			if openErr != nil {
				errs <- openErr
				return
			}
			sqlDB, _ := workerDB.DB()
			defer sqlDB.Close()
			<-start
			errs <- NewRenameNodeHandler(persistence.NewRepository(workerDB), postgres.NewTransactor(workerDB), nil, logs).Handle(context.Background(), cmd)
		}()
	}
	require.NoError(t, <-ready)
	require.NoError(t, <-ready)
	close(start)
	require.NoError(t, <-errs)
	require.NoError(t, <-errs)
	require.Equal(t, []string{"ALLOW"}, logs.decisions())
	persisted, err := repo.GetNode(context.Background(), ns.ID, node.ID)
	require.NoError(t, err)
	require.Equal(t, cmd.Name, persisted.Name)
}
