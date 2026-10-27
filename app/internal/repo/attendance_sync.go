package repo

import (
	"context"
	"github.com/jackc/pgx/v5"
)

func syncPerformanceAttendance(ctx context.Context, tx pgx.Tx, employeeID int, date, period string) error {
	_, err := tx.Exec(ctx, `INSERT INTO attendance_records(employee_id,date,status,attendance_days,source)
		SELECT $1,$2::text::date,CASE WHEN bool_or(work_period='full_day') OR COUNT(DISTINCT work_period)>1 THEN 'work' ELSE 'partial' END,
		CASE WHEN bool_or(work_period='full_day') OR COUNT(DISTINCT work_period)>1 THEN 1 ELSE 0.5 END,'performance'
		FROM daily_performance_records WHERE employee_id=$1 AND work_date=$2::text::date
		ON CONFLICT(employee_id,date) DO UPDATE SET status=EXCLUDED.status,attendance_days=EXCLUDED.attendance_days,source='performance',updated_at=CURRENT_TIMESTAMP
		WHERE attendance_records.source IN('performance','default')`, employeeID, date)
	return err
}
