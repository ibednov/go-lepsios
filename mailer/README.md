# mailer

Provider strategies for sending transactional email. The module has no product limits, templates, or audit storage.

`Router` selects a registered `Strategy` by name for each message. `Receipt` contains the provider's accepted message ID where available; acceptance does not guarantee delivery. Existing `SMTP.Send` and `Sender` remain available. SMTP needs a plain text body and does not return a provider ID.

```go
resend, err := mailer.NewResend(mailer.ResendConfig{
    APIKey: apiKey,
    From:   "Example <noreply@example.com>",
})
if err != nil { return err }
router, err := mailer.NewRouter(map[string]mailer.Strategy{"resend": resend})
if err != nil { return err }
receipt, err := router.Send(ctx, "resend", mailer.Message{
    To: recipient, Subject: subject, Body: body,
    IdempotencyKey: deliveryID,
})
```

Resend uses `POST https://api.resend.com/emails` and returns its email ID. Set a stable idempotency key for each logical message before retrying; Resend retains keys for 24 hours. The adapter makes one request and returns a typed `ProviderError` with HTTP status and `Retry-After`. The caller owns retry policy, quotas, delivery logging, and persistence. Keep API keys in secret configuration.

API reference: https://resend.com/docs/api-reference/emails/send-email
