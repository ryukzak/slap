package handlers

import (
	"sync"
	"time"
)

const (
	rlMaxAttempts = 5
	rlWindow      = 10 * time.Minute
	rlLockout     = 10 * time.Minute
)

type rateLimiter struct {
	mu       sync.Mutex
	failures map[string][]time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{failures: make(map[string][]time.Time)}
}

// isLocked returns true if the key is currently rate-limited.
func (rl *rateLimiter) isLocked(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.countRecent(key) >= rlMaxAttempts
}

// recordFailure adds a failed attempt timestamp for the key.
func (rl *rateLimiter) recordFailure(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.failures[key] = append(rl.prune(key), time.Now())
}

// reset clears all failure records for the key (called on successful login).
func (rl *rateLimiter) reset(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.failures, key)
}

// countRecent returns the number of failures within the lockout window.
// Must be called with mu held.
func (rl *rateLimiter) countRecent(key string) int {
	return len(rl.prune(key))
}

// prune removes stale entries and returns the remaining ones.
// Must be called with mu held.
func (rl *rateLimiter) prune(key string) []time.Time {
	cutoff := time.Now().Add(-rlLockout)
	var recent []time.Time
	for _, t := range rl.failures[key] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	rl.failures[key] = recent
	return recent
}

var signinLimiter = newRateLimiter()
