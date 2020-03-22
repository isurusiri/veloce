package veloce

import (
	"fmt"
	"runtime"
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

type garbageCollector struct {
	Interval time.Duration
	stop     chan bool
}

type cache struct {
	defaultExpiration time.Duration
	items             map[string]Item
	mu                sync.RWMutex
	onEvicted         func(string, interface{})
	garbageCollector  *garbageCollector
}

// Cache represents the in memory key-value store
type Cache struct {
	*cache
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

func (c *cache) delete(key string) (interface{}, bool) {
	if c.onEvicted != nil {
		if item, found := c.items[key]; found {
			delete(c.items, key)
			return item.Object, true
		}
	}

	delete(c.items, key)
	return nil, false
}

// Delete an item from the cache. Does nothing if the key is not
// in the cache.
func (c *cache) Delete(key string) {
	c.mu.Lock()
	item, evicted := c.delete(key)
	c.mu.Unlock()
	if evicted {
		c.onEvicted(key, item)
	}
}

// DeleteExpired, deletes expired items from the cache.
func (c *cache) DeleteExpired() {
	var evictedItems []keyAndValue
	now := time.Now().UnixNano()

	c.mu.Lock()
	for key, value := range c.items {
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

// OnEvicted, sets an function that is called with the key and value when an
// item is evicted from the cache.
func (c *cache) OnEvicted(f func(string, interface{})) {
	c.mu.Lock()
	c.onEvicted = f
	c.mu.Unlock()
}

// Items, copies all unexpired items in the cache into a new map and return it.
func (c *cache) Items() map[string]Item {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cacheMap := make(map[string]Item, len(c.items))
	now := time.Now().UnixNano()

	for key, value := range c.items {
		if value.Expiration > 0 {
			if now > value.Expiration {
				continue
			}
		}
		cacheMap[key] = value
	}
	return cacheMap
}

// ItemCount, returns the count of all items in the cache including the expired
// items.
func (c *cache) ItemCount() int {
	c.mu.RLock()
	itemCount := len(c.items)
	c.mu.RUnlock()
	return itemCount
}

// Delete all items from the cache.
func (c *cache) Flush() {
	c.mu.Lock()
	c.items = map[string]Item{}
	c.mu.Unlock()
}

// Run the garbage collector to clean up expired items from the cache.
func (gc *garbageCollector) Run(c *cache) {
	ticker := time.NewTicker(gc.Interval)

	for {
		select {
		case <-ticker.C:
			c.DeleteExpired()
		case <-gc.stop:
			ticker.Stop()
			return
		}
	}
}

func stopGarbageCollector(c *cache) {
	c.garbageCollector.stop <- true
}

func runGarbageCollector(c *cache, cleanUpInterval time.Duration) {
	gc := &garbageCollector{
		Interval: cleanUpInterval,
		stop:     make(chan bool),
	}
	c.garbageCollector = gc
	go gc.Run(c)
}

func newCache(duration time.Duration, cacheItems map[string]Item) *cache {
	if duration == 0 {
		duration = -1
	}
	c := &cache{
		defaultExpiration: duration,
		items:             cacheItems,
	}
	return c
}

func newCacheWithGarbageCollector(duration time.Duration, cleanUpInterval time.Duration, cacheItems map[string]Item) *Cache {
	c := newCache(duration, cacheItems)

	// makesure gc goroutine doesn't clean C (Cache) once it is returned.
	C := &Cache{c}

	if cleanUpInterval > 0 {
		runGarbageCollector(c, cleanUpInterval)
		runtime.SetFinalizer(C, stopGarbageCollector)
	}
	return C
}

// New returns a new cache with a given expiration time duration. A garbage collector is
// initialized with a given clean up inerval.
// If the expiration duration is less than one the items in the cache never expire, and
// must be deleted manually.
// If the cleanup interval is less than one, expired items are not deleted from the cache
// before calling c.DeleteExpired().
func New(defaultExpiration time.Duration, cleanUpInterval time.Duration) *Cache {
	items := make(map[string]Item)
	return newCacheWithGarbageCollector(defaultExpiration, cleanUpInterval, items)
}

// NewForm returns a new cache with a given expiration time duration. A garbage collector is
// initialized with a given clean up inerval.
// If the expiration duration is less than one the items in the cache never expire, and
// must be deleted manually.
// If the cleanup interval is less than one, expired items are not deleted from the cache
// before calling c.DeleteExpired().
func NewForm(defaultExpiration time.Duration, cleanUpInterval time.Duration, items map[string]Item) *Cache {
	return newCacheWithGarbageCollector(defaultExpiration, cleanUpInterval, items)
}
