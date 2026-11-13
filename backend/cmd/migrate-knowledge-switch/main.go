package main

import (
	"context"
	"flag"
	"log"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/migration"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

func main() {
	rollback := flag.Bool("rollback", false, "switch reads back to legacy content without deleting files")
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
	repo := persistence.NewKnowledgeRepository(db)
	files := acl.NewFilesystemACL(db)
	runner := migration.NewSwitchRunner(repo, files)
	ctx := context.Background()
	if *rollback {
		if err := runner.Rollback(ctx); err != nil {
			log.Fatal(err)
		}
		log.Println("knowledge migration rolled back to legacy reads")
		return
	}
	if err := runner.FreezeSwitch(ctx); err != nil {
		log.Fatal(err)
	}
	log.Println("knowledge migration switched to filesystem reads")
}
