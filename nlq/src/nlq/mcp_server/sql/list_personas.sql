-- list_personas: должности и сколько сотрудников/активных платежей на каждой
-- (§D.7). active_payments — фрагмент, выбираемый кодом: коррелированный подсчёт,
-- когда таблицы B есть, или литерал 0, когда их нет (как в find_people).
-- Пользовательский текст в SQL не попадает: $1 — tsquery, $2 — ILIKE-строка.
-- Плейсхолдеры в комментариях без фигурных скобок.
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
