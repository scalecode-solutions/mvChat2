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
