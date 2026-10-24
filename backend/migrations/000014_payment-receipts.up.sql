-- Чеки к периодам платежей: загруженный файл или ссылка.
CREATE TABLE IF NOT EXISTS payment_receipts (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id    UUID NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    kind          TEXT NOT NULL CHECK (kind IN ('file', 'link')),
    title         TEXT NOT NULL DEFAULT '' CHECK (char_length(title) <= 120),
    url           TEXT CHECK (char_length(url) <= 2048),
    file_name     TEXT CHECK (char_length(file_name) <= 120),
    content_type  TEXT CHECK (char_length(content_type) <= 127),
    size_bytes    BIGINT CHECK (size_bytes >= 0),
    uploaded_by   UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT payment_receipts_shape CHECK (
        (kind = 'link' AND url IS NOT NULL AND file_name IS NULL)
        OR (kind = 'file' AND url IS NULL AND file_name IS NOT NULL AND content_type IS NOT NULL AND size_bytes IS NOT NULL)
    )
);
CREATE INDEX IF NOT EXISTS idx_payment_receipts_payment
    ON payment_receipts (payment_id, created_at);
