package auth

import (
	"sync"
	"time"
)

// LoginLimiter is a fixed-window failure limiter for the login endpoint:
// after maxFailures failed attempts inside window, the key stays locked
// until the window passes. Keys are usernames alone: behind a reverse
// proxy every client shares the proxy address, so an address-keyed limiter
// would let one attacker lock out any username at will. A username-only
// lock bounds the damage to the one name under attack — and that lock is
// exactly the brute-force protection the username's owner wants. The
// limiter lives in memory — restarting m2h clears it, which is acceptable
// for a single-binary deployment.
type LoginLimiter struct {
	mu       sync.Mutex
	failures map[string][]time.Time
	max      int
	window   time.Duration
	now      func() time.Time
}

// maxTrackedFailures bounds the failure map: a spray of distinct usernames
// can grow it, so once past the bound the next write sweeps expired keys
// (and, if that is not enough, drops the oldest half).
const maxTrackedFailures = 1024

// NewLoginLimiter builds a limiter allowing max failures per key inside
// window.
func NewLoginLimiter(max int, window time.Duration) *LoginLimiter {
	return &LoginLimiter{
		failures: make(map[string][]time.Time),
		max:      max,
		window:   window,
		now:      time.Now,
	}
}

// Allowed reports whether the key may attempt a login right now.
func (limiter *LoginLimiter) Allowed(key string) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	return len(limiter.recent(key)) < limiter.max
}

// RecordFailure counts one failed attempt for the key.
func (limiter *LoginLimiter) RecordFailure(key string) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	failures := limiter.recent(key)
	failures = append(failures, limiter.now())
	limiter.failures[key] = failures
	if len(limiter.failures) > maxTrackedFailures {
		limiter.sweep()
	}
}

// sweep drops every key whose failures have all expired; if the map is
// still oversized afterwards (a burst inside one window), the smallest
// entries are dropped — precision bends before memory does.
func (limiter *LoginLimiter) sweep() {
	cutoff := limiter.now().Add(-limiter.window)
	for key, failures := range limiter.failures {
		if len(failures) == 0 || !failures[len(failures)-1].After(cutoff) {
			delete(limiter.failures, key)
		}
	}
	if len(limiter.failures) <= maxTrackedFailures {
		return
	}
	smallest := make([]struct {
		key  string
		size int
	}, 0, len(limiter.failures))
	for key, failures := range limiter.failures {
		smallest = append(smallest, struct {
			key  string
			size int
		}{key, len(failures)})
	}
	// Partial insertion sort: drop the smallest halves until back under
	// the bound.
	for index := 1; index < len(smallest); index++ {
		for offset := index; offset > 0 && smallest[offset].size < smallest[offset-1].size; offset-- {
			smallest[offset], smallest[offset-1] = smallest[offset-1], smallest[offset]
		}
	}
	drop := len(limiter.failures) - maxTrackedFailures
	for _, entry := range smallest {
		if drop <= 0 {
			break
		}
		delete(limiter.failures, entry.key)
		drop--
	}
}

// Reset forgets the key's failures after a successful login.
func (limiter *LoginLimiter) Reset(key string) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	delete(limiter.failures, key)
}

// recent prunes expired failures and returns the live ones. Callers hold
// the mutex.
func (limiter *LoginLimiter) recent(key string) []time.Time {
	cutoff := limiter.now().Add(-limiter.window)
	failures := limiter.failures[key]
	live := failures[:0]
	for _, failure := range failures {
		if failure.After(cutoff) {
			live = append(live, failure)
		}
	}
	if len(live) == 0 {
		delete(limiter.failures, key)
		return nil
	}
	limiter.failures[key] = live
	return live
}
