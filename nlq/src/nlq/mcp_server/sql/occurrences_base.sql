-- Единственное место, где описаны колонки платежей. Суммы хранятся в минимальных единицах,
-- наружу отдаются как (amount / 100)::numeric(18,2). Оплачен: paid_date IS NOT NULL;
-- завершён: шаблон удалён или остановлен (stopped_at IS NOT NULL). Ответственный: payments.user_id.
SELECT p.id::text                                        AS occurrence_id,
       p.recurrent_payment_id::text                      AS payment_id,
       COALESCE(NULLIF(p.description, ''), rp.description, '') AS title,
       (p.amount::numeric / 100)::numeric(18, 2)         AS amount,
       p.currency                                        AS currency,
       p.due_date                                        AS due_date,
       (p.paid_date IS NOT NULL)                         AS is_paid,
       (p.recurrent_payment_id IS NULL OR rp.stopped_at IS NOT NULL) AS is_ended,
       p.user_id                                         AS assignee_id,
       {category_col}                                    AS category,
       u.full_name                                       AS assignee_name
FROM payments p
LEFT JOIN recurrent_payments rp ON rp.id = p.recurrent_payment_id
LEFT JOIN users u ON u.id = p.user_id
