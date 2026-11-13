package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

type CapabilityRepository interface {
	SaveCapability(context.Context, model.CapabilityRecord) error
	GetCapability(context.Context, uuid.UUID) (model.CapabilityRecord, error)
	GetCapabilityByHash(context.Context, string) (model.CapabilityRecord, error)
	RevokeCapability(context.Context, uuid.UUID, time.Time) error
	SaveExportTask(context.Context, model.ExportTask) error
	GetExportTask(context.Context, uuid.UUID) (model.ExportTask, error)
	MarkExportImported(context.Context, uuid.UUID, uuid.UUID) error
}
