package cost

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/DrC0ns0le/net-bird/logging"
	"github.com/DrC0ns0le/net-perf/metrics"
)

const (
	cacheExpiration = 15 * time.Second
)

type cacheEntry struct {
	cost       float64
	expiration time.Time
}

type pathCostCache struct {
	mu    sync.RWMutex
	cache map[string]cacheEntry
}

func newPathCostCache() *pathCostCache {
	cache := &pathCostCache{
		cache: make(map[string]cacheEntry),
	}

	go cache.cleanExpired()
	return cache
}

var globalCache = newPathCostCache()

func generateCacheKey(src, dst int) string {
	return fmt.Sprintf("%d-%d", src, dst)
}

func (c *pathCostCache) cleanExpired() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.cache {
			if now.After(entry.expiration) {
				delete(c.cache, key)
			}
		}
		c.mu.Unlock()
	}
}

// GetPathCost returns the cached cost if available, otherwise calculates and caches it
func GetPathCost(ctx context.Context, src, dst int) float64 {
	key := generateCacheKey(src, dst)

	// Try to get from cache first
	globalCache.mu.RLock()
	if entry, exists := globalCache.cache[key]; exists && time.Now().Before(entry.expiration) {
		globalCache.mu.RUnlock()
		return entry.cost
	}
	globalCache.mu.RUnlock()

	// Not in cache, calculate
	cost := SetPathCost(ctx, src, dst)

	globalCache.mu.Lock()
	globalCache.cache[key] = cacheEntry{
		cost:       cost,
		expiration: time.Now().Add(cacheExpiration),
	}
	globalCache.mu.Unlock()

	return cost
}

// SetPathCost returns the cost of a path
// Custom path costs can be added here
func SetPathCost(ctx context.Context, src, dst int) float64 {

	switch dst {
	case 65000:
		return 0
	}

	_, cost, err := metrics.GetPreferredPath(ctx, src-64512, dst-64512)
	if err != nil {
		logging.Errorf("Error getting preferred path and cost for %d -> %d: %v\n", src, dst, err)
		return 0
	}

	if cost == 0 {
		logging.Errorf("Unexpected cost of 0 for %d -> %d\n", src, dst)
		return 0
	}

	switch dst {
	case 64512:
		cost = 1.5 * cost
	}

	return cost
}
