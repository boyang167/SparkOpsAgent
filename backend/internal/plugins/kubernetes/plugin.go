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
			"backend/internal/plugins/kubernetes/http/clusters.go",
			"backend/internal/plugins/kubernetes/http/router.go",
			"backend/internal/plugins/kubernetes/http/settings.go",
			"backend/internal/plugins/kubernetes/http/settings_store.go",
			"backend/internal/plugins/kubernetes/http/write_guard.go",
			"backend/internal/plugins/kubernetes/proxy/aggregate.go",
			"backend/internal/plugins/kubernetes/proxy/auth.go",
			"backend/internal/plugins/kubernetes/proxy/discovery_cache.go",
			"backend/internal/plugins/kubernetes/proxy/handler.go",
			"backend/internal/plugins/kubernetes/proxy/path.go",
			"backend/internal/plugins/kubernetes/proxy/redaction.go",
			"backend/internal/plugins/kubernetes/proxy/rewrite.go",
			"backend/internal/plugins/kubernetes/proxy/transport.go",
			"backend/internal/plugins/kubernetes/resource/arthas-bin.tar",
			"fronted/apps/web/app/(platform)/plugins/kubernetes/page.tsx",
		},
	})
}

func (p *plugin) Register(engine *gin.Engine) {
	kuberneteshttp.RegisterRoutes(engine)
}

func (p *plugin) RegisterWithContext(engine *gin.Engine, ctx sparkplugin.PluginContext) {
	kuberneteshttp.RegisterRoutes(engine, ctx)
}
