package subscriber

import (
	"context"
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/domain/model"
	workspaceevent "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectReviewRecipientLister interface {
	ListProjectReviewRecipients(
		ctx context.Context,
		wsID kernel.WorkspaceID,
		excludeMemberID kernel.MemberID,
	) ([]kernel.MemberID, error)
}

type OnProjectReviewSubmitted struct {
	create     *command.CreateNotificationHandler
	recipients ProjectReviewRecipientLister
}

func NewOnProjectReviewSubmitted(
	create *command.CreateNotificationHandler,
	recipients ProjectReviewRecipientLister,
) *OnProjectReviewSubmitted {
	return &OnProjectReviewSubmitted{create: create, recipients: recipients}
}

func (s *OnProjectReviewSubmitted) Handle(ctx context.Context, e events.Event) error {
	ev, ok := e.(workspaceevent.ProjectReviewSubmitted)
	if !ok {
		return nil
	}

	memberIDs, err := s.recipients.ListProjectReviewRecipients(ctx, ev.WorkspaceID, ev.SubmittedBy)
	if err != nil {
		return err
	}
	if len(memberIDs) == 0 {
		return nil
	}

	link := fmt.Sprintf("/projects/%s", ev.ProjectID.String())
	title := fmt.Sprintf("项目「%s」待审核", ev.DisplayName)
	body := "有新的项目提交审核，请尽快处理。"

	for _, memberID := range memberIDs {
		if _, err := s.create.Handle(
			ctx,
			memberID,
			ev.WorkspaceID,
			model.NotificationTypeProjectReviewPending,
			title,
			body,
			link,
		); err != nil {
			return err
		}
	}
	return nil
}
