package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ChangePasswordHandler struct {
	repo   repository.MemberRepository
	hasher model.PasswordHasher
}

func NewChangePasswordHandler(repo repository.MemberRepository, hasher model.PasswordHasher) *ChangePasswordHandler {
	return &ChangePasswordHandler{repo: repo, hasher: hasher}
}

func (h *ChangePasswordHandler) Handle(ctx context.Context, memberID kernel.MemberID, currentPassword, newPassword string) error {
	member, err := h.repo.FindByID(ctx, memberID)
	if err != nil {
		return err
	}
	if err := member.ChangePassword(currentPassword, newPassword, h.hasher); err != nil {
		return err
	}
	return h.repo.Save(ctx, member)
}
