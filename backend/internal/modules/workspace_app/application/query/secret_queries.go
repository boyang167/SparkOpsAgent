package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ListSecretsHandler struct {
	repo         repository.SecretRepository
	wsChecker    port.WorkspaceChecker
	memberLookup port.MemberLookup
	crypto       *application.SecretCrypto
}

func NewListSecretsHandler(
	repo repository.SecretRepository,
	wsChecker port.WorkspaceChecker,
	memberLookup port.MemberLookup,
	crypto *application.SecretCrypto,
) *ListSecretsHandler {
	return &ListSecretsHandler{repo: repo, wsChecker: wsChecker, memberLookup: memberLookup, crypto: crypto}
}

func (h *ListSecretsHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID) ([]map[string]any, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	items, err := h.repo.ListByWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	creators, err := h.loadCreators(ctx, items)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, s := range items {
		m, err := h.secretToMap(ctx, s, creators)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

type GetSecretHandler struct {
	repo         repository.SecretRepository
	wsChecker    port.WorkspaceChecker
	memberLookup port.MemberLookup
	crypto       *application.SecretCrypto
}

func NewGetSecretHandler(
	repo repository.SecretRepository,
	wsChecker port.WorkspaceChecker,
	memberLookup port.MemberLookup,
	crypto *application.SecretCrypto,
) *GetSecretHandler {
	return &GetSecretHandler{repo: repo, wsChecker: wsChecker, memberLookup: memberLookup, crypto: crypto}
}

func (h *GetSecretHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, secretID kernel.SecretID) (map[string]any, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	s, err := h.repo.FindByID(ctx, wsID, secretID)
	if err != nil {
		return nil, err
	}
	creators, err := h.memberLookup.FindByIDs(ctx, []kernel.MemberID{s.CreatedBy})
	if err != nil {
		return nil, err
	}
	helper := &ListSecretsHandler{crypto: h.crypto, memberLookup: h.memberLookup}
	return helper.secretToMap(ctx, s, creators)
}

type TestSecretConnectionHandler struct {
	repo      repository.SecretRepository
	wsChecker port.WorkspaceChecker
	crypto    *application.SecretCrypto
}

func NewTestSecretConnectionHandler(
	repo repository.SecretRepository,
	wsChecker port.WorkspaceChecker,
	crypto *application.SecretCrypto,
) *TestSecretConnectionHandler {
	return &TestSecretConnectionHandler{repo: repo, wsChecker: wsChecker, crypto: crypto}
}

func (h *TestSecretConnectionHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, secretID kernel.SecretID) (map[string]any, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	s, err := h.repo.FindByID(ctx, wsID, secretID)
	if err != nil {
		return nil, err
	}
	if s.Status != model.SecretStatusActive {
		return map[string]any{"success": false, "message": "配置未启用"}, nil
	}
	creds, err := h.crypto.DecryptCredentials(s.CredentialBlob)
	if err != nil {
		return nil, err
	}
	success, message := testSecretConnectivity(ctx, s, creds)
	return map[string]any{"success": success, "message": message}, nil
}

func (h *ListSecretsHandler) loadCreators(ctx context.Context, items []*model.Secret) (map[kernel.MemberID]port.MemberSummary, error) {
	ids := make([]kernel.MemberID, 0, len(items))
	for _, s := range items {
		ids = append(ids, s.CreatedBy)
	}
	return h.memberLookup.FindByIDs(ctx, ids)
}

func (h *ListSecretsHandler) secretToMap(_ context.Context, s *model.Secret, creators map[kernel.MemberID]port.MemberSummary) (map[string]any, error) {
	creds, err := h.crypto.DecryptCredentials(s.CredentialBlob)
	if err != nil {
		return nil, err
	}
	flags := application.CredentialFlags(creds)
	masked := application.MaskedCredentials(creds)
	m := map[string]any{
		"id": s.ID.String(), "name": s.Name.String(), "display_name": s.DisplayName,
		"description": s.Description, "service_type": s.ServiceType, "service_url": s.ServiceURL,
		"username": s.Username, "status": string(s.Status),
		"is_default": s.IsDefault, "is_shared": s.IsShared,
		"created_by": s.CreatedBy.String(),
		"created_at": s.CreatedAt, "updated_at": s.UpdatedAt,
		"credentials": masked,
	}
	for k, v := range flags {
		m[k] = v
	}
	if c, ok := creators[s.CreatedBy]; ok {
		m["creator"] = map[string]any{
			"id": c.ID.String(), "email": c.Email, "display_name": c.DisplayName,
		}
	}
	return m, nil
}

func EnrichSecretMap(
	ctx context.Context,
	lookup port.MemberLookup,
	crypto *application.SecretCrypto,
	s *model.Secret,
) (map[string]any, error) {
	creators, err := lookup.FindByIDs(ctx, []kernel.MemberID{s.CreatedBy})
	if err != nil {
		return nil, err
	}
	h := &ListSecretsHandler{crypto: crypto, memberLookup: lookup}
	return h.secretToMap(ctx, s, creators)
}
