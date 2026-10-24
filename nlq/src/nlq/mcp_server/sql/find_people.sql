-- Only public.users and public.personas columns the role may read (section 6.2):
-- no login, no password_hash. active_payments is a code-chosen fragment: the
-- count subquery when B's tables exist, or the literal 0 when they do not, so
-- find_people keeps working while payment tools return source_unavailable
-- (section 5.9). Written without braces so str.format never touches the comment.
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
