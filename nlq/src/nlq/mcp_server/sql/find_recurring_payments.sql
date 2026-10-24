-- Knows B columns (rp.amount minor units, currency, stopped_at). order_sql is
-- chosen by code. The category_col fragment is code-selected
-- (fragments.category_col): B has no category column, so it is NULL::text and
-- the category filter reports unavailable (decision #3). The assignee is the
-- single row in user_recurrent_payments (LIMIT 1 ORDER BY user_id for
-- robustness). No user text reaches SQL. Placeholders are written without braces
-- in comments so str.format never substitutes a fragment into a comment line.
-- Params: $1 today, $2 include_ended (bool), $3 currency, $4 min_amount,
--         $5 max_amount, $6 assignee_id (model), $7 assignee_id (own),
--         $8 title tsquery, $9 title ilike (escaped), $10 limit, $11 category.
SELECT rp.id::text AS payment_id, rp.description AS title,
       (rp.amount::numeric / 100)::numeric(18, 2)::text AS amount, rp.currency,
       ra.user_id::text AS assignee_id, u.full_name AS assignee_name,
       p.name AS assignee_persona,
       {category_col} AS category,
       (rp.stopped_at IS NOT NULL) AS is_ended,
       nx.next_due,
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
LEFT JOIN personas p ON p.id = u.persona_id
LEFT JOIN LATERAL (
    SELECT min(o.due_date) AS next_due
    FROM payments o
    WHERE o.recurrent_payment_id = rp.id AND o.paid_date IS NULL AND o.due_date >= $1
) nx ON true
WHERE ($2::bool OR rp.stopped_at IS NULL)
  AND ($3::text    IS NULL OR rp.currency = $3)
  AND ($4::numeric IS NULL OR (rp.amount::numeric / 100) >= $4)
  AND ($5::numeric IS NULL OR (rp.amount::numeric / 100) <= $5)
  AND ($6::uuid    IS NULL
       OR EXISTS (SELECT 1 FROM user_recurrent_payments x
                  WHERE x.recurrent_payment_id = rp.id AND x.user_id = $6))
  AND ($7::uuid    IS NULL
       OR EXISTS (SELECT 1 FROM user_recurrent_payments x
                  WHERE x.recurrent_payment_id = rp.id AND x.user_id = $7))
  AND ($8::text    IS NULL
       OR to_tsvector('russian', rp.description) @@ plainto_tsquery('russian', $8)
       OR rp.description ILIKE '%' || $9 || '%' ESCAPE '\')
  AND ($11::text   IS NULL OR {category_col} = $11)
ORDER BY {order_sql}
LIMIT $10
