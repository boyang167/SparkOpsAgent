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
}

func (p *plugin) Register(engine *gin.Engine) {
	kuberneteshttp.RegisterRoutes(engine)
}

func (p *plugin) RegisterWithContext(engine *gin.Engine, _ sparkplugin.PluginContext) {
	kuberneteshttp.RegisterRoutes(engine)
}
