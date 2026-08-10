package command

import (
	"context"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type CreateAPITokenResult struct {
	ID          string
	Token       string
	TokenPrefix string
	ExpiresAt   *time.Time
}

type CreateAPITokenHandler struct {
	tokens   *authn.TokenProvider
	store    *persistence.APITokenStore
	roles    repository.RoleRepository
}

func NewCreateAPITokenHandler(
	tokens *authn.TokenProvider,
	store *persistence.APITokenStore,
	roles repository.RoleRepository,
) *CreateAPITokenHandler {
	return &CreateAPITokenHandler{tokens: tokens, store: store, roles: roles}
}

func (h *CreateAPITokenHandler) Handle(
	ctx context.Context,
	workspaceID kernel.WorkspaceID,
	memberID kernel.MemberID,
	roleID *kernel.RoleID,
	remark string,
	expireDays int,
) (*CreateAPITokenResult, error) {
	var roleIDStr *string
	if roleID != nil {
		role, err := h.roles.FindByID(ctx, *roleID)
		if err != nil {
			return nil, err
		}
		if role.WorkspaceID != workspaceID {
			return nil, domain.ErrRoleNotFound
		}
		v := roleID.String()
		roleIDStr = &v
	}
	expireHours := expireDays * 24
	if expireDays <= 0 {
		expireHours = 24 * 365 * 10
	}
	token, err := h.tokens.IssueScoped(memberID, workspaceID, roleID, expireHours)
	if err != nil {
		return nil, err
	}
	prefix := token
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	id := persistence.NewAPITokenID()
	ws := workspaceID.String()
	var expiresAt *time.Time
	if expireDays > 0 {
		t := time.Now().UTC().Add(time.Duration(expireDays) * 24 * time.Hour)
		expiresAt = &t
	}
	if err := h.store.Create(ctx, persistence.APITokenRecord{
		ID: id, MemberID: memberID.String(), WorkspaceID: &ws, RoleID: roleIDStr,
		TokenPrefix: prefix, Status: "active", ExpiresAt: expiresAt, Remark: remark,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		return nil, err
	}
	return &CreateAPITokenResult{ID: id, Token: token, TokenPrefix: prefix, ExpiresAt: expiresAt}, nil
}

type RevokeAPITokenHandler struct {
	store  *persistence.APITokenStore
	tokens *authn.TokenProvider
}

func NewRevokeAPITokenHandler(store *persistence.APITokenStore, tokens *authn.TokenProvider) *RevokeAPITokenHandler {
	return &RevokeAPITokenHandler{store: store, tokens: tokens}
}

func (h *RevokeAPITokenHandler) Handle(ctx context.Context, tokenID string, rawToken string) error {
	if rawToken != "" {
		if err := h.store.Blacklist(ctx, rawToken); err != nil {
			return err
		}
	}
	_, err := h.store.Revoke(ctx, tokenID)
	return err
}
