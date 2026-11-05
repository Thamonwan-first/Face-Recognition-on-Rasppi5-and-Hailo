package ratelimit

import (
	"sync"
	"time"
)

type tokenBucket struct {
	tokens     float64
	lastRefill time.Time
}

// TokenBucketLimiter implements a per-key in-memory token bucket rate limiter
type TokenBucketLimiter struct {
	mu         sync.Mutex
	buckets    map[string]*tokenBucket
	rate       float64       // tokens added per second
	capacity   float64       // maximum burst capacity
	ttl        time.Duration // how long to keep inactive keys
	stopCh     chan struct{}
}

// NewTokenBucketLimiter creates a new limiter.
// ratePerSecond: how many tokens are replenished per second
// burstCapacity: maximum tokens in the bucket
func NewTokenBucketLimiter(ratePerSecond float64, burstCapacity int, ttl time.Duration) *TokenBucketLimiter {
	l := &TokenBucketLimiter{
		buckets:    make(map[string]*tokenBucket),
		rate:       ratePerSecond,
		capacity:   float64(burstCapacity),
		ttl:        ttl,
		stopCh:     make(chan struct{}),
	}

	go l.cleanupLoop()
	return l
}

// Allow checks if the given key has sufficient tokens to proceed
func (l *TokenBucketLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, exists := l.buckets[key]
	if !exists {
		l.buckets[key] = &tokenBucket{
			tokens:     l.capacity - 1.0,
			lastRefill: now,
		}
		return true
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens = b.tokens + (elapsed * l.rate)
	if b.tokens > l.capacity {
		b.tokens = l.capacity
	}
	b.lastRefill = now

	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return true
	}

	return false
}

func (l *TokenBucketLimiter) cleanupLoop() {
	ticker := time.NewTicker(l.ttl)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.mu.Lock()
			now := time.Now()
			for k, b := range l.buckets {
				if now.Sub(b.lastRefill) > l.ttl {
					delete(l.buckets, k)
				}
			}
			l.mu.Unlock()
		case <-l.stopCh:
			return
		}
	}
}

// Close stops the cleanup goroutine
func (l *TokenBucketLimiter) Close() {
	close(l.stopCh)
}
