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

type GitRefView struct {
	Name      string `json:"name"`
	Title     string `json:"title,omitempty"`
	Default   bool   `json:"default"`
	Protected bool   `json:"protected"`
}

type ListGitRefsHandler struct {
	secrets   repository.SecretRepository
	wsChecker port.WorkspaceChecker
	crypto    *application.SecretCrypto
}

func NewListGitRefsHandler(
	secrets repository.SecretRepository,
	wsChecker port.WorkspaceChecker,
	crypto *application.SecretCrypto,
) *ListGitRefsHandler {
	return &ListGitRefsHandler{secrets: secrets, wsChecker: wsChecker, crypto: crypto}
}

func (h *ListGitRefsHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	gitURL, secretID, refType string,
) ([]GitRefView, error) {
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

	refType = strings.TrimSpace(refType)
	if refType == "" {
		refType = "branch"
	}
	if refType != "branch" && refType != "tag" && refType != "commit" {
		return nil, fmt.Errorf("unsupported ref type")
	}

	items, err := gitrefs.ListGitLabRefs(ctx, gitURL, secret.ServiceURL, creds, refType)
	if err != nil {
		return nil, err
	}
	out := make([]GitRefView, 0, len(items))
	for _, item := range items {
		out = append(out, GitRefView{
			Name:      item.Name,
			Title:     item.Title,
			Default:   item.Default,
			Protected: item.Protected,
		})
	}
	return out, nil
}
