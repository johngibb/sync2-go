package sync2

import (
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deliveroo/assert-go"
)

func TestInitOnce(t *testing.T) {
	// Time travel by setting now.
	now := time.Now()
	defer func() { timeNow = time.Now }()
	timeNow = func() time.Time { return now }
	var initCount int
	var cached Cached
	fn := func() (interface{}, error) {
		initCount++
		return "value", nil
	}

	// Get twice, fn should only be called once.
	val, err := cached.Get(fn)
	assert.Must(t, err)
	assert.Equal(t, val, "value")
	assert.Equal(t, initCount, 1)

	val, err = cached.Get(fn)
	assert.Must(t, err)
	assert.Equal(t, val, "value")
	assert.Equal(t, initCount, 1)

	// Reset, fn should still only be called once since not enough time has
	// elapsed.
	cached.Reset()
	val, err = cached.Get(fn)
	assert.Must(t, err)
	assert.Equal(t, val, "value")
	assert.Equal(t, initCount, 1)

	// Time travel ahead 6 seconds, and, fn should now have been called twice.
	now = now.Add(10 * time.Second)
	val, err = cached.Get(fn)
	assert.Must(t, err)
	assert.Equal(t, val, "value")
	assert.Equal(t, initCount, 2)
}

func TestRetry(t *testing.T) {
	// Time travel by setting now.
	now := time.Now()
	defer func() { timeNow = time.Now }()
	timeNow = func() time.Time { return now }

	var initCount int
	var cached Cached
	fn := func() (interface{}, error) {
		initCount++
		return nil, errors.New("init failed")
	}

	// Get twice, fn should only be called once.
	_, err := cached.Get(fn)
	assert.Equal(t, err.Error(), "init failed")
	assert.Equal(t, initCount, 1)

	_, err = cached.Get(fn)
	assert.Equal(t, err.Error(), "init failed")
	assert.Equal(t, initCount, 1)

	// Time travel 10 seconds; fn should be called again.
	now = now.Add(10 * time.Second)
	_, err = cached.Get(fn)
	assert.Equal(t, err.Error(), "init failed")
	assert.Equal(t, initCount, 2)

	// Reset and time travel; fn should be called again.
	cached.Reset()
	now = now.Add(10 * time.Second)
	_, err = cached.Get(fn)
	assert.Equal(t, err.Error(), "init failed")
	assert.Equal(t, initCount, 3)
}

func TestConcurrency(t *testing.T) {
	// Time travel by setting now.
	defer func() { timeNow = time.Now }()
	var (
		now      = time.Now()
		nowDelta int64
	)
	timeNow = func() time.Time {
		delta := atomic.AddInt64(&nowDelta, rand.Int63n(10))
		return now.Add(time.Duration(delta) * time.Second)
	}

	const (
		runs       = 10
		goroutines = 100
	)
	sleepRand := func() {
		time.Sleep(time.Duration(rand.Int63n(50)) * time.Millisecond)
	}
	for i := 0; i < runs; i++ {
		var cached Cached
		var wg sync.WaitGroup
		wg.Add(goroutines)

		for j := 0; j < goroutines; j++ {
			go func() {
				defer wg.Done()
				switch rand.Intn(3) {
				case 0: // Success
					cached.Get(func() (interface{}, error) {
						sleepRand()
						return nil, nil
					})
				case 1: // Error returned
					cached.Get(func() (interface{}, error) {
						sleepRand()
						return nil, errors.New("init failed")
					})
				case 2: // Reset called
					cached.Reset()
				}
			}()
		}
		wg.Wait()
	}
}
