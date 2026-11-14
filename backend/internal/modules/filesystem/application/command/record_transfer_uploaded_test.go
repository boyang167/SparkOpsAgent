package command

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	sharedevents "github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

func TestRecordTransferUploadedPublishesInsideTransaction(t *testing.T) {
	db, namespace := newTransferTestDB(t)
	created, err := newTransferHandler(db).Handle(context.Background(), transferFixture(namespace))
	require.NoError(t, err)
	bus := &transactionRequiredBus{}
	handler := NewRecordTransferUploadedHandler(
		persistence.NewRepository(db),
		postgres.NewTransactor(db),
		bus,
	)

	err = handler.Handle(context.Background(), RecordTransferUploaded{
		SessionID:   created.SessionID,
		WorkspaceID: namespace.WorkspaceID,
		Size:        60,
		SHA256:      strings64("a"),
	})

	require.NoError(t, err)
	require.True(t, bus.published)
}

type transactionRequiredBus struct {
	published bool
}

func (b *transactionRequiredBus) Publish(ctx context.Context, events ...sharedevents.Event) error {
	if _, err := postgres.RequireTx(ctx); err != nil {
		return err
	}
	b.published = len(events) > 0
	return nil
}

func (*transactionRequiredBus) Subscribe(string, string, messaging.Handler) error {
	return nil
}

var _ messaging.Bus = (*transactionRequiredBus)(nil)
