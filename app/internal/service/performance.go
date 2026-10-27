package service

import (
	"context"
	"fmt"
	"time"

	"github.com/nightgopher/cc4-journey/app/internal/repo"
)

// ---- 业绩录入与审核（对齐存量 routes/dailyPerformance.js 管理员/代录路径）----

// SubmitPerformanceInput 代录业绩输入（对齐旧 POST /daily-performance 管理员分支）。
type SubmitPerformanceInput struct {
	EmployeeID  int     `json:"employeeId"`
	WorkDate    string  `json:"workDate"` // YYYY-MM-DD
	ProductID   *int    `json:"productId,omitempty"`
	Orders      int     `json:"orders"`
	SalesAmount float64 `json:"salesAmount,omitempty"` // 无 productId 时使用
	WorkPeriod  string  `json:"workPeriod,omitempty"`  // 缺省 full_day
	SubmittedBy int     `json:"submittedBy"`           // 操作者员工ID（审计）
	Channel     string  `json:"channel,omitempty"`     // "api" | "mcp"（审计）
}

// SubmitPerformanceResult 录入结果。
type SubmitPerformanceResult struct {
	RecordID         int     `json:"recordId"`
	EmployeeID       int     `json:"employeeId"`
	WorkDate         string  `json:"workDate"`
	OrdersCount      int     `json:"ordersCount"`
	SalesAmount      float64 `json:"salesAmount"`
	CommissionAmount float64 `json:"commissionAmount"`
	NeedsApproval    bool    `json:"needsApproval"`
	Note             string  `json:"note,omitempty"`
}

// SubmitPerformance 代录业绩（管理员/经理通道，语义对齐旧代码）：
//   - 无 productId：commission = domain.DailyCommission(orders)（模型一，旧 :526 行为）
//   - 有 productId：commission = Σ 产品分档×当日序号（模型二，旧 :466-474 行为）
//   - products 表为空/无规则时：模型二提成为 0（旧 calculateCommissionByProduct 行为）
//   - ON CONFLICT 累加（同一员工同日同产品多次录入合并）
func SubmitPerformance(ctx context.Context, in SubmitPerformanceInput) (*SubmitPerformanceResult, error) {
	if in.Orders < 0 || in.Orders > 10000 {
		return nil, fmt.Errorf("orders 须在 0–10000 之间")
	}
	if _, err := time.Parse("2006-01-02", in.WorkDate); err != nil {
		return nil, fmt.Errorf("workDate 格式须为 YYYY-MM-DD")
	}
	emp, err := repo.GetEmployeeByID(ctx, in.EmployeeID)
	if err != nil {
		return nil, fmt.Errorf("员工不存在: %w", err)
	}
	if emp.Status != "active" {
		return nil, fmt.Errorf("仅在职员工可录入业绩")
	}
	if in.WorkPeriod != "" && in.WorkPeriod != "morning" && in.WorkPeriod != "afternoon" && in.WorkPeriod != "full_day" {
		return nil, fmt.Errorf("无效的 workPeriod")
	}
	if in.SalesAmount < 0 {
		return nil, fmt.Errorf("salesAmount 不能为负数")
	}

	workPeriod := in.WorkPeriod
	if workPeriod == "" {
		workPeriod = "full_day"
	}

	rec, err := repo.RecordDailyOrders(ctx, &repo.DailyPerformanceUpsert{
		EmployeeID: in.EmployeeID, WorkDate: in.WorkDate,
		ProductID: in.ProductID, OrdersCount: in.Orders,
		SalesAmount: in.SalesAmount,
		WorkPeriod:  workPeriod,
	})
	if err != nil {
		return nil, err
	}

	channel := in.Channel
	if channel == "" {
		channel = "api"
	}
	repo.WriteAudit(ctx, channel, "performance_submit",
		fmt.Sprintf("employee=%d date=%s orders=%d commission=%.2f by=%d",
			in.EmployeeID, in.WorkDate, in.Orders, rec.CommissionAmount, in.SubmittedBy))

	return &SubmitPerformanceResult{
		RecordID: rec.ID, EmployeeID: in.EmployeeID, WorkDate: in.WorkDate,
		OrdersCount: rec.OrdersCount, SalesAmount: rec.SalesAmount,
		CommissionAmount: rec.CommissionAmount, NeedsApproval: false,
	}, nil
}

// ---- 审核（对齐旧 POST /daily-performance/:id/approve，员工自录通道）----

// ApproveOrderDetail 审核员工自录订单明细：
// approved=true → 明细置 approved、汇总 +1 单、按模型二补提成；false → rejected + 原因。
func ApproveOrderDetail(ctx context.Context, detailID int, approved bool, rejectionReason string, approverEmployeeID int) (*SubmitPerformanceResult, error) {
	res, err := repo.ApproveOrderDetail(ctx, detailID, approved, rejectionReason, approverEmployeeID)
	if err != nil {
		return nil, err
	}
	// repo 结构 → service 结果映射。
	out := &SubmitPerformanceResult{
		RecordID: res.DetailID, EmployeeID: res.EmployeeID, WorkDate: res.WorkDate,
		OrdersCount: res.OrdersCount, SalesAmount: res.SalesAmount,
		CommissionAmount: res.CommissionAmount, NeedsApproval: false,
	}
	action := "approve"
	if !approved {
		action = "reject"
	}
	repo.WriteAudit(ctx, "api", "performance_"+action,
		fmt.Sprintf("detail=%d employee=%d commission=%.2f by=%d",
			detailID, res.EmployeeID, res.CommissionAmount, approverEmployeeID))
	return out, nil
}
