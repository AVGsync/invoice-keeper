CREATE TABLE IF NOT EXISTS recurrent_payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    amount BIGINT NOT NULL CHECK (amount > 0),
    currency VARCHAR(3) NOT NULL,

    frequency TEXT NOT NULL,
    interval INTEGER NOT NULL DEFAULT 1 CHECK (interval > 0),
    description VARCHAR(255) NOT NULL,
    day_offset INTEGER NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT recurrent_payments_frequency_check
        CHECK (frequency IN ('weekly', 'monthly')),

    CONSTRAINT recurrent_payments_day_offset_check
        CHECK (
            (frequency = 'weekly' AND day_offset BETWEEN 0 AND 6)
            OR
            (frequency = 'monthly' AND day_offset BETWEEN 1 AND 31)
        )
);