# `auth/recovery`

Product-neutral orchestration for one-time recovery links. The package creates raw URL-safe tokens from 32 cryptographic random bytes, stores only `SHA-256(rawToken)`, coordinates client limits and delivery, validates active tokens, and delegates atomic confirmation to the product store.

The store owns policy and persistence details. `Reserve` must resolve the product identifier, enforce account/IP/daily/monthly quotas atomically, write reservation/event rows, and return delivery data only for an eligible account. `Confirm` must lock and validate the token, run the supplied typed transaction callback, apply the product's credential-generation/token/event updates in the same transaction, and return the subject only after commit. Do not put a transaction handle in `context.Context`.

`Client.Fingerprint` is already a keyed pseudonym prepared by the product. Never pass a raw IP, account email, token, password, or TOTP value as a fingerprint. `RateLimitFingerprint` can preserve an existing limiter key that differs from the audit fingerprint; it must also be a one-way pseudonym. `Composer` owns product-specific copy and URL construction. The `Sender` contract matches `mailer.Router`; provider acceptance is not mailbox delivery. Failed sends invoke `MarkFailed`, which must invalidate the reserved token. Failure to record provider acceptance does not invalidate the already-sent link.

The generic package does not know account tables, account identifiers beyond opaque subject IDs, credential types, HTTP routes, CRM, or email-binding workflows. Wishimi email binding remains a separate product flow.

`service_test.go` contains a small independent in-memory consumer: it uses an opaque account ID, a typed transaction handle, and fake delivery ports without importing Wishimi or its persistence package.

Use a local workspace for development. The auth module declares a semver dependency on `go-lepsios/mailer`; publish the new auth tag only after all consumers can resolve it. Public HTTP error codes remain owned by each consuming product's transport.
