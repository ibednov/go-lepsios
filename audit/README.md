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