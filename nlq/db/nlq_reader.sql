-- Read-only role for the NLQ MCP server (§6.2). Executed by `python -m nlq.dbinit`
-- under the superuser after `migrate`, idempotently, on every `docker compose up`.
--
-- dbinit runs three things that are NOT in this file, because they need runtime
-- values and safe quoting via format():
--   * LOGIN PASSWORD  (format %L, from NLQ_DB_PASSWORD)
--   * GRANT CONNECT ON DATABASE <db>  (format %I, from POSTGRES_DB)
--   * dev schema + fixtures            (db/dev_b_tables.sql, local only)
--
-- Grants are column-level allow-lists. No VIEWs, no objects depending on B's
-- tables, no ALTER DEFAULT PRIVILEGES (§6.1).

-- 1. Role (CREATE ROLE has no IF NOT EXISTS; the role is cluster-wide).
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'nlq_reader') THEN
    CREATE ROLE nlq_reader NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE
                           NOINHERIT NOREPLICATION NOBYPASSRLS;
  END IF;
END $$;

-- 2. Per-role limits (apply in every session).
ALTER ROLE nlq_reader CONNECTION LIMIT 5;
ALTER ROLE nlq_reader SET default_transaction_read_only = on;
ALTER ROLE nlq_reader SET statement_timeout = '3s';
ALTER ROLE nlq_reader SET lock_timeout = '1s';
ALTER ROLE nlq_reader SET idle_in_transaction_session_timeout = '5s';
ALTER ROLE nlq_reader SET work_mem = '4MB';

-- 3. Schema access. No CREATE in public; USAGE still arrives via PUBLIC.
REVOKE CREATE ON SCHEMA public FROM nlq_reader;

-- 4. Reset every table/sequence/function grant, then apply the allow-list.
REVOKE ALL ON ALL TABLES    IN SCHEMA public FROM nlq_reader;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM nlq_reader;
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA public FROM nlq_reader;

GRANT SELECT (id, full_name, role, persona_id) ON public.users    TO nlq_reader;
GRANT SELECT (id, name)                        ON public.personas TO nlq_reader;

-- 5. B's tables: only if they already exist. Table-level today; column-level
--    once B confirms the schema (В-B 3). A dropped column/table takes its grant
--    with it, so nothing here blocks B's migrations.
DO $$
BEGIN
  IF to_regclass('public.recurring_payments') IS NOT NULL THEN
    GRANT SELECT ON public.recurring_payments TO nlq_reader;
    -- target: GRANT SELECT (id, title, amount, currency, assignee_id, ended_at)
    --         ON public.recurring_payments TO nlq_reader;
  END IF;
  IF to_regclass('public.payment_occurrences') IS NOT NULL THEN
    GRANT SELECT ON public.payment_occurrences TO nlq_reader;
    -- target: GRANT SELECT (id, recurring_payment_id, amount, currency, due_date, paid_at)
    --         ON public.payment_occurrences TO nlq_reader;
  END IF;
END $$;

-- refresh_tokens, telegram_*, and any future table: no grants, ever (§6.3).
