package main

import (
	"container/list"
	"sync"
	"sync/atomic"
	"time"
)

const defaultCacheCapacity = 4096

type memoryCache struct {
	mu       sync.Mutex
	order    *list.List
	items    map[string]*list.Element
	capacity int
	hits     atomic.Uint64
	misses   atomic.Uint64
	expires  atomic.Uint64
	evicts   atomic.Uint64
}

type cacheItem struct {
	key       string
	body      []byte
	expiresAt time.Time
}

type cacheStats struct {
	Items    int    `json:"items"`
	Capacity int    `json:"capacity"`
	Hits     uint64 `json:"hits"`
	Misses   uint64 `json:"misses"`
	Expires  uint64 `json:"expires"`
	Evicts   uint64 `json:"evicts"`
}

func newMemoryCache() *memoryCache {
	return newMemoryCacheWithCapacity(defaultCacheCapacity)
}

func newMemoryCacheWithCapacity(capacity int) *memoryCache {
	if capacity <= 0 {
		capacity = defaultCacheCapacity
	}
	return &memoryCache{
		order:    list.New(),
		items:    make(map[string]*list.Element, capacity),
		capacity: capacity,
	}
}

func (c *memoryCache) get(key string) ([]byte, bool) {
	now := time.Now()
	c.mu.Lock()
	el, ok := c.items[key]
	if !ok {
		c.mu.Unlock()
		c.misses.Add(1)
		return nil, false
	}
	item := el.Value.(*cacheItem)
	if !now.Before(item.expiresAt) {
		c.order.Remove(el)
		delete(c.items, key)
		c.mu.Unlock()
		c.expires.Add(1)
		c.misses.Add(1)
		return nil, false
	}
	c.order.MoveToFront(el)
	body := item.body
	c.mu.Unlock()
	c.hits.Add(1)
	return body, true
}

func (c *memoryCache) set(key string, body []byte, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	copied := append([]byte(nil), body...)
	expiresAt := time.Now().Add(ttl)
	c.mu.Lock()
	if el, ok := c.items[key]; ok {
		item := el.Value.(*cacheItem)
		item.body = copied
		item.expiresAt = expiresAt
		c.order.MoveToFront(el)
		c.mu.Unlock()
		return
	}
	el := c.order.PushFront(&cacheItem{key: key, body: copied, expiresAt: expiresAt})
	c.items[key] = el
	evicted := false
	for len(c.items) > c.capacity {
		tail := c.order.Back()
		if tail == nil {
			break
		}
		c.order.Remove(tail)
		delete(c.items, tail.Value.(*cacheItem).key)
		evicted = true
	}
	c.mu.Unlock()
	if evicted {
		c.evicts.Add(1)
	}
}

func (c *memoryCache) stats() cacheStats {
	c.mu.Lock()
	items := len(c.items)
	capacity := c.capacity
	c.mu.Unlock()
	return cacheStats{
		Items:    items,
		Capacity: capacity,
		Hits:     c.hits.Load(),
		Misses:   c.misses.Load(),
		Expires:  c.expires.Load(),
		Evicts:   c.evicts.Load(),
	}
}
