package ratelimit

import (
	"testing"
	"time"
)

func TestTokenBucketLimiter_Allow(t *testing.T) {
	// 5 tokens/sec, burst 3
	limiter := NewTokenBucketLimiter(5.0, 3, 1*time.Minute)
	defer limiter.Close()

	key := "127.0.0.1"

	// Initial burst: 3 allowed
	if !limiter.Allow(key) {
		t.Fatalf("first request should be allowed")
	}
	if !limiter.Allow(key) {
		t.Fatalf("second request should be allowed")
	}
	if !limiter.Allow(key) {
		t.Fatalf("third request should be allowed")
	}

	// 4th request exceeds burst capacity immediately
	if limiter.Allow(key) {
		t.Fatalf("fourth immediate request should be rejected")
	}

	// Wait for token refill (250ms -> +1.25 tokens)
	time.Sleep(250 * time.Millisecond)
	if !limiter.Allow(key) {
		t.Fatalf("request after refill should be allowed")
	}
}
