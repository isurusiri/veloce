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

// Increment an item of type int, int8, int16, int32, int64, uintptr,
// uint, uint8, uint32, or uint64, float32 or float64 by n. This will
// return an error if the item's value is not an integer, if it was
// not found, or if it is not possible to increment it by n.
func (c *cache) Increment(key string, n int64) error {
	c.mu.Lock()
	item, found := c.items[key]

	if !found || item.Expired() {
		c.mu.Unlock()
		return fmt.Errorf("Item %s not found", key)
	}

	switch item.Object.(type) {
	case int:
		item.Object = item.Object.(int) + int(n)
	case int8:
		item.Object = item.Object.(int8) + int8(n)
	case int16:
		item.Object = item.Object.(int16) + int16(n)
	case int32:
		item.Object = item.Object.(int32) + int32(n)
	case int64:
		item.Object = item.Object.(int64) + n
	case uint:
		item.Object = item.Object.(uint) + uint(n)
	case uintptr:
		item.Object = item.Object.(uintptr) + uintptr(n)
	case uint8:
		item.Object = item.Object.(uint8) + uint8(n)
	case uint16:
		item.Object = item.Object.(uint16) + uint16(n)
	case uint32:
		item.Object = item.Object.(uint32) + uint32(n)
	case uint64:
		item.Object = item.Object.(uint64) + uint64(n)
	case float32:
		item.Object = item.Object.(float32) + float32(n)
	case float64:
		item.Object = item.Object.(float64) + float64(n)
	default:
		c.mu.Unlock()
		return fmt.Errorf("The value for %s is not an integer", key)
	}

	c.items[key] = item
	c.mu.Unlock()
	return nil
}

// Increment an item of type float32 or float64 by n. This will returns
// an error if the item's value is not float32 or float64, if it was not
// found, or if it is not possible to increment it by n. Pass a negative
// number to decrement the value.
func (c *cache) IncrementFloat(key string, n float64) error {
	c.mu.Lock()
	item, found := c.items[key]

	if !found || item.Expired() {
		c.mu.Unlock()
		return fmt.Errorf("Item %s not found", key)
	}

	switch item.Object.(type) {
	case float32:
		item.Object = item.Object.(float32) + float32(n)
	case float64:
		item.Object = item.Object.(float64) + n
	default:
		c.mu.Unlock()
		return fmt.Errorf("The value for %s is not type of float32 or float64", key)
	}

	c.items[key] = item
	c.mu.Unlock()
	return nil
}

// Increment an item of type int by n. Returns an error if the item's
// value is not an int, or if it was not found. If there is no error,
// the incremented value is returned.
func (c *cache) IncrementInt(key string, n int) (int, error) {
	c.mu.Lock()
	item, found := c.items[key]

	if !found || item.Expired() {
		c.mu.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}

	value, ok := item.Object.(int)

	if !ok {
		c.mu.Unlock()
		return 0, fmt.Errorf("The value for %s is not an int", key)
	}

	newValue := value + n
	item.Object = newValue
	c.items[key] = item
	c.mu.Unlock()
	return newValue, nil
}

// Increment an item of type int8 by n. Returns an error if the item's
// value is not an int8, or if it was not found. If there is no error,
// the incremented value is returned.
func (c *cache) IncrementInt8(key string, n int8) (int8, error) {
	c.mu.Lock()
	item, found := c.items[key]

	if !found || item.Expired() {
		c.mu.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}

	value, ok := item.Object.(int8)

	if !ok {
		c.mu.Unlock()
		return 0, fmt.Errorf("The value for %s is not an int8", key)
	}

	newValue := value + n
	item.Object = newValue
	c.items[key] = item
	c.mu.Unlock()
	return newValue, nil
}

// Increment an item of type int16 by n. Returns an error if the item's
// value is not an int16, or if it was not found. If there is no error,
// the incremented value is returned.
func (c *cache) IncrementInt16(key string, n int16) (int16, error) {
	c.mu.Lock()
	item, found := c.items[key]

	if !found || item.Expired() {
		c.mu.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}

	value, ok := item.Object.(int16)

	if !ok {
		c.mu.Unlock()
		return 0, fmt.Errorf("The value for %s is not an int16", key)
	}

	newValue := value + n
	item.Object = newValue
	c.items[key] = item
	c.mu.Unlock()
	return newValue, nil
}

// Increment an item of type int32 by n. Returns an error if the item's
// value is not an int32, or if it was not found. If there is no error,
// the incremented value is returned.
func (c *cache) IncrementInt32(key string, n int32) (int32, error) {
	c.mu.Lock()
	item, found := c.items[key]

	if !found || item.Expired() {
		c.mu.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}

	value, ok := item.Object.(int32)

	if !ok {
		c.mu.Unlock()
		return 0, fmt.Errorf("The value for %s is not an int32", key)
	}

	newValue := value + n
	item.Object = newValue
	c.items[key] = item
	c.mu.Unlock()
	return newValue, nil
}

// Increment an item of type int64 by n. Returns an error if the item's
// value is not an int64, or if it was not found. If there is no error,
// the incremented value is returned.
func (c *cache) IncrementInt64(key string, n int64) (int64, error) {
	c.mu.Lock()
	item, found := c.items[key]

	if !found || item.Expired() {
		c.mu.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}

	value, ok := item.Object.(int64)

	if !ok {
		c.mu.Unlock()
		return 0, fmt.Errorf("The value for %s is not an int64", key)
	}

	newValue := value + n
	item.Object = newValue
	c.items[key] = item
	c.mu.Unlock()
	return newValue, nil
}

// Increment an item of type uint by n. Returns an error if the item's
// value is not an uint, or if it was not found. If there is no error,
// the incremented value is returned.
func (c *cache) IncrementUint(key string, n uint) (uint, error) {
	c.mu.Lock()
	item, found := c.items[key]

	if !found || item.Expired() {
		c.mu.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}

	value, ok := item.Object.(uint)

	if !ok {
		c.mu.Unlock()
		return 0, fmt.Errorf("The value for %s is not an uint", key)
	}

	newValue := value + n
	item.Object = newValue
	c.items[key] = item
	c.mu.Unlock()
	return newValue, nil
}

// Increment an item of type uintptr by n. Returns an error if the item's
// value is not an uintptr, or if it was not found. If there is no error,
// the incremented value is returned.
func (c *cache) IncrementUintptr(key string, n uintptr) (uintptr, error) {
	c.mu.Lock()
	item, found := c.items[key]

	if !found || item.Expired() {
		c.mu.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}

	value, ok := item.Object.(uintptr)

	if !ok {
		c.mu.Unlock()
		return 0, fmt.Errorf("The value for %s is not an uintptr", key)
	}

	newValue := value + n
	item.Object = newValue
	c.items[key] = item
	c.mu.Unlock()
	return newValue, nil
}

// Increment an item of type uint8 by n. Returns an error if the item's
// value is not an uint8, or if it was not found. If there is no error,
// the incremented value is returned.
func (c *cache) IncrementUint8(key string, n uint8) (uint8, error) {
	c.mu.Lock()
	item, found := c.items[key]

	if !found || item.Expired() {
		c.mu.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}

	value, ok := item.Object.(uint8)

	if !ok {
		c.mu.Unlock()
		return 0, fmt.Errorf("The value for %s is not an uint8", key)
	}

	newValue := value + n
	item.Object = newValue
	c.items[key] = item
	c.mu.Unlock()
	return newValue, nil
}

// Increment an item of type uint16 by n. Returns an error if the item's
// value is not an uint16, or if it was not found. If there is no error,
// the incremented value is returned.
func (c *cache) IncrementUint16(key string, n uint16) (uint16, error) {
	c.mu.Lock()
	item, found := c.items[key]

	if !found || item.Expired() {
		c.mu.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}

	value, ok := item.Object.(uint16)

	if !ok {
		c.mu.Unlock()
		return 0, fmt.Errorf("The value for %s is not an uint16", key)
	}

	newValue := value + n
	item.Object = newValue
	c.items[key] = item
	c.mu.Unlock()
	return newValue, nil
}

// Increment an item of type uint32 by n. Returns an error if the item's
// value is not an uint32, or if it was not found. If there is no error,
// the incremented value is returned.
func (c *cache) IncrementUint32(key string, n uint32) (uint32, error) {
	c.mu.Lock()
	item, found := c.items[key]

	if !found || item.Expired() {
		c.mu.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}

	value, ok := item.Object.(uint32)

	if !ok {
		c.mu.Unlock()
		return 0, fmt.Errorf("The value for %s is not an uint32", key)
	}

	newValue := value + n
	item.Object = newValue
	c.items[key] = item
	c.mu.Unlock()
	return newValue, nil
}

// Increment an item of type uint64 by n. Returns an error if the item's
// value is not an uint64, or if it was not found. If there is no error,
// the incremented value is returned.
func (c *cache) IncrementUint64(key string, n uint64) (uint64, error) {
	c.mu.Lock()
	item, found := c.items[key]

	if !found || item.Expired() {
		c.mu.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}

	value, ok := item.Object.(uint64)

	if !ok {
		c.mu.Unlock()
		return 0, fmt.Errorf("The value for %s is not an uint64", key)
	}

	newValue := value + n
	item.Object = newValue
	c.items[key] = item
	c.mu.Unlock()
	return newValue, nil
}

// Increment an item of type float32 by n. Returns an error if the item's
// value is not an float32, or if it was not found. If there is no error,
// the incremented value is returned.
func (c *cache) IncrementFloat32(key string, n float32) (float32, error) {
	c.mu.Lock()
	item, found := c.items[key]

	if !found || item.Expired() {
		c.mu.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}

	value, ok := item.Object.(float32)

	if !ok {
		c.mu.Unlock()
		return 0, fmt.Errorf("The value for %s is not an float32", key)
	}

	newValue := value + n
	item.Object = newValue
	c.items[key] = item
	c.mu.Unlock()
	return newValue, nil
}

// Increment an item of type float64 by n. Returns an error if the item's
// value is not an float64, or if it was not found. If there is no error,
// the incremented value is returned.
func (c *cache) IncrementFloat64(key string, n float64) (float64, error) {
	c.mu.Lock()
	item, found := c.items[key]

	if !found || item.Expired() {
		c.mu.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}

	value, ok := item.Object.(float64)

	if !ok {
		c.mu.Unlock()
		return 0, fmt.Errorf("The value for %s is not an float64", key)
	}

	newValue := value + n
	item.Object = newValue
	c.items[key] = item
	c.mu.Unlock()
	return newValue, nil
}
