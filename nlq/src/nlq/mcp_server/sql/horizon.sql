-- Горизонт созданных периодов: coverage_until — самый ранний последний период среди активных платежей.
SELECT min(mx)             AS coverage_until,
       bool_or(mx IS NULL) AS any_without_periods
FROM (
    SELECT rp.id, max(o.due_date) AS mx
    FROM recurrent_payments rp
    LEFT JOIN payments o ON o.recurrent_payment_id = rp.id
    WHERE rp.stopped_at IS NULL
    GROUP BY rp.id
) t
