package ws

import (
	"sync"
	"time"
)

const (
	defaultMaxFramesPerWindow = 30
	defaultMaxSubsPerWindow   = 10
	defaultLimitWindow        = time.Second
)

// windowLimiter is a fixed-window counter (per-conn, in-memory).
type windowLimiter struct {
	mu      sync.Mutex
	max     int
	window  time.Duration
	count   int
	resetAt time.Time
}

func newWindowLimiter(max int, window time.Duration) *windowLimiter {
	if max <= 0 {
		max = 1
	}
	if window <= 0 {
		window = time.Second
	}
	return &windowLimiter{max: max, window: window}
}

func (l *windowLimiter) allow(now time.Time) bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.IsZero() {
		now = time.Now()
	}
	if l.resetAt.IsZero() || !now.Before(l.resetAt) {
		l.count = 0
		l.resetAt = now.Add(l.window)
	}
	if l.count >= l.max {
		return false
	}
	l.count++
	return true
}

// AllowFrame rate-limits inbound client frames on this connection.
func (c *Conn) AllowFrame() bool {
	if c == nil {
		return false
	}
	return c.frameLim.allow(time.Now())
}

// AllowSubscribe rate-limits subscribe frames (in addition to AllowFrame).
func (c *Conn) AllowSubscribe() bool {
	if c == nil {
		return false
	}
	return c.subLim.allow(time.Now())
}
