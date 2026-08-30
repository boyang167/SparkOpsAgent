package plugin

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
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

type NopAudit struct{}

func (NopAudit) Record(context.Context, AuditEvent) error { return nil }

type WriteAuthorizer interface {
	CanWrite(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID) (bool, error)
}

type WorkspaceMemberAuthorizer interface {
	IsWorkspaceMember(ctx context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID) (bool, error)
}

type PluginContext struct {
	Logger   *zap.Logger
	DB       *gorm.DB
	Audit    AuditLogger
	Clusters ClusterClientFactory
	Writer   WriteAuthorizer
	Members  WorkspaceMemberAuthorizer
}

type ContextPlugin interface {
	Plugin
	RegisterWithContext(engine *gin.Engine, ctx PluginContext)
}
