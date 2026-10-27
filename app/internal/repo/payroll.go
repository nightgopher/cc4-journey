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
	tag, err := tx.Exec(ctx, `UPDATE salary_records SET payment_status=$2::text,payment_date=CASE WHEN $2::text='paid' THEN COALESCE(NULLIF($3::text,'')::date,CURRENT_DATE) ELSE NULL END,updated_at=CURRENT_TIMESTAMP WHERE id=ANY($1)`, ids, status, date)
	if err != nil {
		return err
	}
	if int(tag.RowsAffected()) != len(ids) {
		return fmt.Errorf("部分薪资记录不存在或重复")
	}
	for _, key := range keys {
		period := periods[key]
		if _, err = tx.Exec(ctx, `DELETE FROM finance_records WHERE reference_type='salary' AND reference_id=$1`, key); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO finance_records(type,category,amount,transaction_date,description,status,reference_type,reference_id,source,cost_type)
		SELECT 'expense','salary',SUM(net_salary),COALESCE(MAX(payment_date),CURRENT_DATE),$3,'confirmed','salary',$4,'auto','onetime' FROM salary_records WHERE period_year=$1 AND period_month=$2 AND payment_status='paid' HAVING SUM(net_salary)>0`, period[0], period[1], fmt.Sprintf("%d年%d月已发薪资", period[0], period[1]), key)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
