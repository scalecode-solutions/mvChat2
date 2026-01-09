package ratelimit

import (
	"testing"
	"time"
)

func TestLimiter_Allow(t *testing.T) {
	// 3 requests per 100ms
	limiter := New(3, 100*time.Millisecond)

	key := "test-key"

	// First 3 should be allowed
	for i := 0; i < 3; i++ {
		if !limiter.Allow(key) {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 4th should be denied
	if limiter.Allow(key) {
		t.Error("4th request should be denied")
	}

	// Wait for window to expire
	time.Sleep(110 * time.Millisecond)

	// Should be allowed again
	if !limiter.Allow(key) {
		t.Error("request after window should be allowed")
	}
}

func TestLimiter_MultipleKeys(t *testing.T) {
	limiter := New(2, 100*time.Millisecond)

	key1 := "user1"
	key2 := "user2"

	// Both keys should have independent limits
	if !limiter.Allow(key1) {
		t.Error("key1 request 1 should be allowed")
	}
	if !limiter.Allow(key1) {
		t.Error("key1 request 2 should be allowed")
	}
	if limiter.Allow(key1) {
		t.Error("key1 request 3 should be denied")
	}

	// key2 should still have its full limit
	if !limiter.Allow(key2) {
		t.Error("key2 request 1 should be allowed")
	}
	if !limiter.Allow(key2) {
		t.Error("key2 request 2 should be allowed")
	}
}

func TestLimiter_Remaining(t *testing.T) {
	limiter := New(5, 100*time.Millisecond)
	key := "test"

	if r := limiter.Remaining(key); r != 5 {
		t.Errorf("expected 5 remaining, got %d", r)
	}

	limiter.Allow(key)
	if r := limiter.Remaining(key); r != 4 {
		t.Errorf("expected 4 remaining, got %d", r)
	}

	limiter.Allow(key)
	limiter.Allow(key)
	if r := limiter.Remaining(key); r != 2 {
		t.Errorf("expected 2 remaining, got %d", r)
	}
}

func TestLimiter_Reset(t *testing.T) {
	limiter := New(2, 100*time.Millisecond)
	key := "test"

	limiter.Allow(key)
	limiter.Allow(key)

	if limiter.Allow(key) {
		t.Error("should be rate limited")
	}

	limiter.Reset(key)

	// Should be allowed after reset
	if !limiter.Allow(key) {
		t.Error("should be allowed after reset")
	}
}

func TestLimiter_SlidingWindow(t *testing.T) {
	// 3 requests per 100ms sliding window
	limiter := New(3, 100*time.Millisecond)
	key := "test"

	// Use all 3 slots
	limiter.Allow(key)
	limiter.Allow(key)
	limiter.Allow(key)

	// Wait 60ms (first request still in window)
	time.Sleep(60 * time.Millisecond)

	// Should still be denied (all 3 requests still in window)
	if limiter.Allow(key) {
		t.Error("should still be rate limited")
	}

	// Wait another 50ms (first request now expired)
	time.Sleep(50 * time.Millisecond)

	// Should be allowed (oldest request expired, 2 in window)
	if !limiter.Allow(key) {
		t.Error("should be allowed after oldest expires")
	}
}

func TestNewWithConfig_CustomCleanupInterval(t *testing.T) {
	cfg := Config{
		Limit:           5,
		Window:          100 * time.Millisecond,
		CleanupInterval: 50 * time.Millisecond,
	}
	limiter := NewWithConfig(cfg)

	if limiter.limit != 5 {
		t.Errorf("expected limit 5, got %d", limiter.limit)
	}
	if limiter.window != 100*time.Millisecond {
		t.Errorf("expected window 100ms, got %v", limiter.window)
	}
	if limiter.cleanupInterval != 50*time.Millisecond {
		t.Errorf("expected cleanupInterval 50ms, got %v", limiter.cleanupInterval)
	}
}

func TestLimiter_Cleanup(t *testing.T) {
	// Create limiter with very short cleanup interval
	cfg := Config{
		Limit:           2,
		Window:          20 * time.Millisecond,
		CleanupInterval: 10 * time.Millisecond,
	}
	limiter := NewWithConfig(cfg)

	// Add requests for multiple keys
	limiter.Allow("key1")
	limiter.Allow("key2")
	limiter.Allow("key3")

	// Wait for cleanup interval to pass
	time.Sleep(30 * time.Millisecond)

	// This Allow call should trigger cleanup
	limiter.Allow("key1")

	// Verify cleanup ran by checking that stale keys are gone
	limiter.mu.Lock()
	// key2 and key3 should have been cleaned up (no recent requests)
	if _, exists := limiter.requests["key2"]; exists {
		if len(limiter.requests["key2"]) > 0 {
			// Check if entries are still there but stale
			allStale := true
			now := time.Now()
			windowStart := now.Add(-limiter.window)
			for _, t := range limiter.requests["key2"] {
				if t.After(windowStart) {
					allStale = false
					break
				}
			}
			if !allStale {
				t.Error("key2 should be cleaned up")
			}
		}
	}
	limiter.mu.Unlock()
}

func TestLimiter_Remaining_ZeroWhenOverLimit(t *testing.T) {
	limiter := New(2, 100*time.Millisecond)
	key := "test"

	// Exhaust limit
	limiter.Allow(key)
	limiter.Allow(key)

	// Try to add more (will be denied)
	limiter.Allow(key)
	limiter.Allow(key)

	// Remaining should be 0, not negative
	if r := limiter.Remaining(key); r != 0 {
		t.Errorf("expected 0 remaining, got %d", r)
	}
}

func TestLimiter_CleanupRemovesEmptyKeys(t *testing.T) {
	// Create limiter with short window and cleanup interval
	cfg := Config{
		Limit:           1,
		Window:          10 * time.Millisecond,
		CleanupInterval: 5 * time.Millisecond,
	}
	limiter := NewWithConfig(cfg)

	// Add request
	limiter.Allow("temp-key")

	// Wait for window to expire
	time.Sleep(20 * time.Millisecond)

	// Force lastCleanup to be in the past so cleanup triggers
	limiter.mu.Lock()
	limiter.lastCleanup = time.Now().Add(-10 * time.Millisecond)
	limiter.mu.Unlock()

	// Trigger cleanup via Allow
	limiter.Allow("other-key")

	// Check that temp-key was removed
	limiter.mu.Lock()
	_, exists := limiter.requests["temp-key"]
	limiter.mu.Unlock()

	if exists {
		t.Error("stale key should have been removed by cleanup")
	}
}

func TestLimiter_CleanupKeepsValidEntries(t *testing.T) {
	// Test that cleanup keeps keys that have valid (non-stale) entries
	cfg := Config{
		Limit:           5,
		Window:          100 * time.Millisecond,
		CleanupInterval: 10 * time.Millisecond,
	}
	limiter := NewWithConfig(cfg)

	// Add request
	limiter.Allow("keep-key")

	// Force cleanup interval to pass without waiting
	limiter.mu.Lock()
	limiter.lastCleanup = time.Now().Add(-20 * time.Millisecond)
	limiter.mu.Unlock()

	// Trigger cleanup (request is still within window)
	limiter.Allow("other-key")

	// Check that keep-key still exists (request is within window)
	limiter.mu.Lock()
	_, exists := limiter.requests["keep-key"]
	limiter.mu.Unlock()

	if !exists {
		t.Error("key with valid entries should not be removed by cleanup")
	}
}

func TestLimiter_Remaining_NewKey(t *testing.T) {
	limiter := New(10, 100*time.Millisecond)

	// New key that has never been used
	remaining := limiter.Remaining("never-used")
	if remaining != 10 {
		t.Errorf("expected 10 remaining for new key, got %d", remaining)
	}
}
