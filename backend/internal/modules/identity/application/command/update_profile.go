package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type UpdateProfileHandler struct {
	repo repository.MemberRepository
}

func NewUpdateProfileHandler(repo repository.MemberRepository) *UpdateProfileHandler {
	return &UpdateProfileHandler{repo: repo}
}

func (h *UpdateProfileHandler) Handle(ctx context.Context, memberID kernel.MemberID, displayName string) (*model.Member, error) {
	member, err := h.repo.FindByID(ctx, memberID)
	if err != nil {
		return nil, err
	}
	if err := member.UpdateDisplayName(displayName); err != nil {
		return nil, err
	}
	if err := h.repo.Save(ctx, member); err != nil {
		return nil, err
	}
	return member, nil
}
