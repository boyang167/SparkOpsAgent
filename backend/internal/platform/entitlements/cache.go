package entitlements

import (
	"context"
	"os"
	"strconv"
	"sync"
	"time"

	"gorm.io/gorm"
)

const defaultCacheTTL = 30 * time.Second

type cacheEntry struct {
	value     WorkspaceEntitlements
	expiresAt time.Time
}

var (
	cacheMu      sync.RWMutex
	cacheEntries = map[string]cacheEntry{}
	cacheTTL     = defaultCacheTTL
)

func cacheKey(workspaceID string) string {
	if workspaceID == "" {
		return "__platform__"
	}
	return workspaceID
}

func ConfigureFromEnv() {
	if v := os.Getenv("ENTITLEMENTS_CACHE_TTL_SECONDS"); v != "" {
		ConfigureCacheSeconds(parseCacheSeconds(v))
	}
}

func ConfigureCacheSeconds(seconds int) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if seconds <= 0 {
		cacheTTL = 0
		cacheEntries = map[string]cacheEntry{}
		return
	}
	cacheTTL = time.Duration(seconds) * time.Second
}

func SetCacheTTL(ttl time.Duration) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if ttl <= 0 {
		cacheTTL = 0
		cacheEntries = map[string]cacheEntry{}
		return
	}
	cacheTTL = ttl
}

func parseCacheSeconds(raw string) int {
	sec, err := strconv.Atoi(raw)
	if err != nil {
		return 30
	}
	return sec
}

func currentCacheTTL() time.Duration {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	if cacheTTL <= 0 {
		return 0
	}
	return cacheTTL
}

func InvalidateCache() {
	cacheMu.Lock()
	cacheEntries = map[string]cacheEntry{}
	cacheMu.Unlock()
}

func ResolveCached(ctx context.Context, db *gorm.DB) (WorkspaceEntitlements, error) {
	return ResolveCachedForWorkspace(ctx, db, "")
}

func ResolveCachedForWorkspace(ctx context.Context, db *gorm.DB, workspaceID string) (WorkspaceEntitlements, error) {
	ttl := currentCacheTTL()
	key := cacheKey(workspaceID)
	if ttl <= 0 {
		return ResolveForWorkspace(ctx, db, workspaceID)
	}

	now := time.Now()

	cacheMu.RLock()
	if entry, ok := cacheEntries[key]; ok && now.Before(entry.expiresAt) {
		value := entry.value
		cacheMu.RUnlock()
		return value, nil
	}
	cacheMu.RUnlock()

	ents, err := ResolveForWorkspace(ctx, db, workspaceID)
	if err != nil {
		return ents, err
	}

	cacheMu.Lock()
	cacheEntries[key] = cacheEntry{value: ents, expiresAt: now.Add(ttl)}
	cacheMu.Unlock()

	return ents, nil
}
