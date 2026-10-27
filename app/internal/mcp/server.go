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
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nightgopher/cc4-journey/app/internal/access"
	"github.com/nightgopher/cc4-journey/app/internal/domain"
	"github.com/nightgopher/cc4-journey/app/internal/mcpaccess"
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
	r, err := service.CheckLevelTransition(ctx, *in.EmployeeID, service.BusinessNow())
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

type PerformanceSubmitIn struct {
	EmployeeID  int     `json:"employeeId" jsonschema:"员工ID"`
	WorkDate    string  `json:"workDate" jsonschema:"工作日期 YYYY-MM-DD"`
	ProductID   *int    `json:"productId,omitempty" jsonschema:"产品ID（提供则走产品分档提成；缺省走日汇总公式）"`
	Orders      int     `json:"orders" jsonschema:"订单数（>0）"`
	SalesAmount float64 `json:"salesAmount,omitempty" jsonschema:"销售额（无产品ID时使用）"`
	WorkPeriod  string  `json:"workPeriod,omitempty" jsonschema:"时段：full_day/morning/afternoon"`
}

type PerformanceSubmitOut struct {
	RecordID      int     `json:"recordId"`
	OrdersCount   int     `json:"ordersCount"`
	Commission    float64 `json:"commissionAmount"`
	SalesAmount   float64 `json:"salesAmount"`
	NeedsApproval bool    `json:"needsApproval"`
	Note          string  `json:"note"`
}

type PerformanceApproveIn struct {
	DetailID        int    `json:"detailId" jsonschema:"订单明细ID"`
	Approved        bool   `json:"approved" jsonschema:"通过传 true，驳回传 false"`
	RejectionReason string `json:"rejectionReason,omitempty" jsonschema:"驳回原因（驳回时必填）"`
}

type PerformanceApproveOut struct {
	DetailID   int     `json:"detailId"`
	EmployeeID int     `json:"employeeId"`
	Commission float64 `json:"commissionAmount"`
	Note       string  `json:"note"`
}

func performanceSubmit(ctx context.Context, _ *mcp.CallToolRequest, in PerformanceSubmitIn) (*mcp.CallToolResult, PerformanceSubmitOut, error) {
	grant := mcpaccess.FromContext(ctx)
	if grant == nil || grant.Principal.EmployeeID <= 0 {
		return nil, PerformanceSubmitOut{}, access.ErrForbidden
	}
	result, err := service.SubmitPerformance(ctx, service.SubmitPerformanceInput{
		EmployeeID: in.EmployeeID, WorkDate: in.WorkDate,
		ProductID: in.ProductID, Orders: in.Orders,
		SalesAmount: in.SalesAmount, WorkPeriod: in.WorkPeriod,
		SubmittedBy: grant.Principal.EmployeeID, Channel: "mcp",
	})
	if err != nil {
		return nil, PerformanceSubmitOut{}, err
	}
	return nil, PerformanceSubmitOut{
		RecordID: result.RecordID, OrdersCount: result.OrdersCount,
		Commission: result.CommissionAmount, SalesAmount: result.SalesAmount,
		NeedsApproval: result.NeedsApproval,
		Note:          "已按现有规则计算提成并直接入汇总（管理员/代录通道语义）。如需员工自录审批流，走 HTTP 接口。",
	}, nil
}

func performanceApprove(ctx context.Context, _ *mcp.CallToolRequest, in PerformanceApproveIn) (*mcp.CallToolResult, PerformanceApproveOut, error) {
	grant := mcpaccess.FromContext(ctx)
	if grant == nil || grant.Principal.EmployeeID <= 0 {
		return nil, PerformanceApproveOut{}, access.ErrForbidden
	}
	result, err := service.ApproveOrderDetail(ctx, in.DetailID, in.Approved, in.RejectionReason, grant.Principal.EmployeeID)
	if err != nil {
		return nil, PerformanceApproveOut{}, err
	}
	return nil, PerformanceApproveOut{
		DetailID: result.RecordID, EmployeeID: result.EmployeeID, Commission: result.CommissionAmount,
		Note: map[bool]string{true: "已通过：汇总+1单、提成入账", false: "已驳回"}[in.Approved],
	}, nil
}

// ---- Server 装配 ----

// NewServer exposes five query/preview tools and two administrative write tools.
func NewServer() *mcp.Server { return newAuthorizedServer(0, mcpaccess.Resolve) }

func newAuthorizedServer(credentialID int64, resolve grantResolver) *mcp.Server {
	version := os.Getenv("CC4_RELEASE_SHA")
	if version == "" {
		version = "dev"
	}
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "cc4-callcenter",
		Version: version,
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
		Name:        "performance_submit",
		Description: "代录业绩（写操作，管理员/代录通道语义）：直接计入汇总表并按现有规则计算提成。orders>0；有 productId 走产品分档×当日序号，否则走 10/20/30 日汇总公式。所有调用写审计。",
	}, performanceSubmit)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "performance_approve",
		Description: "审核员工自录订单明细（写操作）：approved=true 时明细转 approved、汇总+1单、按模型二补提成；false 驳回须填原因。明细须处于 pending_approval 状态。",
	}, performanceApprove)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "workday_get",
		Description: "查询月度工作日配置（只读）：所有『X个工作日』规则的基准。未配置月份回退周一至周五并明确提示。",
	}, workdayGet)

	installAuthorization(server, credentialID, resolve)
	return server
}

// ServeStdio 以 stdio 传输运行（本地 CLI agent 接入方式）。
// stdout 是协议通道，任何日志不得写 stdout。
func ServeStdio() error {
	ctx := context.Background()
	grant, err := mcpaccess.Authenticate(ctx, os.Getenv("CC4_MCP_TOKEN"))
	if err != nil {
		return err
	}
	return newAuthorizedServer(grant.Credential.ID, mcpaccess.Resolve).Run(ctx, &mcp.StdioTransport{})
}
