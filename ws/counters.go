package ws

import "sync/atomic"

// Counters are process-local gauges/counters (no metrics sink wired yet).
type Counters struct {
	Connections     atomic.Int64
	EventsPublished atomic.Int64
	PublishErrors   atomic.Int64
}

// Snapshot returns ws_connections / ws_events_published / ws_publish_errors.
func (c *Counters) Snapshot() (connections, published, publishErrors int64) {
	if c == nil {
		return 0, 0, 0
	}
	return c.Connections.Load(), c.EventsPublished.Load(), c.PublishErrors.Load()
}
