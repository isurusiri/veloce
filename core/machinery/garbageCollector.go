package machinery

import (
	"time"
)

// GarbageCollector provides the buleprint for garbage collector
type GarbageCollector struct {
	Interval time.Duration
	stop     chan bool
}

// Run the garbage collector to clean up expired items from the cache.
func (gc *GarbageCollector) Run(c *Cache) {
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
