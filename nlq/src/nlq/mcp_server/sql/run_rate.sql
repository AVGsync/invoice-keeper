-- Normalized average monthly load per currency (decision #2, run_rate basis).
-- B ships a real periodicity: recurrent_payments.frequency ('monthly'|'weekly')
-- and "interval" (every N periods). We normalise each active payment to a
-- monthly amount (major units):
--   monthly: amount / interval        (every `interval` months)
--   weekly:  amount * 52 / 12 / interval  (every `interval` weeks -> per month)
-- amount is stored in MINOR units, so divide by 100 first. Only active payments
-- (stopped_at IS NULL) count. Totals are strictly per currency, never mixed.
-- `interval` is a reserved word, always written quoted: rp."interval".
-- Knows B columns, so it lives next to occurrences_base.sql.
-- Params: $1 currency, $2 assignee_id (model), $3 assignee_id (visibility own).
WITH rated AS (
    SELECT rp.currency AS currency,
           CASE rp.frequency
               WHEN 'monthly' THEN (rp.amount::numeric / 100) / rp."interval"
               WHEN 'weekly'  THEN (rp.amount::numeric / 100) * 52 / 12 / rp."interval"
           END AS monthly_amount
    FROM recurrent_payments rp
    WHERE rp.stopped_at IS NULL
      AND ($1::text IS NULL OR rp.currency = $1)
      AND ($2::uuid IS NULL
           OR EXISTS (SELECT 1 FROM user_recurrent_payments x
                      WHERE x.recurrent_payment_id = rp.id AND x.user_id = $2))
      AND ($3::uuid IS NULL
           OR EXISTS (SELECT 1 FROM user_recurrent_payments x
                      WHERE x.recurrent_payment_id = rp.id AND x.user_id = $3))
)
SELECT currency,
       sum(monthly_amount)::numeric(18, 2)::text AS total,
       count(*)                                  AS cnt
FROM rated
GROUP BY currency
ORDER BY currency
