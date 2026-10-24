-- Normalized average monthly load per currency (decision #2, run_rate basis).
-- B has no periodicity column yet, so the interval is inferred from the average
-- gap between consecutive occurrence due_dates of each active payment and mapped
-- to a monthly divisor: monthly x1, quarterly /3, semiannual /6, annual /12.
-- A payment with a single occurrence (no gap) is treated as monthly. Only active
-- payments count. Knows B columns, so it lives next to occurrences_base.sql.
-- When B ships a real periodicity/interval column, replace the inference CTE.
-- Params: $1 currency, $2 assignee_id (model), $3 assignee_id (visibility own).
WITH gaps AS (
    SELECT o.recurring_payment_id AS rp_id,
           (o.due_date - lag(o.due_date) OVER (
               PARTITION BY o.recurring_payment_id ORDER BY o.due_date)) AS gap_days
    FROM payment_occurrences o
),
per_payment AS (
    SELECT rp_id, avg(gap_days)::float AS avg_gap
    FROM gaps
    WHERE gap_days IS NOT NULL
    GROUP BY rp_id
),
rated AS (
    SELECT rp.currency AS currency,
           rp.amount   AS amount,
           CASE
               WHEN pp.avg_gap IS NULL  THEN 1
               WHEN pp.avg_gap <= 45    THEN 1
               WHEN pp.avg_gap <= 135   THEN 3
               WHEN pp.avg_gap <= 270   THEN 6
               ELSE 12
           END AS months
    FROM recurring_payments rp
    LEFT JOIN per_payment pp ON pp.rp_id = rp.id
    WHERE rp.ended_at IS NULL
      AND ($1::text IS NULL OR rp.currency = $1)
      AND ($2::uuid IS NULL OR rp.assignee_id = $2)
      AND ($3::uuid IS NULL OR rp.assignee_id = $3)
)
SELECT currency,
       sum(amount / months)::numeric(18, 2)::text AS total,
       count(*)                                   AS cnt
FROM rated
GROUP BY currency
ORDER BY currency
