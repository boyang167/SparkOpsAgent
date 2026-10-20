package query

import (
	"context"
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type stubConversationRepo struct {
	byID map[kernel.ConversationID]*model.Conversation
}

func (s stubConversationRepo) FindByID(_ context.Context, id kernel.ConversationID) (*model.Conversation, error) {
	item, ok := s.byID[id]
	if !ok {
		return nil, domain.ErrConversationNotFound
	}
	return item, nil
}

func (s stubConversationRepo) ListByAgent(context.Context, kernel.AgentID, kernel.MemberID, kernel.WorkspaceID) ([]*model.Conversation, error) {
	return nil, nil
}

func (s stubConversationRepo) Save(context.Context, *model.Conversation) error { return nil }

type stubRunRepo struct {
	byID map[kernel.RunID]*model.Run
}

func (s stubRunRepo) FindByID(_ context.Context, id kernel.RunID) (*model.Run, error) {
	item, ok := s.byID[id]
	if !ok {
		return nil, domain.ErrRunNotFound
	}
	return item, nil
}

func (s stubRunRepo) Save(context.Context, *model.Run) error { return nil }

func TestGetChatRunHandlerDeniesOtherMember(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberA := kernel.NewMemberID()
	memberB := kernel.NewMemberID()
	convID := kernel.NewConversationID()
	runID := kernel.NewRunID()

	conversations := stubConversationRepo{byID: map[kernel.ConversationID]*model.Conversation{
		convID: {
			ID:          convID,
			WorkspaceID: wsID,
			MemberID:    memberA,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}}
	runs := stubRunRepo{byID: map[kernel.RunID]*model.Run{
		runID: {
			ID:             runID,
			ConversationID: convID,
			Status:         model.RunStatusCompleted,
			Output:         "hello",
		},
	}}

	handler := NewGetChatRunHandler(conversations, runs)
	_, err := handler.Handle(context.Background(), wsID, memberB, runID)
	if err != domain.ErrAccessDenied {
		t.Fatalf("expected access denied, got %v", err)
	}
}

func TestGetChatRunHandlerReturnsRunForOwner(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	convID := kernel.NewConversationID()
	runID := kernel.NewRunID()

	conversations := stubConversationRepo{byID: map[kernel.ConversationID]*model.Conversation{
		convID: {ID: convID, WorkspaceID: wsID, MemberID: memberID},
	}}
	runs := stubRunRepo{byID: map[kernel.RunID]*model.Run{
		runID: {
			ID:             runID,
			ConversationID: convID,
			Status:         model.RunStatusCompleted,
			Output:         "done",
		},
	}}

	handler := NewGetChatRunHandler(conversations, runs)
	out, err := handler.Handle(context.Background(), wsID, memberID, runID)
	if err != nil {
		t.Fatal(err)
	}
	if out["reply"] != "done" || out["status"] != "completed" {
		t.Fatalf("unexpected payload: %+v", out)
	}
}
