package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	tenantpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "list orphan workspaces without deleting")
	flag.Parse()

	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	if cfg.Database.DSN == "" {
		log.Fatal("DATABASE_DSN or config.yaml database.dsn is required")
	}

	db, err := postgres.Open(cfg.Database.DSN)
	if err != nil {
		log.Fatal(err)
	}

	store := tenantpersistence.NewTenantStore(db)
	ctx := context.Background()

	orphans, err := store.ListOrphanWorkspaces(ctx)
	if err != nil {
		log.Fatal(err)
	}

	if len(orphans) == 0 {
		fmt.Println("No orphan workspaces found.")
		return
	}

	fmt.Println("Orphan workspaces (org missing or deleted):")
	for _, row := range orphans {
		fmt.Printf("  - %s (%s) org_id=%s\n", row.Name, row.Slug, row.OrgID)
	}

	if *dryRun {
		fmt.Printf("\nDry run: %d workspace(s) would be deleted.\n", len(orphans))
		os.Exit(0)
	}

	count, err := store.PurgeOrphanWorkspaces(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nDeleted %d orphan workspace(s) and related data.\n", count)
}
