package main

import (
	"context"
	"fmt"
	"log"

	filesystemevent "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	fsstorage "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/storage"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"go.uber.org/zap"
)

func runWorker(ctx context.Context, cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("file-worker: nil config")
	}
	db, err := postgres.Open(cfg.Database.DSN)
	if err != nil {
		return err
	}
	if err := persistence.AutoMigrate(db); err != nil {
		return err
	}
	eventRegistry := events.NewRegistry()
	if err := filesystemevent.Register(eventRegistry); err != nil {
		return err
	}
	logger := zap.NewNop()
	factory, err := messaging.NewFromConfig(cfg, db, eventRegistry, postgres.NewTransactor(db), logger)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := factory.Close(); closeErr != nil {
			log.Printf("messaging close: %v", closeErr)
		}
	}()
	repo := persistence.NewRepository(db)
	catalog, err := fsstorage.NewRuntimeCatalog(repo, fsstorage.RuntimeCatalogConfig{
		LocalFallbackRoot: cfg.Filesystem.TransferGateway.StorageRoot,
	})
	if err != nil {
		return err
	}
	var scan port.Scanner = port.NoopScanner{}
	if err := registerFilesystemTransferWorker(db, factory.Bus, catalog, scan); err != nil {
		return err
	}
	if err := registerFilesystemPreviewWorker(db, factory.Bus, catalog, nil); err != nil {
		return err
	}
	log.Printf("file-worker subscribers registered; waiting")
	<-ctx.Done()
	return nil
}
