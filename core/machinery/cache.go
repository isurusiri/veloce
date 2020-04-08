package machinery

import (
	"fmt"
	"sync"
	"time"

	"../globals"
	"../models"
)

// Cache represents the in memory key value store
type Cache struct {
	DefaultExpiration time.Duration
	Items             map[string]models.Item
	mu                sync.RWMutex
	onEvicted         func(string, interface{})
	GarbageCollector  *GarbageCollector
}

// Represents a key value pair
type keyAndValue struct {
	key   string
	value interface{}
}

// Set add an item to the cache, replacing any existing item.
// If the duration is 0 (DefaultExpiration), the cache's default
// expiration time is used. If it is -1 (NoExpiration), the item
// never expired
func (c *Cache) Set(key string, value interface{}, duration time.Duration) {
	var expiration int64
	if duration == globals.DefaultExpiration {
		duration = c.DefaultExpiration
	}

	if duration > 0 {
		expiration = time.Now().Add(duration).UnixNano()
	}

	c.mu.Lock()
	c.Items[key] = models.Item{
		Object:     value,
		Expiration: expiration,
	}
	c.mu.Unlock()
}

func (c *Cache) set(key string, value interface{}, duration time.Duration) {
	var expiration int64
	if duration == globals.DefaultExpiration {
		duration = c.DefaultExpiration
	}

	if duration > 0 {
		expiration = time.Now().Add(duration).UnixNano()
	}

	c.Items[key] = models.Item{
		Object:     value,
		Expiration: expiration,
	}
}

// SetDefault adds an item to the cache only if an item doesn't
// already exist for the given key, or if the existing item has
// expired. Returns an error otherwise.
func (c *Cache) SetDefault(key string, value interface{}) {
	c.Set(key, value, globals.DefaultExpiration)
}

// Add an item to the cache only if an item doesn't alrady exist
// for the given key, or if the existing item has expired. Returns
// an error otherwise.
func (c *Cache) Add(key string, value interface{}, duration time.Duration) error {
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
func (c *Cache) Replace(key string, value interface{}, duration time.Duration) error {
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
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	item, found := c.Items[key]

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
func (c *Cache) GetWithExpiration(key string) (interface{}, time.Time, bool) {
	c.mu.RLock()
	item, found := c.Items[key]

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

func (c *Cache) get(key string) (interface{}, bool) {
	item, found := c.Items[key]

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

func (c *Cache) delete(key string) (interface{}, bool) {
	if c.onEvicted != nil {
		if item, found := c.Items[key]; found {
			delete(c.Items, key)
			return item.Object, true
		}
	}

	delete(c.Items, key)
	return nil, false
}

// Delete an item from the cache. Does nothing if the key is not
// in the cache.
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	item, evicted := c.delete(key)
	c.mu.Unlock()
	if evicted {
		c.onEvicted(key, item)
	}
}

// DeleteExpired deletes expired items from the cache.
func (c *Cache) DeleteExpired() {
	var evictedItems []keyAndValue
	now := time.Now().UnixNano()

	c.mu.Lock()
	for key, value := range c.Items {
		if value.Expiration > 0 && now > value.Expiration {
			oldValue, evicted := c.delete(key)
			if evicted {
				evictedItems = append(evictedItems, keyAndValue{key, oldValue})
			}
		}
	}
	c.mu.Unlock()

	for _, value := range evictedItems {
		c.onEvicted(value.key, value.value)
	}
}

// OnEvicted sets an function that is called with the key and value when an
// item is evicted from the cache.
func (c *Cache) OnEvicted(f func(string, interface{})) {
	c.mu.Lock()
	c.onEvicted = f
	c.mu.Unlock()
}

// GetItems copies all unexpired items in the cache into a new map and return it.
func (c *Cache) GetItems() map[string]models.Item {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cacheMap := make(map[string]models.Item, len(c.Items))
	now := time.Now().UnixNano()

	for key, value := range c.Items {
		if value.Expiration > 0 {
			if now > value.Expiration {
				continue
			}
		}
		cacheMap[key] = value
	}
	return cacheMap
}

// ItemCount returns the count of all items in the cache including the expired
// items.
func (c *Cache) ItemCount() int {
	c.mu.RLock()
	itemCount := len(c.Items)
	c.mu.RUnlock()
	return itemCount
}

// Flush all items from the cache.
func (c *Cache) Flush() {
	c.mu.Lock()
	c.Items = map[string]models.Item{}
	c.mu.Unlock()
}
