package k8s

import (
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	platformk8s "github.com/sparkops/sparkops-agent/backend/internal/platform/k8s"
)

type Adapters struct {
	Lister    port.PodLister
	Logs      port.PodLogReader
	Terminal  port.PodTerminal
	Resources *ResourceService
	Deployer  port.ManifestDeployer
}

func NewAdapters(
	cfg *config.Config,
	factory *platformk8s.Factory,
	checker port.ProjectChecker,
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
		adapters.Lister = lister
		adapters.Logs = logs
		adapters.Terminal = terminal
		adapters.Resources = resources
		return adapters, nil
	}

	if cfg.Runtime.Driver == "k8s" && (factory == nil || len(factory.Names()) == 0) {
		// Deployment still uses per-project kubeconfig via ManifestDeployer above.
		// Pod/resource APIs stay on mock adapters until platform k8s is configured.
	}

	return Adapters{
		Lister:    NewMockPodLister(),
		Logs:      NewMockPodLogs(),
		Terminal:  NewMockPodTerminal(),
		Resources: NewMockResourceService(checker),
		Deployer:  adapters.Deployer,
	}, nil
}
