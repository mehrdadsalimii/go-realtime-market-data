package websocket

import "time"

type RateLimiter struct {
	limit       int
	window      time.Duration
	windowStart time.Time
	count       int
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{limit: limit, window: window}
}

func (r *RateLimiter) Allow(now time.Time) bool {
	if r.limit <= 0 {
		return true
	}
	if r.windowStart.IsZero() || now.Sub(r.windowStart) >= r.window {
		r.windowStart = now
		r.count = 0
	}
	if r.count >= r.limit {
		return false
	}
	r.count++
	return true
}
