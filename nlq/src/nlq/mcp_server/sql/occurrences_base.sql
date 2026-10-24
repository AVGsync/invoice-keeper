-- The ONLY place that knows developer B's payment columns (section 5.2).
-- When B fixes the schema, only this file, find_recurring_payments.sql,
-- coverage.sql and run_rate.sql change; tool contracts, the prompt and eval
-- stay the same.
-- The category_col fragment is chosen by code (fragments.category_col): the
-- real rp.category when B's column exists, or NULL::text so the adapter keeps
-- working while the category filter is reported as unavailable (decision #3).
-- It is a code-selected fragment, never user text.
-- amount/currency may live on rp instead of o; paid marker paid_at|paid_date;
-- ended marker rp.ended_at|rp.deleted_at. Confirm with B (section 6.4, V-B 3).
SELECT o.id::text                AS occurrence_id,
       rp.id::text               AS payment_id,
       rp.title                  AS title,
       o.amount                  AS amount,
       o.currency                AS currency,
       o.due_date                AS due_date,
       (o.paid_at IS NOT NULL)   AS is_paid,
       (rp.ended_at IS NOT NULL) AS is_ended,
       rp.assignee_id            AS assignee_id,
       {category_col}            AS category,
       u.full_name               AS assignee_name
FROM payment_occurrences o
JOIN recurring_payments rp ON rp.id = o.recurring_payment_id
LEFT JOIN users u ON u.id = rp.assignee_id
