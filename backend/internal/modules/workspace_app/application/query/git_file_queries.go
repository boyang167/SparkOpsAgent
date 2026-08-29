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

type GitFileContentView struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type GetGitFileContentHandler struct {
	secrets   repository.SecretRepository
	wsChecker port.WorkspaceChecker
	crypto    *application.SecretCrypto
}

func NewGetGitFileContentHandler(
	secrets repository.SecretRepository,
	wsChecker port.WorkspaceChecker,
	crypto *application.SecretCrypto,
) *GetGitFileContentHandler {
	return &GetGitFileContentHandler{secrets: secrets, wsChecker: wsChecker, crypto: crypto}
}

func (h *GetGitFileContentHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	gitURL, secretID, ref, filePath string,
) (*GitFileContentView, error) {
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
	filePath = gitrefs.NormalizeRepoFilePath(filePath)
	if filePath == "" {
		return nil, fmt.Errorf("file path is required")
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

	content, err := gitrefs.GetGitLabFileRaw(ctx, gitURL, secret.ServiceURL, creds, ref, filePath)
	if err != nil {
		return nil, err
	}
	return &GitFileContentView{
		Path:    filePath,
		Content: content,
	}, nil
}
