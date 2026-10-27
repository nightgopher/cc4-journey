-- Salary expenses were one row per month (reference_id salary-YYYY-M) dated by the
-- month's latest payment, so each new payment moved earlier ones to its own date.
-- Replace them with one row per paid salary record, dated by that record's payment.
WITH legacy AS (
    DELETE FROM finance_records
    WHERE reference_type = 'salary' AND reference_id ~ '^salary-[0-9]+-[0-9]+$'
    RETURNING split_part(reference_id, '-', 2)::int AS period_year,
              split_part(reference_id, '-', 3)::int AS period_month,
              transaction_date
)
INSERT INTO finance_records(type, category, amount, transaction_date, description, status, reference_type, reference_id, source, cost_type)
SELECT 'expense', 'salary', s.net_salary, COALESCE(s.payment_date, legacy.transaction_date),
       format('%s年%s月已发薪资·%s', s.period_year, s.period_month, COALESCE(e.name, '员工' || s.employee_id)),
       'confirmed', 'salary', 'salary-record-' || s.id, 'auto', 'onetime'
FROM legacy
JOIN salary_records s ON s.period_year = legacy.period_year AND s.period_month = legacy.period_month
LEFT JOIN employees e ON e.id = s.employee_id
WHERE s.payment_status = 'paid' AND s.net_salary > 0;

CREATE UNIQUE INDEX salary_finance_once ON finance_records(reference_id) WHERE reference_type = 'salary';
