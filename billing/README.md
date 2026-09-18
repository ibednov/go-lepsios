# billing

Reusable SaaS billing primitives (not commerce/marketplace catalog).

## Packages

| Import | Role |
|--------|------|
| `github.com/ibednov/go-lepsios/billing` | `PaymentIntent`, `Strategy`, `AdminConfirm`, `AssignSubscription` |
| `.../billing/subscription` | Plan visibility, plan DTO, subscription lifecycle (`IsActive`) |
| `.../billing/purchase` | One-time product / purchase types |
| `.../billing/entitlements` | `Limit`, `GetLimit`, feature merge helpers |

Payment gateway is pluggable via `Strategy`. Baseline: `AdminConfirm`.

Future commerce goods → separate `commerce/*` modules, never a bare `catalog`.

## Consumer notes

- No ORM tags — products map to their own storage.
- Product feature keys (e.g. Wishimi `price_monitor_gap_hours`) stay in the product.
