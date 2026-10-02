package ws

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCountersSnapshot(t *testing.T) {
	t.Parallel()
	var c Counters
	c.Connections.Store(2)
	c.EventsPublished.Store(3)
	c.PublishErrors.Store(1)
	conn, pub, errn := c.Snapshot()
	require.Equal(t, int64(2), conn)
	require.Equal(t, int64(3), pub)
	require.Equal(t, int64(1), errn)
}

func TestNilCountersSnapshot(t *testing.T) {
	t.Parallel()
	var c *Counters
	conn, pub, errn := c.Snapshot()
	require.Equal(t, int64(0), conn)
	require.Equal(t, int64(0), pub)
	require.Equal(t, int64(0), errn)
}

func TestBusPublishIncrementsCounters(t *testing.T) {
	t.Parallel()
	hub := NewHub()
	bus := NewBus(nil, hub, "test:user:")
	require.NoError(t, bus.PublishUser(context.Background(), "u1", []byte(`{"type":"x"}`)))
	_, pub, errs := hub.Counters().Snapshot()
	require.Equal(t, int64(1), pub)
	require.Equal(t, int64(0), errs)
}

func TestHubUnregisterDecrementsOnce(t *testing.T) {
	t.Parallel()
	h := NewHub()
	c := &Conn{
		id:     "c1",
		userID: "u1",
		hub:    h,
		send:   make(chan []byte, sendBuffer),
		subs:   make(map[RoomRef]struct{}),
	}
	h.mu.Lock()
	h.users["u1"] = map[*Conn]struct{}{c: {}}
	h.mu.Unlock()
	h.counters.Connections.Store(1)

	h.Unregister(c)
	h.Unregister(c)
	conn, _, _ := h.Counters().Snapshot()
	require.Equal(t, int64(0), conn, "double unregister must not underflow")
}


--- Rules in scope for this file ---
[From: /Users/alepsios/dev/mine/wishimi/AGENTS.md]
## Core Rules
- **Сделал изменение — сразу коммить** в том sibling-репо, куда правил (не копить, не ждать «закоммить»). Пуш — только по явной просьбе.
- **Для всех задач проекта Wishimi работай только через профильных проектных агентов** из `.cursor/agents/` или `.grok/agents/`; не выполняй работу напрямую без агента.
- Файлы, созданные или обновлённые git hooks при коммите, оставляй в этом же коммите. **Не удаляй, не исключай и не выноси их в отдельный коммит** (включая `docs/dependency-graph-*.dot` / `*.svg` в `wishimi-front`); не переписывай коммит и не обходи hook ради их удаления. Если результат неожиданен — сохрани его и сообщи пользователю.
- Тесты не запускай сам; только дай команду пользователю.
- Код всегда должен быть SOLID и без гонок данных.
- Устойчивые решения и замечания пользователя **сразу пиши в этот файл и/или `docs/`** — чаты меняются, «запомнить» нельзя.

## Project Context (Architecture)
В Cloud Environment / локально это **не монорепозиторий**: прод-код в отдельных GitHub-репо (sibling clones). Workspace — мета-репо (docs, agents, `.cursor`).

| GitHub | Локальный alias | Что это |
|--------|-----------------|---------|
| `wishimi/web` | `web` / `wishimi-front` | Nuxt 4 клиент (Pinia, SSR/SPA) |
| `wishimi/api` | `api` / `wishimi-back` | Go API + telegram-bot sidecar |
| `ibednov/go-lepsios` | `go-lepsios` | Shared Go: auth, httpx, db, i18n, log… |
| `ibednov/nuxt-lepsios` | `nuxt-lepsios` | Shared Nuxt UI layer (shadcn-vue, tokens) |
| `wishimi/workspace` | workspace | Мета-доки / agents — **не** прод-код |

**Локальный Cursor-root:** `workspace/` = клон этого репо; в корне папки — symlink’и `.agents` / `.cursor` / `AGENTS.md` / `docs` / `content` / … → `workspace/…`. Канон правок meta — только внутри `workspace/`.

**Правило слоёв:** переиспользуемая логика → `go-lepsios` / `nuxt-lepsios`. В продуктах (`api`, `web`) — тонкие адаптеры / DI / домен.

### Billing / subscriptions (`go-lepsios`)
- Переиспользуемое: `billing` (PaymentIntent, Strategy, `AdminConfirm`, `AssignSubscription`), `billing/subscription`, `billing/purchase`, `billing/entitlements`, `featureflags`.
- Префиксы: SaaS → `billing/*`; будущий магазин товаров → `commerce/*` (не голый `catalog`).
- В Wishimi: GORM-модели/ключи фич/admin stats остаются в `api`; assign плана идёт через `billing.AssignSubscription` + `AdminConfirm` (intent пока не персистится).
- Модули опубликованы тегами `billing/v0.1.0` и `featureflags/v0.1.0`; в `wishimi-back` — обычный `require` (без `replace`).
- **go-lepsios локально = `go.work` (НЕ `replace`).** `wishimi-back/go.work` (в `.gitignore`, трекается только `go.work.example`) уже содержит `use ../go-lepsios/<модуль>` — локальная сборка/тесты берут исходник из соседнего клона автоматически. **НИКОГДА** не добавляй `replace github.com/ibednov/go-lepsios/... => ../go-lepsios/...` в `go.mod` (он коммитится → путь `../go-lepsios` ломает CI/прод). Разработка незатегированного модуля: правь в `go-lepsios`, локально ловится через `go.work`. Релиз: тег `<модуль>/vX.Y.Z` в go-lepsios → bump `require` в `wishimi-back/go.mod`. Аналогично `go-lepsios/go.work` связывает его собственные подмодули.

### Subagents (Cursor + Grok)

Канон агентов — Claude competo (git-ban целиком, HANDOFF, PLAUSIBLE, гейт комментариев/тестов). Порт в wishimi:

| Где | Путь |
|-----|------|
| Cursor | `.cursor/agents/*.md` |
| Grok | `.grok/agents/*.md` (`spawn_subagent` `subagent_type` = имя файла) |

Инженеры: `back-go-engineer`, `front-orchestrator`, `front-nuxt-engineer`, `front-wishimi-engineer`, `front-e2e-tester`, `go-guide`.  
Ревьюеры (RO): `code-reviewer`, `security-reviewer`, `go-concurrency-reviewer`, `go-test-reviewer`.

Python/Airflow агенты competo **не** переносим — в wishimi нет umico-airflow.

Grok: ревьюеры `permission_mode: plan`; инженеры `default`. Мутация git запрещена во всех. Тесты не гонять — давать команду.

**Подробный гайд (Telegram + пути кода + guidelines для чатов):** [`docs/platform-telegram-guide.md`](docs/platform-telegram-guide.md)

Вспомогательное в workspace: `content/` (дизайн/маскот), `db/` (дампы), `agents/` (промпты), `e2e-tests-example/`, `eco-template/`.

### Backend (`api`)
- **DI order:** `initRuntimeModules` (notifier) **before** `initChatModule`. Otherwise chat gets a typed-nil `Notifier` iface (`(*Service)(nil)`): `s.notifier == nil` is false, `Emit` panics → Recovery 500 while the message row is already saved.
- Clean Architecture: modules (auth, wishlist, wish, notifier, price_monitor, subscription, admin_audit…).
- **DI:** ручной `internal/di.Container` (`NewContainer` → `init*Module` → геттеры). **Не** Uber fx / dig.
- **fx (если когда-нибудь):** не big-bang rewrite `NewContainer`. Сначала отдельный зелёный entrypoint (`wishimi run worker` / pricemon) на fx рядом; основной `run server` не ломать. Перенос по кускам только если fx зашёл.
- Auth через go-lepsios (email/password, email_2fa, telegram).
- Postgres + Redis; JWT access короткий, refresh длиннее.
- Pricemon in-process; gap проверки цены — **per-plan** (`features.price_monitor_gap_hours`, fallback 72h).
- **Aggregate priority (после parse):** `price.manual` → `links.parsed_price` → `links.manual_price`. Нельзя ставить `manual_price` ссылки выше parsed — иначе UI/notify залипают на цене создания.
- Admin force recheck: `POST /api/v1/admin/monitoring/recheck` (enqueue всех `monitor_price=true`, игнор gap).
- **Shop.by parser:** цена/валюта из JSON-LD `AggregateOffer.lowPrice` / `priceCurrency` (`application/ld+json` в `@graph` Product). Старые microdata `itemprop=lowPrice` meta shop.by убрал — парсер без JSON-LD падает с `product data not found`. Title: `data-title` / og:title. Microdata оставлен как fallback.
- **Ozon parser:** HTTP first, при Variti 403 — Chromium sidecar (`pkg/browser`): warmup origin ~12s, дальше `fetch(composer-api)` из page context. Env: `PARSER_OZON_BROWSER_ENABLED` (default true), `PARSER_BROWSER_EXECUTABLE`, опционально `PARSER_OZON_HTTP_PROXY`. Chromium **с того же cloud IP**, что и API, антибот всё равно режет — нужен residential proxy, не только headless. Локально: `go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6201.0 install chromium`. Docker: debian + system Chromium, `shm_size: 256mb`, `SERVER_WRITE_TIMEOUT>=60s`.
- **1688 parser:** код **не удалять**. По умолчанию **выключен** (`PARSER_1688_ENABLED=false`) — не регистрируется в sidecar. `qr.1688.com` с cloud IP открывается, `detail.1688.com` / `m.1688.com` — Aliyun WAF `cloud_ip_bl`. Chromium с Dokploy IP не обходит IP-бан. Включить только с residential `PARSER_1688_HTTP_PROXY` / `PARSER_HTTP_PROXY`. Анонс `app_versions` 2.5.0 снят миграцией `20260823180000`.
- **Taobao parser:** HTTP + cookie jar. Короткие `e.tb.cn` / шаринг → `id` и `price=` из `var url = '…item.htm?id=…&price=…'`. Карточка: `www.taobao.com/list/item/{id}.htm`, fallback `world.taobao.com/item/{id}.htm`. Если Dokploy IP отдаёт пустую/WAF-страницу (200 без `og:title`/`promotionPrice`) — **не падать** с `product data not found`, брать цену с короткой ссылки. Полное имя из шаринга `「…」` длиннее обрезанного og. `item.taobao.com` / H5 — оболочка, не источник. Прокси: `PARSER_TAOBAO_HTTP_PROXY` / `PARSER_HTTP_PROXY`.
- **Pinduoduo (yangkeduo):** share `goods1.html?ps=` редиректит в URL с `goods_id`, но SSR — login-wall (`needLogin: true`, og:title «拼多多商城»). С cloud IP карточку не достать. **Не регистрировать** и не ставить на лендинг, пока нет сессии/прокси.
- Notifier in-app + push в TG при связке.
- TG price_changed: кнопка «Открыть товар» → `{TELEGRAM_WEB_APP_URL}/me/wishlists/{wishlist_id}` (пуш шлёт **API**, не bot worker).
- Команды: HTTP server + `wishimi run telegram` (отдельный compose в Dokploy).
- **Domain ES:** `wishlists_wishes` + `wishlists_wishes_events`; `wishlists` + `wishlists_events` (versioned streams). См. [`docs/product/domain-events.md`](docs/product/domain-events.md).
- **Actor audit:** одна таблица `action_events` + `actor_kind` (`admin|user|moderator|system`) — не плодить user/admin tables. См. тот же doc + [`docs/product/admin-audit.md`](docs/product/admin-audit.md).
- **Soft-delete users:** `DELETE /admin/users/:id` (не hard delete).
- **Pre-migrate backup:** при pending goose → dump через `files.Adapter` (`FILE_STORAGE_TYPE`: local/s3/minio) в `backups/pre-migrate/`; fail closed. Ручной smoke: `wishimi migrate backup`.
- **Chat WebSocket:** переиспользуемый hub/bus/envelope → `go-lepsios/ws`; в `wishimi-back` — тонкий adapter (`internal/platform/ws` handler + chat event types), `GET /api/v1/me/ws`. Redis prefix `wishimi:chat:user:`, auth first-frame `auth`. Publish **per-recipient** DTO (`mine` / `hide_givers`) + user-level `inbox.chat.message` (все вкладки, без subscribe). Badges: `GET /me/events`. Front plugin `inbox-realtime.client.ts` держит WS на сессию. Спека: [`docs/product/chat-websocket.md`](docs/product/chat-websocket.md), epic: [`docs/tasks/epic-chat-websocket.md`](docs/tasks/epic-chat-websocket.md). Poll thread 15s / events 30s — fallback. Локально go-lepsios резолвится через `go.work` (см. правило ниже), **не** через `replace` в `go.mod`. Read receipts: `read_status` + WS `chat.read`; TG chat push delayed 3m if unread. Spec: [`docs/product/chat-websocket.md`](docs/product/chat-websocket.md).

### Frontend (`web`)
- **Error localization (жёсткий запрет):** фронтенд вообще не переводит backend-ошибки: нельзя маппить error codes (`AUTH.*`, `WISH.*`, `PARSER.*` и т. п.), подставлять frontend i18n fallback для ошибок API или дублировать их в `app/lang/*.json`. Любая ошибка, которую видит пользователь, должна прийти уже локализованной из API через request locale; фронтенд показывает поле `message` как есть. Новые пользовательские ошибки добавляются только в API translations.
- **PWA:** `@vite-pwa/nuxt` (`config/pwa.config.ts`, `public/pwa/`). В `app.vue` обязателен `<VitePwaManifest />`; nginx: MIME `webmanifest` + `sw.js` без immutable. SW autoUpdate; API NetworkOnly; Dev SW off. Web Push (VAPID) — `/me/push/*` + SW; чат с delay 3m как TG. Док: [`docs/product/pwa.md`](docs/product/pwa.md).

- Nuxt 4 + Pinia; UI из nuxt-lepsios.
- Логин/2FA/wishlists/wishes; колокол уведомлений.
- Лендинг: тарифы с `GET /api/v1/public/subscription/plans` + валюты через `/public/currency/rates`.
- **Лендинг визуал / 3D:** идеи в [`docs/design/landing-visual.md`](docs/design/landing-visual.md). Реализация skip. TresJS-полка (jpg + чёрный canvas) **rejected**, не развивать. Предпочтение: CSS 3D реальных wish/wishlist-карточек.
- Admin: soft-delete пользователя с карточки (confirm; нельзя себя).
- TG challenge poll на фронте — этап 2 (не сделан).
- **User settings / locale:** `users.settings` jsonb (`language` сейчас; расширяемо). API `GET|PATCH /me/settings`. Auth `user.settings` в login/register. i18n URL: `prefix_except_default` (`/` = en, `/ru` = ru). Для авторизованного **settings.language важнее URL** — cookie `wishimi_locale` + middleware `locale-preference.global` + post-auth `setLocale`. Смена языка в UI → PATCH + `switchLangPath`. Регистрация берёт язык из `Accept-Language` (фронт шлёт текущий locale).
- **Locale pitfall:** `auth.global` на лендинге для авторизованного не должен `GoToPage`/`localePath` от **текущего** URL-locale (`/` = en) — иначе `/` → `/me/wishlists` без `/ru`, язык догоняет только на следующем клике. Нужен `localePath(route, wishimi_locale)`. В `locale-preference` — `setLocale` сразу + `localePath(to, preferred)`, не `switchLocalePath` (в middleware «текущий» route ещё старый).
- **Session end:** refresh fail / no access token → `endSession` (redirect), не throw «Токен не найден». Guest menu на `/me/*` = баг. Layout watch токена — safety net (middleware не бежит без navigation).
- **List pages:** `null` (not loaded) ≠ `[]` (empty). Skeleton до первого fetch; empty только после. Global `isLoading` стартует в `onRequest` *после* refresh — нельзя на него одного опираться.
- **Vue watch anti-pattern:** не делать двусторонний `watch(fieldsData)` + `watch(storeFields)` — на каждый keystroke store пишет обратно и **пересобирает** форму (петля → лаги/память). Sync только через `@update:model-value` / submit. `watch` оставлять для route id / session / one-shot side effects.
- **Background polls:** menu badges / silent chat reload — всегда `noLoading: true`. Иначе `shared-ui-loader-full` мигает каждые ~15s. Chat WS token — через `useAuth().getToken()`, не голый destructure стора.
- **Chat sounds:** WAV только из `nuxt-lepsios` (`#layers/lepsios/app/assets/audio/audio-notifications/…`). Не дублировать в `wishimi-front/app/assets/sound`.
[… truncated at ~4070 of 5683 tokens — use ctx_read with lines= parameter to see specific sections]
