package repo

import (
	"context"
	"fmt"
	"sort"
)

func SetSalaryPayment(ctx context.Context, ids []int, status, date string) error {
	tx, err := Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT DISTINCT period_year,period_month FROM salary_records WHERE id=ANY($1) ORDER BY 1,2`, ids)
	if err != nil {
		return err
	}
	periods := map[string][2]int{}
	for rows.Next() {
		var y, m int
		if err = rows.Scan(&y, &m); err != nil {
			rows.Close()
			return err
		}
		periods[fmt.Sprintf("salary-%d-%d", y, m)] = [2]int{y, m}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	keys := []string{}
	for key := range periods {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, key); err != nil {
			return err
		}
	}
	// Paying an already paid record again is a no-op: it keeps the original payment date.
	tag, err := tx.Exec(ctx, `UPDATE salary_records SET payment_status=$2::text,payment_date=CASE WHEN $2::text='paid' THEN COALESCE(CASE WHEN payment_status='paid' THEN payment_date END,NULLIF($3::text,'')::date,CURRENT_DATE) ELSE NULL END,updated_at=CURRENT_TIMESTAMP WHERE id=ANY($1)`, ids, status, date)
	if err != nil {
		return err
	}
	if int(tag.RowsAffected()) != len(ids) {
		return fmt.Errorf("部分薪资记录不存在或重复")
	}
	for _, key := range keys {
		period := periods[key]
		// One expense per paid salary record, dated by its own payment date, so a later
		// payment never moves or merges an earlier one. The legacy month-level row is
		// replaced on the period's first change after migration 005.
		if _, err = tx.Exec(ctx, `DELETE FROM finance_records WHERE reference_type='salary' AND (reference_id=$3 OR reference_id IN
			(SELECT 'salary-record-'||id FROM salary_records WHERE period_year=$1 AND period_month=$2))`, period[0], period[1], key); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO finance_records(type,category,amount,transaction_date,description,status,reference_type,reference_id,source,cost_type)
		SELECT 'expense','salary',s.net_salary,COALESCE(s.payment_date,CURRENT_DATE),format('%s年%s月已发薪资·%s',s.period_year,s.period_month,COALESCE(e.name,'员工'||s.employee_id)),
			'confirmed','salary','salary-record-'||s.id,'auto','onetime'
		FROM salary_records s LEFT JOIN employees e ON e.id=s.employee_id
		WHERE s.period_year=$1 AND s.period_month=$2 AND s.payment_status='paid' AND s.net_salary>0`, period[0], period[1])
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
