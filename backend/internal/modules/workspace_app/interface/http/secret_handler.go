package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type secretCredentialsRequest struct {
	Password    string `json:"password"`
	Token       string `json:"token"`
	APIKey      string `json:"api_key"`
	PrivateKey  string `json:"private_key"`
	PublicKey   string `json:"public_key"`
	Certificate string `json:"certificate"`
	ExtraConfig string `json:"extra_config"`
}

type createSecretRequest struct {
	Name         string                   `json:"name" binding:"required"`
	DisplayName  string                   `json:"display_name"`
	Description  string                   `json:"description"`
	ServiceType  string                   `json:"service_type" binding:"required"`
	ServiceURL   string                   `json:"service_url" binding:"required"`
	Username     string                   `json:"username"`
	Credentials  secretCredentialsRequest `json:"credentials"`
	Status       string                   `json:"status"`
	IsDefault    bool                     `json:"is_default"`
	IsShared     bool                     `json:"is_shared"`
}

type updateSecretRequest struct {
	DisplayName  string                   `json:"display_name"`
	Description  string                   `json:"description"`
	ServiceType  string                   `json:"service_type" binding:"required"`
	ServiceURL   string                   `json:"service_url" binding:"required"`
	Username     string                   `json:"username"`
	Credentials  secretCredentialsRequest `json:"credentials"`
	Status       string                   `json:"status"`
	IsDefault    bool                     `json:"is_default"`
	IsShared     bool                     `json:"is_shared"`
}

func (h *Handler) CreateSecret(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	var req createSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	s, err := h.createSecret.Handle(c.Request.Context(), wsID, memberID, command.CreateSecretInput{
		Name: req.Name, DisplayName: req.DisplayName, Description: req.Description,
		ServiceType: req.ServiceType, ServiceURL: req.ServiceURL, Username: req.Username,
		Credentials: toSecretCredentials(req.Credentials),
		Status: model.SecretStatus(req.Status), IsDefault: req.IsDefault, IsShared: req.IsShared,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	item, err := query.EnrichSecretMap(c.Request.Context(), h.memberLookup, h.secretCrypto, s)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) ListSecrets(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	items, err := h.listSecrets.Handle(c.Request.Context(), wsID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) GetSecret(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	secretID, err := kernel.ParseSecretID(c.Param("secretId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid secret id", http.StatusBadRequest)
		return
	}
	item, err := h.getSecret.Handle(c.Request.Context(), wsID, memberID, secretID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) UpdateSecret(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	secretID, err := kernel.ParseSecretID(c.Param("secretId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid secret id", http.StatusBadRequest)
		return
	}
	var req updateSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	s, err := h.updateSecret.Handle(c.Request.Context(), wsID, memberID, secretID, command.UpdateSecretInput{
		DisplayName: req.DisplayName, Description: req.Description,
		ServiceType: req.ServiceType, ServiceURL: req.ServiceURL, Username: req.Username,
		Credentials: toSecretCredentials(req.Credentials),
		Status: model.SecretStatus(req.Status), IsDefault: req.IsDefault, IsShared: req.IsShared,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	item, err := query.EnrichSecretMap(c.Request.Context(), h.memberLookup, h.secretCrypto, s)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) DeleteSecret(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	secretID, err := kernel.ParseSecretID(c.Param("secretId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid secret id", http.StatusBadRequest)
		return
	}
	if err := h.deleteSecret.Handle(c.Request.Context(), wsID, memberID, secretID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"deleted": true})
}

func (h *Handler) TestSecretConnection(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	secretID, err := kernel.ParseSecretID(c.Param("secretId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid secret id", http.StatusBadRequest)
		return
	}
	result, err := h.testSecret.Handle(c.Request.Context(), wsID, memberID, secretID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, result)
}

func toSecretCredentials(req secretCredentialsRequest) model.SecretCredentials {
	return model.SecretCredentials{
		Password: req.Password, Token: req.Token, APIKey: req.APIKey,
		PrivateKey: req.PrivateKey, PublicKey: req.PublicKey,
		Certificate: req.Certificate, ExtraConfig: req.ExtraConfig,
	}
}
