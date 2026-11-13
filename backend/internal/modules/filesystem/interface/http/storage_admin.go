package http

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

type StorageAdminOptions struct {
	RegisterProvider      func(context.Context, command.RegisterProvider) (command.RegisterProviderResult, error)
	ListStorageProviders  func(context.Context, query.StorageAdminList) ([]*model.StorageProvider, error)
	CreateStoragePool     func(context.Context, command.CreateStoragePool) (command.CreateStoragePoolResult, error)
	ListStoragePools      func(context.Context, query.StorageAdminList) ([]model.StoragePoolSnapshot, error)
	CreateStorageBinding  func(context.Context, command.CreateStorageBinding) (command.CreateStorageBindingResult, error)
	ListStorageBindings   func(context.Context, query.StorageAdminList) ([]model.StorageBinding, error)
	UpsertLifecyclePolicy func(context.Context, command.UpsertLifecyclePolicy) (command.UpsertLifecyclePolicyResult, error)
	ListLifecyclePolicies func(context.Context, query.StorageAdminList) ([]model.LifecyclePolicy, error)
}

type storageProviderMemberRequest struct {
	ProviderID string `json:"provider_id"`
	Mode       string `json:"mode"`
	Priority   int    `json:"priority"`
}

type createStoragePoolRequest struct {
	Name    string                           `json:"name"`
	Members []storageProviderMemberRequest   `json:"members"`
}

type registerStorageProviderRequest struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	ProviderType string         `json:"provider_type"`
	Endpoint     string         `json:"endpoint"`
	SecretRef    string         `json:"secret_ref"`
	Capabilities map[string]any `json:"capabilities"`
	Status       string         `json:"status"`
}

type createStorageBindingRequest struct {
	ScopeType string `json:"scope_type"`
	ScopeID   string `json:"scope_id"`
	PoolID    string `json:"pool_id"`
	Priority  int    `json:"priority"`
}

type upsertLifecyclePolicyRequest struct {
	Name    string         `json:"name"`
	Version int64          `json:"version"`
	Rules   map[string]any `json:"rules"`
	Status  string         `json:"status"`
}

func (h *Handler) adminContext(c *gin.Context, mutation bool) (requestContext, bool) {
	rc, ok := h.requestContext(c, mutation)
	if !ok {
		return requestContext{}, false
	}
	if rc.namespace.OrganizationID == uuid.Nil {
		filesystemError(c, domain.ErrPersistence)
		return requestContext{}, false
	}
	return rc, true
}

func (h *Handler) ListStorageProviders(c *gin.Context) {
	rc, ok := h.adminContext(c, false)
	if !ok || h.listStorageProviders == nil {
		if ok {
			filesystemError(c, domain.ErrPersistence)
		}
		return
	}
	items, err := h.listStorageProviders(c.Request.Context(), query.StorageAdminList{OrganizationID: rc.namespace.OrganizationID})
	if err != nil {
		filesystemError(c, err)
		return
	}
	views := make([]gin.H, 0, len(items))
	for _, item := range items {
		views = append(views, storageProviderView(item))
	}
	httpx.OK(c, gin.H{"items": views})
}

func (h *Handler) RegisterStorageProvider(c *gin.Context) {
	rc, ok := h.adminContext(c, true)
	if !ok {
		return
	}
	var body registerStorageProviderRequest
	if !decodeJSON(c, &body) {
		return
	}
	providerID := uuid.New()
	if body.ID != "" {
		var valid bool
		providerID, valid = canonicalUUID(body.ID)
		if !valid {
			invalidRequest(c)
			return
		}
	}
	if h.registerProvider == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	result, err := h.registerProvider(c.Request.Context(), command.RegisterProvider{
		ID: providerID, OrganizationID: rc.namespace.OrganizationID, Name: body.Name,
		ProviderType: model.ProviderType(body.ProviderType), Endpoint: body.Endpoint,
		SecretRef: body.SecretRef, Capabilities: body.Capabilities, Status: body.Status,
	})
	if err != nil {
		filesystemError(c, err)
		return
	}
	created(c, gin.H{"id": result.ID})
}

func (h *Handler) ListStoragePools(c *gin.Context) {
	rc, ok := h.adminContext(c, false)
	if !ok || h.listStoragePools == nil {
		if ok {
			filesystemError(c, domain.ErrPersistence)
		}
		return
	}
	items, err := h.listStoragePools(c.Request.Context(), query.StorageAdminList{OrganizationID: rc.namespace.OrganizationID})
	if err != nil {
		filesystemError(c, err)
		return
	}
	views := make([]gin.H, 0, len(items))
	for i := range items {
		views = append(views, storagePoolView(&items[i]))
	}
	httpx.OK(c, gin.H{"items": views})
}

func (h *Handler) CreateStoragePool(c *gin.Context) {
	rc, ok := h.adminContext(c, true)
	if !ok {
		return
	}
	var body createStoragePoolRequest
	if !decodeJSON(c, &body) {
		return
	}
	members := make([]command.CreateStoragePoolMember, 0, len(body.Members))
	for _, member := range body.Members {
		providerID, valid := canonicalUUID(member.ProviderID)
		if !valid {
			invalidRequest(c)
			return
		}
		members = append(members, command.CreateStoragePoolMember{
			ProviderID: providerID, Mode: model.ProviderMode(member.Mode), Priority: member.Priority,
		})
	}
	if h.createStoragePool == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	result, err := h.createStoragePool(c.Request.Context(), command.CreateStoragePool{
		CommandID: rc.commandID, OrganizationID: rc.namespace.OrganizationID,
		Name: body.Name, Members: members,
	})
	if err != nil {
		filesystemError(c, err)
		return
	}
	created(c, gin.H{"id": result.ID, "name": result.Name})
}

func (h *Handler) ListStorageBindings(c *gin.Context) {
	rc, ok := h.adminContext(c, false)
	if !ok || h.listStorageBindings == nil {
		if ok {
			filesystemError(c, domain.ErrPersistence)
		}
		return
	}
	items, err := h.listStorageBindings(c.Request.Context(), query.StorageAdminList{OrganizationID: rc.namespace.OrganizationID})
	if err != nil {
		filesystemError(c, err)
		return
	}
	views := make([]gin.H, 0, len(items))
	for i := range items {
		views = append(views, storageBindingView(&items[i]))
	}
	httpx.OK(c, gin.H{"items": views})
}

func (h *Handler) CreateStorageBinding(c *gin.Context) {
	rc, ok := h.adminContext(c, true)
	if !ok {
		return
	}
	var body createStorageBindingRequest
	if !decodeJSON(c, &body) {
		return
	}
	scopeID, ok := canonicalUUID(body.ScopeID)
	if !ok {
		invalidRequest(c)
		return
	}
	poolID, ok := canonicalUUID(body.PoolID)
	if !ok {
		invalidRequest(c)
		return
	}
	if h.createStorageBinding == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	result, err := h.createStorageBinding(c.Request.Context(), command.CreateStorageBinding{
		CommandID: rc.commandID, OrganizationID: rc.namespace.OrganizationID,
		ScopeType: model.ScopeType(body.ScopeType), ScopeID: scopeID, PoolID: poolID, Priority: body.Priority,
	})
	if err != nil {
		filesystemError(c, err)
		return
	}
	created(c, gin.H{"id": result.ID})
}

func (h *Handler) ListLifecyclePolicies(c *gin.Context) {
	rc, ok := h.adminContext(c, false)
	if !ok || h.listLifecyclePolicies == nil {
		if ok {
			filesystemError(c, domain.ErrPersistence)
		}
		return
	}
	items, err := h.listLifecyclePolicies(c.Request.Context(), query.StorageAdminList{OrganizationID: rc.namespace.OrganizationID})
	if err != nil {
		filesystemError(c, err)
		return
	}
	views := make([]gin.H, 0, len(items))
	for i := range items {
		views = append(views, lifecyclePolicyView(&items[i]))
	}
	httpx.OK(c, gin.H{"items": views})
}

func (h *Handler) UpsertLifecyclePolicy(c *gin.Context) {
	rc, ok := h.adminContext(c, true)
	if !ok {
		return
	}
	var body upsertLifecyclePolicyRequest
	if !decodeJSON(c, &body) {
		return
	}
	if h.upsertLifecyclePolicy == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	result, err := h.upsertLifecyclePolicy(c.Request.Context(), command.UpsertLifecyclePolicy{
		CommandID: rc.commandID, OrganizationID: rc.namespace.OrganizationID,
		Name: body.Name, Version: body.Version, Rules: body.Rules,
		Status: model.LifecyclePolicyStatus(strings.ToUpper(strings.TrimSpace(body.Status))),
	})
	if err != nil {
		filesystemError(c, err)
		return
	}
	created(c, gin.H{"id": result.ID, "version": result.Version})
}

func storageProviderView(provider *model.StorageProvider) gin.H {
	return gin.H{
		"id": provider.ID, "organization_id": provider.OrganizationID, "name": provider.Name,
		"provider_type": provider.ProviderType, "endpoint": provider.Endpoint,
		"secret_ref": provider.SecretRef, "capabilities": provider.Capabilities,
		"config_version": provider.ConfigVersion, "status": provider.Status,
	}
}

func storagePoolView(pool *model.StoragePoolSnapshot) gin.H {
	members := make([]gin.H, 0, len(pool.Members))
	for _, member := range pool.Members {
		members = append(members, gin.H{
			"provider_id": member.ProviderID, "mode": member.Mode, "priority": member.Priority,
		})
	}
	return gin.H{
		"id": pool.ID, "name": pool.Name, "status": pool.Status,
		"placement_policy_version": pool.PlacementPolicyVersion, "members": members,
	}
}

func storageBindingView(binding *model.StorageBinding) gin.H {
	return gin.H{
		"id": binding.ID, "organization_id": binding.OrganizationID,
		"scope_type": binding.ScopeType, "scope_id": binding.ScopeID,
		"pool_id": binding.PoolID, "priority": binding.Priority,
	}
}

func lifecyclePolicyView(policy *model.LifecyclePolicy) gin.H {
	return gin.H{
		"id": policy.ID, "organization_id": policy.OrganizationID, "name": policy.Name,
		"version": policy.Version, "rules": policy.Rules, "status": policy.Status,
	}
}
