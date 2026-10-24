-- Средняя месячная нагрузка по валютам: monthly = amount / interval, weekly = amount * 52 / 12 / interval.
-- Суммы хранятся в минимальных единицах, считаются только активные платежи. interval — зарезервированное слово.
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
