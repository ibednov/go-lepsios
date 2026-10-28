# Shared recovery errors

This package does not define HTTP response codes. Consumers map the typed errors to their existing public envelope and document each public error in their own domain catalog.

| Error | Source | Meaning | Consumer handling |
| --- | --- | --- | --- |
| `ErrInvalidToken` | `Validate`, `Confirm` | Token is empty, unknown, expired, used, or no longer eligible | Use the product's existing invalid-link response; do not retry the same token |
| `ErrDisabled` | `Validate`, `Confirm` | Recovery is disabled by current product policy | Use the product's established unavailable/invalid-link response |
| `ErrRateLimited` | `Validate`, `Confirm` | Client limit rejected a token operation | Return the product's documented limit response and `Retry-After`; request limits are intentionally suppressed as a generic accepted result |
| Other error | Any operation | Policy, storage, message composition, delivery, or bookkeeping failed | Preserve the cause for internal tracing; never return provider bodies, tokens, addresses, or credentials |

When a provider accepted the message but `MarkAccepted` fails, the reserved token stays valid because the recipient may already have the link. When delivery fails, `MarkFailed` must delete/invalidate the token before returning; surface cleanup failure together with the delivery cause for internal diagnosis.
