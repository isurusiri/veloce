package globals

import "time"

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
