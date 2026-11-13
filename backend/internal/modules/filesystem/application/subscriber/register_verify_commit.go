package subscriber

import (
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	filesystemevent "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"gorm.io/gorm"
)

func RegisterVerifyCommit(
	db *gorm.DB,
	bus messaging.Bus,
	providers command.StorageProviderCatalog,
	scanner port.Scanner,
) error {
	if db == nil || bus == nil {
		return fmt.Errorf("filesystem transfer worker requires database and event bus")
	}
	if providers == nil {
		return fmt.Errorf("filesystem transfer worker requires storage providers")
	}
	if scanner == nil {
		scanner = port.NoopScanner{}
	}
	repo := persistence.NewRepository(db)
	tx := postgres.NewTransactor(db)
	verify := command.NewVerifyTransferHandler(repo, tx, providers, scanner)
	commit := command.NewCommitTransferHandler(repo, tx, bus)
	return bus.Subscribe(
		filesystemevent.TransferUploadedName,
		TransferVerifyCommitSubscriber,
		NewOnTransferUploaded(verify, commit, repo).Handle,
	)
}
