package kubernetes

import (
	_ "embed"

	"github.com/gin-gonic/gin"

	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	kuberneteshttp "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/http"
)

// ManifestYAML is the bundled manifest persisted for the built-in plugin seed.
//
//go:embed plugin.yaml
var ManifestYAML string

type plugin struct{}

func init() {
	sparkplugin.Register(&plugin{})
	registerBundledDescriptor()
}

func registerBundledDescriptor() {
	sparkplugin.RegisterBundled("kubernetes", sparkplugin.BundledDescriptor{
		ManifestYAML: ManifestYAML,
		InstalledFiles: []string{
			"backend/internal/plugins/kubernetes/plugin.go",
			"backend/internal/plugins/kubernetes/plugin.yaml",
			"backend/internal/plugins/kubernetes/chat/query.go",
			"backend/internal/plugins/kubernetes/diagnosis/arthas.go",
			"backend/internal/plugins/kubernetes/files/handler.go",
			"backend/internal/plugins/kubernetes/http/clusters.go",
			"backend/internal/plugins/kubernetes/http/cluster_users.go",
			"backend/internal/plugins/kubernetes/http/kubeconfig.go",
			"backend/internal/plugins/kubernetes/http/metrics.go",
			"backend/internal/plugins/kubernetes/http/router.go",
			"backend/internal/plugins/kubernetes/http/settings.go",
			"backend/internal/plugins/kubernetes/http/settings_store.go",
			"backend/internal/plugins/kubernetes/http/write_guard.go",
			"backend/internal/plugins/kubernetes/metrics/query.go",
			"backend/internal/plugins/kubernetes/proxy/aggregate.go",
			"backend/internal/plugins/kubernetes/proxy/auth.go",
			"backend/internal/plugins/kubernetes/proxy/discovery_cache.go",
			"backend/internal/plugins/kubernetes/proxy/handler.go",
			"backend/internal/plugins/kubernetes/proxy/path.go",
			"backend/internal/plugins/kubernetes/proxy/redaction.go",
			"backend/internal/plugins/kubernetes/proxy/rewrite.go",
			"backend/internal/plugins/kubernetes/proxy/transport.go",
			"backend/internal/plugins/kubernetes/rbac/apply.go",
			"backend/internal/plugins/kubernetes/rbac/names.go",
			"backend/internal/plugins/kubernetes/records/handler.go",
			"backend/internal/plugins/kubernetes/resource/arthas-bin.tar",
			"backend/internal/plugins/kubernetes/resource/assets.go",
			"backend/internal/plugins/kubernetes/schema/migrate.go",
			"backend/internal/plugins/kubernetes/schema/models.go",
			"backend/internal/plugins/kubernetes/stream/exec.go",
			"backend/internal/plugins/kubernetes/stream/logs.go",
			"backend/internal/plugins/kubernetes/terminal/handler.go",
			"backend/internal/plugins/kubernetes/terminal/protocol.go",
			"backend/internal/plugins/kubernetes/terminal/session.go",
			"backend/internal/plugins/kubernetes/terminal/ticket.go",
			"backend/internal/plugins/kubernetes/tty/cloudtty.go",
			"backend/internal/plugins/kubernetes/tty/nodetty.go",
			"backend/internal/plugins/kubernetes/velaux/proxy.go",
			"fronted/apps/web/app/(platform)/plugins/kubernetes/page.tsx",
			"fronted/apps/web/app/(platform)/plugins/kubernetes/layout.tsx",
			"fronted/apps/web/app/(platform)/plugins/kubernetes/clusters/page.tsx",
			"fronted/apps/web/app/(platform)/plugins/kubernetes/nodes/page.tsx",
			"fronted/apps/web/app/(platform)/plugins/kubernetes/workloads/page.tsx",
			"fronted/apps/web/app/(platform)/plugins/kubernetes/openkruise/page.tsx",
			"fronted/apps/web/app/(platform)/plugins/kubernetes/kruise-rollouts/page.tsx",
			"fronted/apps/web/app/(platform)/plugins/kubernetes/kubeblocks/page.tsx",
			"fronted/apps/web/app/(platform)/plugins/kubernetes/kubevela/page.tsx",
			"fronted/apps/web/app/(platform)/plugins/kubernetes/namespaces/page.tsx",
			"fronted/apps/web/app/(platform)/plugins/kubernetes/networks/page.tsx",
			"fronted/apps/web/app/(platform)/plugins/kubernetes/gateway/page.tsx",
			"fronted/apps/web/app/(platform)/plugins/kubernetes/configs/page.tsx",
			"fronted/apps/web/app/(platform)/plugins/kubernetes/storages/page.tsx",
			"fronted/apps/web/app/(platform)/plugins/kubernetes/access-controls/page.tsx",
			"fronted/apps/web/app/(platform)/plugins/kubernetes/backup/page.tsx",
			"fronted/apps/web/app/(platform)/plugins/kubernetes/records/page.tsx",
			"fronted/apps/web/app/(platform)/plugins/kubernetes/ops-tools/page.tsx",
			"fronted/apps/web/app/(platform)/plugins/kubernetes/diagnosis/page.tsx",
			"fronted/apps/web/app/(platform)/plugins/kubernetes/chat/page.tsx",
			"fronted/apps/web/app/(platform)/plugins/kubernetes/cloudtty/page.tsx",
			"fronted/apps/web/app/(platform)/plugins/kubernetes/velaux/page.tsx",
			"fronted/apps/web/app/api/bff/v1/plugins/kubernetes/[...path]/route.ts",
			"fronted/apps/web/src/modules/plugins/kubernetes/ApiErrorBanner.tsx",
			"fronted/apps/web/src/modules/plugins/kubernetes/ClusterPicker.tsx",
			"fronted/apps/web/src/modules/plugins/kubernetes/ClusterCreateForm.tsx",
			"fronted/apps/web/src/modules/plugins/kubernetes/cluster-settings-form.tsx",
			"fronted/apps/web/src/modules/plugins/kubernetes/cluster-status.ts",
			"fronted/apps/web/src/modules/plugins/kubernetes/cluster-types.ts",
			"fronted/apps/web/src/modules/plugins/kubernetes/node-view.ts",
			"fronted/apps/web/src/modules/plugins/kubernetes/pod-terminal.ts",
			"fronted/apps/web/src/modules/plugins/kubernetes/KubernetesConsole.tsx",
			"fronted/apps/web/src/modules/plugins/kubernetes/OperationsPanel.tsx",
			"fronted/apps/web/src/modules/plugins/kubernetes/ResourceMenuPage.tsx",
			"fronted/apps/web/src/modules/plugins/kubernetes/ResourceTable.tsx",
			"fronted/apps/web/src/modules/plugins/kubernetes/YamlEditor.tsx",
			"fronted/apps/web/src/modules/plugins/kubernetes/api.ts",
			"fronted/apps/web/src/modules/plugins/kubernetes/catalog.ts",
			"fronted/apps/web/src/modules/plugins/kubernetes/page-server.tsx",
			"fronted/apps/web/src/modules/plugins/kubernetes/operations.ts",
			"fronted/apps/web/src/modules/plugins/kubernetes/resource-url.ts",
			"fronted/apps/web/src/modules/plugins/kubernetes/sse.ts",
			"fronted/apps/web/src/modules/plugins/kubernetes/velaux-frame.ts",
			"fronted/apps/web/src/modules/plugins/kubernetes/yaml-document.ts",
			"fronted/apps/web/src/modules/plugins/kubernetes/specialized/ChatPage.tsx",
			"fronted/apps/web/src/modules/plugins/kubernetes/specialized/CloudTTYPage.tsx",
			"fronted/apps/web/src/modules/plugins/kubernetes/specialized/ClusterAssociateDialog.tsx",
			"fronted/apps/web/src/modules/plugins/kubernetes/specialized/ClusterAuthorizeDialog.tsx",
			"fronted/apps/web/src/modules/plugins/kubernetes/specialized/ClusterAuthorizeRoles.tsx",
			"fronted/apps/web/src/modules/plugins/kubernetes/specialized/cluster-authorize-roles.ts",
			"fronted/apps/web/src/modules/plugins/kubernetes/specialized/ClusterAuthorizeUsers.tsx",
			"fronted/apps/web/src/modules/plugins/kubernetes/specialized/ClustersPanel.tsx",
			"fronted/apps/web/src/modules/plugins/kubernetes/specialized/DiagnosisPanel.tsx",
			"fronted/apps/web/src/modules/plugins/kubernetes/specialized/FormParts.tsx",
			"fronted/apps/web/src/modules/plugins/kubernetes/specialized/NodeDetailView.tsx",
			"fronted/apps/web/src/modules/plugins/kubernetes/specialized/NodesPanel.tsx",
			"fronted/apps/web/src/modules/plugins/kubernetes/specialized/OverviewPanel.tsx",
			"fronted/apps/web/src/modules/plugins/kubernetes/specialized/PodTerminal.tsx",
			"fronted/apps/web/src/modules/plugins/kubernetes/specialized/RecordsPage.tsx",
			"fronted/apps/web/src/modules/plugins/kubernetes/specialized/VelauxPage.tsx",
		},
	})
}

func (p *plugin) Register(engine *gin.Engine) {
	kuberneteshttp.RegisterRoutes(engine)
}

func (p *plugin) RegisterWithContext(engine *gin.Engine, ctx sparkplugin.PluginContext) {
	kuberneteshttp.RegisterRoutes(engine, ctx)
}
