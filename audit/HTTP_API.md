# Audit HTTP contract

Use this contract for a product's authenticated CRM audit endpoint. The audit
package owns the shared domain record and these wire DTOs; each product keeps
its authentication, authorization, route registration, and repository.

## List events

`GET /api/v1/admin/audit-events`

Query parameters:

| Name | Meaning |
| --- | --- |
| `limit` | Page size, default 50; products may enforce a lower maximum. |
| `offset` | Zero-based offset, default 0. |
| `type` | Exact event type, or a prefix ending in `*`. |
| `actor_id` | Filter by actor ID. |
| `actor_kind` | Filter by `admin`, `user`, `moderator`, or `system`. |
| `target_type` | Filter by target type. |
| `target_id` | Filter by target ID. |
| `from`, `to` | Inclusive RFC3339 time bounds over `occurred_at`. |

Success uses the standard `httpx/response` envelope. Its `data` value has this
shape:

```json
{
  "items": [
    {
      "id": "125",
      "type": "admin.user.updated",
      "actor_id": "staff-42",
      "actor_kind": "admin",
      "target_type": "user",
      "target_id": "user-17",
      "payload": {"plan": "pro"},
      "occurred_at": "2026-10-23T00:00:00Z"
    }
  ],
  "total": 120,
  "limit": 50,
  "offset": 0
}
```

`id` is a string on the wire, including when the database uses an integer. This
supports UUID-backed products and avoids integer precision loss in JavaScript.
`actor_email` is an optional display field; it does not change actor identity.
`payload` is always a JSON object (use `{}` when there are no details).

Event names, target names, database schema, authorization policy, and
retention remain product-owned. Do not use this audit contract for a domain
timeline such as expenses/income or a commercial proposal's status history;
those records have their own business meaning and lifecycle.
