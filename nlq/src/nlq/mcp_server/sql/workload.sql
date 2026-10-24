-- workload: нагрузка по ответственным или должностям — число платежей и суммы
-- по каждой валюте (§D.7). base = occurrences_base.sql; status_sql и group_name
-- выбираются кодом (не из пользовательского текста). Суммы по валютам не
-- смешиваются. Плейсхолдеры в комментариях без фигурных скобок.
-- Params: $1 today, $2 date_from, $3 date_to, $4 currency, $5 assignee_id (own).
WITH base AS (
{base}
),
f AS (
    SELECT b.*, u2.full_name AS u_name, p2.name AS p_name
    FROM base b
    LEFT JOIN users u2    ON u2.id = b.assignee_id
    LEFT JOIN personas p2 ON p2.id = u2.persona_id
    WHERE ($1::date IS NOT NULL)
      AND ({status_sql})
      AND ($2::date IS NULL OR b.due_date >= $2)
      AND ($3::date IS NULL OR b.due_date <= $3)
      AND ($4::text IS NULL OR b.currency = $4)
      AND ($5::uuid IS NULL OR b.assignee_id = $5)
),
g AS (
    SELECT {group_name}                                                      AS name,
           f.currency                                                        AS currency,
           count(*)                                                          AS cnt,
           sum(f.amount)                                                     AS total_num,
           count(*) FILTER (WHERE NOT f.is_paid AND NOT f.is_ended
                            AND f.due_date < $1)                             AS overdue_count
    FROM f
    GROUP BY name, f.currency
)
SELECT g.name,
       g.currency,
       g.cnt,
       g.total_num::text        AS total,
       g.overdue_count,
       count(*) OVER ()         AS total_groups
FROM g
ORDER BY g.currency, g.total_num DESC, g.name
LIMIT 60
