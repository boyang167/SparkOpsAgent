package command

import (
	"context"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type LoginInput struct {
	Email    string
	Password string
}

type LoginResult struct {
	Token    string
	MemberID kernel.MemberID
	Email    string
}

type LoginHandler struct {
	repo    repository.MemberRepository
	hasher  model.PasswordHasher
	tokens  *authn.TokenProvider
}

func NewLoginHandler(repo repository.MemberRepository, tokens *authn.TokenProvider) *LoginHandler {
	return &LoginHandler{
		repo:   repo,
		hasher: model.BcryptHasher{},
		tokens: tokens,
	}
}

func (h *LoginHandler) Handle(ctx context.Context, in LoginInput) (*LoginResult, error) {
	email := strings.TrimSpace(strings.ToLower(in.Email))
	member, err := h.repo.FindByEmail(ctx, email)
	if err != nil {
		if de, ok := kernel.IsDomainError(err); ok && de.Code() == domain.ErrMemberNotFound.Code() {
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}
	if err := member.Authenticate(in.Password, h.hasher); err != nil {
		return nil, err
	}
	token, err := h.tokens.Issue(member.ID)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		Token:    token,
		MemberID: member.ID,
		Email:    member.Email,
	}, nil
}
