package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/infrastructure/persistence"
)

type PlatformMemberView struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	DisplayName  string `json:"display_name"`
	Status       string `json:"status"`
	IsSuperAdmin bool   `json:"is_super_admin"`
	CreatedAt    string `json:"created_at"`
}

type ListPlatformMembersHandler struct {
	repo        repository.MemberRepository
	systemRoles *persistence.SystemRoleStore
}

func NewListPlatformMembersHandler(repo repository.MemberRepository, systemRoles *persistence.SystemRoleStore) *ListPlatformMembersHandler {
	return &ListPlatformMembersHandler{repo: repo, systemRoles: systemRoles}
}

func (h *ListPlatformMembersHandler) Handle(ctx context.Context) ([]PlatformMemberView, error) {
	members, err := h.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	superAdmins, err := h.systemRoles.ListSuperAdminMemberIDs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PlatformMemberView, 0, len(members))
	for _, m := range members {
		_, isSuper := superAdmins[m.ID.String()]
		out = append(out, PlatformMemberView{
			ID:           m.ID.String(),
			Email:        m.Email,
			DisplayName:  m.DisplayName,
			Status:       string(m.Status),
			IsSuperAdmin: isSuper,
			CreatedAt:    m.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return out, nil
}
