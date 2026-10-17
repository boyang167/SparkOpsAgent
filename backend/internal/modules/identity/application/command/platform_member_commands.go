package command

import (
	"context"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type CreatePlatformMemberInput struct {
	Email        string
	Password     string
	DisplayName  string
	IsSuperAdmin bool
}

type CreatePlatformMemberHandler struct {
	repo        repository.MemberRepository
	systemRoles *persistence.SystemRoleStore
	register    *RegisterMemberHandler
}

func NewCreatePlatformMemberHandler(
	repo repository.MemberRepository,
	systemRoles *persistence.SystemRoleStore,
	bus eventbus.Bus,
	tx postgres.Transactor,
) *CreatePlatformMemberHandler {
	return &CreatePlatformMemberHandler{
		repo:        repo,
		systemRoles: systemRoles,
		register:    NewRegisterMemberHandler(repo, bus, tx),
	}
}

func (h *CreatePlatformMemberHandler) Handle(ctx context.Context, in CreatePlatformMemberInput) (*RegisterMemberResult, error) {
	res, err := h.register.Handle(ctx, RegisterMemberInput{
		Email: in.Email, Password: in.Password, DisplayName: in.DisplayName,
	})
	if err != nil {
		return nil, err
	}
	if in.IsSuperAdmin {
		if err := h.systemRoles.Grant(ctx, res.MemberID, authz.SystemRoleSuperAdmin); err != nil {
			return nil, err
		}
	}
	return res, nil
}

type UpdatePlatformMemberInput struct {
	DisplayName  *string
	Status       *string
	IsSuperAdmin *bool
}

type UpdatePlatformMemberHandler struct {
	repo        repository.MemberRepository
	systemRoles *persistence.SystemRoleStore
	hasher      model.PasswordHasher
}

func NewUpdatePlatformMemberHandler(
	repo repository.MemberRepository,
	systemRoles *persistence.SystemRoleStore,
) *UpdatePlatformMemberHandler {
	return &UpdatePlatformMemberHandler{repo: repo, systemRoles: systemRoles, hasher: model.BcryptHasher{}}
}

func (h *UpdatePlatformMemberHandler) Handle(
	ctx context.Context,
	actorID kernel.MemberID,
	targetID kernel.MemberID,
	in UpdatePlatformMemberInput,
) error {
	if actorID == targetID && (in.Status != nil || in.IsSuperAdmin != nil) {
		return domain.ErrCannotModifySelf
	}
	member, err := h.repo.FindByID(ctx, targetID)
	if err != nil {
		return err
	}
	if in.DisplayName != nil {
		if err := member.UpdateDisplayName(*in.DisplayName); err != nil {
			return err
		}
	}
	if in.Status != nil {
		switch strings.ToLower(strings.TrimSpace(*in.Status)) {
		case string(model.MemberStatusActive):
			member.SetStatus(model.MemberStatusActive)
		case string(model.MemberStatusDisabled):
			member.SetStatus(model.MemberStatusDisabled)
		default:
			return domain.ErrInvalidProfile
		}
	}
	if err := h.repo.Save(ctx, member); err != nil {
		return err
	}
	if in.IsSuperAdmin == nil {
		return nil
	}
	return h.setSuperAdmin(ctx, actorID, targetID, *in.IsSuperAdmin)
}

func (h *UpdatePlatformMemberHandler) setSuperAdmin(ctx context.Context, actorID, targetID kernel.MemberID, grant bool) error {
	isSuper, err := h.systemRoles.IsSuperAdmin(ctx, targetID)
	if err != nil {
		return err
	}
	if grant && !isSuper {
		return h.systemRoles.Grant(ctx, targetID, authz.SystemRoleSuperAdmin)
	}
	if !grant && isSuper {
		if actorID == targetID {
			return domain.ErrCannotModifySelf
		}
		count, err := h.systemRoles.CountByRole(ctx, authz.SystemRoleSuperAdmin)
		if err != nil {
			return err
		}
		if count <= 1 {
			return domain.ErrLastSuperAdmin
		}
		return h.systemRoles.Revoke(ctx, targetID, authz.SystemRoleSuperAdmin)
	}
	return nil
}

type AdminResetPasswordHandler struct {
	repo   repository.MemberRepository
	hasher model.PasswordHasher
}

func NewAdminResetPasswordHandler(repo repository.MemberRepository) *AdminResetPasswordHandler {
	return &AdminResetPasswordHandler{repo: repo, hasher: model.BcryptHasher{}}
}

type DeletePlatformMemberHandler struct {
	repo        repository.MemberRepository
	systemRoles *persistence.SystemRoleStore
}

func NewDeletePlatformMemberHandler(
	repo repository.MemberRepository,
	systemRoles *persistence.SystemRoleStore,
) *DeletePlatformMemberHandler {
	return &DeletePlatformMemberHandler{repo: repo, systemRoles: systemRoles}
}

func (h *DeletePlatformMemberHandler) Handle(
	ctx context.Context,
	actorID kernel.MemberID,
	targetID kernel.MemberID,
) error {
	if actorID == targetID {
		return domain.ErrCannotDeleteSelf
	}
	if _, err := h.repo.FindByID(ctx, targetID); err != nil {
		return err
	}
	isSuper, err := h.systemRoles.IsSuperAdmin(ctx, targetID)
	if err != nil {
		return err
	}
	if isSuper {
		count, err := h.systemRoles.CountByRole(ctx, authz.SystemRoleSuperAdmin)
		if err != nil {
			return err
		}
		if count <= 1 {
			return domain.ErrLastSuperAdmin
		}
	}
	return h.repo.SoftDelete(ctx, targetID)
}

func (h *AdminResetPasswordHandler) Handle(ctx context.Context, targetID kernel.MemberID, newPassword string) error {
	member, err := h.repo.FindByID(ctx, targetID)
	if err != nil {
		return err
	}
	if err := member.ResetPassword(newPassword, h.hasher); err != nil {
		return err
	}
	return h.repo.Save(ctx, member)
}
