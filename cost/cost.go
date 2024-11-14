package cost

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/DrC0ns0le/net-perf/metrics"
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

func calculatePathCost(ctx context.Context, src, dst int) float64 {
	_, m, _, err := metrics.GetPreferredVersion(ctx, src-64512, dst-64512)
	if err != nil {
		return 0
	}
	if m == nil || m.Availability == 0 || m.Latency == 0 {
		return math.Inf(1)
	}

	const (
		K1 = 1.0 // Latency weight
		K2 = 1.0 // Load/Loss weight
		K3 = 0.5 // Jitter weight
	)

	latencyMs := m.Latency / 1e6
	normalizedLoss := m.PacketLoss / 100

	if normalizedLoss >= 1 {
		return math.Inf(1)
	}

	jitterMs := m.Jitter / 1e6

	cost := K1*latencyMs +
		K2*(latencyMs*normalizedLoss/(1-normalizedLoss)) +
		K3*jitterMs

	if m.Availability < 1 {
		cost /= m.Availability
	}

	return cost * 1000
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
	cost := calculatePathCost(ctx, src, dst)

	globalCache.mu.Lock()
	globalCache.cache[key] = cacheEntry{
		cost:       cost,
		expiration: time.Now().Add(1 * time.Minute),
	}
	globalCache.mu.Unlock()

	return cost
}
