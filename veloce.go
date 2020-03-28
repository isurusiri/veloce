package veloce

import (
	"runtime"
	"time"

	"./core/machinery"
	"./core/models"
)

// Cache represents the in memory key-value store
type Cache struct {
	*machinery.Cache
}

func stopGarbageCollector(c *machinery.Cache) {
	c.garbageCollector.stop <- true
}

func runGarbageCollector(c *machinery.Cache, cleanUpInterval time.Duration) {
	gc := &garbageCollector{
		Interval: cleanUpInterval,
		stop:     make(chan bool),
	}
	c.garbageCollector = gc
	go gc.Run(c)
}

func newCache(duration time.Duration, cacheItems map[string]models.Item) *machinery.Cache {
	if duration == 0 {
		duration = -1
	}
	c := &machinery.Cache{
		defaultExpiration: duration,
		items:             cacheItems,
	}
	return c
}

func newCacheWithGarbageCollector(duration time.Duration, cleanUpInterval time.Duration, cacheItems map[string]models.Item) *Cache {
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
	items := make(map[string]models.Item)
	return newCacheWithGarbageCollector(defaultExpiration, cleanUpInterval, items)
}

// NewForm returns a new cache with a given expiration time duration. A garbage collector is
// initialized with a given clean up inerval.
// If the expiration duration is less than one the items in the cache never expire, and
// must be deleted manually.
// If the cleanup interval is less than one, expired items are not deleted from the cache
// before calling c.DeleteExpired().
func NewForm(defaultExpiration time.Duration, cleanUpInterval time.Duration, items map[string]models.Item) *Cache {
	return newCacheWithGarbageCollector(defaultExpiration, cleanUpInterval, items)
}
