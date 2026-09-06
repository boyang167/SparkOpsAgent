package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/query"
)

type syncTestTemplate struct {
	ID          string `gorm:"column:id;primaryKey"`
	WorkspaceID string
	Name        string
	Kind        string
	Language    string
	Detail      string `gorm:"type:text"`
	Rules       string `gorm:"type:text"`
	Status      string
	IsPublic    bool
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt
}

func (syncTestTemplate) TableName() string { return "workspace_app.templates" }

func openSyncTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`ATTACH DATABASE ':memory:' AS workspace_app`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&syncTestTemplate{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestSyncBuiltinTemplatesUpgradesLegacyJenkinsfile(t *testing.T) {
	db := openSyncTestDB(t)
	ctx := context.Background()
	id := "11111111-1111-1111-1111-111111111111"
	wsID := "22222222-2222-2222-2222-222222222222"
	legacy := `pipeline { agent any stages { stage('Build') { steps { checkout scm } } } }`
	if err := db.Create(&syncTestTemplate{
		ID: id, WorkspaceID: wsID, Name: "jenkinsfile", Kind: "jenkinsFile",
		Language: "groovy", Detail: legacy, Status: "active", IsPublic: true, CreatedBy: wsID,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := command.SyncBuiltinTemplates(ctx, db); err != nil {
		t.Fatal(err)
	}

	var row syncTestTemplate
	if err := db.First(&row, "id = ?", id).Error; err != nil {
		t.Fatal(err)
	}
	if row.Detail != query.BuiltinJenkinsfileTemplateDetail() {
		t.Fatalf("detail not upgraded:\n%s", row.Detail)
	}
}

func TestSyncBuiltinTemplatesSkipsAlreadyUpgraded(t *testing.T) {
	db := openSyncTestDB(t)
	ctx := context.Background()
	id := "33333333-3333-3333-3333-333333333333"
	wsID := "44444444-4444-4444-4444-444444444444"
	custom := query.BuiltinJenkinsfileTemplateDetail() + "\n// custom marker"
	if err := db.Create(&syncTestTemplate{
		ID: id, WorkspaceID: wsID, Name: "jenkinsfile", Kind: "jenkinsFile",
		Language: "groovy", Detail: custom, Status: "active", IsPublic: true, CreatedBy: wsID,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := command.SyncBuiltinTemplates(ctx, db); err != nil {
		t.Fatal(err)
	}

	var row syncTestTemplate
	if err := db.First(&row, "id = ?", id).Error; err != nil {
		t.Fatal(err)
	}
	if row.Detail != custom {
		t.Fatalf("custom template was overwritten:\n%s", row.Detail)
	}
}
