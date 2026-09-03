# telegram

Reusable Telegram bot infrastructure: Bot API client, Redis chat sessions, notify bridge.

```go
import (
    tgbot "github.com/ibednov/go-lepsios/telegram/bot"
    tgsession "github.com/ibednov/go-lepsios/telegram/session"
    tgredis "github.com/ibednov/go-lepsios/telegram/session/redis"
    "github.com/ibednov/go-lepsios/telegram/notify"
)
```

| Package | Role |
|---------|------|
| `bot/` | Long-poll Bot + send-only Sender (`go-telegram/bot`) |
| `session/` | Chat session model, store interface, `WithAPI` silent reauth helper |
| `session/redis/` | Redis `SessionStore` (`telegram:session:{chatID}`) |
| `notify/` | Generic push bridge (Directory + KeyboardFunc) |
| `schema/` | Reference SQL for `users_telegrams` / challenges |

Auth mechanism (routes, HMAC assertion, challenges) stays in
`github.com/ibednov/go-lepsios/auth/mechanism/local/telegram` — this module does not register HTTP routes.
