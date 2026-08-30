package plugin

import "sync"

var (
	registryMu sync.Mutex
	registry   []Plugin
)

func Register(p Plugin) {
	if p == nil {
		return
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = append(registry, p)
}

func Registered() []Plugin {
	registryMu.Lock()
	defer registryMu.Unlock()
	out := make([]Plugin, len(registry))
	copy(out, registry)
	return out
}

func ResetRegistryForTest() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = nil
}
