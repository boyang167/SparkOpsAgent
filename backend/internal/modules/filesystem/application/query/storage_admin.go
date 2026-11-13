package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

type storageAdminReader interface {
	ListStorageProviders(context.Context, uuid.UUID) ([]*model.StorageProvider, error)
	ListStoragePools(context.Context, uuid.UUID) ([]model.StoragePoolSnapshot, error)
	ListStorageBindings(context.Context, uuid.UUID) ([]model.StorageBinding, error)
	ListLifecyclePolicies(context.Context, uuid.UUID) ([]model.LifecyclePolicy, error)
}

type StorageAdminList struct {
	OrganizationID uuid.UUID
}

type ListStorageProvidersHandler struct {
	repo storageAdminReader
}

func NewListStorageProvidersHandler(repo storageAdminReader) *ListStorageProvidersHandler {
	if repo == nil {
		panic("list storage providers requires repository")
	}
	return &ListStorageProvidersHandler{repo: repo}
}

func (h *ListStorageProvidersHandler) Handle(ctx context.Context, q StorageAdminList) ([]*model.StorageProvider, error) {
	return h.repo.ListStorageProviders(ctx, q.OrganizationID)
}

type ListStoragePoolsHandler struct {
	repo storageAdminReader
}

func NewListStoragePoolsHandler(repo storageAdminReader) *ListStoragePoolsHandler {
	if repo == nil {
		panic("list storage pools requires repository")
	}
	return &ListStoragePoolsHandler{repo: repo}
}

func (h *ListStoragePoolsHandler) Handle(ctx context.Context, q StorageAdminList) ([]model.StoragePoolSnapshot, error) {
	return h.repo.ListStoragePools(ctx, q.OrganizationID)
}

type ListStorageBindingsHandler struct {
	repo storageAdminReader
}

func NewListStorageBindingsHandler(repo storageAdminReader) *ListStorageBindingsHandler {
	if repo == nil {
		panic("list storage bindings requires repository")
	}
	return &ListStorageBindingsHandler{repo: repo}
}

func (h *ListStorageBindingsHandler) Handle(ctx context.Context, q StorageAdminList) ([]model.StorageBinding, error) {
	return h.repo.ListStorageBindings(ctx, q.OrganizationID)
}

type ListLifecyclePoliciesHandler struct {
	repo storageAdminReader
}

func NewListLifecyclePoliciesHandler(repo storageAdminReader) *ListLifecyclePoliciesHandler {
	if repo == nil {
		panic("list lifecycle policies requires repository")
	}
	return &ListLifecyclePoliciesHandler{repo: repo}
}

func (h *ListLifecyclePoliciesHandler) Handle(ctx context.Context, q StorageAdminList) ([]model.LifecyclePolicy, error) {
	return h.repo.ListLifecyclePolicies(ctx, q.OrganizationID)
}
