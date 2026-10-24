-- Индексы под серверный поиск (GET /payment-occurrences/search, /recurring-payments/search).
CREATE INDEX IF NOT EXISTS idx_payments_paid_date ON payments (paid_date) WHERE paid_date IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_payments_currency_due ON payments (currency, due_date);
CREATE INDEX IF NOT EXISTS idx_payments_created_at ON payments (created_at);
CREATE INDEX IF NOT EXISTS idx_payments_unpaid_rp_due ON payments (recurrent_payment_id, due_date) WHERE paid_date IS NULL;
CREATE INDEX IF NOT EXISTS idx_recurrent_payments_stopped_at ON recurrent_payments (stopped_at);
