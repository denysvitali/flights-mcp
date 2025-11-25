package flights

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiter_Allow(t *testing.T) {
	// Create a rate limiter with 3 requests per 100ms
	rl := NewRateLimiter(3, 100*time.Millisecond)

	// First 3 requests should be allowed
	assert.True(t, rl.Allow())
	assert.True(t, rl.Allow())
	assert.True(t, rl.Allow())

	// 4th request should be denied
	assert.False(t, rl.Allow())
	assert.False(t, rl.Allow())

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Should be allowed again
	assert.True(t, rl.Allow())
}

func TestRateLimiter_Remaining(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)

	assert.Equal(t, 5, rl.Remaining())

	rl.Allow()
	assert.Equal(t, 4, rl.Remaining())

	rl.Allow()
	rl.Allow()
	assert.Equal(t, 2, rl.Remaining())
}

func TestRateLimiter_Reset(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)

	rl.Allow()
	rl.Allow()
	rl.Allow()
	assert.False(t, rl.Allow())

	rl.Reset()
	assert.True(t, rl.Allow())
	assert.Equal(t, 2, rl.Remaining())
}

func TestRateLimiter_WindowReset(t *testing.T) {
	// This test verifies that the rate limiter correctly uses total seconds
	// instead of just the seconds component (which was a bug in the Python version)
	rl := NewRateLimiter(2, 100*time.Millisecond)

	rl.Allow()
	rl.Allow()
	assert.False(t, rl.Allow())

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Window should have reset
	assert.True(t, rl.Allow())
	assert.True(t, rl.Allow())
	assert.False(t, rl.Allow())
}
