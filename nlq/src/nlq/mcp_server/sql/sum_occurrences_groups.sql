-- Разбивка по (валюта, ответственный) для group_by=currency_assignee; total_groups нужен для groups_truncated.
-- Params: $1 today, $2 date_from, $3 date_to, $4 currency, $5 title tsquery,
--         $6 title ilike (escaped), $7 assignee_id (model), $8 assignee_id (own),
--         $9 category.
WITH base AS (
{base}
),
f AS (
    SELECT * FROM base b
    WHERE ($1::date IS NOT NULL)
      AND ({status_sql})
      AND ($2::date IS NULL OR b.due_date >= $2)
      AND ($3::date IS NULL OR b.due_date <= $3)
      AND ($4::text IS NULL OR b.currency = $4)
      AND ($5::text IS NULL
           OR to_tsvector('russian', b.title) @@ plainto_tsquery('russian', $5)
           OR b.title ILIKE '%' || $6 || '%' ESCAPE '\')
      AND ($7::uuid IS NULL OR b.assignee_id = $7)
      AND ($8::uuid IS NULL OR b.assignee_id = $8)
      AND ($9::text IS NULL OR b.category = $9)
)
SELECT f.currency,
       f.assignee_id::text AS assignee_id,
       f.assignee_name,
       sum(f.amount)::text                                          AS total,
       COALESCE(sum(f.amount) FILTER (WHERE f.is_paid), 0)::text     AS paid_total,
       COALESCE(sum(f.amount) FILTER (WHERE NOT f.is_paid), 0)::text AS unpaid_total,
       count(*)                                                     AS cnt,
       count(*) OVER ()                                             AS total_groups
FROM f
GROUP BY f.currency, f.assignee_id, f.assignee_name
ORDER BY f.currency, f.assignee_name
LIMIT 60
