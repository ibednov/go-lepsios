# ws

In-process WebSocket hub + Redis Pub/Sub fan-out for multi-instance APIs.

- `Hub` / `Conn` — per-user connections, per-conn room subscriptions, write pump + backpressure
- `Bus` — `PUBLISH` / `PSUBSCRIBE` with configurable channel prefix
- `Envelope` — JSON `{type, ts, payload}` helpers

Product-specific auth, ACL and event schemas live in the app (e.g. wishimi-back handler).
