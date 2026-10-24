-- Детали шаблонов по описанию: ближайший срок, последняя оплата, просрочки и оплаты за 365 дней.
-- Params: $1 today, $2 include_ended (bool), $3 assignee_id (model),
--         $4 assignee_id (own), $5 title tsquery, $6 title ilike (escaped).
SELECT rp.id::text AS payment_id,
       rp.description AS title,
       (rp.amount::numeric / 100)::numeric(18, 2)::text AS amount,
       rp.currency,
       rp.frequency,
       rp."interval" AS interval,
       rp.start_date,
       (rp.stopped_at IS NOT NULL) AS is_ended,
       ra.user_id::text AS assignee_id,
       u.full_name AS assignee_name,
       pe.name AS assignee_persona,
       nx.next_due,
       lp.last_paid_date,
       COALESCE(ov.overdue_count, 0) AS overdue_count,
       COALESCE(pc.paid_count_12m, 0) AS paid_count_12m,
       count(*) OVER () AS total_count
FROM recurrent_payments rp
LEFT JOIN LATERAL (
    SELECT ur.user_id
    FROM user_recurrent_payments ur
    WHERE ur.recurrent_payment_id = rp.id
    ORDER BY ur.user_id
    LIMIT 1
) ra ON true
LEFT JOIN users u    ON u.id = ra.user_id
LEFT JOIN personas pe ON pe.id = u.persona_id
LEFT JOIN LATERAL (
    SELECT min(o.due_date) AS next_due
    FROM payments o
    WHERE o.recurrent_payment_id = rp.id AND o.paid_date IS NULL AND o.due_date >= $1
) nx ON true
LEFT JOIN LATERAL (
    SELECT max(o.paid_date) AS last_paid_date
    FROM payments o
    WHERE o.recurrent_payment_id = rp.id AND o.paid_date IS NOT NULL
) lp ON true
LEFT JOIN LATERAL (
    SELECT count(*) AS overdue_count
    FROM payments o
    WHERE o.recurrent_payment_id = rp.id AND o.paid_date IS NULL AND o.due_date < $1
) ov ON true
LEFT JOIN LATERAL (
    SELECT count(*) AS paid_count_12m
    FROM payments o
    WHERE o.recurrent_payment_id = rp.id
      AND o.paid_date IS NOT NULL
      AND o.paid_date >= ($1::date - INTERVAL '365 days')
) pc ON true
WHERE ($5::text IS NULL
       OR to_tsvector('russian', rp.description) @@ plainto_tsquery('russian', $5)
       OR rp.description ILIKE '%' || $6 || '%' ESCAPE '\')
  AND ($2::bool OR rp.stopped_at IS NULL)
  AND ($3::uuid IS NULL
       OR EXISTS (SELECT 1 FROM user_recurrent_payments x
                  WHERE x.recurrent_payment_id = rp.id AND x.user_id = $3))
  AND ($4::uuid IS NULL
       OR EXISTS (SELECT 1 FROM user_recurrent_payments x
                  WHERE x.recurrent_payment_id = rp.id AND x.user_id = $4))
ORDER BY rp.description, rp.id
LIMIT 3
