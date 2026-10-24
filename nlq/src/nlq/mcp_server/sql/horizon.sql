-- Horizon of created occurrences, per active payment (section 5.4, decision #4).
-- coverage_until is the EARLIEST last-occurrence across active payments, so a
-- single payment generated far ahead cannot mask others whose periods were not
-- created yet. any_without_periods flags active payments that have no
-- occurrences at all. Knows B columns, so it lives next to occurrences_base.sql.
SELECT min(mx)             AS coverage_until,
       bool_or(mx IS NULL) AS any_without_periods
FROM (
    SELECT rp.id, max(o.due_date) AS mx
    FROM recurring_payments rp
    LEFT JOIN payment_occurrences o ON o.recurring_payment_id = rp.id
    WHERE rp.ended_at IS NULL
    GROUP BY rp.id
) t
