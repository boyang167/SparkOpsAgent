package main

import (
	"context"
	"log"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/migration"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

func main() {
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
	runner := migration.NewInventoryRunner(repo)
	if err := runner.PrintJSON(context.Background()); err != nil {
		log.Fatal(err)
	}
}
