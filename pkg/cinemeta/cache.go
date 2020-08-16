package cinemeta

import (
	"sync"
	"time"
)

// CacheItem combines a meta object and a creation time in a single struct.
// This can be useful for implementing the Cache interface, but is not necessarily required.
// See the InMemoryCache example implementation of the Cache interface for its usage.
type CacheItem struct {
	Meta    Meta
	Created time.Time
}

// Cache is the interface that the cinemeta client uses for caching meta.
// A package user must pass an implementation of this interface.
// Usually you create a simple wrapper around an existing cache package.
// An example implementation is the InMemoryCache in this package.
type Cache interface {
	Set(key string, movie Meta) error
	Get(key string) (Meta, time.Time, bool, error)
}

var _ Cache = (*InMemoryCache)(nil)

// InMemoryCache is an example implementation of the Cache interface.
// It doesn't persist its data, so it's not suited for production use of the cinemeta package.
type InMemoryCache struct {
	cache map[string]CacheItem
	lock  *sync.RWMutex
}

// NewInMemoryCache creates a new InMemoryCache.
func NewInMemoryCache() *InMemoryCache {
	return &InMemoryCache{
		cache: map[string]CacheItem{},
		lock:  &sync.RWMutex{},
	}
}

// Set stores a meta object and the current time in the cache.
func (c *InMemoryCache) Set(key string, meta Meta) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.cache[key] = CacheItem{
		Meta:    meta,
		Created: time.Now(),
	}
	return nil
}

// Get returns a meta object and the time it was cached from the cache.
// The boolean return value signals if the value was found in the cache.
func (c *InMemoryCache) Get(key string) (Meta, time.Time, bool, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	cacheItem, found := c.cache[key]
	return cacheItem.Meta, cacheItem.Created, found, nil
}
