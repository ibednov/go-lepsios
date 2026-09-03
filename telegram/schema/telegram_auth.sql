-- Reference template for Telegram identity + web↔bot challenges.
-- Product migrations (goose) live in the consuming service.

CREATE TABLE IF NOT EXISTS users_telegrams (
    user_id uuid PRIMARY KEY REFERENCES users(id),
    telegram_user_id bigint NOT NULL,
    telegram_chat_id bigint NOT NULL DEFAULT 0,
    telegram_username text NOT NULL DEFAULT '',
    first_name text NOT NULL DEFAULT '',
    last_name text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_telegrams_telegram_user_id
    ON users_telegrams (telegram_user_id)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS telegram_auth_challenges (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code text NOT NULL,
    purpose text NOT NULL DEFAULT 'login',
    status text NOT NULL DEFAULT 'pending',
    user_id uuid REFERENCES users(id),
    telegram_user_id bigint,
    telegram_chat_id bigint,
    expires_at timestamptz NOT NULL,
    approved_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_telegram_auth_challenges_code
    ON telegram_auth_challenges (code)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_telegram_auth_challenges_status_expires
    ON telegram_auth_challenges (status, expires_at);
