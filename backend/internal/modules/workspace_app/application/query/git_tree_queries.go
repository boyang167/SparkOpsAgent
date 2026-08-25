package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	gitrefs "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/git"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type GitTreeEntryView struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
}

type ListGitTreeHandler struct {
	secrets   repository.SecretRepository
	wsChecker port.WorkspaceChecker
	crypto    *application.SecretCrypto
}

func NewListGitTreeHandler(
	secrets repository.SecretRepository,
	wsChecker port.WorkspaceChecker,
	crypto *application.SecretCrypto,
) *ListGitTreeHandler {
	return &ListGitTreeHandler{secrets: secrets, wsChecker: wsChecker, crypto: crypto}
}

func (h *ListGitTreeHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	gitURL, secretID, ref, path string,
) ([]GitTreeEntryView, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}

	gitURL = strings.TrimSpace(gitURL)
	if gitURL == "" {
		return nil, domain.ErrProjectConfigIncomplete
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("git ref is required")
	}
	sid, err := kernel.ParseSecretID(strings.TrimSpace(secretID))
	if err != nil {
		return nil, domain.ErrSecretNotFound
	}
	secret, err := h.secrets.FindByID(ctx, wsID, sid)
	if err != nil {
		return nil, err
	}
	if secret.Status != model.SecretStatusActive {
		return nil, fmt.Errorf("secret is not active")
	}
	creds, err := h.crypto.DecryptCredentials(secret.CredentialBlob)
	if err != nil {
		return nil, err
	}

	items, err := gitrefs.ListGitLabTree(ctx, gitURL, secret.ServiceURL, creds, ref, path)
	if err != nil {
		return nil, err
	}
	out := make([]GitTreeEntryView, 0, len(items))
	for _, item := range items {
		out = append(out, GitTreeEntryView{
			Name: item.Name,
			Path: item.Path,
			Type: item.Type,
		})
	}
	return out, nil
}
