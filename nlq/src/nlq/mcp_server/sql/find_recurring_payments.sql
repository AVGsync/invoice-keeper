-- Knows B columns (rp.amount/currency/ended_at). order_sql chosen by code. The
-- category_col fragment is code-selected (fragments.category_col): rp.category
-- when B's column exists, else NULL::text (decision #3). No user text reaches
-- SQL. Placeholders are written without braces in comments.
-- Params: $1 today, $2 include_ended (bool), $3 currency, $4 min_amount,
--         $5 max_amount, $6 assignee_id (model), $7 assignee_id (own),
--         $8 title tsquery, $9 title ilike (escaped), $10 limit, $11 category.
SELECT rp.id::text AS payment_id, rp.title,
       rp.amount::text AS amount, rp.currency,
       rp.assignee_id::text AS assignee_id, u.full_name AS assignee_name,
       p.name AS assignee_persona,
       {category_col} AS category,
       (rp.ended_at IS NOT NULL) AS is_ended,
       nx.next_due,
       count(*) OVER () AS total_count
FROM recurring_payments rp
LEFT JOIN users u    ON u.id = rp.assignee_id
LEFT JOIN personas p ON p.id = u.persona_id
LEFT JOIN LATERAL (
    SELECT min(o.due_date) AS next_due
    FROM payment_occurrences o
    WHERE o.recurring_payment_id = rp.id AND o.paid_at IS NULL AND o.due_date >= $1
) nx ON true
WHERE ($2::bool OR rp.ended_at IS NULL)
  AND ($3::text    IS NULL OR rp.currency = $3)
  AND ($4::numeric IS NULL OR rp.amount >= $4)
  AND ($5::numeric IS NULL OR rp.amount <= $5)
  AND ($6::uuid    IS NULL OR rp.assignee_id = $6)
  AND ($7::uuid    IS NULL OR rp.assignee_id = $7)
  AND ($8::text    IS NULL
       OR to_tsvector('russian', rp.title) @@ plainto_tsquery('russian', $8)
       OR rp.title ILIKE '%' || $9 || '%' ESCAPE '\')
  AND ($11::text   IS NULL OR rp.category = $11)
ORDER BY {order_sql}
LIMIT $10
