package subscriber

import (
	"context"
	"log"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/operation"
	filesystemevent "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

const PreviewGenerationSubscriber = "filesystem.revision.generate-preview"

type revisionPreviewer interface {
	Execute(context.Context, operation.GeneratePreviewPayload) error
}

type OnRevisionCreated struct {
	preview revisionPreviewer
}

func NewOnRevisionCreated(preview revisionPreviewer) *OnRevisionCreated {
	return &OnRevisionCreated{preview: preview}
}

func (s *OnRevisionCreated) Handle(ctx context.Context, event events.Event) error {
	created, ok := event.(filesystemevent.FileRevisionCreated)
	if !ok || s == nil || s.preview == nil {
		return nil
	}
	err := s.preview.Execute(ctx, operation.GeneratePreviewPayload{
		FileID: created.FileID.String(), RevisionID: created.RevisionID.String(),
	})
	if err != nil {
		log.Printf("filesystem preview generation failed for file %s revision %s: %v", created.FileID, created.RevisionID, err)
	}
	return nil
}
