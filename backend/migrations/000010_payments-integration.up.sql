-- Интеграция модуля платежей с ботом и NL-ассистентом: якорь графика, мягкая
-- остановка, дата оплаты без зоны, защита от дублей периодов, сохранение истории.
ALTER TABLE recurrent_payments
    ADD COLUMN IF NOT EXISTS start_date DATE,
    ADD COLUMN IF NOT EXISTS stopped_at TIMESTAMPTZ;
UPDATE recurrent_payments SET start_date = (created_at AT TIME ZONE 'UTC')::date WHERE start_date IS NULL;
ALTER TABLE recurrent_payments ALTER COLUMN start_date SET NOT NULL;
-- DROP ... IF EXISTS перед ADD делает миграцию идемпотентной при повторном up.
ALTER TABLE recurrent_payments DROP CONSTRAINT IF EXISTS recurrent_payments_currency_check,
    ADD CONSTRAINT recurrent_payments_currency_check
    CHECK (currency IN ('RUB','USD','EUR')) NOT VALID;

ALTER TABLE payments ADD COLUMN IF NOT EXISTS paid_date DATE;
UPDATE payments SET paid_date = (paid_at AT TIME ZONE 'UTC')::date WHERE paid_at IS NOT NULL AND paid_date IS NULL;
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_paid_date_check,
    ADD CONSTRAINT payments_paid_date_check CHECK ((paid_date IS NULL) = (paid_at IS NULL));
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_currency_check,
    ADD CONSTRAINT payments_currency_check CHECK (currency IN ('RUB','USD','EUR')) NOT VALID;
CREATE UNIQUE INDEX IF NOT EXISTS uq_payments_recurrent_due
    ON payments (recurrent_payment_id, due_date) WHERE recurrent_payment_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_payments_due_date ON payments (due_date);
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_user_id_fkey,
    ADD CONSTRAINT payments_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT;
-- Ответственный за шаблон тоже защищён: удаление пользователя с назначенным
-- шаблоном (но ещё без периодов) должно давать 409, а не молча осиротить шаблон.
ALTER TABLE user_recurrent_payments DROP CONSTRAINT IF EXISTS user_recurrent_payments_user_id_fkey,
    ADD CONSTRAINT user_recurrent_payments_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT;
