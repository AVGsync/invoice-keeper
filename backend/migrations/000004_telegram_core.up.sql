-- Ядро Telegram-бота: привязки пользователей, одноразовые коды привязки,
-- командный чат и дедуп входящих апдейтов (docs/TELEGRAM_BOT.md §5.1).
-- FK на таблицы модуля платежей B здесь нет: эти таблицы независимы и
-- применяются в любом порядке относительно миграций B.

-- Привязка аккаунта к Telegram. DM chat_id совпадает с tg_user_id.
CREATE TABLE IF NOT EXISTS telegram_links (
    user_id      UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    tg_user_id   BIGINT      NOT NULL UNIQUE,
    tg_username  TEXT,
    dm_enabled   BOOLEAN     NOT NULL DEFAULT TRUE,
    bot_blocked  BOOLEAN     NOT NULL DEFAULT FALSE,
    linked_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Одноразовые коды привязки из веба. В БД лежит только хеш кода (hex(sha256)).
CREATE TABLE IF NOT EXISTS telegram_link_codes (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash      TEXT        NOT NULL UNIQUE,
    claimed_by_tg  BIGINT,
    expires_at     TIMESTAMPTZ NOT NULL,
    used_at        TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- Частичный индекс для подсчёта активных (неиспользованных) кодов пользователя.
CREATE INDEX IF NOT EXISTS idx_tg_link_codes_user_active
    ON telegram_link_codes (user_id) WHERE used_at IS NULL;

-- Командный чат. Активным может быть только один (частичный уникальный индекс).
CREATE TABLE IF NOT EXISTS telegram_chats (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tg_chat_id      BIGINT      NOT NULL UNIQUE,
    type            TEXT        NOT NULL CHECK (type IN ('group','supergroup')),
    title           TEXT,
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    bound_by        UUID REFERENCES users(id) ON DELETE SET NULL,
    bound_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deactivated_at  TIMESTAMPTZ,
    last_error      TEXT
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_telegram_chats_one_active
    ON telegram_chats ((TRUE)) WHERE is_active;

-- Дедуп входящих апдейтов: Telegram повторяет апдейт при таймауте или не-2xx.
CREATE TABLE IF NOT EXISTS telegram_updates (
    update_id    BIGINT PRIMARY KEY,
    received_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_tg_updates_received ON telegram_updates (received_at);
