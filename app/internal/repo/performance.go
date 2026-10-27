package repo

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"sort"
	"time"
)

// scanner 最小行扫描接口（pgx.Row 与 pgx.Rows 均满足）。
type scanner interface {
	Scan(dest ...any) error
}

func scanEmployee(r scanner, e *Employee) error {
	return r.Scan(&e.ID, &e.UserID, &e.EmployeeCode, &e.Name, &e.Phone, &e.Email,
		&e.Department, &e.Position, &e.Level, &e.JoinDate, &e.AvatarURL,
		&e.SalaryBase, &e.Status, &e.TeamID, &e.CreatedAt, &e.UpdatedAt)
}

// ---- 每日业绩汇总 ----

// DailyPerformance 对齐 daily_performance_records 表。
type DailyPerformance struct {
	ID               int       `json:"id"`
	EmployeeID       int       `json:"employee_id"`
	WorkDate         string    `json:"work_date"`
	OrdersCount      int       `json:"orders_count"`
	SalesAmount      float64   `json:"sales_amount"`
	CommissionAmount float64   `json:"commission_amount"`
	ProductID        *int      `json:"product_id"`
	WorkPeriod       *string   `json:"work_period"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	EmployeeName     *string   `json:"employee_name,omitempty"` // JOIN 附加
	ProductName      *string   `json:"product_name,omitempty"`  // JOIN 附加
}

// PerformanceQuery 对齐旧 GET /daily-performance 的过滤参数。
type PerformanceQuery struct {
	EmployeeID *int
	TeamID     *int
	StartDate  string
	EndDate    string
	WorkDate   string
	Limit      int
	Offset     int
}

// ListPerformance 返回业绩汇总列表与总数（含员工姓名 JOIN，对齐旧响应）。
func ListPerformance(ctx context.Context, q PerformanceQuery) ([]DailyPerformance, int, error) {
	where := []string{"1=1"}
	args := []any{}
	add := func(v any) string {
		args = append(args, v)
		return itoa(len(args))
	}
	if q.EmployeeID != nil {
		where = append(where, `dpr.employee_id = $`+add(*q.EmployeeID))
	}
	if q.TeamID != nil {
		where = append(where, `e.team_id = $`+add(*q.TeamID))
	}
	if q.StartDate != "" {
		where = append(where, `dpr.work_date >= $`+add(q.StartDate))
	}
	if q.EndDate != "" {
		where = append(where, `dpr.work_date <= $`+add(q.EndDate))
	}
	if q.WorkDate != "" {
		where = append(where, `dpr.work_date = $`+add(q.WorkDate))
	}
	whereSQL := "WHERE " + joinAnd(where)

	var total int
	if err := Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM daily_performance_records dpr
		 JOIN employees e ON e.id = dpr.employee_id `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	sql := `SELECT dpr.id, dpr.employee_id, TO_CHAR(dpr.work_date, 'YYYY-MM-DD'),
			COALESCE(dpr.orders_count,0), COALESCE(dpr.sales_amount,0), COALESCE(dpr.commission_amount,0),
			dpr.product_id, dpr.work_period, dpr.created_at, dpr.updated_at,
			e.name, p.name
		FROM daily_performance_records dpr
		JOIN employees e ON e.id = dpr.employee_id
		LEFT JOIN products p ON p.id = dpr.product_id
		` + whereSQL + ` ORDER BY dpr.work_date DESC, dpr.id DESC
		LIMIT $` + add(q.Limit) + ` OFFSET $` + add(q.Offset)
	rows, err := Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []DailyPerformance{}
	for rows.Next() {
		var d DailyPerformance
		if err := rows.Scan(&d.ID, &d.EmployeeID, &d.WorkDate, &d.OrdersCount, &d.SalesAmount,
			&d.CommissionAmount, &d.ProductID, &d.WorkPeriod, &d.CreatedAt, &d.UpdatedAt,
			&d.EmployeeName, &d.ProductName); err != nil {
			return nil, 0, err
		}
		out = append(out, d)
	}
	return out, total, rows.Err()
}

// MonthlyOrderTotals 返回员工某月「工作日期 → 当日订单数」映射（薪资引擎输入）。
func MonthlyOrderTotals(ctx context.Context, employeeID int, year, month int) (map[string]int, error) {
	rows, err := Pool.Query(ctx, `
		SELECT TO_CHAR(work_date, 'YYYY-MM-DD'), COALESCE(SUM(orders_count), 0)
		FROM daily_performance_records
		WHERE employee_id = $1
		  AND EXTRACT(YEAR FROM work_date) = $2
		  AND EXTRACT(MONTH FROM work_date) = $3 GROUP BY work_date`, employeeID, year, month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var d string
		var n int
		if err := rows.Scan(&d, &n); err != nil {
			return nil, err
		}
		out[d] = n
	}
	return out, rows.Err()
}

// AttendanceDaysInMonth 统计员工某月出勤天数（业绩日期 ∩ 工作日配置，按去重日期）。
// 注意：这与旧引擎按记录行数统计不同——按文档口径去重，D3 修复点。
func AttendanceDaysInMonth(ctx context.Context, employeeID int, year, month int, workdays []int) (int, error) {
	days, err := AttendanceUnitsInMonth(ctx, employeeID, year, month, workdays)
	return int(days), err
}

// Explicit attendance (including leave/absence) overrides performance-derived presence.
// Each date contributes at most one day, regardless of the number of products.
func AttendanceUnitsInMonth(ctx context.Context, employeeID int, year, month int, workdays []int) (float64, error) {
	if len(workdays) == 0 {
		return 0, nil
	}
	wdSet := map[int]bool{}
	for _, d := range workdays {
		wdSet[d] = true
	}
	rows, err := Pool.Query(ctx, `
		SELECT EXTRACT(DAY FROM date)::int,
		CASE WHEN status='absent' THEN 0 WHEN status='partial' THEN 0.5 WHEN status='work' THEN LEAST(1,GREATEST(0,COALESCE(attendance_days,1))) ELSE 0 END::float
		FROM attendance_records WHERE employee_id=$1 AND EXTRACT(YEAR FROM date)=$2 AND EXTRACT(MONTH FROM date)=$3
		UNION ALL
		SELECT DISTINCT EXTRACT(DAY FROM d.work_date)::int,1::float FROM daily_performance_records d
		WHERE d.employee_id=$1 AND EXTRACT(YEAR FROM d.work_date)=$2 AND EXTRACT(MONTH FROM d.work_date)=$3 AND COALESCE(d.orders_count,0)>0
		AND NOT EXISTS(SELECT 1 FROM attendance_records a WHERE a.employee_id=d.employee_id AND a.date=d.work_date)`, employeeID, year, month)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	days := 0.0
	for rows.Next() {
		var d int
		var units float64
		if err := rows.Scan(&d, &units); err != nil {
			return 0, err
		}
		if wdSet[d] {
			days += units
		}
	}
	return days, rows.Err()
}

// ---- 工作日序列（等级流转窗口计算用）----

// WorkdaysUpTo 返回从 levelChangeDate（含）到 endDate（含）的工作日日期列表。
// 缺月配置时按周一至周五缺省（对齐旧引擎 getWorkdaysInRange 行为）。
func WorkdaysUpTo(ctx context.Context, levelChangeDate time.Time, endDate time.Time) ([]time.Time, error) {
	out := []time.Time{}
	start := time.Date(levelChangeDate.Year(), levelChangeDate.Month(), levelChangeDate.Day(), 0, 0, 0, 0, time.UTC)
	endDate = time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, time.UTC)
	// 逐月取配置，缺月回退周一至周五。
	cur := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, start.Location())
	for !cur.After(endDate) {
		y, m := cur.Year(), int(cur.Month())
		monthDays, err := WorkdaysOfMonth(ctx, y, m)
		if err != nil {
			return nil, err
		}
		sort.Ints(monthDays)
		seen := map[int]bool{}
		for _, d := range monthDays {
			day := time.Date(y, time.Month(m), d, 0, 0, 0, 0, start.Location())
			if seen[d] || day.Month() != time.Month(m) || day.Before(start) || day.After(endDate) {
				continue
			}
			seen[d] = true
			out = append(out, day)
		}
		cur = time.Date(y, time.Month(m)+1, 1, 0, 0, 0, 0, start.Location())
	}
	return out, nil
}

func WorkdaysOfMonth(ctx context.Context, year, month int) ([]int, error) {
	w, err := GetWorkdayConfig(ctx, year, month)
	if errors.Is(err, pgx.ErrNoRows) {
		// 未配置：周一至周五缺省（对齐旧引擎）。
		return defaultWorkdays(year, month), nil
	}
	if err != nil {
		return nil, err
	}
	return w.Workdays, nil
}

func defaultWorkdays(year, month int) []int {
	var out []int
	t := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	for t.Month() == time.Month(month) {
		if wd := t.Weekday(); wd != time.Saturday && wd != time.Sunday {
			out = append(out, t.Day())
		}
		t = t.AddDate(0, 0, 1)
	}
	return out
}

// ---- 审批 ----

// ApprovalRequest 对齐 approval_requests 表。
type ApprovalRequest struct {
	ID              int            `json:"id"`
	RequestType     string         `json:"request_type"`
	Title           string         `json:"title"`
	Description     *string        `json:"description"`
	Priority        string         `json:"priority"`
	RequestedBy     int            `json:"requested_by"`
	CurrentApprover *int           `json:"current_approver"`
	Status          string         `json:"status"`
	Metadata        map[string]any `json:"metadata"`
	RequestedAt     time.Time      `json:"requested_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// InsertApprovalRequest 创建审批单（等级检查 cron 与 MCP 写操作共用）。
func InsertApprovalRequest(ctx context.Context, a *ApprovalRequest) (int, error) {
	var id int
	err := Pool.QueryRow(ctx, `
		INSERT INTO approval_requests (request_type, title, description, priority, requested_by, current_approver, status, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7)
		RETURNING id`,
		a.RequestType, a.Title, a.Description, a.Priority, a.RequestedBy, a.CurrentApprover, a.Metadata).Scan(&id)
	return id, err
}

// ---- 薪资记录 ----

// SalaryRecord 对齐 salary_records 表。
type SalaryRecord struct {
	ID               int     `json:"id"`
	EmployeeID       int     `json:"employee_id"`
	PeriodYear       int     `json:"period_year"`
	PeriodMonth      int     `json:"period_month"`
	BaseSalary       float64 `json:"base_salary"`
	CommissionAmount float64 `json:"commission_amount"`
	BonusAmount      float64 `json:"bonus_amount"`
	OvertimePay      float64 `json:"overtime_pay"`
	Deductions       float64 `json:"deductions"`
	NetSalary        float64 `json:"net_salary"`
	PaymentDate      *string `json:"payment_date"`
	PaymentStatus    string  `json:"payment_status"`
	CalcDetails      *string `json:"calculation_details"`
	Notes            *string `json:"notes"`
	// Read-only, from the list query: departed employees are not in the active lookup.
	EmployeeName   string `json:"employee_name,omitempty"`
	EmployeeStatus string `json:"employee_status,omitempty"`
}

// UpsertSalary 保存或更新某员工某月薪资（计算明细存 JSON 字符串，对齐旧表）。
func UpsertSalary(ctx context.Context, s *SalaryRecord) (int, error) {
	tx, err := Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("salary-%d-%d", s.PeriodYear, s.PeriodMonth)); err != nil {
		return 0, err
	}
	var id int
	err = tx.QueryRow(ctx, `
		INSERT INTO salary_records (employee_id, period_year, period_month, base_salary,
			commission_amount, bonus_amount, overtime_pay, deductions, net_salary,
			payment_status, calculation_details)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'pending',$10)
		ON CONFLICT (employee_id, period_year, period_month)
		DO UPDATE SET base_salary=$4, commission_amount=$5, bonus_amount=$6,
			overtime_pay=$7, deductions=$8, net_salary=$9, calculation_details=$10,
			updated_at=CURRENT_TIMESTAMP
		WHERE COALESCE(salary_records.payment_status,'pending')!='paid'
		RETURNING id`,
		s.EmployeeID, s.PeriodYear, s.PeriodMonth, s.BaseSalary,
		s.CommissionAmount, s.BonusAmount, s.OvertimePay, s.Deductions, s.NetSalary,
		s.CalcDetails).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("已发放的薪资不能重新计算，请先撤销发放")
	}
	if err != nil {
		return 0, err
	}
	return id, tx.Commit(ctx)
}

// ---- 审计（MCP 写操作与 API 共用）----

// WriteAudit 记录一次工具/接口调用（表不存在时静默跳过，避免阻塞主流程）。
func WriteAudit(ctx context.Context, actor, action, detail string) {
	_, _ = Pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS agent_tool_audit (
			id SERIAL PRIMARY KEY,
			actor TEXT, action TEXT, detail TEXT,
			created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP)`)
	_, _ = Pool.Exec(ctx,
		`INSERT INTO agent_tool_audit (actor, action, detail) VALUES ($1,$2,$3)`,
		actor, action, detail)
}
