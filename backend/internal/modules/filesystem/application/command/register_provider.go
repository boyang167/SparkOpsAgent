package command

import (
	"context"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

type storageProviderWriter interface {
	CreateStorageProvider(context.Context, *model.StorageProvider) error
}

type RegisterProvider struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	ProviderType   model.ProviderType
	Endpoint       string
	SecretRef      string
	Capabilities   map[string]any
	Status         string
}

type RegisterProviderResult struct {
	ID uuid.UUID
}

type RegisterProviderHandler struct {
	repo storageProviderWriter
}

func NewRegisterProviderHandler(repo storageProviderWriter) *RegisterProviderHandler {
	if repo == nil {
		panic("register provider requires repository")
	}
	return &RegisterProviderHandler{repo: repo}
}

func (h *RegisterProviderHandler) Handle(ctx context.Context, cmd RegisterProvider) (RegisterProviderResult, error) {
	provider, err := model.NewStorageProvider(model.StorageProviderInput{
		ID: cmd.ID, OrganizationID: cmd.OrganizationID, Name: cmd.Name,
		ProviderType: cmd.ProviderType, Endpoint: cmd.Endpoint, SecretRef: cmd.SecretRef,
		Capabilities: cmd.Capabilities, Status: cmd.Status,
	})
	if err != nil {
		return RegisterProviderResult{}, err
	}
	if err := h.repo.CreateStorageProvider(ctx, provider); err != nil {
		return RegisterProviderResult{}, err
	}
	return RegisterProviderResult{ID: provider.ID}, nil
}
