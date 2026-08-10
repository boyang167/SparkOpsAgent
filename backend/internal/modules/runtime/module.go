package runtime

import (
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/infrastructure/consul"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/infrastructure/k8s"
	runtimehttp "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/interface/http"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

type Module struct {
	tokens *authn.TokenProvider
}

func New(tokens *authn.TokenProvider) *Module {
	return &Module{tokens: tokens}
}

func (m *Module) Name() string { return "runtime" }

func (m *Module) Register(c *appctx.Container) error {
	if c.DB == nil {
		return fmt.Errorf("runtime module requires database")
	}
	if c.PermissionChecker == nil {
		return fmt.Errorf("runtime module requires permission checker")
	}

	checker := acl.NewWorkspaceAppACL(c.DB)
	adapters, err := k8s.NewAdapters(c.Config, c.K8sFactory, checker)
	if err != nil {
		return fmt.Errorf("runtime adapters: %w", err)
	}

	k8sQuery := query.NewK8sResources(checker, adapters.Resources)
	resHandler := runtimehttp.NewResourcesHandler(k8sQuery)

	consulStore, err := consul.NewKVStore(c.Config, c.DB, c.Logger)
	if err != nil {
		return fmt.Errorf("consul kv store: %w", err)
	}
	consulHandler := runtimehttp.NewConsulHandler(consulStore)

	handler := runtimehttp.NewHandler(
		query.NewListPodsHandler(checker, adapters.Lister),
		query.NewGetPodHandler(checker, adapters.Lister),
		query.NewGetPodLogsHandler(checker, adapters.Lister, adapters.Logs),
		query.NewStreamPodTerminalHandler(checker, adapters.Lister, adapters.Terminal),
	)

	auth := authn.RequireAuth(m.tokens)
	ws := httpx.RequireWorkspace()
	wsMember := authz.RequireWorkspaceMember(c.WorkspaceMemberChecker)

	c.Engine.GET("/api/v1/projects/:projectId/pods",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListPods)
	c.Engine.GET("/api/v1/projects/:projectId/pods/:podId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.GetPod)
	c.Engine.GET("/api/v1/projects/:projectId/pods/:podId/logs",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.GetPodLogs)
	c.Engine.GET("/api/v1/projects/:projectId/pods/:podId/terminal/stream",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.StreamPodTerminal)

	base := "/api/v1/workspaces/:wsId/k8s/projects/:projectId"
	c.Engine.GET(base+"/deployments",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), resHandler.ListDeployments)
	c.Engine.GET(base+"/deployments/:name",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), resHandler.GetDeployment)
	c.Engine.PUT(base+"/deployments/:name/scale",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), resHandler.ScaleDeployment)
	c.Engine.GET(base+"/ingresses",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), resHandler.ListIngresses)
	c.Engine.GET(base+"/ingresses/:name",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), resHandler.GetIngress)
	c.Engine.GET(base+"/configmaps",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), resHandler.ListConfigMaps)
	c.Engine.GET(base+"/configmaps/:name",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), resHandler.GetConfigMap)
	c.Engine.GET(base+"/services",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), resHandler.ListServices)
	c.Engine.GET(base+"/services/:name",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), resHandler.GetService)
	c.Engine.GET(base+"/pvcs",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), resHandler.ListPVCs)
	c.Engine.GET(base+"/pvcs/:name",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), resHandler.GetPVC)
	c.Engine.GET("/api/v1/workspaces/:wsId/k8s/storageclasses",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), resHandler.ListStorageClasses)
	c.Engine.GET("/api/v1/workspaces/:wsId/k8s/storageclasses/:name",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), resHandler.GetStorageClass)
	c.Engine.GET(base+"/virtualservices",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), resHandler.ListVirtualServices)
	c.Engine.GET(base+"/virtualservices/:name",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), resHandler.GetVirtualService)
	c.Engine.GET("/api/v1/workspaces/:wsId/consul/kv",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), consulHandler.ListKV)
	c.Engine.PUT("/api/v1/workspaces/:wsId/consul/kv",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), consulHandler.PutKV)
	c.Engine.DELETE("/api/v1/workspaces/:wsId/consul/kv",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), consulHandler.DeleteKV)

	return nil
}
