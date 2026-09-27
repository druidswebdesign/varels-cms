package auth

import (
	"sync"
	"time"
)

// LoginLimiter is a tiny in-memory rate limiter for the break-glass login. It
// allows max attempts per window per key (client IP). Single-instance app, so
// in-memory state is sufficient (ADR-0015).
type LoginLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	max      int
	window   time.Duration
	now      func() time.Time
}

// NewLoginLimiter returns a limiter allowing max failures per window.
func NewLoginLimiter(max int, window time.Duration) *LoginLimiter {
	return &LoginLimiter{
		attempts: make(map[string][]time.Time),
		max:      max,
		window:   window,
		now:      time.Now,
	}
}

// Allow reports whether key may attempt a login.
func (l *LoginLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recent(key)) < l.max
}

// Fail records a failed attempt.
func (l *LoginLimiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.attempts[key] = append(l.recent(key), l.now())
}

// Reset clears the attempts for key (call on success).
func (l *LoginLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

// recent returns the attempts for key inside the window, pruning older ones.
func (l *LoginLimiter) recent(key string) []time.Time {
	cutoff := l.now().Add(-l.window)
	kept := l.attempts[key][:0]
	for _, t := range l.attempts[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	l.attempts[key] = kept
	return kept
}
