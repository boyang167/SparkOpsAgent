package main

import (
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/subscriber"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"gorm.io/gorm"
)

func registerFilesystemTransferWorker(
	db *gorm.DB,
	bus messaging.Bus,
	providers command.StorageProviderCatalog,
	scanner port.Scanner,
) error {
	return subscriber.RegisterVerifyCommit(db, bus, providers, scanner)
}
