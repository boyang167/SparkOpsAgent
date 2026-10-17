package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/integration"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type CreateSecretInput struct {
	Name, DisplayName, Description, ServiceType, ServiceURL, Username string
	Credentials                                                       model.SecretCredentials
	Status                                                            model.SecretStatus
	IsDefault, IsShared                                               bool
}

type CreateSecretHandler struct {
	repo      repository.SecretRepository
	wsChecker port.WorkspaceChecker
	crypto    *application.SecretCrypto
}

func NewCreateSecretHandler(
	repo repository.SecretRepository,
	wsChecker port.WorkspaceChecker,
	crypto *application.SecretCrypto,
) *CreateSecretHandler {
	return &CreateSecretHandler{repo: repo, wsChecker: wsChecker, crypto: crypto}
}

func (h *CreateSecretHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, in CreateSecretInput) (*model.Secret, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	name, err := model.NewSecretName(in.Name)
	if err != nil {
		return nil, err
	}
	blob, err := h.crypto.EncryptCredentials(in.Credentials)
	if err != nil {
		return nil, err
	}
	s := model.NewSecret(wsID, name, in.DisplayName, in.Description, in.ServiceType, in.ServiceURL, in.Username, blob, in.Status, in.IsDefault, in.IsShared, memberID)
	if err := h.repo.Save(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}

type UpdateSecretInput struct {
	DisplayName, Description, ServiceType, ServiceURL, Username string
	Credentials                                                 model.SecretCredentials
	ReplaceCredentials                                          bool
	Status                                                      model.SecretStatus
	IsDefault, IsShared                                         bool
}

type UpdateSecretHandler struct {
	repo      repository.SecretRepository
	wsChecker port.WorkspaceChecker
	crypto    *application.SecretCrypto
	bus       eventbus.Bus
	tx        postgres.Transactor
}

func NewUpdateSecretHandler(
	repo repository.SecretRepository,
	wsChecker port.WorkspaceChecker,
	crypto *application.SecretCrypto,
	bus eventbus.Bus,
	tx postgres.Transactor,
) *UpdateSecretHandler {
	return &UpdateSecretHandler{repo: repo, wsChecker: wsChecker, crypto: crypto, bus: bus, tx: tx}
}

func (h *UpdateSecretHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, secretID kernel.SecretID, in UpdateSecretInput) (*model.Secret, error) {
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
	blob := s.CredentialBlob
	if in.ReplaceCredentials {
		blob, err = h.crypto.EncryptCredentials(in.Credentials)
		if err != nil {
			return nil, err
		}
	} else {
		existing, err := h.crypto.DecryptCredentials(s.CredentialBlob)
		if err != nil {
			return nil, err
		}
		merged := mergeCredentials(existing, in.Credentials)
		blob, err = h.crypto.EncryptCredentials(merged)
		if err != nil {
			return nil, err
		}
	}
	s.Update(in.DisplayName, in.Description, in.ServiceType, in.ServiceURL, in.Username, blob, in.Status, in.IsDefault, in.IsShared)
	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := h.repo.Update(txCtx, s); err != nil {
			return err
		}
		if credentialsPatchProvided(in) && h.bus != nil {
			return h.bus.Publish(txCtx, integration.NewSecretUpdated(wsID, secretID, s.ServiceType))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s, nil
}

func credentialsPatchProvided(in UpdateSecretInput) bool {
	if in.ReplaceCredentials {
		return true
	}
	c := in.Credentials
	return c.Password != "" || c.Token != "" || c.APIKey != "" ||
		c.PrivateKey != "" || c.PublicKey != "" || c.Certificate != "" || c.ExtraConfig != ""
}

func mergeCredentials(base, patch model.SecretCredentials) model.SecretCredentials {
	out := base
	if patch.Password != "" {
		out.Password = patch.Password
	}
	if patch.Token != "" {
		out.Token = patch.Token
	}
	if patch.APIKey != "" {
		out.APIKey = patch.APIKey
	}
	if patch.PrivateKey != "" {
		out.PrivateKey = patch.PrivateKey
	}
	if patch.PublicKey != "" {
		out.PublicKey = patch.PublicKey
	}
	if patch.Certificate != "" {
		out.Certificate = patch.Certificate
	}
	if patch.ExtraConfig != "" {
		out.ExtraConfig = patch.ExtraConfig
	}
	return out
}

type DeleteSecretHandler struct {
	repo      repository.SecretRepository
	wsChecker port.WorkspaceChecker
}

func NewDeleteSecretHandler(repo repository.SecretRepository, wsChecker port.WorkspaceChecker) *DeleteSecretHandler {
	return &DeleteSecretHandler{repo: repo, wsChecker: wsChecker}
}

func (h *DeleteSecretHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, secretID kernel.SecretID) error {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrWorkspaceAccessDenied
	}
	if _, err := h.repo.FindByID(ctx, wsID, secretID); err != nil {
		return err
	}
	return h.repo.Delete(ctx, secretID)
}
