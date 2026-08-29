package k8s

import (
	"fmt"
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
	if cfg.Runtime.Driver == "k8s" {
		if factory == nil || len(factory.Names()) == 0 {
			return Adapters{}, fmt.Errorf("runtime driver k8s requires configured kubernetes clusters")
		}
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
		return Adapters{
			Lister: lister, Logs: logs, Terminal: terminal, Resources: resources,
			Deployer: NewKubeconfigManifestDeployer(time.Second),
		}, nil
	}

	return Adapters{
		Lister:    NewMockPodLister(),
		Logs:      NewMockPodLogs(),
		Terminal:  NewMockPodTerminal(),
		Resources: NewMockResourceService(checker),
		Deployer:  NewMockManifestDeployer(),
	}, nil
}
