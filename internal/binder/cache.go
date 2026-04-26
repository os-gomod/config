package binder

import (
	"sync"
	"time"

	"github.com/os-gomod/config/v2/internal/domain/value"
)

// ---------------------------------------------------------------------------
// Cache
// ---------------------------------------------------------------------------

// cacheEntry holds a cached binding result with an expiration time.
type cacheEntry struct {
	data    map[string]value.Value
	expires time.Time
}

// Cache provides a TTL-based cache for binding results. It is safe for
// concurrent use. The cache is instance-based — there are NO global caches.
type Cache struct {
	mu    sync.RWMutex
	items map[string]cacheEntry
	ttl   time.Duration
}

// NewCache creates a new Cache with the given TTL. If ttl is 0, entries
// never expire.
func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		items: make(map[string]cacheEntry),
		ttl:   ttl,
	}
}

// Get retrieves cached data for the given key. Returns the data and true
// if found and not expired, nil and false otherwise.
func (c *Cache) Get(key string) (map[string]value.Value, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.items[key]
	if !exists {
		return nil, false
	}

	// Check expiration.
	if !entry.expires.IsZero() && time.Now().After(entry.expires) {
		return nil, false
	}

	// Return a copy of the data.
	result := make(map[string]value.Value, len(entry.data))
	for k, v := range entry.data {
		result[k] = v
	}
	return result, true
}

// Set stores data in the cache with the given key.
func (c *Cache) Set(key string, data map[string]value.Value) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var expires time.Time
	if c.ttl > 0 {
		expires = time.Now().Add(c.ttl)
	}

	// Copy the data to avoid external mutation.
	copied := make(map[string]value.Value, len(data))
	for k, v := range data {
		copied[k] = v
	}

	c.items[key] = cacheEntry{
		data:    copied,
		expires: expires,
	}
}

// Invalidate removes a specific entry from the cache.
func (c *Cache) Invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// Clear removes all entries from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]cacheEntry)
}

// Len returns the number of cached entries (including potentially expired ones).
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// PurgeExpired removes all expired entries from the cache.
func (c *Cache) PurgeExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	purged := 0
	for key, entry := range c.items {
		if !entry.expires.IsZero() && now.After(entry.expires) {
			delete(c.items, key)
			purged++
		}
	}
	return purged
}

// Keys returns all cache keys (including potentially expired ones).
func (c *Cache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.items))
	for k := range c.items {
		keys = append(keys, k)
	}
	return keys
}

// TTL returns the configured cache TTL.
func (c *Cache) TTL() time.Duration {
	return c.ttl
}

// SetTTL changes the cache TTL. Existing entries are not affected.
func (c *Cache) SetTTL(ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ttl = ttl
}
