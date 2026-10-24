-- Должности и число сотрудников и активных платежей на каждой.
-- Params: $1 query tsquery, $2 query ilike (escaped), $3 limit.
SELECT p.name AS persona,
       count(DISTINCT u.id) AS users_count,
       {active_payments}    AS active_payments
FROM personas p
LEFT JOIN users u ON u.persona_id = p.id
WHERE $1::text IS NULL
   OR to_tsvector('russian', p.name) @@ plainto_tsquery('russian', $1)
   OR p.name ILIKE '%' || $2 || '%' ESCAPE '\'
GROUP BY p.id, p.name
ORDER BY p.name
LIMIT $3
