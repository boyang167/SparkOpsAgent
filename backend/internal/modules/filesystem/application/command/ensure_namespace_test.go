package command

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

func TestEnsureNamespaceIsIdempotentUnderConcurrentCalls(t *testing.T) {
	mainDSN := "file:" + filepath.Join(t.TempDir(), "main.db") + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	filesystemPath := filepath.Join(t.TempDir(), "filesystem.db")
	db, err := gorm.Open(sqlite.Open(mainDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.Exec("ATTACH DATABASE ? AS filesystem", filesystemPath).Error)
	require.NoError(t, persistence.AutoMigrate(db))

	organizationID, workspaceID, ownerID := uuid.New(), uuid.New(), uuid.New()
	start := make(chan struct{})
	results := make(chan error, 2)
	namespaceIDs := make(chan uuid.UUID, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			<-start
			workerDB, workerErr := openEnsureWorkerDB(mainDSN, filesystemPath)
			if workerErr != nil {
				results <- workerErr
				return
			}
			workerSQLDB, workerErr := workerDB.DB()
			if workerErr != nil {
				results <- workerErr
				return
			}
			defer workerSQLDB.Close()
			namespace, ensureErr := NewEnsureNamespaceHandler(persistence.NewRepository(workerDB), postgres.NewTransactor(workerDB)).Handle(context.Background(), EnsureNamespace{
				OrganizationID: organizationID,
				WorkspaceID:    workspaceID,
				OwnerUserID:    ownerID,
				NamespaceType:  model.NamespaceTypeWorkspace,
				ActorID:        ownerID,
			})
			results <- ensureErr
			if namespace != nil {
				namespaceIDs <- namespace.ID
			}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(namespaceIDs)
	for ensureErr := range results {
		require.NoError(t, ensureErr)
	}
	var ids []uuid.UUID
	for namespaceID := range namespaceIDs {
		ids = append(ids, namespaceID)
	}
	require.Len(t, ids, 2)
	require.Equal(t, ids[0], ids[1])

	var namespaceCount, rootCount, linkCount, quotaCount int64
	require.NoError(t, db.Table("filesystem.namespaces").Where("workspace_id = ?", workspaceID).Count(&namespaceCount).Error)
	require.NoError(t, db.Table("filesystem.nodes").Where("name = ? AND namespace_id = (SELECT id FROM filesystem.namespaces WHERE workspace_id = ?)", ".root", workspaceID).Count(&rootCount).Error)
	require.NoError(t, db.Table("filesystem.file_links").Where("link_type = ? AND link_id = ?", model.FileLinkTypeNamespaceRoot, ids[0]).Count(&linkCount).Error)
	require.NoError(t, db.Table("filesystem.quota_accounts").Where("workspace_id = ?", workspaceID).Count(&quotaCount).Error)
	require.Equal(t, int64(1), namespaceCount)
	require.Equal(t, int64(1), rootCount)
	require.Equal(t, int64(1), linkCount)
	require.Equal(t, int64(1), quotaCount)
}

func TestEnsureOrganizationAndUserNamespacesDoNotCreateWorkspaceQuota(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	require.NoError(t, persistence.AutoMigrate(db))
	repo := persistence.NewRepository(db)
	handler := NewEnsureNamespaceHandler(repo, postgres.NewTransactor(db))
	organizationID, ownerID := uuid.New(), uuid.New()

	organization, err := handler.Handle(context.Background(), EnsureNamespace{
		OrganizationID: organizationID, WorkspaceID: uuid.New(), OwnerUserID: ownerID,
		NamespaceType: model.NamespaceTypeOrganization, ActorID: ownerID,
	})
	require.NoError(t, err)
	user, err := handler.Handle(context.Background(), EnsureNamespace{
		OrganizationID: organizationID, WorkspaceID: uuid.New(), OwnerUserID: ownerID,
		NamespaceType: model.NamespaceTypeUser, ActorID: ownerID,
	})
	require.NoError(t, err)
	require.NotNil(t, organization.RootNodeID)
	require.NotNil(t, user.RootNodeID)
	var namespaceCount, rootCount, linkCount, quotaCount int64
	require.NoError(t, db.Table("filesystem.namespaces").Where("namespace_type IN ?", []model.NamespaceType{model.NamespaceTypeOrganization, model.NamespaceTypeUser}).Count(&namespaceCount).Error)
	require.NoError(t, db.Table("filesystem.nodes").Where("namespace_id IN ?", []uuid.UUID{organization.ID, user.ID}).Count(&rootCount).Error)
	require.NoError(t, db.Table("filesystem.file_links").Where("link_type = ? AND link_id IN ?", model.FileLinkTypeNamespaceRoot, []uuid.UUID{organization.ID, user.ID}).Count(&linkCount).Error)
	require.NoError(t, db.Table("filesystem.quota_accounts").Count(&quotaCount).Error)
	require.Equal(t, int64(2), namespaceCount)
	require.Equal(t, int64(2), rootCount)
	require.Equal(t, int64(2), linkCount)
	require.Zero(t, quotaCount)
}

func TestEnsureWorkspacePreservesDefaultLocalProviderBootstrap(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	require.NoError(t, persistence.AutoMigrate(db))
	repo := persistence.NewRepository(db)
	storageRoot := t.TempDir()
	handler := NewEnsureNamespaceHandler(repo, postgres.NewTransactor(db)).
		WithDefaultLocalProvider(storageRoot, NewEnsureDefaultLocalProviderHandler(repo))

	_, err = handler.Handle(context.Background(), EnsureNamespace{
		OrganizationID: uuid.New(), WorkspaceID: uuid.New(), NamespaceType: model.NamespaceTypeWorkspace,
		ActorID: uuid.New(),
	})
	require.NoError(t, err)
	var providers int64
	require.NoError(t, db.Table("filesystem.storage_providers").
		Where("name = ?", DefaultLocalProviderName).Count(&providers).Error)
	require.Equal(t, int64(1), providers)
}

func TestNamespaceIDNormalizesUnusedScopeFields(t *testing.T) {
	organizationID, workspaceID, ownerID := uuid.New(), uuid.New(), uuid.New()
	organization := namespaceID(EnsureNamespace{
		OrganizationID: organizationID, NamespaceType: model.NamespaceTypeOrganization,
	})
	organizationWithNoise := namespaceID(EnsureNamespace{
		OrganizationID: organizationID, WorkspaceID: workspaceID, OwnerUserID: ownerID,
		NamespaceType: model.NamespaceTypeOrganization,
	})
	user := namespaceID(EnsureNamespace{
		OrganizationID: organizationID, OwnerUserID: ownerID, NamespaceType: model.NamespaceTypeUser,
	})
	userWithNoise := namespaceID(EnsureNamespace{
		OrganizationID: organizationID, WorkspaceID: workspaceID, OwnerUserID: ownerID,
		NamespaceType: model.NamespaceTypeUser,
	})
	require.Equal(t, organization, organizationWithNoise)
	require.Equal(t, user, userWithNoise)
}

func openEnsureWorkerDB(mainDSN, filesystemPath string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(mainDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, err
	}
	if err := db.Exec("ATTACH DATABASE ? AS filesystem", filesystemPath).Error; err != nil {
		return nil, fmt.Errorf("attach filesystem: %w", err)
	}
	return db, nil
}
