// Package ratelimit provides a simple sliding window rate limiter.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter implements a sliding window rate limiter.
// It tracks requests per key and rejects requests that exceed the limit.
type Limiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration

	// Cleanup configuration
	cleanupInterval time.Duration
	lastCleanup     time.Time
}

// Config holds limiter configuration.
type Config struct {
	// Limit is the maximum number of requests allowed per window.
	Limit int

	// Window is the time window for rate limiting.
	Window time.Duration

	// CleanupInterval controls how often stale entries are cleaned up.
	// If 0, defaults to 10 * Window.
	CleanupInterval time.Duration
}

// New creates a new rate limiter.
func New(limit int, window time.Duration) *Limiter {
	return NewWithConfig(Config{
		Limit:  limit,
		Window: window,
	})
}

// NewWithConfig creates a new rate limiter with custom configuration.
func NewWithConfig(cfg Config) *Limiter {
	cleanupInterval := cfg.CleanupInterval
	if cleanupInterval == 0 {
		cleanupInterval = cfg.Window * 10
	}

	return &Limiter{
		requests:        make(map[string][]time.Time),
		limit:           cfg.Limit,
		window:          cfg.Window,
		cleanupInterval: cleanupInterval,
		lastCleanup:     time.Now(),
	}
}

// Allow checks if a request should be allowed for the given key.
// Returns true if the request is allowed, false if rate limited.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-l.window)

	// Periodic cleanup of stale keys
	if now.Sub(l.lastCleanup) > l.cleanupInterval {
		l.cleanup(windowStart)
		l.lastCleanup = now
	}

	// Filter requests within the window
	times := l.requests[key]
	valid := times[:0]
	for _, t := range times {
		if t.After(windowStart) {
			valid = append(valid, t)
		}
	}
	l.requests[key] = valid

	// Check if over limit
	if len(valid) >= l.limit {
		return false
	}

	// Add this request
	l.requests[key] = append(l.requests[key], now)
	return true
}

// cleanup removes stale entries from the map.
// Must be called with mu held.
func (l *Limiter) cleanup(windowStart time.Time) {
	for key, times := range l.requests {
		valid := times[:0]
		for _, t := range times {
			if t.After(windowStart) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(l.requests, key)
		} else {
			l.requests[key] = valid
		}
	}
}

// Remaining returns the number of requests remaining for the given key.
func (l *Limiter) Remaining(key string) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-l.window)

	times := l.requests[key]
	count := 0
	for _, t := range times {
		if t.After(windowStart) {
			count++
		}
	}

	remaining := l.limit - count
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Reset clears the rate limit for a given key.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.requests, key)
}
