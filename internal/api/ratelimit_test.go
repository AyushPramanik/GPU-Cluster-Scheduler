package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenBucket_AllowsBurstThenThrottles(t *testing.T) {
	b := newBucket(0, 3) // no refill, capacity 3
	assert.True(t, b.allow())
	assert.True(t, b.allow())
	assert.True(t, b.allow())
	assert.False(t, b.allow(), "fourth request should exceed the burst")
}

func TestRateLimiter_IsolatesKeys(t *testing.T) {
	rl := newRateLimiter(0, 1) // 1 token per key, no refill
	assert.True(t, rl.allow("1.1.1.1"))
	assert.False(t, rl.allow("1.1.1.1"), "second request from same IP is throttled")
	assert.True(t, rl.allow("2.2.2.2"), "a different IP has its own bucket")
}
