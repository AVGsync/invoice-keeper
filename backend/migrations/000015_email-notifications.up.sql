-- Подтверждение адреса кодом и очередь писем-напоминаний.
ALTER TABLE user_notification_settings
    ADD COLUMN IF NOT EXISTS verify_code_hash TEXT,
    ADD COLUMN IF NOT EXISTS verify_code_expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS verify_attempts INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS verify_sent_at TIMESTAMPTZ;

-- По кейсу напоминать нужно минимум за 3 дня.
ALTER TABLE user_notification_settings ALTER COLUMN days_before_due SET DEFAULT 3;

CREATE TABLE IF NOT EXISTS email_notifications (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    payment_id      UUID NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    due_date        DATE NOT NULL,
    kind            TEXT NOT NULL CHECK (kind IN ('due_soon', 'overdue')),
    state           TEXT NOT NULL DEFAULT 'pending'
                    CHECK (state IN ('pending', 'failed', 'sent', 'dead', 'cancelled')),
    attempts        INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error      TEXT,
    sent_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Одно письмо на человека, платёж, срок и вид.
    CONSTRAINT uq_email_notifications UNIQUE (user_id, payment_id, due_date, kind)
);
CREATE INDEX IF NOT EXISTS idx_email_notifications_due
    ON email_notifications (next_attempt_at) WHERE state IN ('pending', 'failed');
