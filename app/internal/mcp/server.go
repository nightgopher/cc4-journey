// Package mcp 将业务能力包装为 MCP 工具，供 AI Agent（EasyCode / easyagent / Claude Code）调用。
//
// 设计原则：
//   - 计算类工具直连 domain 纯函数（dry-run，不落库）——agent 与页面看到同一份规则；
//   - 查询类工具走 repo（只读 SQL）；
//   - 所有调用写 agent_tool_audit 审计表；
//   - 双传输：stdio（本地 CLI agent）与 Streamable HTTP（远程 agent，/mcp 端点，Bearer 鉴权）。
package mcp

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nightgopher/cc4-journey/app/internal/domain"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
	"github.com/nightgopher/cc4-journey/app/internal/service"
)

// ---- 工具入参/出参结构（JSON schema 由 SDK 从 struct tag 自动生成）----

type SalaryPreviewIn struct {
	EmployeeID int `json:"employeeId" jsonschema:"员工ID"`
	Year       int `json:"periodYear" jsonschema:"薪资年份，如 2025"`
	Month      int `json:"periodMonth" jsonschema:"薪资月份，1-12"`
}

type SalaryPreviewOut struct {
	EmployeeID int     `json:"employeeId"`
	Level      string  `json:"level"`
	BaseSalary float64 `json:"baseSalary"`
	Commission float64 `json:"commissionAmount"`
	Bonus      float64 `json:"bonusAmount"`
	NetSalary  float64 `json:"netSalary"`
	Note       string  `json:"note"`
}

type CommissionPreviewIn struct {
	DailyOrders []int `json:"dailyOrders" jsonschema:"每日订单数序列，如 [3,5,8]"`
}

type CommissionPreviewOut struct {
	DailyCommission []int  `json:"dailyCommission"`
	MonthlyTotal    int    `json:"monthlyTotal"`
	Rule            string `json:"rule"`
}

type LevelCheckIn struct {
	EmployeeID   *int   `json:"employeeId,omitempty" jsonschema:"员工ID（live 模式，从库取真实数据）"`
	Level        string `json:"level,omitempty" jsonschema:"（可选）直接指定等级 dry-run：trainee/c_level/b_level/a_level"`
	WorkdaysIn   int    `json:"workdaysInLevel,omitempty" jsonschema:"（dry-run 模式）在当前等级已工作的工作日数"`
	RecentOrders []int  `json:"recentOrders,omitempty" jsonschema:"（dry-run 模式）最近N个工作日订单序列"`
}

type LevelCheckOut struct {
	EmployeeID int    `json:"employeeId,omitempty"`
	Level      string `json:"level"`
	Action     string `json:"action"`
	Target     string `json:"targetLevel"`
	Reason     string `json:"reason"`
	NeedsVote  bool   `json:"needsTwoLevelApproval"`
	Mode       string `json:"mode"`
}

type EmployeeGetIn struct {
	EmployeeID int `json:"employeeId" jsonschema:"员工ID"`
}

type EmployeeOut struct {
	ID         int     `json:"id"`
	Name       string  `json:"name"`
	Level      string  `json:"level"`
	Status     string  `json:"status"`
	Department *string `json:"department"`
	JoinDate   *string `json:"join_date"`
}

type WorkdayGetIn struct {
	Year  int `json:"year" jsonschema:"年份"`
	Month int `json:"month" jsonschema:"月份 1-12"`
}

type WorkdayOut struct {
	Year          int    `json:"year"`
	Month         int    `json:"month"`
	Workdays      []int  `json:"workdays"`
	TotalWorkdays int    `json:"totalWorkdays"`
	Note          string `json:"note,omitempty"`
}

// ---- 工具实现 ----

func salaryPreview(ctx context.Context, _ *mcp.CallToolRequest, in SalaryPreviewIn) (*mcp.CallToolResult, SalaryPreviewOut, error) {
	detail, level, err := service.CalculateSalary(ctx, in.EmployeeID, in.Year, in.Month)
	if err != nil {
		return nil, SalaryPreviewOut{}, err
	}
	repo.WriteAudit(ctx, "mcp", "salary_preview",
		fmt.Sprintf("employee=%d period=%d-%d net=%.2f", in.EmployeeID, in.Year, in.Month, detail.NetSalary))
	return nil, SalaryPreviewOut{
		EmployeeID: in.EmployeeID,
		Level:      level,
		BaseSalary: detail.BaseSalary,
		Commission: detail.CommissionAmount,
		Bonus:      detail.BonusAmount,
		NetSalary:  detail.NetSalary,
		Note:       "按文档口径计算（B级88元总量顺延D1、A级出勤去重D3）；与旧引擎差异见差分报告",
	}, nil
}

func commissionPreview(ctx context.Context, _ *mcp.CallToolRequest, in CommissionPreviewIn) (*mcp.CallToolResult, CommissionPreviewOut, error) {
	daily := make([]int, 0, len(in.DailyOrders))
	for _, n := range in.DailyOrders {
		daily = append(daily, domain.DailyCommission(n))
	}
	repo.WriteAudit(ctx, "mcp", "commission_preview", fmt.Sprintf("orders=%v total=%d", in.DailyOrders, sumInts(daily)))
	return nil, CommissionPreviewOut{
		DailyCommission: daily,
		MonthlyTotal:    sumInts(daily),
		Rule:            "n<=3: n×10；4-5: 30+(n-3)×20；>=6: 70+(n-5)×30；每日独立",
	}, nil
}

func levelCheck(ctx context.Context, _ *mcp.CallToolRequest, in LevelCheckIn) (*mcp.CallToolResult, LevelCheckOut, error) {
	// dry-run 模式：直接给参数，纯函数判定，不碰库。
	if in.Level != "" {
		r := domain.EvaluateLevelTransition(domain.LevelTransitionInput{
			Level: in.Level, WorkdaysInLevel: in.WorkdaysIn, RecentOrders: in.RecentOrders,
		})
		repo.WriteAudit(ctx, "mcp", "level_check_dryrun",
			fmt.Sprintf("level=%s workdays=%d orders=%v -> %s", in.Level, in.WorkdaysIn, in.RecentOrders, r.Action))
		return nil, LevelCheckOut{
			Level: in.Level, Action: r.Action, Target: r.Target,
			Reason: r.Reason, NeedsVote: r.NeedsVote, Mode: "dry-run",
		}, nil
	}
	if in.EmployeeID == nil {
		return nil, LevelCheckOut{}, fmt.Errorf("live 模式需要 employeeId；或用 dry-run：提供 level + recentOrders")
	}
	// 真实模式：从库取员工与业绩窗口。
	r, err := service.CheckLevelTransition(ctx, *in.EmployeeID, time.Now())
	if err != nil {
		return nil, LevelCheckOut{}, err
	}
	emp, err := repo.GetEmployeeByID(ctx, *in.EmployeeID)
	if err != nil {
		return nil, LevelCheckOut{}, err
	}
	repo.WriteAudit(ctx, "mcp", "level_check_live",
		fmt.Sprintf("employee=%d -> %s", *in.EmployeeID, r.Action))
	return nil, LevelCheckOut{
		EmployeeID: *in.EmployeeID, Level: emp.Level,
		Action: r.Action, Target: r.Target,
		Reason: r.Reason, NeedsVote: r.NeedsVote, Mode: "live",
	}, nil
}

func employeeGet(ctx context.Context, _ *mcp.CallToolRequest, in EmployeeGetIn) (*mcp.CallToolResult, EmployeeOut, error) {
	emp, err := repo.GetEmployeeByID(ctx, in.EmployeeID)
	if err != nil {
		return nil, EmployeeOut{}, err
	}
	repo.WriteAudit(ctx, "mcp", "employee_get", fmt.Sprintf("employee=%d", in.EmployeeID))
	var join *string
	if emp.JoinDate != nil {
		s := emp.JoinDate.Format("2006-01-02")
		join = &s
	}
	return nil, EmployeeOut{
		ID: emp.ID, Name: emp.Name, Level: emp.Level, Status: emp.Status,
		Department: emp.Department, JoinDate: join,
	}, nil
}

func workdayGet(ctx context.Context, _ *mcp.CallToolRequest, in WorkdayGetIn) (*mcp.CallToolResult, WorkdayOut, error) {
	w, err := repo.GetWorkdayConfig(ctx, in.Year, in.Month)
	if err != nil {
		// 未配置月份：按旧引擎行为回退周一至周五，明确标注。
		fallback := fallbackWorkdays(in.Year, in.Month)
		repo.WriteAudit(ctx, "mcp", "workday_get_fallback", fmt.Sprintf("%d-%d", in.Year, in.Month))
		return nil, WorkdayOut{
			Year: in.Year, Month: in.Month, Workdays: fallback,
			TotalWorkdays: len(fallback),
			Note:          "该月无配置，按周一至周五缺省（与旧引擎一致）。注意法定节假日月份需管理员配置。",
		}, nil
	}
	repo.WriteAudit(ctx, "mcp", "workday_get", fmt.Sprintf("%d-%d", in.Year, in.Month))
	return nil, WorkdayOut{Year: w.Year, Month: w.Month, Workdays: w.Workdays, TotalWorkdays: w.TotalWorkdays}, nil
}

func sumInts(xs []int) int {
	t := 0
	for _, x := range xs {
		t += x
	}
	return t
}

func fallbackWorkdays(year, month int) []int {
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

// ---- Server 装配 ----

// NewServer 构建 MCP Server 实例（工具清单：6 个只读 + dry-run）。
func NewServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "cc4-callcenter",
		Version: "1.0.0",
	}, &mcp.ServerOptions{
		Instructions: "Call Center 4.0 业务系统工具集。薪资/提成/等级判定均为只读 dry-run；" +
			"业务口径以 GOLDEN.md 金标准为准（B级88元总量顺延、A级出勤按去重日期、A零业绩转人工）。",
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "salary_preview",
		Description: "薪资试算（只读）：按现有规则计算员工某月薪资明细。规则含C级3天封顶跨月、B级88元总量顺延、A级出勤比例底薪+全勤+绩效。返回 baseSalary/commissionAmount/bonusAmount/netSalary。",
	}, salaryPreview)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "commission_preview",
		Description: "提成试算（只读）：按日订单序列计算每日提成与月度累计。公式 10/20/30 阶梯，每日独立不跨日。",
	}, commissionPreview)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "level_transition_check",
		Description: "等级流转判定：live 模式（给 employeeId，从库取真实业绩窗口）或 dry-run 模式（直接给 level/workdays/recentOrders）。返回晋升/淘汰/降级建议与原因。所有流转建议需两级人工审批。",
	}, levelCheck)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "employee_get",
		Description: "查询员工档案（只读）：等级、状态、部门、入职日期。",
	}, employeeGet)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "workday_get",
		Description: "查询月度工作日配置（只读）：所有『X个工作日』规则的基准。未配置月份回退周一至周五并明确提示。",
	}, workdayGet)

	return server
}

// ServeStdio 以 stdio 传输运行（本地 CLI agent 接入方式）。
// stdout 是协议通道，任何日志不得写 stdout。
func ServeStdio() error {
	return NewServer().Run(context.Background(), &mcp.StdioTransport{})
}

// HTTPHandler 返回挂到主 HTTP 服务的 Streamable HTTP MCP 端点。
// authToken 非空时校验 Authorization: Bearer <token>。
func HTTPHandler(authToken string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authToken != "" {
			if r.Header.Get("Authorization") != "Bearer "+authToken {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
		}
		mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
			return NewServer()
		}, nil).ServeHTTP(w, r)
	})
}
