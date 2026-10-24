-- Роль nlq_reader только для чтения. Выполняется `python -m nlq.dbinit` после миграций, идемпотентно.
-- Пароль, GRANT CONNECT и dev-схема задаются в dbinit. Гранты column-level, без VIEW и без ALTER DEFAULT PRIVILEGES.

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

-- 5. Payment tables: column-level allow-list, each guarded by to_regclass.
--    "interval" is a reserved word and must be quoted.
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
-- future table: no grants, ever.
