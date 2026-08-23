package query

import (
	"context"
	"errors"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type OrganizationView struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	MyRole   string `json:"my_role,omitempty"`
	IsMember bool   `json:"is_member"`
	ReadOnly bool   `json:"read_only"`
}

type ListOrganizationsHandler struct {
	repo         repository.OrganizationRepository
	systemRoles  authz.SystemRoleChecker
}

func NewListOrganizationsHandler(
	repo repository.OrganizationRepository,
	systemRoles authz.SystemRoleChecker,
) *ListOrganizationsHandler {
	return &ListOrganizationsHandler{repo: repo, systemRoles: systemRoles}
}

func (h *ListOrganizationsHandler) Handle(ctx context.Context, memberID kernel.MemberID) ([]OrganizationView, error) {
	isSuper, err := h.systemRoles.IsSuperAdmin(ctx, memberID)
	if err != nil {
		return nil, err
	}
	if isSuper {
		return h.listAllForSuperAdmin(ctx, memberID)
	}
	return h.listByMember(ctx, memberID)
}

func (h *ListOrganizationsHandler) listByMember(ctx context.Context, memberID kernel.MemberID) ([]OrganizationView, error) {
	items, err := h.repo.ListByMember(ctx, memberID)
	if err != nil {
		return nil, err
	}
	out := make([]OrganizationView, 0, len(items))
	for _, org := range items {
		role, _ := h.repo.GetMemberRole(ctx, org.ID, memberID)
		out = append(out, OrganizationView{
			ID: org.ID.String(), Name: org.Name, Slug: org.Slug,
			MyRole: role, IsMember: true, ReadOnly: false,
		})
	}
	return out, nil
}

func (h *ListOrganizationsHandler) listAllForSuperAdmin(ctx context.Context, memberID kernel.MemberID) ([]OrganizationView, error) {
	items, err := h.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]OrganizationView, 0, len(items))
	for _, org := range items {
		view := OrganizationView{
			ID: org.ID.String(), Name: org.Name, Slug: org.Slug,
		}
		isMember, err := h.repo.IsMember(ctx, org.ID, memberID)
		if err != nil {
			return nil, err
		}
		if isMember {
			role, roleErr := h.repo.GetMemberRole(ctx, org.ID, memberID)
			if roleErr != nil && !errors.Is(roleErr, domain.ErrOrgMemberNotFound) {
				return nil, roleErr
			}
			view.MyRole = role
			view.IsMember = true
			view.ReadOnly = false
		} else {
			view.IsMember = false
			view.ReadOnly = true
		}
		out = append(out, view)
	}
	return out, nil
}
