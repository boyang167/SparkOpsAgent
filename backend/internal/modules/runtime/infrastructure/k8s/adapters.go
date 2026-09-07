package k8s

import (
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	platformk8s "github.com/sparkops/sparkops-agent/backend/internal/platform/k8s"
	"gorm.io/gorm"
)

type Adapters struct {
	Lister        port.PodLister
	Restarter     port.PodRestarter
	Logs          port.PodLogReader
	Terminal      port.PodTerminal
	PodMetrics    port.PodMetricsReader
	Resources     *ResourceService
	Deployer      port.ManifestDeployer
	Inspector     *ProjectRuntimeInspector
	Configurator  *ProjectRuntimeConfigurator
}

func NewAdapters(
	cfg *config.Config,
	factory *platformk8s.Factory,
	checker port.ProjectChecker,
	db *gorm.DB,
	projectAccess *ProjectClusterAccess,
) (Adapters, error) {
	adapters := Adapters{}
	if cfg.Runtime.Driver == "mock" {
		adapters.Deployer = NewMockManifestDeployer()
	} else {
		adapters.Deployer = NewKubeconfigManifestDeployer(time.Second)
	}

	if cfg.Runtime.Driver == "k8s" && factory != nil && len(factory.Names()) > 0 {
		cluster := cfg.Runtime.Cluster
		if cluster == "" {
			cluster = factory.Names()[0]
		}
		lister, err := NewPodLister(factory, cluster, checker)
		if err != nil {
			return Adapters{}, err
		}
		client, err := factory.Get(cluster)
		if err != nil {
			return Adapters{}, err
		}
		logs, err := NewPodLogReader(factory, cluster)
		if err != nil {
			return Adapters{}, err
		}
		terminal, err := NewPodTerminal(factory, cluster)
		if err != nil {
			return Adapters{}, err
		}
		resources, err := NewResourceService(cfg, factory, cluster, checker)
		if err != nil {
			return Adapters{}, err
		}
		podMetrics, err := NewPlatformPodMetricsReader(factory, cluster)
		if err != nil {
			return Adapters{}, err
		}
		adapters.Lister = lister
		adapters.Restarter = NewPlatformPodRestarter(client)
		adapters.Logs = logs
		adapters.Terminal = terminal
		adapters.PodMetrics = podMetrics
		adapters.Resources = resources
	} else if projectAccess != nil && db != nil {
		adapters.Lister = NewProjectPodLister(projectAccess, db)
		adapters.Restarter = NewProjectPodRestarter(projectAccess, db)
		adapters.Logs = NewProjectPodLogReader(projectAccess, db)
		adapters.Terminal = NewProjectPodTerminal(projectAccess, db)
		adapters.PodMetrics = NewProjectPodMetricsReader(projectAccess, db)
		adapters.Resources = NewMockResourceService(checker)
		adapters.Inspector = NewProjectRuntimeInspector(projectAccess)
		adapters.Configurator = NewProjectRuntimeConfigurator(projectAccess)
	} else {
		adapters.Lister = NewMockPodLister()
		adapters.Restarter = NewMockPodRestarter()
		adapters.Logs = NewMockPodLogs()
		adapters.Terminal = NewMockPodTerminal()
		adapters.PodMetrics = NewMockPodMetricsReader()
		adapters.Resources = NewMockResourceService(checker)
	}

	if adapters.PodMetrics == nil {
		adapters.PodMetrics = NewMockPodMetricsReader()
	}
	if adapters.Deployer == nil {
		adapters.Deployer = NewMockManifestDeployer()
	}
	return adapters, nil
}
