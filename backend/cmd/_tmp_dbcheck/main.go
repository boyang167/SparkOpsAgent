package main

import (
	"fmt"
	"log"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	db, err := postgres.Open(cfg.Database.DSN)
	if err != nil {
		log.Fatal(err)
	}
	type row struct {
		ID             string  `gorm:"column:id"`
		NamespaceType  string  `gorm:"column:namespace_type"`
		OrganizationID string  `gorm:"column:organization_id"`
		WorkspaceID    *string `gorm:"column:workspace_id"`
		OwnerUserID    *string `gorm:"column:owner_user_id"`
		RootNodeID     *string `gorm:"column:root_node_id"`
	}
	var rows []row
	if err := db.Raw(`SELECT id::text, namespace_type, organization_id::text, workspace_id::text, owner_user_id::text, root_node_id::text FROM filesystem.namespaces ORDER BY namespace_type, created_at NULLS LAST LIMIT 20`).Scan(&rows).Error; err != nil {
		log.Fatal(err)
	}
	fmt.Printf("count=%d\n", len(rows))
	for _, r := range rows {
		fmt.Printf("%s type=%s org=%s ws=%v owner=%v root=%v\n", r.ID, r.NamespaceType, r.OrganizationID, r.WorkspaceID, r.OwnerUserID, r.RootNodeID)
	}
	var v int
	var d bool
	_ = db.Raw(`SELECT version, dirty FROM schema_migrations`).Scan(&struct {
		Version *int
		Dirty   *bool
	}{&v, &d})
	fmt.Printf("migration version=%d dirty=%v\n", v, d)
}