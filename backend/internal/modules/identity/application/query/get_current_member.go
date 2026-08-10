package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type MemberView struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	DisplayName  string `json:"display_name"`
	Status       string `json:"status"`
	IsSuperAdmin bool   `json:"is_super_admin"`
}

type GetCurrentMemberHandler struct {
	repo         repository.MemberRepository
	systemRoles  *SystemRoleChecker
}

func NewGetCurrentMemberHandler(repo repository.MemberRepository, systemRoles *SystemRoleChecker) *GetCurrentMemberHandler {
	return &GetCurrentMemberHandler{repo: repo, systemRoles: systemRoles}
}

func (h *GetCurrentMemberHandler) Handle(ctx context.Context, memberID kernel.MemberID) (*MemberView, error) {
	member, err := h.repo.FindByID(ctx, memberID)
	if err != nil {
		return nil, err
	}
	isSuperAdmin := false
	if h.systemRoles != nil {
		isSuperAdmin, err = h.systemRoles.IsSuperAdmin(ctx, memberID)
		if err != nil {
			return nil, err
		}
	}
	return &MemberView{
		ID:           member.ID.String(),
		Email:        member.Email,
		DisplayName:  member.DisplayName,
		Status:       string(member.Status),
		IsSuperAdmin: isSuperAdmin,
	}, nil
}
