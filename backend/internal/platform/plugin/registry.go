package plugin

import "sync"

var (
	registryMu sync.Mutex
	registry   []Plugin

	bundledMu sync.Mutex
	bundled   = map[string]BundledDescriptor{}
)

type BundledDescriptor struct {
	ManifestYAML   string
	InstalledFiles []string
}

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

func RegisterBundled(id string, descriptor BundledDescriptor) {
	if id == "" || descriptor.ManifestYAML == "" {
		return
	}
	bundledMu.Lock()
	defer bundledMu.Unlock()
	bundled[id] = BundledDescriptor{
		ManifestYAML:   descriptor.ManifestYAML,
		InstalledFiles: append([]string(nil), descriptor.InstalledFiles...),
	}
}

func Bundled(id string) (BundledDescriptor, bool) {
	bundledMu.Lock()
	defer bundledMu.Unlock()
	descriptor, ok := bundled[id]
	if !ok {
		return BundledDescriptor{}, false
	}
	descriptor.InstalledFiles = append([]string(nil), descriptor.InstalledFiles...)
	return descriptor, true
}

func ResetRegistryForTest() {
	registryMu.Lock()
	registry = nil
	registryMu.Unlock()

	bundledMu.Lock()
	bundled = map[string]BundledDescriptor{}
	bundledMu.Unlock()
}
