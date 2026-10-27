package repo

import (
	"context"
	"fmt"
)

func AggregateMonthlyPerformance(ctx context.Context, year, month int, team *int) (int64, error) {
	tx, err := Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("monthly-performance-%d-%d", year, month)); err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM performance_records p USING employees e WHERE e.id=p.employee_id AND p.period_year=$1 AND p.period_month=$2 AND ($3::int IS NULL OR e.team_id=$3)`, year, month, team); err != nil {
		return 0, err
	}
	tag, err := tx.Exec(ctx, `INSERT INTO performance_records(employee_id,period_year,period_month,product_id,orders_count,sales_amount,commission_amount)
	SELECT d.employee_id,$1::int,$2::int,d.product_id,SUM(COALESCE(d.orders_count,0)),SUM(COALESCE(d.sales_amount,0)),SUM(COALESCE(d.commission_amount,0))
	FROM daily_performance_records d JOIN employees e ON e.id=d.employee_id WHERE EXTRACT(YEAR FROM d.work_date)::int=$1::int AND EXTRACT(MONTH FROM d.work_date)::int=$2::int AND($3::int IS NULL OR e.team_id=$3)GROUP BY d.employee_id,d.product_id`, year, month, team)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), tx.Commit(ctx)
}

func GenerateDefaultAttendance(ctx context.Context, date string) (int64, error) {
	tag, err := Pool.Exec(ctx, `INSERT INTO attendance_records(employee_id,date,status,attendance_days,source)
	SELECT e.id,$1::text::date,'absent',0,'default' FROM employees e LEFT JOIN users u ON u.id=e.user_id WHERE e.status='active' AND COALESCE(u.role,'employee')!='admin'
	AND NOT EXISTS(SELECT 1 FROM daily_performance_records d WHERE d.employee_id=e.id AND d.work_date=$1::text::date)
	ON CONFLICT(employee_id,date)DO NOTHING`, date)
	return tag.RowsAffected(), err
}
