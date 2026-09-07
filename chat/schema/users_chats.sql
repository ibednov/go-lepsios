-- Reference template for generic chat rooms + members + messages.
-- Product migrations (goose) live in the consuming service.
-- Consumer may add FK to users(id) on user_id columns when the product uses uuid users.

CREATE TABLE IF NOT EXISTS users_chats (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind text NOT NULL,
    external_id text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_chats_kind_external_active
    ON users_chats (kind, external_id)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS users_chats_members (
    chat_id uuid NOT NULL REFERENCES users_chats(id),
    user_id uuid NOT NULL,
    role text NOT NULL DEFAULT 'member',
    joined_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    PRIMARY KEY (chat_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_users_chats_members_user_active
    ON users_chats_members (user_id)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS users_chats_messages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id uuid NOT NULL REFERENCES users_chats(id),
    sender_user_id uuid NOT NULL,
    body text NOT NULL DEFAULT '',
    attachment_file_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    deleted_by uuid
);

CREATE INDEX IF NOT EXISTS idx_users_chats_messages_chat_created
    ON users_chats_messages (chat_id, created_at ASC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_users_chats_messages_chat_created_all
    ON users_chats_messages (chat_id, created_at ASC);
