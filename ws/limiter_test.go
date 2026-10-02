package ws

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWindowLimiterRejectsOverMax(t *testing.T) {
	t.Parallel()
	l := newWindowLimiter(2, time.Second)
	now := time.Unix(1_700_000_000, 0)
	require.True(t, l.allow(now))
	require.True(t, l.allow(now))
	require.False(t, l.allow(now), "third frame in window must be rejected")
}

func TestWindowLimiterResetsAfterWindow(t *testing.T) {
	t.Parallel()
	l := newWindowLimiter(1, time.Second)
	now := time.Unix(1_700_000_000, 0)
	require.True(t, l.allow(now))
	require.False(t, l.allow(now))
	require.True(t, l.allow(now.Add(time.Second)), "new window must refill")
}

func TestConnAllowSubscribeIndependentOfFrames(t *testing.T) {
	t.Parallel()
	c := &Conn{
		frameLim: newWindowLimiter(10, time.Second),
		subLim:   newWindowLimiter(1, time.Second),
	}
	require.True(t, c.AllowFrame())
	require.True(t, c.AllowSubscribe())
	require.True(t, c.AllowFrame())
	require.False(t, c.AllowSubscribe(), "subscribe budget exhausted while frames remain")
}

func TestNilLimiterAllows(t *testing.T) {
	t.Parallel()
	var l *windowLimiter
	require.True(t, l.allow(time.Now()))
	c := &Conn{}
	require.True(t, c.AllowFrame())
	require.True(t, c.AllowSubscribe())
}
