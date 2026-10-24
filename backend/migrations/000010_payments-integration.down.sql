-- Откат 000010: снимаем ограничения, индексы и колонки, FK payments.user_id возвращаем на ON DELETE CASCADE.
ALTER TABLE user_recurrent_payments DROP CONSTRAINT IF EXISTS user_recurrent_payments_user_id_fkey,
    ADD CONSTRAINT user_recurrent_payments_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_user_id_fkey,
    ADD CONSTRAINT payments_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
DROP INDEX IF EXISTS idx_payments_due_date;
DROP INDEX IF EXISTS uq_payments_recurrent_due;
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_currency_check;
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_paid_date_check;
ALTER TABLE payments DROP COLUMN IF EXISTS paid_date;

ALTER TABLE recurrent_payments DROP CONSTRAINT IF EXISTS recurrent_payments_currency_check;
ALTER TABLE recurrent_payments DROP COLUMN IF EXISTS stopped_at;
ALTER TABLE recurrent_payments DROP COLUMN IF EXISTS start_date;
