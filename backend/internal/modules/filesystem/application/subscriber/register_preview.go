package subscriber

import (
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/operation"
	filesystemevent "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"gorm.io/gorm"
)

func RegisterPreviewGeneration(
	db *gorm.DB,
	bus messaging.Bus,
	providers command.StorageProviderCatalog,
	previewer port.Previewer,
) error {
	if db == nil || bus == nil {
		return fmt.Errorf("filesystem preview worker requires database and event bus")
	}
	if providers == nil {
		return fmt.Errorf("filesystem preview worker requires storage providers")
	}
	if previewer == nil {
		previewer = port.NoopPreviewer{}
	}
	repo := persistence.NewRepository(db)
	preview := operation.NewGeneratePreviewHandler(repo, providers, previewer)
	return bus.Subscribe(
		filesystemevent.FileRevisionCreatedName,
		PreviewGenerationSubscriber,
		NewOnRevisionCreated(preview).Handle,
	)
}
