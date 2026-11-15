package command

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

type storageProviderUpdater interface {
	GetStorageProvider(context.Context, uuid.UUID) (*model.StorageProvider, error)
	UpdateStorageProvider(context.Context, *model.StorageProvider) error
}

type UpdateProvider struct {
	OrganizationID uuid.UUID
	ProviderID     uuid.UUID
	Name           string
	Endpoint       string
	SecretRef      string
	Status         string
}

type UpdateProviderHandler struct {
	repo storageProviderUpdater
}

func NewUpdateProviderHandler(repo storageProviderUpdater) *UpdateProviderHandler {
	if repo == nil {
		panic("update provider requires repository")
	}
	return &UpdateProviderHandler{repo: repo}
}

func (h *UpdateProviderHandler) Handle(ctx context.Context, cmd UpdateProvider) error {
	if cmd.OrganizationID == uuid.Nil || cmd.ProviderID == uuid.Nil {
		return model.ErrInvalidStorageProvider
	}
	existing, err := h.repo.GetStorageProvider(ctx, cmd.ProviderID)
	if err != nil {
		return err
	}
	if existing.OrganizationID != cmd.OrganizationID {
		return domain.ErrStorageProviderNotFound
	}
	secretRef := cmd.SecretRef
	if secretRef == "" {
		secretRef = existing.SecretRef
	}
	endpoint := strings.TrimSpace(cmd.Endpoint)
	if endpoint == "" {
		endpoint = existing.Endpoint
	}
	if err := validateProviderEndpoint(existing.ProviderType, endpoint); err != nil {
		return err
	}
	updated, buildErr := model.NewStorageProvider(model.StorageProviderInput{
		ID:             existing.ID,
		OrganizationID: existing.OrganizationID,
		Name:           cmd.Name,
		ProviderType:   existing.ProviderType,
		Endpoint:       endpoint,
		SecretRef:      secretRef,
		Capabilities:   existing.Capabilities,
		Health:         existing.Health,
		ConfigVersion:  existing.ConfigVersion,
		Status:         cmd.Status,
	})
	if buildErr != nil {
		return buildErr
	}
	return h.repo.UpdateStorageProvider(ctx, updated)
}