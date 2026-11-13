package acl

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/port"
)

func TestStoreDocumentIsIdempotentAcrossInstances(t *testing.T) {
	db := openACLDB(t)
	ctx := context.Background()
	in := port.StoreDocumentInput{
		WorkspaceID: uuid.New(),
		DocumentID:  uuid.New(),
		Title:       "intro",
		Content:     []byte("legacy knowledge body"),
		MIMEType:    "text/plain; charset=utf-8",
	}

	firstStore := NewFilesystemACL(db)
	first, err := firstStore.StoreDocument(ctx, in)
	require.NoError(t, err)

	secondStore := NewFilesystemACL(db)
	second, err := secondStore.StoreDocument(ctx, in)
	require.NoError(t, err)
	require.Equal(t, first.FileID, second.FileID)

	count, err := secondStore.CountFilesForDocument(ctx, in.DocumentID)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	got, err := secondStore.ReadDocumentBytes(ctx, second.FileID)
	require.NoError(t, err)
	require.Equal(t, in.Content, got)
}

func openACLDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS knowledge").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	return db
}
