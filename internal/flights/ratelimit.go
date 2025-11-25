package flights

import (
	"sync"
	"time"
)

// RateLimiter implements a simple sliding window rate limiter.
type RateLimiter struct {
	maxRequests int
	window      time.Duration
	requests    int
	windowStart time.Time
	mu          sync.Mutex
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(maxRequests int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		maxRequests: maxRequests,
		window:      window,
		requests:    0,
		windowStart: time.Now(),
	}
}

// Allow checks if a request is allowed under the rate limit.
// Returns true if the request is allowed, false otherwise.
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	// Reset window if expired - using total_seconds correctly!
	if now.Sub(r.windowStart) > r.window {
		r.requests = 0
		r.windowStart = now
	}

	// Check limit
	if r.requests >= r.maxRequests {
		return false
	}

	r.requests++
	return true
}

// Reset resets the rate limiter.
func (r *RateLimiter) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.requests = 0
	r.windowStart = time.Now()
}

// Remaining returns the number of remaining requests in the current window.
func (r *RateLimiter) Remaining() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	// Check if window expired
	if now.Sub(r.windowStart) > r.window {
		return r.maxRequests
	}

	remaining := r.maxRequests - r.requests
	if remaining < 0 {
		return 0
	}
	return remaining
}

// WindowResetTime returns the time when the current window resets.
func (r *RateLimiter) WindowResetTime() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.windowStart.Add(r.window)
}
