package repo

import (
	"context"
	"time"
)

// ---- 考勤 attendance_records（真实列：date/status/leave_type/attendance_days）----

// AttendanceRecord 对齐 attendance_records 表。
type AttendanceRecord struct {
	ID             int       `json:"id"`
	EmployeeID     int       `json:"employee_id"`
	Date           string    `json:"date"`
	Status         string    `json:"status"` // present / absent / leave / late
	LeaveType      *string   `json:"leave_type"`
	AttendanceDays *float64  `json:"attendance_days"`
	Notes          *string   `json:"notes"`
	CreatedAt      time.Time `json:"created_at"`
	EmployeeName   *string   `json:"employee_name,omitempty"`
}

// ListAttendance 考勤列表。
func ListAttendance(ctx context.Context, employeeID, teamID *int, startDate, endDate string, limit, offset int) ([]AttendanceRecord, int, error) {
	where := "WHERE 1=1"
	args := []any{}
	add := func(v any) string { args = append(args, v); return itoa(len(args)) }
	if employeeID != nil {
		where += ` AND a.employee_id = $` + add(*employeeID)
	}
	if teamID != nil {
		where += ` AND a.employee_id IN (SELECT id FROM employees WHERE team_id=$` + add(*teamID) + `)`
	}
	if startDate != "" {
		where += ` AND a.date >= $` + add(startDate)
	}
	if endDate != "" {
		where += ` AND a.date <= $` + add(endDate)
	}
	var total int
	if err := Pool.QueryRow(ctx, `SELECT COUNT(*) FROM attendance_records a `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := Pool.Query(ctx, `
		SELECT a.id, a.employee_id, TO_CHAR(a.date,'YYYY-MM-DD'), a.status, a.leave_type,
		       a.attendance_days, a.notes, a.created_at, e.name
		FROM attendance_records a JOIN employees e ON e.id = a.employee_id
		`+where+` ORDER BY a.date DESC LIMIT $`+add(limit)+` OFFSET $`+add(offset), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []AttendanceRecord{}
	for rows.Next() {
		var a AttendanceRecord
		if err := rows.Scan(&a.ID, &a.EmployeeID, &a.Date, &a.Status, &a.LeaveType,
			&a.AttendanceDays, &a.Notes, &a.CreatedAt, &a.EmployeeName); err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

// UpsertAttendance 记录考勤（employee_id+date 唯一，存在则更新）。
func UpsertAttendance(ctx context.Context, employeeID int, date, status, leaveType, notes string) (int, error) {
	var id int
	err := Pool.QueryRow(ctx, `
		INSERT INTO attendance_records (employee_id, date, status, leave_type, notes,attendance_days,source)
		VALUES ($1,$2,$3::text, NULLIF($4,''), NULLIF($5,''),CASE WHEN $3::text='work' THEN 1 WHEN $3::text='partial' THEN 0.5 ELSE 0 END,'manual')
		ON CONFLICT (employee_id, date) DO UPDATE SET status = $3::text, leave_type = NULLIF($4,''), notes = NULLIF($5,''),attendance_days=EXCLUDED.attendance_days,source='manual', updated_at = CURRENT_TIMESTAMP
		RETURNING id`, employeeID, date, status, leaveType, notes).Scan(&id)
	return id, err
}

// ---- 团队 teams（真实列：name/code/description/manager_id/status）----

// Team 对齐 teams 表。
type Team struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Code        *string `json:"code"`
	Description *string `json:"description"`
	ManagerID   *int    `json:"manager_id"`
	Status      string  `json:"status"`
	ManagerName *string `json:"manager_name,omitempty"`
	MemberCount int     `json:"member_count,omitempty"`
}

// ListTeams 团队列表。
func ListTeams(ctx context.Context) ([]Team, error) {
	rows, err := Pool.Query(ctx, `
		SELECT t.id, t.name, t.code, t.description, t.manager_id, COALESCE(t.status,'active'),
		       m.name,
		       (SELECT COUNT(*) FROM employees e WHERE e.team_id = t.id)
		FROM teams t LEFT JOIN employees m ON m.id = t.manager_id
		ORDER BY t.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Team{}
	for rows.Next() {
		var tm Team
		if err := rows.Scan(&tm.ID, &tm.Name, &tm.Code, &tm.Description, &tm.ManagerID,
			&tm.Status, &tm.ManagerName, &tm.MemberCount); err != nil {
			return nil, err
		}
		out = append(out, tm)
	}
	return out, rows.Err()
}

// CreateTeam 创建团队。
func CreateTeam(ctx context.Context, tm *Team) (int, error) {
	var id int
	if tm.Code == nil || *tm.Code == "" {
		code := "T" + time.Now().Format("20060102150405.000000000")
		tm.Code = &code
	}
	err := Pool.QueryRow(ctx, `
		INSERT INTO teams (name, code, description, manager_id) VALUES ($1,$2,$3,$4) RETURNING id`,
		tm.Name, tm.Code, tm.Description, tm.ManagerID).Scan(&id)
	return id, err
}

// UpdateTeam 更新团队。
func UpdateTeam(ctx context.Context, id int, tm *Team) error {
	_, err := Pool.Exec(ctx, `
		UPDATE teams SET name = $2, code = $3, description = $4, manager_id = $5, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1`, id, tm.Name, tm.Code, tm.Description, tm.ManagerID)
	return err
}

// DeleteTeam 删除团队。
func DeleteTeam(ctx context.Context, id int) error {
	_, err := Pool.Exec(ctx, `DELETE FROM teams WHERE id = $1`, id)
	return err
}

// TeamMembers 团队成员。
func TeamMembers(ctx context.Context, teamID int) ([]Employee, error) {
	rows, err := Pool.Query(ctx, `
		SELECT `+employeeCols+` FROM employees WHERE team_id = $1 ORDER BY id`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Employee{}
	for rows.Next() {
		var e Employee
		if err := scanEmployee(rows, &e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ---- 通知 notifications ----

// Notification 对齐 notifications 表。
type Notification struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Type      string    `json:"type"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

// ListNotifications 通知列表。
func ListNotifications(ctx context.Context, userID int, unreadOnly bool, limit, offset int) ([]Notification, int, error) {
	where := "WHERE user_id = $1"
	args := []any{userID}
	if unreadOnly {
		where += " AND is_read = false"
	}
	var total int
	if err := Pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := Pool.Query(ctx, `
		SELECT id, user_id, title, message, COALESCE(type,'info'), COALESCE(is_read,false), created_at
		FROM notifications `+where+` ORDER BY created_at DESC LIMIT $2 OFFSET $3`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Notification{}
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Title, &n.Message, &n.Type, &n.IsRead, &n.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, n)
	}
	return out, total, rows.Err()
}

// MarkNotificationRead 标记单条已读。
func MarkNotificationRead(ctx context.Context, id, userID int) error {
	tag, err := Pool.Exec(ctx, `UPDATE notifications SET is_read = true, read_at = CURRENT_TIMESTAMP WHERE id = $1 AND user_id = $2`, id, userID)
	return affected(tag, err)
}

// MarkAllNotificationsRead 全部已读。
func MarkAllNotificationsRead(ctx context.Context, userID int) error {
	_, err := Pool.Exec(ctx, `UPDATE notifications SET is_read = true, read_at = CURRENT_TIMESTAMP WHERE user_id = $1 AND is_read = false`, userID)
	return err
}

// DeleteNotification 删除通知。
func DeleteNotification(ctx context.Context, id, userID int) error {
	tag, err := Pool.Exec(ctx, `DELETE FROM notifications WHERE id = $1 AND user_id = $2`, id, userID)
	return affected(tag, err)
}

// UnreadCount 未读数。
func UnreadCount(ctx context.Context, userID int) (int, error) {
	var n int
	err := Pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = false`, userID).Scan(&n)
	return n, err
}

// ---- 系统设置 system_settings（真实列：key/value/description/category/is_public）----

// Setting 设置项。
type Setting struct {
	Key         string  `json:"key"`
	Value       string  `json:"value"`
	Description *string `json:"description"`
	Category    *string `json:"category"`
	IsPublic    bool    `json:"is_public"`
}

// ListSettings 全部设置。
func ListSettings(ctx context.Context) ([]Setting, error) {
	rows, err := Pool.Query(ctx, `
		SELECT key, COALESCE(value,''), description, category, COALESCE(is_public, false)
		FROM system_settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Setting{}
	for rows.Next() {
		var s Setting
		if err := rows.Scan(&s.Key, &s.Value, &s.Description, &s.Category, &s.IsPublic); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// UpsertSetting 保存设置项。
func UpsertSetting(ctx context.Context, s *Setting) error {
	_, err := Pool.Exec(ctx, `
		INSERT INTO system_settings (key, value, description, category,is_public)
		VALUES ($1,$2,$3,COALESCE($4,'general'),$5)
		ON CONFLICT (key) DO UPDATE SET value = $2, description = $3, category = COALESCE($4,system_settings.category),is_public=$5, updated_at = CURRENT_TIMESTAMP`,
		s.Key, s.Value, s.Description, s.Category, s.IsPublic)
	return err
}

// DeleteSetting 删除设置项。
func DeleteSetting(ctx context.Context, key string) error {
	_, err := Pool.Exec(ctx, `DELETE FROM system_settings WHERE key = $1`, key)
	return err
}
