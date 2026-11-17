package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/port"
	fsstorage "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/storage"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: replay-transfer <session-id>")
	}
	sessionID, err := uuid.Parse(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	db, err := postgres.Open(cfg.Database.DSN)
	if err != nil {
		log.Fatal(err)
	}
	repo := persistence.NewRepository(db)
	tx := postgres.NewTransactor(db)
	catalog, err := fsstorage.NewRuntimeCatalog(repo, fsstorage.RuntimeCatalogConfig{
		LocalFallbackRoot: cfg.Filesystem.TransferGateway.StorageRoot,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = catalog.Close() }()

	ctx := context.Background()
	session, err := repo.GetTransferByID(ctx, sessionID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("before: status=%s version=%d\n", session.Status(), session.Version())
	size, hasSize := session.ActualSize()
	fmt.Printf("actual_size present=%v value=%d\n", hasSize, size)

	verify := command.NewVerifyTransferHandler(repo, tx, catalog, port.NoopScanner{})
	commit := command.NewCommitTransferHandler(repo, tx, nil)

	if err := verify.Handle(ctx, command.VerifyTransfer{
		SessionID:   sessionID,
		WorkspaceID: session.WorkspaceID(),
	}); err != nil {
		fmt.Printf("verify error: %v\n", err)
	} else {
		fmt.Println("verify: ok")
	}

	session, err = repo.GetTransferByID(ctx, sessionID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("after verify: status=%s version=%d\n", session.Status(), session.Version())

	result, err := commit.Handle(ctx, command.CommitTransfer{
		SessionID:   sessionID,
		WorkspaceID: session.WorkspaceID(),
	})
	if err != nil {
		fmt.Printf("commit error: %v\n", err)
		fmt.Printf("  persistence=%v name_conflict=%v\n",
			errors.Is(err, domain.ErrPersistence), errors.Is(err, domain.ErrNodeNameConflict))
		os.Exit(1)
	}
	fmt.Printf("commit ok: node=%s revision=%s\n", result.NodeID, result.RevisionID)
}