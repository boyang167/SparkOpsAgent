package event

import (
	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

const (
	FileCreatedName         = "filesystem.file.created.v1"
	FileRevisionCreatedName = "filesystem.file.revision-created.v1"
)

type FileCreated struct {
	events.BaseEvent
	FileID      uuid.UUID `json:"file_id"`
	NamespaceID uuid.UUID `json:"namespace_id"`
}

type FileRevisionCreated struct {
	events.BaseEvent
	FileID     uuid.UUID `json:"file_id"`
	RevisionID uuid.UUID `json:"revision_id"`
	RevisionNo int       `json:"revision_no"`
}

func Register(registry *events.Registry) error {
	if err := events.RegisterJSON[FileCreated](
		registry,
		FileCreatedName,
		1,
		func(ev FileCreated) events.AggregateRef {
			return events.AggregateRef{Type: "file", ID: ev.FileID.String()}
		},
	); err != nil {
		return err
	}
	return events.RegisterJSON[FileRevisionCreated](
		registry,
		FileRevisionCreatedName,
		1,
		func(ev FileRevisionCreated) events.AggregateRef {
			return events.AggregateRef{Type: "file", ID: ev.FileID.String()}
		},
	)
}
