package query

import (
	"context"
	"time"

	"github.com/google/uuid"

	resourceauth "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/authorization"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
)

type SharedNode = repository.SharedNode

type SharedWithMeFilter = repository.SharedNodeFilter

type SharedWithMe struct {
	ActorID        uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	Query          string
	Cursor         string
	Limit          int
}

type SharedWithMeRepository interface {
	ListSharedNodes(context.Context, uuid.UUID, []repository.PrincipalRef, repository.SharedNodeFilter, time.Time) ([]SharedNode, string, error)
}

type SharedWithMeHandler struct {
	repo       SharedWithMeRepository
	principals resourceauth.PrincipalResolver
	now        func() time.Time
}

func NewSharedWithMeHandler(repo SharedWithMeRepository, principals resourceauth.PrincipalResolver) *SharedWithMeHandler {
	return &SharedWithMeHandler{repo: repo, principals: principals, now: func() time.Time { return time.Now().UTC() }}
}

func (h *SharedWithMeHandler) Handle(ctx context.Context, q SharedWithMe) ([]SharedNode, string, error) {
	principals, err := h.principals.Resolve(ctx, resourceauth.ResolvePrincipalsInput{
		ActorID: q.ActorID, OrganizationID: q.OrganizationID, WorkspaceID: q.WorkspaceID,
	})
	if err != nil {
		return nil, "", err
	}
	filter := SharedWithMeFilter{
		Query: q.Query, Cursor: q.Cursor, Limit: q.Limit, Principals: principals,
	}
	return h.repo.ListSharedNodes(ctx, q.OrganizationID, principals, filter, h.now())
}
