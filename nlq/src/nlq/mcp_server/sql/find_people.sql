-- Только колонки users и personas, доступные роли: без login и password_hash.
-- Params: $1 query tsquery, $2 query ilike (escaped), $3 limit.
SELECT u.id::text AS user_id, u.full_name, u.role, p.name AS persona,
       {active_payments} AS active_payments
FROM users u
LEFT JOIN personas p ON p.id = u.persona_id
WHERE $1::text IS NULL
   OR to_tsvector('russian', u.full_name || ' ' || COALESCE(p.name, '')) @@ plainto_tsquery('russian', $1)
   OR u.full_name ILIKE '%' || $2 || '%' ESCAPE '\'
   OR p.name      ILIKE '%' || $2 || '%' ESCAPE '\'
ORDER BY u.full_name
LIMIT $3
