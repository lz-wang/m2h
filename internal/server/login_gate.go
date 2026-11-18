package server

import (
	"sync"
	"time"
)

// loginGate bounds both concurrent authentication work and aggregate attempts.
type loginGate struct {
	slots    chan struct{}
	mu       sync.Mutex
	attempts []time.Time
}

func newLoginGate() *loginGate { return &loginGate{slots: make(chan struct{}, 4)} }

func (gate *loginGate) acquire() bool {
	select {
	case gate.slots <- struct{}{}:
	default:
		return false
	}
	gate.mu.Lock()
	defer gate.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-time.Minute)
	live := gate.attempts[:0]
	for _, attempt := range gate.attempts {
		if attempt.After(cutoff) {
			live = append(live, attempt)
		}
	}
	gate.attempts = live
	if len(live) >= 60 {
		<-gate.slots
		return false
	}
	gate.attempts = append(gate.attempts, now)
	return true
}

func (gate *loginGate) release() { <-gate.slots }
