package subscriber_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/application/subscriber"
	notificationpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/notification/infrastructure/persistence"
	workspaceevent "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type stubReviewRecipients struct {
	ids []kernel.MemberID
}

func (s stubReviewRecipients) ListProjectReviewRecipients(
	_ context.Context,
	_ kernel.WorkspaceID,
	_ kernel.MemberID,
) ([]kernel.MemberID, error) {
	return s.ids, nil
}

func TestOnProjectReviewSubmittedNotifiesReviewers(t *testing.T) {
	db := openNotificationInboxDB(t)
	repo := notificationpersistence.NewNotificationRepository(db)
	create := command.NewCreateNotificationHandler(repo)
	handler := subscriber.NewOnProjectReviewSubmitted(create, stubReviewRecipients{
		ids: []kernel.MemberID{kernel.NewMemberID(), kernel.NewMemberID()},
	})

	wsID := kernel.NewWorkspaceID()
	projectID := kernel.NewProjectID()
	submitter := kernel.NewMemberID()
	ev := workspaceevent.NewProjectReviewSubmitted(projectID, wsID, "test-repo2", "test-repo2", submitter)
	require.NoError(t, handler.Handle(context.Background(), ev))

	var count int64
	require.NoError(t, db.Table("notification.notifications").Count(&count).Error)
	require.Equal(t, int64(2), count)
}
