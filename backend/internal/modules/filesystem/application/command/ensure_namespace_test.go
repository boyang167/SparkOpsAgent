package command

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

func TestEnsureNamespaceIsIdempotentUnderConcurrentCalls(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	filesystemPath := filepath.Join(t.TempDir(), "filesystem.db")
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.Exec("ATTACH DATABASE ? AS filesystem", filesystemPath).Error)
	require.NoError(t, persistence.AutoMigrate(db))

	repo := persistence.NewRepository(db)
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
			namespace, ensureErr := NewEnsureNamespaceHandler(repo, postgres.NewTransactor(db)).Handle(context.Background(), EnsureNamespace{
				OrganizationID: organizationID,
				WorkspaceID:    workspaceID,
				OwnerUserID:    ownerID,
				NamespaceType:  "WORKSPACE",
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
	require.NoError(t, db.Table("filesystem.file_links").Where("link_type = ? AND link_id = ?", "NAMESPACE_ROOT", ids[0]).Count(&linkCount).Error)
	require.NoError(t, db.Table("filesystem.quota_accounts").Where("workspace_id = ?", workspaceID).Count(&quotaCount).Error)
	require.Equal(t, int64(1), namespaceCount)
	require.Equal(t, int64(1), rootCount)
	require.Equal(t, int64(1), linkCount)
	require.Equal(t, int64(1), quotaCount)
}
