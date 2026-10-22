# audit

Reusable actor-action audit log ("who did what").

| Import path | Requires |
|-------------|----------|
| `github.com/ibednov/go-lepsios/audit` | `log` |

## What's shared

- `ActorKind` — `admin | user | moderator | system`.
- `Event`, `AppendInput`, `ListFilter` — generic record shape.
- `Repository` interface — consumer owns the storage (e.g. Postgres).
- `Service` — `Append`, `AppendBestEffort`, `List` with the shared rules:
  - incomplete inputs are skipped; empty payload becomes `{}`;
  - `ActorKind` defaults to `admin`;
  - best-effort writes only log on failure.

Product event types (e.g. `admin.user.deleted`), target enums and the SQL table
(`action_events`) stay in the consumer.

For CRM HTTP APIs, use the shared JSON DTOs in `http_contract.go` and follow
[`HTTP_API.md`](./HTTP_API.md). Authentication, authorization and repository
implementations remain in the product. Domain timelines such as expense
history or proposal status changes are not audit logs.
