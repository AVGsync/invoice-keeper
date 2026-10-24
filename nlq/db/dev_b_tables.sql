-- Copy of backend/migrations 000006, 000007, 000008 + the 000010 integration
-- columns/constraints, for LOCAL development and tests ONLY (§6.5). B's real
-- migrations CREATE these in public, so we must NOT create them there — dbinit
-- puts them in a separate schema (NLQ_DEV_SCHEMA, default nlq_dev) and sets
-- search_path before running this file. The MCP pool uses
-- search_path="<dev>, public" so unqualified names resolve here while users and
-- personas come from public.
--
-- This must mirror the real DDL exactly (column names and types): the
-- test_dev_schema_matches_public test compares information_schema.columns
-- against public once migrations have run, to guard against drift. There is NO
-- category column (decision #3). When B merges, set NLQ_DEV_SCHEMA empty and run
-- `python -m nlq.dbinit --drop-dev`.

-- 000006 recurrent_payments + 000010 (start_date, stopped_at, currency CHECK)
CREATE TABLE IF NOT EXISTS recurrent_payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    amount BIGINT NOT NULL CHECK (amount > 0),
    currency VARCHAR(3) NOT NULL,

    frequency TEXT NOT NULL,
    "interval" INTEGER NOT NULL DEFAULT 1 CHECK ("interval" > 0),
    description VARCHAR(255) NOT NULL,
    day_offset INTEGER NOT NULL,

    start_date DATE NOT NULL,
    stopped_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT recurrent_payments_frequency_check
        CHECK (frequency IN ('weekly', 'monthly')),
    CONSTRAINT recurrent_payments_day_offset_check
        CHECK (
            (frequency = 'weekly' AND day_offset BETWEEN 0 AND 6)
            OR
            (frequency = 'monthly' AND day_offset BETWEEN 1 AND 31)
        ),
    CONSTRAINT recurrent_payments_currency_check
        CHECK (currency IN ('RUB', 'USD', 'EUR'))
);

-- 000007 user_recurrent_payments
CREATE TABLE IF NOT EXISTS user_recurrent_payments (
    user_id UUID NOT NULL
        REFERENCES users(id) ON DELETE CASCADE,
    recurrent_payment_id UUID NOT NULL
        REFERENCES recurrent_payments(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, recurrent_payment_id)
);

CREATE INDEX IF NOT EXISTS idx_user_recurrent_payments_payment_id
    ON user_recurrent_payments(recurrent_payment_id);

-- 000008 payments + 000010 (paid_date, CHECKs, UNIQUE, FK RESTRICT)
CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id UUID NOT NULL
        REFERENCES users(id) ON DELETE RESTRICT,
    recurrent_payment_id UUID
        REFERENCES recurrent_payments(id) ON DELETE SET NULL,

    amount BIGINT NOT NULL CHECK (amount > 0),
    currency VARCHAR(3) NOT NULL,
    description TEXT NOT NULL DEFAULT '',

    due_date DATE NOT NULL,
    paid_at TIMESTAMPTZ,
    paid_date DATE,

    status VARCHAR(10) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'paid', 'overdue')),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CHECK (
        (status = 'paid' AND paid_at IS NOT NULL)
        OR
        (status IN ('pending', 'overdue') AND paid_at IS NULL)
    ),
    CONSTRAINT payments_paid_date_check CHECK ((paid_date IS NULL) = (paid_at IS NULL)),
    CONSTRAINT payments_currency_check CHECK (currency IN ('RUB', 'USD', 'EUR'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_payments_recurrent_due
    ON payments (recurrent_payment_id, due_date) WHERE recurrent_payment_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_payments_due_date ON payments (due_date);
CREATE INDEX IF NOT EXISTS idx_payments_user_id ON payments (user_id);
CREATE INDEX IF NOT EXISTS idx_payments_recurrent_payment_id ON payments (recurrent_payment_id);
