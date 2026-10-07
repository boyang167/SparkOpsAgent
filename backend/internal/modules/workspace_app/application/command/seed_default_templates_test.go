package command_test

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/query"
)

type seedTestWorkspace struct {
	ID        string `gorm:"column:id;primaryKey"`
	Name      string
	DeletedAt gorm.DeletedAt
}

func (seedTestWorkspace) TableName() string { return "tenant.workspaces" }

func openSeedTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := openSyncTestDB(t)
	if err := db.Exec(`ATTACH DATABASE ':memory:' AS tenant`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&seedTestWorkspace{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func listSeedTemplates(t *testing.T, db *gorm.DB, wsID string) []syncTestTemplate {
	t.Helper()
	var rows []syncTestTemplate
	if err := db.Unscoped().Where("workspace_id = ?", wsID).Order("name").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestSeedDefaultTemplatesCreatesAllKindsForLiveWorkspaces(t *testing.T) {
	db := openSeedTestDB(t)
	ctx := context.Background()
	live := "55555555-5555-5555-5555-555555555555"
	gone := "66666666-6666-6666-6666-666666666666"
	db.Create(&seedTestWorkspace{ID: live, Name: "live"})
	db.Create(&seedTestWorkspace{ID: gone, Name: "gone", DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}})

	if err := command.SeedDefaultTemplates(ctx, db); err != nil {
		t.Fatal(err)
	}

	rows := listSeedTemplates(t, db, live)
	kinds := map[string]string{}
	for _, row := range rows {
		kinds[row.Kind] = row.Detail
		if row.Status != "active" || row.IsPublic {
			t.Fatalf("unexpected status/public for %s: %s %v", row.Name, row.Status, row.IsPublic)
		}
	}
	for _, kind := range []string{
		"Deployment", "Service", "Ingress", "ConfigMap", "Secret",
		"PersistentVolumeClaim", "PersistentVolume", "StorageClass", "StatefulSet", "Job",
		"VirtualService", "DestinationRule", "Gateway", "jenkinsFile",
	} {
		if kinds[kind] == "" {
			t.Fatalf("missing %s default template, got %v", kind, rows)
		}
	}
	if kinds["jenkinsFile"] != query.BuiltinJenkinsfileTemplateDetail() {
		t.Fatal("jenkinsfile default should use the built-in pipeline source")
	}
	if got := listSeedTemplates(t, db, gone); len(got) != 0 {
		t.Fatalf("deleted workspace should not be seeded, got %d", len(got))
	}
}

func TestSeedDefaultTemplatesIsIdempotentAndKeepsDeletions(t *testing.T) {
	db := openSeedTestDB(t)
	ctx := context.Background()
	wsID := "77777777-7777-7777-7777-777777777777"
	db.Create(&seedTestWorkspace{ID: wsID, Name: "ws"})
	legacyID := "88888888-8888-8888-8888-888888888888"
	db.Create(&syncTestTemplate{
		ID: legacyID, WorkspaceID: wsID, Name: "jenkinsfile", Kind: "jenkinsFile",
		Detail: "custom", Status: "active", CreatedBy: wsID,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	})

	if err := command.SeedDefaultTemplates(ctx, db); err != nil {
		t.Fatal(err)
	}
	want := len(listSeedTemplates(t, db, wsID))
	if want != 14 {
		t.Fatalf("legacy jenkinsfile should count as the default; want 14 rows, got %d", want)
	}

	if err := db.Where("workspace_id = ? AND name = ?", wsID, "default-ingress").Delete(&syncTestTemplate{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := command.SeedDefaultTemplates(ctx, db); err != nil {
		t.Fatal(err)
	}
	if got := len(listSeedTemplates(t, db, wsID)); got != want {
		t.Fatalf("reseed should neither duplicate nor revive deleted defaults; want %d, got %d", want, got)
	}
}
