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

-- 5. B's payment tables (backend/migrations 000006..000010): column-level
--    allow-list, each guarded by to_regclass so it is a no-op until migrations
--    have run. Migrations run before nlq-db-init, so 000010's columns
--    (start_date, stopped_at, paid_date) already exist here. A dropped
--    column/table takes its grant with it, so nothing here blocks B's
--    migrations. "interval" is a reserved word -> must be quoted.
DO $$
BEGIN
  IF to_regclass('public.recurrent_payments') IS NOT NULL THEN
    GRANT SELECT (id, amount, currency, frequency, "interval", description,
                  day_offset, start_date, stopped_at, created_at)
        ON public.recurrent_payments TO nlq_reader;
  END IF;
  IF to_regclass('public.user_recurrent_payments') IS NOT NULL THEN
    GRANT SELECT (user_id, recurrent_payment_id)
        ON public.user_recurrent_payments TO nlq_reader;
  END IF;
  IF to_regclass('public.payments') IS NOT NULL THEN
    GRANT SELECT (id, user_id, recurrent_payment_id, amount, currency,
                  description, due_date, paid_date, status)
        ON public.payments TO nlq_reader;
  END IF;
END $$;

-- user_notification_settings (email), refresh_tokens, telegram_*, and any
-- future table: no grants, ever (§6.3). The email column is never readable.
