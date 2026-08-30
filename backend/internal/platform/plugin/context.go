package plugin

import (
	"context"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type AuditEvent struct {
	WorkspaceID string
	MemberID    string
	Action      string
	Detail      map[string]any
}

type AuditLogger interface {
	Record(ctx context.Context, event AuditEvent) error
}

type PluginContext struct {
	Logger   *zap.Logger
	DB       *gorm.DB
	Audit    AuditLogger
	Clusters ClusterClientFactory
}

type ContextPlugin interface {
	Plugin
	RegisterWithContext(engine *gin.Engine, ctx PluginContext)
}
