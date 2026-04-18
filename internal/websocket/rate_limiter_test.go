package websocket

import (
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(2, time.Second)
	now := time.Now()

	if !limiter.Allow(now) {
		t.Fatalf("first request should pass")
	}
	if !limiter.Allow(now.Add(10 * time.Millisecond)) {
		t.Fatalf("second request should pass")
	}
	if limiter.Allow(now.Add(20 * time.Millisecond)) {
		t.Fatalf("third request in same window should be rejected")
	}
	if !limiter.Allow(now.Add(time.Second + 1*time.Millisecond)) {
		t.Fatalf("request after window reset should pass")
	}
}
