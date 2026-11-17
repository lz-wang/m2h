package auth

import (
	"testing"
	"time"
)

func TestLoginLimiterPrunesStaleKeys(t *testing.T) {
	now := time.Now()
	limiter := NewLoginLimiter(3, time.Minute)
	limiter.now = func() time.Time { return now }

	// Many distinct keys fail once and are never seen again — the map
	// must not keep them forever.
	for index := range 5000 {
		limiter.RecordFailure("attacker-" + time.Duration(index).String())
	}
	if len(limiter.failures) > 1024 {
		t.Fatalf("failure map grew to %d entries without pruning", len(limiter.failures))
	}

	// After the window slides past, everything is gone.
	now = now.Add(2 * time.Minute)
	limiter.RecordFailure("late-key")
	if len(limiter.failures) > 64 {
		t.Fatalf("stale entries survived the window: %d", len(limiter.failures))
	}
}
