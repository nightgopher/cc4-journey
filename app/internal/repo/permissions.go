package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type Module struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Group     string `json:"group"`
	Type      string `json:"type"`
	Mandatory bool   `json:"mandatory,omitempty"`
}

var Modules = []Module{
	{Key: "dashboard", Label: "概览", Group: "core", Type: "page"}, {Key: "reports", Label: "数据报表", Group: "core", Type: "page"},
	{Key: "leads", Label: "线索管理", Group: "business", Type: "page"}, {Key: "wechat-leads", Label: "微信线索", Group: "business", Type: "page"},
	{Key: "customers", Label: "客户管理", Group: "business", Type: "page"}, {Key: "performance", Label: "月度业绩", Group: "business", Type: "page"},
	{Key: "daily-performance", Label: "每日业绩", Group: "business", Type: "page"}, {Key: "attendance", Label: "考勤管理", Group: "hr", Type: "page"},
	{Key: "employees", Label: "人员管理", Group: "hr", Type: "page"}, {Key: "teams", Label: "团队管理", Group: "hr", Type: "page"},
	{Key: "approval", Label: "审批管理", Group: "hr", Type: "page"}, {Key: "notifications", Label: "通知中心", Group: "system", Type: "page"},
	{Key: "salary", Label: "薪资管理", Group: "finance", Type: "page"}, {Key: "products", Label: "产品管理", Group: "business", Type: "page"},
	{Key: "financial", Label: "财务管理", Group: "finance", Type: "page"}, {Key: "workdays", Label: "工作日", Group: "hr", Type: "page"},
	{Key: "profile", Label: "个人中心", Group: "system", Type: "page", Mandatory: true}, {Key: "settings", Label: "系统设置", Group: "system", Type: "page"},
}

func DefaultModules(role string) []string {
	if role == "admin" {
		out := []string{}
		for _, m := range Modules {
			out = append(out, m.Key)
		}
		return out
	}
	m := map[string][]string{
		"hr":                {"dashboard", "reports", "daily-performance", "attendance", "employees", "approval", "notifications", "salary", "profile"},
		"manager":           {"dashboard", "leads", "wechat-leads", "customers", "performance", "daily-performance", "attendance", "employees", "teams", "approval", "notifications", "salary", "products", "financial", "profile"},
		"workplace_manager": {"dashboard", "leads", "wechat-leads", "customers", "performance", "daily-performance", "attendance", "employees", "teams", "approval", "notifications", "products", "workdays", "financial", "settings", "profile"},
		"employee":          {"daily-performance", "leads", "wechat-leads", "attendance", "approval", "salary", "notifications", "profile"}, "finance": {"dashboard", "financial", "salary", "notifications", "profile"},
	}
	if v, ok := m[role]; ok {
		return slices.Clone(v)
	}
	return slices.Clone(m["employee"])
}

func PositionModules(ctx context.Context, db DBTX, name string) ([]string, bool, error) {
	var value string
	err := db.QueryRow(ctx, `SELECT value FROM system_settings WHERE key=$1`, "position_permissions_"+name).Scan(&value)
	if err == nil {
		var out []string
		if err = json.Unmarshal([]byte(value), &out); err != nil {
			return nil, false, err
		}
		if !slices.Contains(out, "profile") {
			out = append(out, "profile")
		}
		return out, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}
	rows, err := db.Query(ctx, `SELECT module_key FROM position_module_permissions WHERE position_name=$1 ORDER BY id`, name)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var key string
		if err = rows.Scan(&key); err != nil {
			return nil, false, err
		}
		out = append(out, key)
	}
	if err = rows.Err(); err != nil {
		return nil, false, err
	}
	if len(out) > 0 {
		if !slices.Contains(out, "profile") {
			out = append(out, "profile")
		}
		return out, true, nil
	}
	return nil, false, nil
}

func SavePositionModules(ctx context.Context, name string, modules []string) error {
	valid := map[string]bool{}
	for _, m := range Modules {
		valid[m.Key] = true
	}
	out := []string{}
	for _, key := range modules {
		if !valid[key] {
			return fmt.Errorf("无效的模块: %s", key)
		}
		if !slices.Contains(out, key) {
			out = append(out, key)
		}
	}
	if !slices.Contains(out, "profile") {
		out = append(out, "profile")
	}
	tx, err := Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	b, _ := json.Marshal(out)
	if _, err = tx.Exec(ctx, `INSERT INTO system_settings(key,value,category,description)VALUES($1,$2,'permissions',$3)ON CONFLICT(key)DO UPDATE SET value=EXCLUDED.value,updated_at=CURRENT_TIMESTAMP`, "position_permissions_"+name, string(b), "职位权限配置: "+name); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM position_module_permissions WHERE position_name=$1`, name); err != nil {
		return err
	}
	for _, key := range out {
		if _, err = tx.Exec(ctx, `INSERT INTO position_module_permissions(position_name,module_key)VALUES($1,$2)`, name, key); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

type Position struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	RoleHint      string `json:"role_hint"`
	IsDefault     bool   `json:"isDefault"`
	Deleted       bool   `json:"deleted,omitempty"`
	EmployeeCount int    `json:"employee_count"`
	Configured    bool   `json:"configured"`
}

var defaultPositions = []Position{{ID: "default_1", Name: "员工", RoleHint: "employee", IsDefault: true}, {ID: "default_2", Name: "职场经理", RoleHint: "workplace_manager", IsDefault: true}, {ID: "default_3", Name: "团队经理", RoleHint: "manager", IsDefault: true}, {ID: "default_4", Name: "财务", RoleHint: "finance", IsDefault: true}, {ID: "default_5", Name: "人力", RoleHint: "hr", IsDefault: true}}

func loadPositions(ctx context.Context, db DBTX) ([]Position, error) {
	var raw string
	err := db.QueryRow(ctx, `SELECT value FROM system_settings WHERE key='custom_positions'`).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return slices.Clone(defaultPositions), nil
	}
	if err != nil {
		return nil, err
	}
	var custom []Position
	if err = json.Unmarshal([]byte(raw), &custom); err != nil {
		return nil, err
	}
	all := slices.Clone(defaultPositions)
	for _, p := range custom {
		i := slices.IndexFunc(all, func(d Position) bool { return d.ID == p.ID || d.Name == p.Name })
		if i < 0 {
			all = append(all, p)
		} else {
			all[i] = p
		}
	}
	return slices.DeleteFunc(all, func(p Position) bool { return p.Deleted }), nil
}
func ListPositions(ctx context.Context) ([]Position, error) {
	all, err := loadPositions(ctx, Pool)
	if err != nil {
		return nil, err
	}
	for i, p := range all {
		if err = Pool.QueryRow(ctx, `SELECT COUNT(*) FROM employees WHERE position=$1`, p.Name).Scan(&all[i].EmployeeCount); err != nil {
			return nil, err
		}
		_, all[i].Configured, err = PositionModules(ctx, Pool, p.Name)
		if err != nil {
			return nil, err
		}
	}
	return all, nil
}

func MutatePosition(ctx context.Context, id, name, role string, remove bool) (Position, error) {
	tx, err := Pool.Begin(ctx)
	if err != nil {
		return Position{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('positions',0))`); err != nil {
		return Position{}, err
	}
	all, err := loadPositions(ctx, tx)
	if err != nil {
		return Position{}, err
	}
	name = strings.TrimSpace(name)
	i := slices.IndexFunc(all, func(p Position) bool { return p.ID == id })
	if id != "" && i < 0 {
		return Position{}, pgx.ErrNoRows
	}
	if !remove {
		if name == "" {
			return Position{}, fmt.Errorf("职位名称不能为空")
		}
		if slices.ContainsFunc(all, func(p Position) bool { return p.Name == name && p.ID != id }) {
			return Position{}, fmt.Errorf("职位名称已存在")
		}
	}
	p := Position{ID: fmt.Sprintf("custom_%d", time.Now().UnixNano()), Name: name, RoleHint: role}
	if p.RoleHint == "" {
		p.RoleHint = "employee"
	}
	if i >= 0 {
		p = all[i]
		if p.Name == "员工" && (remove || name != p.Name) {
			return Position{}, fmt.Errorf("不能删除或重命名默认员工职位")
		}
		if remove {
			if _, err = tx.Exec(ctx, `UPDATE employees SET position='员工' WHERE position=$1`, p.Name); err != nil {
				return Position{}, err
			}
			if _, err = tx.Exec(ctx, `DELETE FROM system_settings WHERE key=$1`, "position_permissions_"+p.Name); err != nil {
				return Position{}, err
			}
			if _, err = tx.Exec(ctx, `DELETE FROM position_module_permissions WHERE position_name=$1`, p.Name); err != nil {
				return Position{}, err
			}
			p.Deleted = true
		} else {
			if _, err = tx.Exec(ctx, `UPDATE employees SET position=$1 WHERE position=$2`, name, p.Name); err != nil {
				return Position{}, err
			}
			if _, err = tx.Exec(ctx, `UPDATE system_settings SET key=$1 WHERE key=$2`, "position_permissions_"+name, "position_permissions_"+p.Name); err != nil {
				return Position{}, err
			}
			if _, err = tx.Exec(ctx, `UPDATE position_module_permissions SET position_name=$1 WHERE position_name=$2`, name, p.Name); err != nil {
				return Position{}, err
			}
			p.Name = name
			if role != "" {
				p.RoleHint = role
			}
		}
		p.IsDefault = false
		all[i] = p
	} else {
		all = append(all, p)
	}
	// Persist deletion markers for built-in positions, so they do not reappear.
	for _, d := range defaultPositions {
		if !slices.ContainsFunc(all, func(p Position) bool { return p.ID == d.ID }) {
			d.Deleted = true
			all = append(all, d)
		}
	}
	b, _ := json.Marshal(all)
	_, err = tx.Exec(ctx, `INSERT INTO system_settings(key,value,category)VALUES('custom_positions',$1,'positions')ON CONFLICT(key)DO UPDATE SET value=$1,updated_at=CURRENT_TIMESTAMP`, string(b))
	if err != nil {
		return Position{}, err
	}
	return p, tx.Commit(ctx)
}
