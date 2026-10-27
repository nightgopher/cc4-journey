// Package access shares current account, module and employee-scope policies
// between the REST API and MCP. No role or scope supplied by a client is trusted.
package access

import (
	"context"
	"errors"
	"slices"

	"github.com/nightgopher/cc4-journey/app/internal/repo"
)

var ErrUnauthenticated = errors.New("账号不存在、已停用或员工已离职")
var ErrForbidden = errors.New("无权限执行此操作")

type Principal struct {
	UserID     int
	Role       string
	EmployeeID int
	Position   *string
	TeamID     *int
}

func Load(ctx context.Context, userID int) (*Principal, error) {
	p := &Principal{UserID: userID}
	var status string
	var employeeID *int
	var employeeStatus *string
	var employeeCount int
	err := repo.Pool.QueryRow(ctx, `SELECT u.role,u.status,e.id,e.status,e.position,e.team_id,count(e.id) OVER ()
		FROM users u LEFT JOIN employees e ON e.user_id=u.id WHERE u.id=$1`, userID).
		Scan(&p.Role, &status, &employeeID, &employeeStatus, &p.Position, &p.TeamID, &employeeCount)
	// Ambiguous employee bindings must not pick an arbitrary role/team scope.
	if err != nil || status != "active" || employeeCount > 1 || (employeeID != nil && (employeeStatus == nil || *employeeStatus != "active")) {
		return nil, ErrUnauthenticated
	}
	if employeeID != nil {
		p.EmployeeID = *employeeID
	}
	return p, nil
}

func (p *Principal) Modules(ctx context.Context) ([]string, error) {
	allowed := repo.DefaultModules(p.Role)
	if p.Role != "admin" && p.Position != nil {
		configured, ok, err := repo.PositionModules(ctx, repo.Pool, *p.Position)
		if err != nil {
			return nil, err
		}
		if ok {
			allowed = configured
		}
	}
	return allowed, nil
}

func (p *Principal) HasModule(ctx context.Context, module string) bool {
	modules, err := p.Modules(ctx)
	return err == nil && slices.Contains(modules, module)
}

func (p *Principal) Scope() (employeeID, teamID *int, err error) {
	switch p.Role {
	case "employee":
		if p.EmployeeID == 0 {
			return nil, nil, ErrForbidden
		}
		return &p.EmployeeID, nil, nil
	case "manager", "workplace_manager":
		if p.EmployeeID == 0 || p.TeamID == nil {
			return nil, nil, ErrForbidden
		}
		return nil, p.TeamID, nil
	default:
		return nil, nil, nil
	}
}

func (p *Principal) CheckEmployee(ctx context.Context, targetID int) error {
	self, team, err := p.Scope()
	if err != nil || targetID <= 0 || (self != nil && *self != targetID) {
		return ErrForbidden
	}
	if team != nil {
		targetTeam, err := repo.TeamIDOfEmployee(ctx, targetID)
		if err != nil || targetTeam == nil || *targetTeam != *team {
			return ErrForbidden
		}
	}
	return nil
}

func CanManagePerformance(role string) bool {
	return slices.Contains([]string{"admin", "hr", "manager", "workplace_manager"}, role)
}

type principalKey struct{}

func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

func FromContext(ctx context.Context) *Principal {
	p, _ := ctx.Value(principalKey{}).(*Principal)
	return p
}
