package appctx

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	integrationport "github.com/sparkops/sparkops-agent/backend/internal/modules/integration/domain/port"
	runtimeport "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/k8s"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/policy"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/redis"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/routes"
)

type Module interface {
	Name() string
	Register(c *Container) error
}

type Container struct {
	Context                    context.Context
	Config                     *config.Config
	DB                         *gorm.DB
	Redis                      *redis.Client
	Logger                     *zap.Logger
	Engine                     *gin.Engine
	EventBus                   eventbus.Bus
	MessageQueue               messaging.Queue
	K8sFactory                 *k8s.Factory
	PermissionChecker          authz.PermissionChecker
	SystemRoleChecker          authz.SystemRoleChecker
	WorkspaceMemberChecker     authz.WorkspaceMemberChecker
	OrgWorkspaceCreatorChecker authz.OrgWorkspaceCreatorChecker
	OrgPermissionChecker       authz.OrgPermissionChecker
	ProjectAccessChecker       authz.ProjectAccessChecker
	PolicyEngine               policy.Engine
	RouteRegistry              *routes.Registry
	AuditLogger                audit.Logger
	BuildTrigger               BuildTrigger
	DeploymentTrigger          DeploymentTrigger
	WorkflowRunTrigger         WorkflowRunTrigger
	SprintProcessPort          SprintProcessPort
	ManifestDeployer           runtimeport.ManifestDeployer
	APIPolicyChecker           authz.WorkspaceAPIPolicyChecker
	LLMClient                  integrationport.LLMClient
	Server                     *http.Server
}

func (c *Container) DBOrNil() *gorm.DB {
	return c.DB
}
