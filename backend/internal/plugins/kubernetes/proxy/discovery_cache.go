package proxy

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

const discoveryTimeout = 5 * time.Second

var (
	defaultDiscoveryCache   = newDiscoveryCache(5 * time.Minute)
	errDiscoveryFailed      = errors.New("kubernetes discovery failed")
	errUpstreamBodyTooLarge = errors.New("upstream response too large")
)

type discoveryCache struct {
	ttl      time.Duration
	mu       sync.Mutex
	versions map[string]versionCacheEntry
	scopes   map[scopeCacheKey]scopeCacheEntry
}

type versionCacheEntry struct {
	minor     int
	expiresAt time.Time
}

type scopeCacheKey struct {
	clusterID    string
	groupVersion string
	resource     string
}

type scopeCacheEntry struct {
	namespaced bool
	expiresAt  time.Time
}

func newDiscoveryCache(ttl time.Duration) *discoveryCache {
	return &discoveryCache{
		ttl:      ttl,
		versions: make(map[string]versionCacheEntry),
		scopes:   make(map[scopeCacheKey]scopeCacheEntry),
	}
}

func (c *discoveryCache) versionMinor(clusterID kernel.ClusterInfoID, restCfg *rest.Config) (int, bool) {
	key := clusterID.String()
	now := time.Now()
	c.mu.Lock()
	if entry, ok := c.versions[key]; ok && now.Before(entry.expiresAt) {
		c.mu.Unlock()
		return entry.minor, true
	}
	c.mu.Unlock()

	client, err := discovery.NewDiscoveryClientForConfig(discoveryConfig(restCfg))
	if err != nil {
		return 0, false
	}
	version, err := client.ServerVersion()
	if err != nil || version == nil {
		return 0, false
	}
	minor, ok := parseMinor(version.Minor)
	if !ok {
		return 0, false
	}
	c.mu.Lock()
	c.versions[key] = versionCacheEntry{minor: minor, expiresAt: now.Add(c.ttl)}
	c.mu.Unlock()
	return minor, true
}

func (c *discoveryCache) resourceNamespaced(clusterID kernel.ClusterInfoID, restCfg *rest.Config, groupVersion, resource string) (bool, error) {
	key := scopeCacheKey{clusterID: clusterID.String(), groupVersion: groupVersion, resource: resource}
	now := time.Now()
	c.mu.Lock()
	if entry, ok := c.scopes[key]; ok && now.Before(entry.expiresAt) {
		c.mu.Unlock()
		return entry.namespaced, nil
	}
	c.mu.Unlock()

	client, err := discovery.NewDiscoveryClientForConfig(discoveryConfig(restCfg))
	if err != nil {
		return false, errDiscoveryFailed
	}
	resources, err := client.ServerResourcesForGroupVersion(groupVersion)
	if err != nil || resources == nil {
		return false, errDiscoveryFailed
	}
	for _, apiResource := range resources.APIResources {
		if apiResource.Name == resource {
			c.mu.Lock()
			c.scopes[key] = scopeCacheEntry{namespaced: apiResource.Namespaced, expiresAt: now.Add(c.ttl)}
			c.mu.Unlock()
			return apiResource.Namespaced, nil
		}
	}
	return false, errDiscoveryFailed
}

func (c *discoveryCache) resolveGroupVersion(
	clusterID kernel.ClusterInfoID,
	restCfg *rest.Config,
	group, preferredVersion, resource string,
) (string, bool) {
	for _, version := range gatewayVersionCandidates(preferredVersion) {
		groupVersion := group + "/" + version
		if _, err := c.resourceNamespaced(clusterID, restCfg, groupVersion, resource); err == nil {
			return version, true
		}
	}
	return preferredVersion, false
}

func discoveryConfig(restCfg *rest.Config) *rest.Config {
	cfg := rest.CopyConfig(restCfg)
	cfg.Timeout = discoveryTimeout
	return cfg
}

func parseMinor(raw string) (int, bool) {
	var digits strings.Builder
	for _, r := range raw {
		if r < '0' || r > '9' {
			break
		}
		digits.WriteRune(r)
	}
	if digits.Len() == 0 {
		return 0, false
	}
	minor, err := strconv.Atoi(digits.String())
	return minor, err == nil
}
