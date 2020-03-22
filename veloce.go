package veloce

import (
	"fmt"
	"sync"
	"time"
)

// Item represents the struct of an item to be
// stored in the cache
type Item struct {
	Object     interface{}
	Expiration int64
}

// Expired returns true if the Item has expired
func (item Item) Expired() bool {
	if item.Expiration == 0 {
		return false
	}

	return time.Now().UnixNano() > item.Expiration
}

const (
	// NoExpiration set the default value to use
	// with functions that take an expiration time.
	NoExpiration time.Duration = -1
	// DefaultExpiration set the default value to use
	// with functions that take an expiration time.
	// Equivalant to passing in the same expiration
	// duration as was given to New() or NewFrom()
	// when the cache was created (e.g. 5 minutes.).
	DefaultExpiration time.Duration = 0
)

type janitor struct {
	Interval time.Duration
	stop     chan bool
}

type cache struct {
	defaultExpiration time.Duration
	items             map[string]Item
	mu                sync.RWMutex
	onEvicted         func(string, interface{})
	janitor           *janitor
}

// Cache represents the in memory key-value store
type Cache struct {
	*cache
}

// Set add an item to the cache, replacing any existing item.
// If the duration is 0 (DefaultExpiration), the cache's default
// expiration time is used. If it is -1 (NoExpiration), the item
// never expired
func (c *cache) Set(key string, value interface{}, duration time.Duration) {
	var expiration int64
	if duration == DefaultExpiration {
		duration = c.defaultExpiration
	}

	if duration > 0 {
		expiration = time.Now().Add(duration).UnixNano()
	}

	c.mu.Lock()
	c.items[key] = Item{
		Object:     value,
		Expiration: expiration,
	}
	c.mu.Unlock()
}

func (c *cache) set(key string, value interface{}, duration time.Duration) {
	var expiration int64
	if duration == DefaultExpiration {
		duration = c.defaultExpiration
	}

	if duration > 0 {
		expiration = time.Now().Add(duration).UnixNano()
	}

	c.items[key] = Item{
		Object:     value,
		Expiration: expiration,
	}
}

// SetDefault, adds an item to the cache only if an item doesn't
// already exist for the given key, or if the existing item has
// expired. Returns an error otherwise.
func (c *cache) SetDefault(key string, value interface{}) {
	c.Set(key, value, DefaultExpiration)
}

// Add an item to the cache only if an item doesn't alrady exist
// for the given key, or if the existing item has expired. Returns
// an error otherwise.
func (c *cache) Add(key string, value interface{}, duration time.Duration) error {
	c.mu.Lock()
	_, found := c.get(key)

	if found {
		c.mu.Unlock()
		return fmt.Errorf("Item %s already exists", key)
	}

	c.set(key, value, duration)
	c.mu.Unlock()

	return nil
}

// Replace sets a new value for the cache key only if it already exists,
// and the existing item hasn't expired. Returns an error otherwise.
func (c *cache) Replace(key string, value interface{}, duration time.Duration) error {
	c.mu.Lock()
	_, found := c.get(key)

	if !found {
		c.mu.Unlock()
		return fmt.Errorf("Item %s doesn't exist", key)
	}

	c.set(key, value, duration)
	c.mu.Unlock()
	return nil
}

// Get an item from the cache. Returns the item or nil, and a bool
// indicating whether the key was found.
func (c *cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	item, found := c.items[key]

	if !found {
		c.mu.RUnlock()
		return nil, false
	}

	if item.Expiration > 0 {
		if time.Now().UnixNano() > item.Expiration {
			c.mu.RUnlock()
			return nil, false
		}
	}

	c.mu.RUnlock()
	return item.Object, true
}

// GetWithExpiration returns an item and its expiration time from the cache.
// It returns the item or nil, the expiration time if one is set and a bool
// indicating whether the key was found.
func (c *cache) GetWithExpiration(key string) (interface{}, time.Time, bool) {
	c.mu.RLock()
	item, found := c.items[key]

	if !found {
		c.mu.RUnlock()
		return nil, time.Time{}, false
	}

	if item.Expiration > 0 {
		if time.Now().UnixNano() > item.Expiration {
			c.mu.RUnlock()
			return nil, time.Time{}, false
		}

		// returns the item and the expiration time
		c.mu.RUnlock()
		return item.Object, time.Unix(0, item.Expiration), true
	}

	// Expiration is <= 0 means no expiration is set, therefore return
	// the item and a zero as time
	c.mu.RUnlock()
	return item.Object, time.Time{}, true
}

func (c *cache) get(key string) (interface{}, bool) {
	item, found := c.items[key]

	if !found {
		return nil, false
	}

	if item.Expiration > 0 {
		if time.Now().UnixNano() > item.Expiration {
			return nil, false
		}
	}

	return item.Object, true
}
