package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/infrastructure/gatewaycontrol"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

type InternalGatewayHandlers struct {
	Control *gatewaycontrol.Service
	Token   string
}

func RegisterInternalGatewayRoutes(
	engine *gin.Engine,
	cfg *config.Config,
	handlers InternalGatewayHandlers,
	sftp InternalSFTPHandlers,
) {
	if engine == nil || handlers.Control == nil {
		return
	}
	token := strings.TrimSpace(handlers.Token)
	if token == "" && cfg != nil {
		token = strings.TrimSpace(cfg.Filesystem.TransferGateway.InternalToken)
		if token == "" {
			token = cfg.JWT.Secret
		}
	}
	group := engine.Group("/internal/filesystem/v1")
	group.Use(requireInternalToken(token))
	group.GET("/transfers/:sessionId/upload-grant", func(c *gin.Context) {
		grant, err := handlers.Control.ResolveUpload(c.Request.Context(), c.Param("sessionId"))
		if err != nil {
			filesystemError(c, err)
			return
		}
		httpx.OK(c, gin.H{
			"temporary_object_key": grant.TemporaryObjectKey,
			"expected_size":        grant.ExpectedSize,
			"classification":       grant.Classification,
		})
	})
	group.POST("/transfers/:sessionId/uploaded", func(c *gin.Context) {
		var body struct {
			Size   int64  `json:"size"`
			SHA256 string `json:"sha256"`
		}
		if !decodeJSON(c, &body) {
			return
		}
		if err := handlers.Control.RecordUploaded(c.Request.Context(), c.Param("sessionId"), body.Size, body.SHA256); err != nil {
			filesystemError(c, err)
			return
		}
		httpx.OK(c, gin.H{"recorded": true})
	})
	RegisterInternalSFTPRoutes(group, sftp)
}

func requireInternalToken(expected string) gin.HandlerFunc {
	expected = strings.TrimSpace(expected)
	return func(c *gin.Context) {
		if expected == "" {
			httpx.Fail(c, "FORBIDDEN", "internal token not configured", http.StatusForbidden)
			c.Abort()
			return
		}
		if strings.TrimSpace(c.GetHeader("X-SparkOps-Internal-Token")) != expected {
			httpx.Fail(c, "FORBIDDEN", "invalid internal token", http.StatusForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}
