-- Outbox напоминаний (docs/TELEGRAM_BOT.md §5.1). Доставка at-least-once,
-- машина состояний pending -> sent | failed -> dead | cancelled.
-- FK на таблицы модуля платежей B нет: occurrence_id — логическая ссылка.
CREATE TABLE IF NOT EXISTS telegram_notifications (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    occurrence_id    UUID        NOT NULL,        -- логическая ссылка на период B, БЕЗ FK
    due_date         DATE        NOT NULL,        -- срок на момент постановки (часть ключа дедупа)
    kind             TEXT        NOT NULL DEFAULT 'due_soon'
                     CHECK (kind IN ('due_soon','overdue')),
    recipient        TEXT        NOT NULL,        -- 'team' (личку не делаем, решение пользователя №1)
    state            TEXT        NOT NULL DEFAULT 'pending'
                     CHECK (state IN ('pending','failed','sent','dead','cancelled')),
    attempts         INT         NOT NULL DEFAULT 0,
    next_attempt_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error       TEXT,
    chat_id          BIGINT,                      -- куда фактически ушло (для editMessageText)
    message_id       BIGINT,
    shown_status     TEXT        NOT NULL DEFAULT 'open'
                     CHECK (shown_status IN ('open','paid','ended')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at          TIMESTAMPTZ,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Дедуп: один период одному адресату один раз на каждый срок и вид (§5.1).
    CONSTRAINT uq_telegram_notifications UNIQUE (occurrence_id, due_date, kind, recipient)
);

-- Очередь на отправку: строки, которым пора.
CREATE INDEX IF NOT EXISTS idx_tg_notif_due
    ON telegram_notifications (next_attempt_at) WHERE state IN ('pending','failed');
-- Открытые отправленные сообщения — для правок (reconcile, §5.6).
CREATE INDEX IF NOT EXISTS idx_tg_notif_open_sent
    ON telegram_notifications (occurrence_id) WHERE state = 'sent' AND shown_status = 'open';
