package repo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type EmployeeInput struct {
	Name         string  `json:"name"`
	EmployeeCode string  `json:"employee_code"`
	Phone        string  `json:"phone"`
	Email        string  `json:"email"`
	Department   string  `json:"department"`
	Position     string  `json:"position"`
	Level        string  `json:"level"`
	JoinDate     string  `json:"join_date"`
	SalaryBase   float64 `json:"salary_base"`
	TeamID       *int    `json:"team_id"`
	Role         string  `json:"role"`
	Password     string  `json:"password"`
}

func CreateEmployee(ctx context.Context, in EmployeeInput) (*Employee, error) {
	if strings.TrimSpace(in.Name) == "" || in.Phone == "" || len(in.Password) < 6 {
		return nil, fmt.Errorf("姓名、手机号和至少 6 位密码为必填项")
	}
	if in.EmployeeCode == "" {
		in.EmployeeCode = fmt.Sprintf("E%d", time.Now().UnixNano())
	}
	if in.Level == "" {
		in.Level = "trainee"
	}
	if in.Role == "" {
		in.Role = "employee"
	}
	if in.Position == "" {
		in.Position = "员工"
	}
	if in.JoinDate == "" {
		in.JoinDate = time.Now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", in.JoinDate); err != nil {
		return nil, fmt.Errorf("入职日期无效")
	}
	if in.SalaryBase < 0 {
		return nil, fmt.Errorf("底薪不能为负数")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	tx, err := Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var userID, id int
	if err = tx.QueryRow(ctx, `INSERT INTO users(phone,email,password_hash,role,status)VALUES($1,NULLIF($2,''),$3,$4,'active')RETURNING id`, in.Phone, in.Email, string(hash), in.Role).Scan(&userID); err != nil {
		return nil, err
	}
	if err = tx.QueryRow(ctx, `INSERT INTO employees(user_id,employee_code,name,phone,email,department,position,level,join_date,salary_base,status,team_id)
		VALUES($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9::text::date,$10,'active',$11)RETURNING id`, userID, in.EmployeeCode, in.Name, in.Phone, in.Email, in.Department, in.Position, in.Level, in.JoinDate, in.SalaryBase, in.TeamID).Scan(&id); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO employee_level_history(employee_id,from_level,to_level,change_date,reason)VALUES($1,$2,$2,$3::text::date,'入职初始等级')`, id, in.Level, in.JoinDate); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return GetEmployeeByID(ctx, id)
}

var EmployeeFields = map[string]string{"name": "", "phone": "", "email": "", "department": "", "position": "", "join_date": "date", "avatar_url": "", "salary_base": "", "status": "", "team_id": ""}

func ChangeEmployeeLevel(ctx context.Context, id int, to, date, reason string, by *int) error {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return fmt.Errorf("等级生效日期无效")
	}
	tx, err := Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var from string
	if err = tx.QueryRow(ctx, `SELECT level FROM employees WHERE id=$1 FOR UPDATE`, id).Scan(&from); err != nil {
		return err
	}
	if to == from {
		return fmt.Errorf("目标等级与当前等级相同")
	}
	if to == "eliminated" || to == "suggested_elimination" {
		if _, err = tx.Exec(ctx, `UPDATE employees SET status='inactive',updated_at=CURRENT_TIMESTAMP WHERE id=$1`, id); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO employee_level_history(employee_id,from_level,to_level,change_date,reason,approved_by)VALUES($1,$2,$2,$3::text::date,$4,$5)`, id, from, date, "淘汰: "+reason, by); err != nil {
			return err
		}
	} else {
		if _, err = tx.Exec(ctx, `UPDATE employees SET level=$2,updated_at=CURRENT_TIMESTAMP WHERE id=$1`, id, to); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO employee_level_history(employee_id,from_level,to_level,change_date,reason,approved_by)VALUES($1,$2,$3,$4::text::date,$5,$6)`, id, from, to, date, reason, by); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
