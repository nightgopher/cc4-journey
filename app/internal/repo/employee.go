package repo

import (
	"context"
	"fmt"
	"time"
)

// Employee 对齐存量 employees 表（见 migrations/000_baseline_schema.sql）。
// JSON tag 与旧 API 响应的字段名逐一对齐（snake_case）。
type Employee struct {
	ID           int        `json:"id"`
	UserID       *int       `json:"user_id"`
	EmployeeCode *string    `json:"employee_code"`
	Name         string     `json:"name"`
	Phone        *string    `json:"phone"`
	Email        *string    `json:"email"`
	Department   *string    `json:"department"`
	Position     *string    `json:"position"`
	Level        string     `json:"level"`
	JoinDate     *time.Time `json:"join_date"`
	AvatarURL    *string    `json:"avatar_url"`
	SalaryBase   *float64   `json:"salary_base"`
	Status       string     `json:"status"`
	TeamID       *int       `json:"team_id"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	// JOIN 附加字段（对齐旧列表响应）：
	UserRole              *string `json:"user_role,omitempty"`
	LoginPhone            *string `json:"login_phone,omitempty"` // users.phone: the account's login name
	LastLogin             *string `json:"last_login,omitempty"`
	RecentAttendanceCount *int    `json:"recent_attendance_count"`
	CurrentMonthOrders    *int    `json:"current_month_orders"`
}

const employeeCols = `id, user_id, employee_code, name, phone, email, department, position,
	level, join_date, avatar_url, salary_base, status, team_id, created_at, updated_at`

// EmployeeListParams 对齐旧 GET /api/v1/employees 的查询参数与权限过滤。
type EmployeeListParams struct {
	EmployeeID *int
	Search     string
	Department string
	Level      string
	Status     string
	TeamID     *int // manager/workplace_manager 数据隔离
	Limit      int
	Offset     int
}

// ListEmployees 返回员工列表与总数（对齐旧响应 employees + pagination.total）。
func ListEmployees(ctx context.Context, p EmployeeListParams) ([]Employee, int, error) {
	where := []string{"1=1"}
	args := []any{}
	add := func(v any) string {
		args = append(args, v)
		return itoa(len(args))
	}
	if p.Search != "" {
		n := add("%" + p.Search + "%")
		where = append(where, `(e.name ILIKE $`+n+` OR e.employee_code ILIKE $`+n+` OR e.phone ILIKE $`+n+` OR u.phone ILIKE $`+n+`)`)
	}
	if p.Department != "" {
		where = append(where, `e.department = $`+add(p.Department))
	}
	if p.Level != "" {
		where = append(where, `e.level = $`+add(p.Level))
	}
	if p.Status != "" {
		where = append(where, `e.status = $`+add(p.Status))
	}
	if p.TeamID != nil {
		where = append(where, `e.team_id = $`+add(*p.TeamID))
	}
	if p.EmployeeID != nil {
		where = append(where, `e.id=$`+add(*p.EmployeeID))
	}
	whereSQL := "WHERE " + joinAnd(where)

	var total int
	if err := Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM employees e LEFT JOIN users u ON u.id = e.user_id `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	sql := `SELECT e.id, e.user_id, e.employee_code, e.name, e.phone, e.email, e.department, e.position,
			e.level, e.join_date, e.avatar_url, e.salary_base, e.status, e.team_id, e.created_at, e.updated_at,
			u.role, u.phone, TO_CHAR(u.last_login AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"'),
			(SELECT COUNT(DISTINCT a.date)::int FROM attendance_records a WHERE a.employee_id=e.id AND a.date BETWEEN CURRENT_DATE-6 AND CURRENT_DATE AND (a.status='work' OR COALESCE(a.attendance_days,0)>0)),
			(SELECT COALESCE(SUM(d.orders_count),0)::int FROM daily_performance_records d WHERE d.employee_id=e.id AND d.work_date>=date_trunc('month',CURRENT_DATE)::date AND d.work_date<(date_trunc('month',CURRENT_DATE)+interval '1 month')::date)
		FROM employees e LEFT JOIN users u ON u.id = e.user_id ` + whereSQL +
		` ORDER BY e.created_at DESC LIMIT $` + add(p.Limit) + ` OFFSET $` + add(p.Offset)
	rows, err := Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []Employee{}
	for rows.Next() {
		var e Employee
		if err := rows.Scan(&e.ID, &e.UserID, &e.EmployeeCode, &e.Name, &e.Phone, &e.Email,
			&e.Department, &e.Position, &e.Level, &e.JoinDate, &e.AvatarURL,
			&e.SalaryBase, &e.Status, &e.TeamID, &e.CreatedAt, &e.UpdatedAt,
			&e.UserRole, &e.LoginPhone, &e.LastLogin, &e.RecentAttendanceCount, &e.CurrentMonthOrders); err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

// GetEmployeeByID 按主键取员工。
func GetEmployeeByID(ctx context.Context, id int) (*Employee, error) {
	row := Pool.QueryRow(ctx, `SELECT `+employeeCols+` FROM employees WHERE id = $1`, id)
	var e Employee
	err := scanEmployee(row, &e)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// GetEmployeeByUserID 按 user_id 取员工（employee 角色数据隔离用）。
func GetEmployeeByUserID(ctx context.Context, userID int) (*Employee, error) {
	row := Pool.QueryRow(ctx, `SELECT `+employeeCols+` FROM employees WHERE user_id = $1`, userID)
	var e Employee
	if err := scanEmployee(row, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// TeamIDOfEmployee 经理数据隔离：取员工所属团队。
func TeamIDOfEmployee(ctx context.Context, employeeID int) (*int, error) {
	var teamID *int
	err := Pool.QueryRow(ctx, `SELECT team_id FROM employees WHERE id = $1`, employeeID).Scan(&teamID)
	return teamID, err
}

// ---- level history ----

// LevelHistory 对齐 employee_level_history 表。
type LevelHistory struct {
	ID               int        `json:"id"`
	EmployeeID       int        `json:"employee_id"`
	FromLevel        string     `json:"from_level"`
	ToLevel          string     `json:"to_level"`
	ChangeDate       time.Time  `json:"change_date"`
	Reason           *string    `json:"reason"`
	ApprovedBy       *int       `json:"approved_by"`
	ConditionMetDate *time.Time `json:"condition_met_date"`
	CreatedAt        time.Time  `json:"created_at"`
}

// LatestLevelChange 取员工最近一次变更到 toLevel 的记录（B 级 88 元与 C 级额度计算依赖）。
func LatestLevelChange(ctx context.Context, employeeID int, toLevel string) (*LevelHistory, error) {
	return LatestLevelChangeOnOrBefore(ctx, employeeID, toLevel, "9999-12-31")
}

// LatestLevelChangeOnOrBefore excludes later reentries when calculating an old
// payroll period; a future promotion cannot reset an already earned allowance.
func LatestLevelChangeOnOrBefore(ctx context.Context, employeeID int, toLevel, through string) (*LevelHistory, error) {
	row := Pool.QueryRow(ctx, `
		SELECT id, employee_id, from_level, to_level, change_date, reason, approved_by,
		       condition_met_date, created_at
		FROM employee_level_history
		WHERE employee_id = $1 AND to_level = $2 AND change_date <= $3::text::date
		ORDER BY change_date DESC,id DESC LIMIT 1`, employeeID, toLevel, through)

	var h LevelHistory
	err := row.Scan(&h.ID, &h.EmployeeID, &h.FromLevel, &h.ToLevel, &h.ChangeDate,
		&h.Reason, &h.ApprovedBy, &h.ConditionMetDate, &h.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

// EmployeeLevelAsOf uses the period's history rather than today's employee row.
// Legacy accounts with no history retain their existing level. If history does
// exist but cannot identify an earlier level, fail instead of silently paying 0.
func EmployeeLevelAsOf(ctx context.Context, employeeID int, through string) (string, error) {
	var level *string
	err := Pool.QueryRow(ctx, `SELECT CASE WHEN EXISTS(SELECT 1 FROM employee_level_history h WHERE h.employee_id=e.id)
		THEN COALESCE((SELECT h.to_level FROM employee_level_history h WHERE h.employee_id=e.id AND h.change_date <= $2::text::date ORDER BY h.change_date DESC,h.id DESC LIMIT 1),
		(SELECT h.from_level FROM employee_level_history h WHERE h.employee_id=e.id ORDER BY h.change_date,h.id LIMIT 1))
		ELSE e.level END FROM employees e WHERE e.id=$1`, employeeID, through).Scan(&level)
	if err != nil {
		return "", err
	}
	if level == nil || *level == "" {
		return "", fmt.Errorf("无法确定所算月份的历史等级")
	}
	return *level, nil
}

// ---- workday config ----

// WorkdayConfig 对齐 workday_config 表。
type WorkdayConfig struct {
	ID            int    `json:"id"`
	Year          int    `json:"year"`
	Month         int    `json:"month"`
	Workdays      []int  `json:"workdays"`
	TotalWorkdays int    `json:"total_workdays"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// GetWorkdayConfig 取指定月份配置；未配置返回 nil（调用方按周一至周五缺省，与旧引擎一致）。
func GetWorkdayConfig(ctx context.Context, year, month int) (*WorkdayConfig, error) {
	row := Pool.QueryRow(ctx, `
		SELECT id, year, month, workdays, total_workdays,
		       TO_CHAR(created_at, 'YYYY-MM-DD HH24:MI:SS'), TO_CHAR(updated_at, 'YYYY-MM-DD HH24:MI:SS')
		FROM workday_config WHERE year = $1 AND month = $2`, year, month)
	var w WorkdayConfig
	err := row.Scan(&w.ID, &w.Year, &w.Month, &w.Workdays, &w.TotalWorkdays, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

// UpsertWorkdayConfig 创建或更新月度工作日（对齐旧 POST /workdays 的 ON CONFLICT 行为）。
func UpsertWorkdayConfig(ctx context.Context, year, month int, workdays []int) (*WorkdayConfig, error) {
	row := Pool.QueryRow(ctx, `
		INSERT INTO workday_config (year, month, workdays, total_workdays)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (year, month)
		DO UPDATE SET workdays = $3, total_workdays = $4, updated_at = CURRENT_TIMESTAMP
		RETURNING id, year, month, workdays, total_workdays,
		          TO_CHAR(created_at, 'YYYY-MM-DD HH24:MI:SS'), TO_CHAR(updated_at, 'YYYY-MM-DD HH24:MI:SS')`,
		year, month, workdays, len(workdays))
	var w WorkdayConfig
	err := row.Scan(&w.ID, &w.Year, &w.Month, &w.Workdays, &w.TotalWorkdays, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

// ---- helpers ----

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func joinAnd(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " AND "
		}
		out += p
	}
	return out
}
