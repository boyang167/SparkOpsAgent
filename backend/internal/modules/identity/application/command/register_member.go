package command

import (
	"context"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type RegisterMemberInput struct {
	Email       string
	Password    string
	DisplayName string
}

type RegisterMemberResult struct {
	MemberID kernel.MemberID
	Email    string
}

type RegisterMemberHandler struct {
	repo     repository.MemberRepository
	hasher   model.PasswordHasher
	eventBus eventbus.Bus
	tx       postgres.Transactor
}

func NewRegisterMemberHandler(repo repository.MemberRepository, bus eventbus.Bus, tx postgres.Transactor) *RegisterMemberHandler {
	return &RegisterMemberHandler{
		repo:     repo,
		hasher:   model.BcryptHasher{},
		eventBus: bus,
		tx:       tx,
	}
}

func (h *RegisterMemberHandler) Handle(ctx context.Context, in RegisterMemberInput) (*RegisterMemberResult, error) {
	email := strings.TrimSpace(strings.ToLower(in.Email))
	if email == "" || in.Password == "" || strings.TrimSpace(in.DisplayName) == "" {
		return nil, kernel.NewDomainError("IDENTITY_INVALID_INPUT", "email, password and display_name are required")
	}
	exists, err := h.repo.ExistsByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, domain.ErrEmailAlreadyExists
	}
	hash, err := h.hasher.Hash(in.Password)
	if err != nil {
		return nil, err
	}
	member := model.NewMember(email, strings.TrimSpace(in.DisplayName), hash)
	var result *RegisterMemberResult
	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := h.repo.Save(txCtx, member); err != nil {
			return err
		}
		if err := h.eventBus.Publish(txCtx, event.NewMemberRegistered(member.ID, member.Email)); err != nil {
			return err
		}
		result = &RegisterMemberResult{MemberID: member.ID, Email: member.Email}
		return nil
	})
	return result, err
}
