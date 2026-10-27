// Package mcpaccess combines a revocable integration credential with the user's
// current CC4 business permissions. Credentials never contain a role snapshot.
package mcpaccess

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/nightgopher/cc4-journey/app/internal/access"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
)

type Grant struct {
	Credential *repo.MCPCredential
	Principal  *access.Principal
	Modules    []string
}

func Resolve(ctx context.Context, credentialID int64) (*Grant, error) {
	c, err := repo.ActiveMCPCredential(ctx, credentialID)
	if err != nil {
		return nil, access.ErrUnauthenticated
	}
	p, err := access.Load(ctx, c.UserID)
	if err != nil {
		return nil, err
	}
	if p.EmployeeID == 0 && p.Role != "admin" {
		return nil, access.ErrUnauthenticated
	}
	modules, err := p.Modules(ctx)
	if err != nil {
		return nil, err
	}
	return &Grant{Credential: c, Principal: p, Modules: modules}, nil
}

func Authenticate(ctx context.Context, raw string) (*Grant, error) {
	if len(raw) != 72 || !strings.HasPrefix(raw, "cc4_mcp_") {
		return nil, access.ErrUnauthenticated
	}
	c, err := repo.FindMCPCredentialByToken(ctx, raw)
	if err != nil {
		return nil, access.ErrUnauthenticated
	}
	g, err := Resolve(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	updated, err := repo.Pool.Exec(ctx, `UPDATE mcp_credentials SET last_used_at=now() WHERE id=$1 AND revoked_at IS NULL`, c.ID)
	if err != nil {
		return nil, err
	}
	if updated.RowsAffected() != 1 {
		return nil, access.ErrUnauthenticated
	}
	return g, nil
}

func (g *Grant) CanTool(ctx context.Context, tool string) bool {
	p := g.Principal
	has := func(module string) bool { return slices.Contains(g.Modules, module) }
	switch tool {
	case "employee_get":
		return has("profile") || has("employees")
	case "salary_preview":
		return has("salary")
	case "commission_preview":
		return has("salary") || has("daily-performance")
	case "level_transition_check":
		return has("employees") || has("daily-performance")
	case "workday_get":
		return has("workdays")
	case "performance_submit", "performance_approve":
		return g.Credential.AccessMode == "read_write" && p.EmployeeID > 0 && access.CanManagePerformance(p.Role) && has("daily-performance")
	default:
		return false
	}
}

func CanWrite(ctx context.Context, p *access.Principal) bool {
	return p.EmployeeID > 0 && access.CanManagePerformance(p.Role) && p.HasModule(ctx, "daily-performance")
}

func (g *Grant) CheckTarget(ctx context.Context, tool string, arguments any) error {
	var in struct {
		EmployeeID int    `json:"employeeId"`
		DetailID   int    `json:"detailId"`
		Level      string `json:"level"`
	}
	raw, err := json.Marshal(arguments)
	if err != nil || json.Unmarshal(raw, &in) != nil {
		return fmt.Errorf("工具参数无效")
	}
	p := g.Principal
	switch tool {
	case "performance_approve":
		detail, err := repo.GetOrderDetail(ctx, in.DetailID)
		if err != nil {
			return access.ErrForbidden
		}
		return p.CheckEmployee(ctx, detail.EmployeeID)
	case "employee_get":
		if in.EmployeeID == p.EmployeeID && p.EmployeeID > 0 && slices.Contains(g.Modules, "profile") {
			return nil
		}
		if !slices.Contains(g.Modules, "employees") {
			return access.ErrForbidden
		}
		return p.CheckEmployee(ctx, in.EmployeeID)
	case "salary_preview", "performance_submit":
		return p.CheckEmployee(ctx, in.EmployeeID)
	case "level_transition_check":
		if in.Level == "" {
			return p.CheckEmployee(ctx, in.EmployeeID)
		}
	}
	return nil
}

type grantKey struct{}

func WithGrant(ctx context.Context, g *Grant) context.Context {
	return context.WithValue(ctx, grantKey{}, g)
}
func FromContext(ctx context.Context) *Grant {
	g, _ := ctx.Value(grantKey{}).(*Grant)
	return g
}

func Audit(ctx context.Context, g *Grant, tool string, arguments any, result string) {
	if repo.Pool == nil {
		return
	}
	var object struct {
		EmployeeID int `json:"employeeId,omitempty"`
		DetailID   int `json:"detailId,omitempty"`
	}
	raw, _ := json.Marshal(arguments)
	_ = json.Unmarshal(raw, &object)
	detail, _ := json.Marshal(map[string]any{"userId": g.Principal.UserID, "employeeId": g.Principal.EmployeeID,
		"credentialId": g.Credential.ID, "targetEmployeeId": object.EmployeeID, "detailId": object.DetailID, "result": result})
	repo.WriteAudit(ctx, fmt.Sprintf("user:%d", g.Principal.UserID), "mcp_"+tool, string(detail))
}
