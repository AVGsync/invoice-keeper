-- Суммы по месяцам и валютам; валюты не смешиваются.
-- Params: $1 today, $2 date_from, $3 date_to, $4 currency, $5 title tsquery,
--         $6 title ilike (escaped), $7 assignee_id (model), $8 assignee_id (own).
WITH base AS (
{base}
),
f AS (
    SELECT * FROM base b
    WHERE ($1::date IS NOT NULL)
      AND ({status_sql})
      AND (b.due_date >= $2::date)
      AND (b.due_date <= $3::date)
      AND ($4::text IS NULL OR b.currency = $4)
      AND ($5::text IS NULL
           OR to_tsvector('russian', b.title) @@ plainto_tsquery('russian', $5)
           OR b.title ILIKE '%' || $6 || '%' ESCAPE '\')
      AND ($7::uuid IS NULL OR b.assignee_id = $7)
      AND ($8::uuid IS NULL OR b.assignee_id = $8)
)
SELECT to_char(date_trunc('month', f.due_date), 'YYYY-MM')       AS month,
       f.currency                                                AS currency,
       sum(f.amount)::text                                       AS total,
       COALESCE(sum(f.amount) FILTER (WHERE f.is_paid), 0)::text     AS paid,
       COALESCE(sum(f.amount) FILTER (WHERE NOT f.is_paid), 0)::text AS unpaid,
       count(*)                                                  AS cnt
FROM f
GROUP BY month, f.currency
ORDER BY month, f.currency
