-- base is occurrences_base.sql; status_sql and order_sql are chosen by code
-- from a fixed dictionary by enum. No user-supplied text ever reaches SQL
-- (section 5.1 p.7). Placeholders are written without braces in comments so
-- str.format never substitutes a multi-line fragment into this comment.
-- Params: $1 today, $2 date_from, $3 date_to, $4 currency, $5 min_amount,
--         $6 max_amount, $7 assignee_id (model), $8 assignee_id (visibility own),
--         $9 title tsquery, $10 title ilike (escaped), $11 limit, $12 category.
WITH base AS (
{base}
)
SELECT b.occurrence_id, b.payment_id, b.title, b.amount::text AS amount, b.currency,
       b.due_date, b.is_paid, b.assignee_id::text AS assignee_id, b.assignee_name,
       count(*) OVER () AS total_count
FROM base b
WHERE ($1::date IS NOT NULL)
  AND ({status_sql})
  AND ($2::date    IS NULL OR b.due_date >= $2)
  AND ($3::date    IS NULL OR b.due_date <= $3)
  AND ($4::text    IS NULL OR b.currency = $4)
  AND ($5::numeric IS NULL OR b.amount >= $5)
  AND ($6::numeric IS NULL OR b.amount <= $6)
  AND ($7::uuid    IS NULL OR b.assignee_id = $7)
  AND ($8::uuid    IS NULL OR b.assignee_id = $8)
  AND ($9::text    IS NULL
       OR to_tsvector('russian', b.title) @@ plainto_tsquery('russian', $9)
       OR b.title ILIKE '%' || $10 || '%' ESCAPE '\')
  AND ($12::text   IS NULL OR b.category = $12)
ORDER BY {order_sql}
LIMIT $11
