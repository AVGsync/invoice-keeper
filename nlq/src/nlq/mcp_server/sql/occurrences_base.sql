-- The ONLY place that knows developer B's payment columns (section 5.2).
-- When B's schema changes, only this file, find_recurring_payments.sql,
-- horizon.sql and run_rate.sql change; tool contracts, the prompt and eval
-- stay the same.
-- Real schema (backend/migrations 000006..000010):
--   payments(id, user_id NOT NULL, recurrent_payment_id NULL, amount BIGINT minor
--            units, currency, description, due_date, paid_date, status),
--   recurrent_payments(id, description, currency, frequency, interval, start_date,
--            stopped_at, ...).
-- amount is stored in MINOR units (BIGINT): we expose MAJOR units to the model as
-- (amount / 100)::numeric(18,2). RUB, USD and EUR all have 2 decimals.
-- paid marker = paid_date IS NOT NULL; ended marker = the template was deleted
-- (recurrent_payment_id IS NULL) or soft-stopped (rp.stopped_at IS NOT NULL).
-- The assignee of a period is payments.user_id (the responsible person at the
-- moment the row was generated), NOT NULL.
-- The category_col fragment is chosen by code (fragments.category_col): B has no
-- category column, so it is NULL::text and the category filter is reported as
-- unavailable (decision #3). It is a code-selected fragment, never user text.
SELECT p.id::text                                        AS occurrence_id,
       p.recurrent_payment_id::text                      AS payment_id,
       COALESCE(NULLIF(p.description, ''), rp.description, '') AS title,
       (p.amount::numeric / 100)::numeric(18, 2)         AS amount,
       p.currency                                        AS currency,
       p.due_date                                        AS due_date,
       (p.paid_date IS NOT NULL)                         AS is_paid,
       (p.recurrent_payment_id IS NULL OR rp.stopped_at IS NOT NULL) AS is_ended,
       p.user_id                                         AS assignee_id,
       {category_col}                                    AS category,
       u.full_name                                       AS assignee_name
FROM payments p
LEFT JOIN recurrent_payments rp ON rp.id = p.recurrent_payment_id
LEFT JOIN users u ON u.id = p.user_id
