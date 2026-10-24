-- Fixture copies of developer B's tables, for LOCAL development and tests only
-- (§6.5). B's real migration will CREATE these in public, so we must NOT create
-- them there — dbinit puts them in a separate schema (NLQ_DEV_SCHEMA, default
-- nlq_dev) and sets search_path before running this file. The MCP pool uses
-- search_path="<dev>, public" so unqualified names resolve here while users and
-- personas come from public.
--
-- Columns mirror the logical contract in occurrences_base.sql. When B merges,
-- set NLQ_DEV_SCHEMA empty and run `python -m nlq.dbinit --drop-dev`.

CREATE TABLE IF NOT EXISTS recurring_payments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title       TEXT        NOT NULL,
    amount      NUMERIC(18, 2) NOT NULL,
    currency    TEXT        NOT NULL CHECK (currency IN ('RUB', 'USD', 'EUR')),
    assignee_id UUID,                         -- references public.users(id); NULL allowed
    category    TEXT,                         -- optional (decision #3); B may or may not ship it
    ended_at    TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS payment_occurrences (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recurring_payment_id UUID NOT NULL REFERENCES recurring_payments(id),
    amount               NUMERIC(18, 2) NOT NULL,
    currency             TEXT NOT NULL CHECK (currency IN ('RUB', 'USD', 'EUR')),
    due_date             DATE NOT NULL,
    paid_at              TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_dev_occ_payment ON payment_occurrences (recurring_payment_id);
CREATE INDEX IF NOT EXISTS idx_dev_occ_due     ON payment_occurrences (due_date);
