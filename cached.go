package sync2

import (
	"sync"
	"time"
)

const (
	defaultInterval = 5 * time.Second
)

var timeNow = time.Now // local copy to override in tests

// Cached provides a concurrency-safe way to lazily-load and then cache a
// long-lived value. If an error is returned, or the cache is manually reset,
// the value will be retrieved, but only after 5 seconds have elapsed since the
// last fetch.
type Cached struct {
	fetched   bool
	lastFetch time.Time
	val       interface{}
	err       error

	mux sync.RWMutex
}

func (c *Cached) needsFetch() bool {
	return !c.fetched && c.lastFetch.Add(defaultInterval).Before(timeNow())
}

// Get retrieves the cached object, lazily fetching it using fn if needed.
func (c *Cached) Get(fn func() (interface{}, error)) (interface{}, error) {
	c.mux.RLock()

	if c.needsFetch() {
		// Upgrade to a write lock to ensure only one goroutine enters the
		// critical zone at a time.
		c.mux.RUnlock()
		c.mux.Lock()
		defer c.mux.Unlock()

		// Check again to see if another goroutine has already fetched the value
		// after acquiring the write lock.
		if !c.needsFetch() {
			return c.val, c.err
		}

		// Call fn and return the error, if any.
		c.val, c.err = fn()
		c.fetched = c.err == nil
		c.lastFetch = timeNow()
		return c.val, c.err
	}

	// If a fetch wasn't needed, release the lock and return the cached value.
	defer c.mux.RUnlock()
	return c.val, c.err
}

// Reset forces a fetch on the next call to Get, so long as 5 seconds have
// elapsed since the last fetch.
func (c *Cached) Reset() {
	c.mux.Lock()
	c.fetched = false
	c.mux.Unlock()
}
