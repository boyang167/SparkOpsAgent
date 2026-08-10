package k8s

import (
	"fmt"
	"sync"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/dynamic"
)

type ClusterConfig struct {
	Name           string
	KubeconfigPath string
	Context        string
}

type Config struct {
	DefaultCluster string
	Clusters       []ClusterConfig
}

type Factory struct {
	mu             sync.RWMutex
	defaultCluster string
	clients        map[string]kubernetes.Interface
	configs        map[string]*rest.Config
}

func NewFactory(cfg Config) (*Factory, error) {
	f := &Factory{
		defaultCluster: cfg.DefaultCluster,
		clients:        make(map[string]kubernetes.Interface),
		configs:        make(map[string]*rest.Config),
	}
	if len(cfg.Clusters) == 0 {
		return f, nil
	}
	for _, cluster := range cfg.Clusters {
		if cluster.KubeconfigPath == "" || cluster.Context == "" {
			return nil, fmt.Errorf("cluster %q requires kubeconfig_path and context", cluster.Name)
		}
		restCfg, client, err := newClient(cluster.KubeconfigPath, cluster.Context)
		if err != nil {
			return nil, fmt.Errorf("cluster %q: %w", cluster.Name, err)
		}
		f.clients[cluster.Name] = client
		f.configs[cluster.Name] = restCfg
		if f.defaultCluster == "" {
			f.defaultCluster = cluster.Name
		}
	}
	return f, nil
}

func newClient(kubeconfigPath, context string) (*rest.Config, kubernetes.Interface, error) {
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{CurrentContext: context},
	)
	restCfg, err := loader.ClientConfig()
	if err != nil {
		return nil, nil, err
	}
	client, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, nil, err
	}
	return restCfg, client, nil
}

func (f *Factory) Get(clusterName string) (kubernetes.Interface, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if clusterName == "" {
		clusterName = f.defaultCluster
	}
	if clusterName == "" {
		return nil, fmt.Errorf("no kubernetes cluster configured")
	}
	client, ok := f.clients[clusterName]
	if !ok {
		return nil, fmt.Errorf("kubernetes cluster %q not found", clusterName)
	}
	return client, nil
}

func (f *Factory) Default() (kubernetes.Interface, error) {
	return f.Get("")
}

func (f *Factory) RESTConfig(clusterName string) (*rest.Config, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if clusterName == "" {
		clusterName = f.defaultCluster
	}
	if clusterName == "" {
		return nil, fmt.Errorf("no kubernetes cluster configured")
	}
	cfg, ok := f.configs[clusterName]
	if !ok {
		return nil, fmt.Errorf("kubernetes cluster %q not found", clusterName)
	}
	return cfg, nil
}

func (f *Factory) Dynamic(clusterName string) (dynamic.Interface, error) {
	restCfg, err := f.RESTConfig(clusterName)
	if err != nil {
		return nil, err
	}
	return dynamic.NewForConfig(restCfg)
}

func (f *Factory) Names() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	names := make([]string, 0, len(f.clients))
	for name := range f.clients {
		names = append(names, name)
	}
	return names
}
