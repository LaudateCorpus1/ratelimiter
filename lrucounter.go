// This package provides a simple LRU cache. It is based on the
// LRU implementation in groupcache:
// https://github.com/golang/groupcache/tree/master/lru
// https://github.com/hashicorp/golang-lru/blob/master/lru.go

// This package aims to provide an LRU cache that operates on counters as values.
// There are situations where while processing data you only care about something the first
// n times it happens. After that it's not as useful. Redis has a nice feature of it's incr command
// that returns the current count. If you're lucky enough to have your data partitioned to a particular
// node then you can utilze a local REDIS type incr command
// You can use this pattern for basic Rate Limiting, by passing in the valid seconds a given count is good for
// if it passes those seconds we zero out the counter again

// Package lru implements an LRU cache.
package ratelimiter

import (
	"container/list"
	"errors"
	"sync"
	"time"
)

// Cache is an LRU cache. It is not safe for concurrent access.
type Cache struct {

	// MaxEntries is the maximum number of cache entries before
	// an item is evicted. Zero means no limit.
	MaxEntries int

	// OnEvicted optionally specificies a callback function to be
	// executed when an entry is purged from the cache.
	OnEvicted func(key interface{}, value interface{})

	// how long of a period of time does the rate limit apply
	ratePeriod time.Duration

	evictList *list.List
	cache     map[interface{}]*list.Element

	lock sync.RWMutex
}

type entry struct {
	key   interface{}
	value uint64
	// stores the time that the entry was first incremented
	updated time.Time
}

// New creates a new Cache.
// ratePeriod is the window between now and seconds ago the rate limit applies
func New(maxEntries int, ratePeriod time.Duration) (*Cache, error) {
	if maxEntries <= 0 {
		return nil, errors.New("Must provide a positive size")
	}
	return &Cache{
		MaxEntries: maxEntries,
		evictList:  list.New(),
		cache:      make(map[interface{}]*list.Element),
		ratePeriod: ratePeriod,
	}, nil
}

// Incr allows you to increment a key, if it's over the rate limit maxValue and it's been shorter
// than the grace period then it will return false for the underRateLimit boolean
func (c *Cache) Incr(key interface{}, maxValue int) (uint64, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	underRateLimit := true

	// check to make sure we have space, if not purge the oldest item
	if c.evictList.Len() > c.MaxEntries-1 {
		c.removeOldest()
	}

	if ee, ok := c.cache[key]; ok {
		c.evictList.MoveToFront(ee)
		ee.Value.(*entry).value++
		if ee.Value.(*entry).value > uint64(maxValue) {

			// check to see if we're over our rate limit AND we're within the ratePeriod duration
			// if so then fail the rate limit otherwise reset the times and values for the current period
			if c.ratePeriod > 0 {
				dur := time.Now().UTC().Sub(ee.Value.(*entry).updated)
				if dur > c.ratePeriod {
					ee.Value.(*entry).value = 1
					ee.Value.(*entry).updated = time.Now().UTC()
				} else {
					underRateLimit = false
				}
			} else {
				underRateLimit = false
			}

		}

		return ee.Value.(*entry).value, underRateLimit

	} else {
		// new item
		item := &entry{key, uint64(1), time.Now().UTC()}

		entry := c.evictList.PushFront(item)
		c.cache[key] = entry

		return item.value, underRateLimit
	}

}

// Get looks up a key's value from the cache.
func (c *Cache) Get(key interface{}) (value uint64, ok bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if ent, ok := c.cache[key]; ok {
		c.evictList.MoveToFront(ent)
		return ent.Value.(*entry).value, true
	}
	return
}

// Remove removes the provided key from the cache.
func (c *Cache) Remove(key interface{}) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if ent, ok := c.cache[key]; ok {
		c.removeElement(ent)
	}
}

// Len returns the number of items in the cache.
func (c *Cache) Len() int {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.evictList.Len()
}

// removeOldest removes the oldest item from the cache.
func (c *Cache) removeOldest() {
	ent := c.evictList.Back()
	if ent != nil {
		c.removeElement(ent)
	}
}

// removeElement is used to remove a given list element from the cache
func (c *Cache) removeElement(e *list.Element) {
	c.evictList.Remove(e)
	kv := e.Value.(*entry)
	delete(c.cache, kv.key)
}
