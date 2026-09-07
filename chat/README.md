# chat

Reusable chat rooms, members and soft-deletable messages.

| Import path | Requires |
|-------------|----------|
| `github.com/ibednov/go-lepsios/chat` | (stdlib + testify for tests) |

## What's shared

- Entities: `Chat`, `Member`, `Message`
- `Repository` — consumer owns Postgres (see `schema/users_chats.sql`)
- `Service`:
  - `EnsureChat` — find-or-create by `(kind, external_id)`
  - `AddMember` / `ListMembers` / `IsMember`
  - `SendMessage` — member-only; empty body requires attachment
  - `ListMessages` — member-only; soft-deleted hidden by default
  - `SoftDeleteMessage` — sender-only; sets `deleted_at` / `deleted_by`

## Tables (reference)

- `users_chats`
- `users_chats_members`
- `users_chats_messages`

Product rules (reservation ACL, `hide_givers`, rate limit, WS) stay in the consumer.
