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
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func main() {
	batchSize := flag.Int("batch-size", 100, "number of documents per batch")
	afterID := flag.String("after-id", "", "resume after document id")
	dryRun := flag.Bool("dry-run", false, "analyze without writing files")
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
	runner := migration.NewPrecopyRunner(repo, files)

	var cursor *kernel.DocumentID
	if *afterID != "" {
		id, err := kernel.ParseDocumentID(*afterID)
		if err != nil {
			log.Fatal(err)
		}
		cursor = &id
	}
	ctx := migration.WithDryRun(context.Background(), *dryRun)
	n, err := runner.RunBatch(ctx, migration.PrecopyOptions{
		BatchSize: *batchSize,
		AfterID:   cursor,
		DryRun:    *dryRun,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("precopy processed %d documents (dry_run=%v)", n, *dryRun)
}
