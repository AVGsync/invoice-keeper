-- Вход на сайт через бота: одноразовые запросы, подтверждаемые кнопкой в Telegram.
CREATE TABLE IF NOT EXISTS telegram_web_logins (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code_hash  TEXT NOT NULL UNIQUE,
    status     TEXT NOT NULL DEFAULT 'pending'
               CHECK (status IN ('pending', 'confirmed', 'consumed', 'cancelled')),
    user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_telegram_web_logins_expires ON telegram_web_logins (expires_at);
