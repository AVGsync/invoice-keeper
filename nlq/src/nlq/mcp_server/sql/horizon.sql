-- Horizon of created periods, per active recurrent payment (section 5.4,
-- decision #4). coverage_until is the EARLIEST last-period across active
-- payments, so a single payment generated far ahead cannot mask others whose
-- periods were not created yet. any_without_periods flags active payments that
-- have no periods at all. The backend scheduler generates periods up to
-- today + 31 days, so a window reaching past that is reported incomplete.
-- Knows B columns, so it lives next to occurrences_base.sql.
SELECT min(mx)             AS coverage_until,
       bool_or(mx IS NULL) AS any_without_periods
FROM (
    SELECT rp.id, max(o.due_date) AS mx
    FROM recurrent_payments rp
    LEFT JOIN payments o ON o.recurrent_payment_id = rp.id
    WHERE rp.stopped_at IS NULL
    GROUP BY rp.id
) t
