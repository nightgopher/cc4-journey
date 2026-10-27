package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nightgopher/cc4-journey/app/internal/domain"
)

// ---- 产品提成规则（模型二依赖）----

// CommissionTier 对齐 products.commission_rules 的 tiers 数组项。
type CommissionTier struct {
	From   int  `json:"from"`
	To     *int `json:"to"` // null = 无上限
	Amount int  `json:"amount"`
}

// ProductOrdersOnDate 员工某日某产品已有订单数（当日序号起点）。
// 无记录返回 0（对齐旧 currentCountResult.rows.length > 0 检查）。
func ProductOrdersOnDate(ctx context.Context, employeeID int, workDate string, productID int) (int, error) {
	var n int
	err := Pool.QueryRow(ctx, `
		SELECT COALESCE(orders_count, 0) FROM daily_performance_records
		WHERE employee_id = $1 AND work_date = $2 AND product_id = $3`,
		employeeID, workDate, productID).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return n, nil
}

// ProductCommissionTiers 解析产品 commission_rules JSON。
// 无规则/空规则返回空切片（提成 0，对齐旧 calculateCommissionByProduct）。
func ProductCommissionTiers(ctx context.Context, productID int) ([]CommissionTier, error) {
	var raw []byte
	err := Pool.QueryRow(ctx,
		`SELECT COALESCE(commission_rules, 'null'::jsonb) FROM products WHERE id = $1`,
		productID).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Tiers []CommissionTier `json:"tiers"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, nil // 规则格式错误 → 视为无规则（对齐旧行为：warn + 0 元）
	}
	return wrapper.Tiers, nil
}

// ProductPrice 产品单价（销售额 = price × orders）。
func ProductPrice(ctx context.Context, productID int) (float64, error) {
	var price float64
	err := Pool.QueryRow(ctx, `SELECT price FROM products WHERE id = $1`, productID).Scan(&price)
	return price, err
}

// ---- 业绩汇总 upsert（管理员/代录通道）----

// DailyPerformanceUpsert 录入参数。
type DailyPerformanceUpsert struct {
	EmployeeID       int
	WorkDate         string
	ProductID        *int
	OrdersCount      int
	SalesAmount      float64
	CommissionAmount float64
	WorkPeriod       string
}

// UpsertDailyPerformance 对齐旧 ON CONFLICT 累加语义：
// 同一员工同日同产品再次录入时，订单/销售额/提成做加法合并。
func UpsertDailyPerformance(ctx context.Context, u *DailyPerformanceUpsert) (*DailyPerformance, error) {
	return accumulatePerformance(ctx, u, false)
}

// RecordDailyOrders serializes the ordinal calculation and write for one employee/day/product.
// IS NOT DISTINCT FROM also merges the legacy model where product_id is NULL.
func RecordDailyOrders(ctx context.Context, u *DailyPerformanceUpsert) (*DailyPerformance, error) {
	return accumulatePerformance(ctx, u, true)
}

func lockPerformance(ctx context.Context, tx pgx.Tx, employeeID int, date string, productID *int) error {
	product := 0
	if productID != nil {
		product = *productID
	}
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("performance:%d:%s:%d", employeeID, date, product))
	return err
}

func accumulatePerformance(ctx context.Context, u *DailyPerformanceUpsert, calculate bool) (*DailyPerformance, error) {
	tx, err := Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = lockPerformance(ctx, tx, u.EmployeeID, u.WorkDate, u.ProductID); err != nil {
		return nil, err
	}
	var id, existing int
	err = tx.QueryRow(ctx, `SELECT id,COALESCE(orders_count,0) FROM daily_performance_records WHERE employee_id=$1 AND work_date=$2 AND product_id IS NOT DISTINCT FROM $3::int ORDER BY id LIMIT 1 FOR UPDATE`, u.EmployeeID, u.WorkDate, u.ProductID).Scan(&id, &existing)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if calculate {
		if u.ProductID == nil {
			u.CommissionAmount = float64(domain.DailyCommission(existing+u.OrdersCount) - domain.DailyCommission(existing))
		} else {
			var raw []byte
			var price float64
			if err = tx.QueryRow(ctx, `SELECT COALESCE(commission_rules,'null'::jsonb),price FROM products WHERE id=$1 AND status='active'`, *u.ProductID).Scan(&raw, &price); err != nil {
				return nil, err
			}
			var rules struct {
				Tiers []CommissionTier `json:"tiers"`
			}
			if err = json.Unmarshal(raw, &rules); err != nil {
				return nil, err
			}
			u.CommissionAmount = 0
			for n := 1; n <= u.OrdersCount; n++ {
				u.CommissionAmount += float64(CommissionForIndex(rules.Tiers, existing+n))
			}
			u.SalesAmount = price * float64(u.OrdersCount)
		}
	}
	if u.WorkPeriod == "" {
		u.WorkPeriod = "full_day"
	}
	var row pgx.Row
	const returning = ` RETURNING id,employee_id,TO_CHAR(work_date,'YYYY-MM-DD'),COALESCE(orders_count,0),COALESCE(sales_amount,0),COALESCE(commission_amount,0),product_id`
	if id == 0 {
		row = tx.QueryRow(ctx, `INSERT INTO daily_performance_records(employee_id,work_date,product_id,orders_count,sales_amount,commission_amount,work_period) VALUES($1,$2,$3,$4,$5,$6,$7)`+returning, u.EmployeeID, u.WorkDate, u.ProductID, u.OrdersCount, u.SalesAmount, u.CommissionAmount, u.WorkPeriod)
	} else {
		row = tx.QueryRow(ctx, `UPDATE daily_performance_records SET orders_count=COALESCE(orders_count,0)+$2,sales_amount=COALESCE(sales_amount,0)+$3,commission_amount=COALESCE(commission_amount,0)+$4,work_period=$5,updated_at=CURRENT_TIMESTAMP WHERE id=$1`+returning, id, u.OrdersCount, u.SalesAmount, u.CommissionAmount, u.WorkPeriod)
	}
	var d DailyPerformance
	if err = row.Scan(&d.ID, &d.EmployeeID, &d.WorkDate, &d.OrdersCount, &d.SalesAmount, &d.CommissionAmount, &d.ProductID); err != nil {
		return nil, err
	}
	if err = syncDailyFinance(ctx, tx, &d); err != nil {
		return nil, err
	}
	if err = syncPerformanceAttendance(ctx, tx, u.EmployeeID, u.WorkDate, u.WorkPeriod); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &d, nil
}

// ---- 员工自录明细审核 ----

// OrderDetailStatus 查询明细当前状态与归属。
type OrderDetailStatus struct {
	ID             int    `json:"id"`
	PerformanceID  int    `json:"performance_record_id"`
	EmployeeID     int    `json:"employee_id"`
	WorkDate       string `json:"work_date"`
	ProductID      *int   `json:"product_id"`
	ApprovalStatus string `json:"approval_status"`
}

// GetOrderDetail 读明细行（审核前置检查）。
func GetOrderDetail(ctx context.Context, detailID int) (*OrderDetailStatus, error) {
	row := Pool.QueryRow(ctx, `
		SELECT od.id, od.performance_record_id, od.employee_id,
		       TO_CHAR(od.work_date, 'YYYY-MM-DD'), od.product_id, od.approval_status
		FROM daily_performance_order_details od WHERE od.id = $1`, detailID)
	var o OrderDetailStatus
	err := row.Scan(&o.ID, &o.PerformanceID, &o.EmployeeID, &o.WorkDate, &o.ProductID, &o.ApprovalStatus)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// CommissionForIndex 模型二：按订单序号匹配产品档位。
// from<=idx<=to 或 to=null 无上限；未匹配 → 0 元（对齐旧 calculateCommissionByProduct）。
func CommissionForIndex(tiers []CommissionTier, index int) int {
	if index < 1 {
		return 0
	}
	for _, t := range tiers {
		if index >= t.From && (t.To == nil || index <= *t.To) {
			return t.Amount
		}
	}
	return 0
}

// ApproveDetailResult 审核结果（repo 层自有结构，service 层映射）。
type ApproveDetailResult struct {
	DetailID         int     `json:"detailId"`
	EmployeeID       int     `json:"employeeId"`
	WorkDate         string  `json:"workDate"`
	OrdersCount      int     `json:"ordersCount"`
	SalesAmount      float64 `json:"salesAmount"`
	CommissionAmount float64 `json:"commissionAmount"`
}

// ApproveOrderDetail 单事务完成审核：
// 明细状态翻转 → 汇总 +1 单/加销售额/加提成（模型二按序号）→ 返回结果。
// 幂等保护：明细非 pending_approval 状态时报错（对齐旧"审批请求已处理"）。
func ApproveOrderDetail(ctx context.Context, detailID int, approved bool, rejectionReason string, approverEmployeeID int) (*ApproveDetailResult, error) {
	tx, err := Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// 1. 锁定并检查明细。
	var perfID, empID int
	var workDate time.Time
	var productID *int
	var status string
	err = tx.QueryRow(ctx, `
		SELECT performance_record_id, employee_id, work_date, product_id, approval_status
		FROM daily_performance_order_details WHERE id = $1 FOR UPDATE`, detailID).
		Scan(&perfID, &empID, &workDate, &productID, &status)
	if err != nil {
		return nil, fmtErr("明细不存在", err)
	}
	if status != "pending_approval" {
		return nil, fmt.Errorf("该明细已处理（当前状态 %s），不可重复审核", status)
	}
	if err = lockPerformance(ctx, tx, empID, workDate.Format("2006-01-02"), productID); err != nil {
		return nil, err
	}

	newStatus := "rejected"
	if approved {
		newStatus = "approved"
	}

	// 2. 翻转明细状态。
	if approved {
		// approverEmployeeID=0（MCP 通道无真实审批人）→ approved_by 置 NULL（外键约束）；
		// 操作者身份由 agent_tool_audit 留痕。
		_, err = tx.Exec(ctx, `
			UPDATE daily_performance_order_details
			SET approval_status = $2, approved_at = CURRENT_TIMESTAMP,approved_by=NULLIF($3,0),updated_at=CURRENT_TIMESTAMP
			WHERE id = $1`, detailID, newStatus, approverEmployeeID)
	} else {
		if rejectionReason == "" {
			return nil, fmt.Errorf("驳回必须填写原因")
		}
		_, err = tx.Exec(ctx, `
			UPDATE daily_performance_order_details
			SET approval_status = $2, rejection_reason = $3, approved_at = CURRENT_TIMESTAMP,approved_by=NULLIF($4,0),updated_at=CURRENT_TIMESTAMP
			WHERE id = $1`, detailID, newStatus, rejectionReason, approverEmployeeID)
	}
	if err != nil {
		return nil, err
	}

	result := &ApproveDetailResult{
		DetailID: detailID, EmployeeID: empID,
		WorkDate: workDate.Format("2006-01-02"),
	}

	// 3. 通过时：汇总行 +1 单 + 提成（模型二按当日序号）。
	if approved {
		var existing int
		err = tx.QueryRow(ctx, `
			SELECT COALESCE(orders_count, 0) FROM daily_performance_records
			WHERE id = $1 FOR UPDATE`, perfID).Scan(&existing)
		if err != nil {
			return nil, err
		}

		var commission float64
		var price float64
		if productID != nil {
			var raw []byte
			if err = tx.QueryRow(ctx, `SELECT COALESCE(commission_rules,'null'::jsonb),price FROM products WHERE id=$1`, *productID).Scan(&raw, &price); err != nil {
				return nil, err
			}
			var rules struct {
				Tiers []CommissionTier `json:"tiers"`
			}
			if err = json.Unmarshal(raw, &rules); err != nil {
				return nil, err
			}
			commission = float64(CommissionForIndex(rules.Tiers, existing+1))
		} else {
			commission = float64(domain.DailyCommission(existing+1) - domain.DailyCommission(existing))
		}

		if _, err = tx.Exec(ctx, `
			UPDATE daily_performance_records
			SET orders_count = COALESCE(orders_count,0) + 1,
			    sales_amount = COALESCE(sales_amount,0) + $2,
			    commission_amount = COALESCE(commission_amount,0) + $3,
			    updated_at = CURRENT_TIMESTAMP
			WHERE id = $1`, perfID, price, commission); err != nil {
			return nil, err
		}
		result.CommissionAmount = commission
		result.OrdersCount = existing + 1
		result.SalesAmount = price
		var d DailyPerformance
		if err = tx.QueryRow(ctx, `SELECT id,employee_id,TO_CHAR(work_date,'YYYY-MM-DD'),orders_count,sales_amount,commission_amount,product_id FROM daily_performance_records WHERE id=$1`, perfID).Scan(&d.ID, &d.EmployeeID, &d.WorkDate, &d.OrdersCount, &d.SalesAmount, &d.CommissionAmount, &d.ProductID); err != nil {
			return nil, err
		}
		if err = syncDailyFinance(ctx, tx, &d); err != nil {
			return nil, err
		}
		if err = syncPerformanceAttendance(ctx, tx, empID, d.WorkDate, ""); err != nil {
			return nil, err
		}
	}

	// 4. 通知（对齐旧 notifyPerformanceApproved；notifications.user_id 关联 users；
	//    失败不阻塞主流程，与旧代码的容错一致）。
	_, err = tx.Exec(ctx, `
		INSERT INTO notifications (user_id, title, message, type)
		SELECT e.user_id, $2, $3, 'info' FROM employees e WHERE e.id = $1 AND e.user_id IS NOT NULL`,
		empID,
		map[bool]string{true: "业绩审核通过", false: "业绩被驳回"}[approved],
		map[bool]string{true: "您提交的业绩已审核通过", false: "业绩被驳回：" + rejectionReason}[approved])
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}
