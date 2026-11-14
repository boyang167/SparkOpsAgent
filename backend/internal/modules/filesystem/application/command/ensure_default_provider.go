package command

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

const DefaultLocalProviderName = "default-local"

func DefaultLocalProviderID(organizationID uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("sparkops-filesystem-local:"+organizationID.String()))
}

type defaultProviderRepo interface {
	GetStorageProvider(context.Context, uuid.UUID) (*model.StorageProvider, error)
	CreateStorageProvider(context.Context, *model.StorageProvider) error
}

type EnsureDefaultLocalProvider struct {
	OrganizationID uuid.UUID
	StorageRoot    string
}

type EnsureDefaultLocalProviderHandler struct {
	repo defaultProviderRepo
}

func NewEnsureDefaultLocalProviderHandler(repo defaultProviderRepo) *EnsureDefaultLocalProviderHandler {
	if repo == nil {
		panic("ensure default provider requires repository")
	}
	return &EnsureDefaultLocalProviderHandler{repo: repo}
}

func (h *EnsureDefaultLocalProviderHandler) Handle(ctx context.Context, cmd EnsureDefaultLocalProvider) (uuid.UUID, error) {
	if cmd.OrganizationID == uuid.Nil {
		return uuid.Nil, model.ErrInvalidStorageProvider
	}
	root := strings.TrimSpace(cmd.StorageRoot)
	if root == "" {
		return uuid.Nil, fmt.Errorf("local storage root is required")
	}
	id := DefaultLocalProviderID(cmd.OrganizationID)
	if existing, err := h.repo.GetStorageProvider(ctx, id); err == nil && existing != nil {
		return existing.ID, nil
	} else if err != nil && !errors.Is(err, domain.ErrStorageProviderNotFound) {
		return uuid.Nil, err
	}
	provider, err := model.NewStorageProvider(model.StorageProviderInput{
		ID: id, OrganizationID: cmd.OrganizationID, Name: DefaultLocalProviderName,
		ProviderType: model.ProviderTypeLocal, Endpoint: root, SecretRef: "local://default",
		Status: "ACTIVE",
	})
	if err != nil {
		return uuid.Nil, err
	}
	if err := h.repo.CreateStorageProvider(ctx, provider); err != nil {
		if errors.Is(err, domain.ErrStorageProviderConflict) {
			return id, nil
		}
		return uuid.Nil, err
	}
	return id, nil
}
