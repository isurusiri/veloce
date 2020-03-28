package models

import "time"

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
