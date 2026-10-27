package repo

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/nightgopher/cc4-journey/app/internal/domain"
)

type OrderSubmission struct {
	EmployeeID               int    `json:"employeeId"`
	WorkDate                 string `json:"workDate"`
	ProductID                *int   `json:"productId"`
	WorkPeriod               string `json:"workPeriod"`
	ParentPhone              string `json:"parentPhone"`
	OrderID                  string `json:"orderId"`
	Notes                    string `json:"notes"`
	ParentAddedTeacherWechat bool   `json:"parentAddedTeacherWechat"`
	PayURL                   string `json:"wechatPayScreenshotUrl"`
	ChatURL                  string `json:"wechatChatScreenshotUrl"`
	ContactURL               string `json:"wechatContactScreenshotUrl"`
}

func SubmitOrder(ctx context.Context, in OrderSubmission) (Object, error) {
	tx, err := Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = lockPerformance(ctx, tx, in.EmployeeID, in.WorkDate, in.ProductID); err != nil {
		return nil, err
	}
	var perfID int
	err = tx.QueryRow(ctx, `SELECT id FROM daily_performance_records WHERE employee_id=$1 AND work_date=$2 AND product_id IS NOT DISTINCT FROM $3::int ORDER BY id LIMIT 1 FOR UPDATE`, in.EmployeeID, in.WorkDate, in.ProductID).Scan(&perfID)
	if err == pgx.ErrNoRows {
		err = tx.QueryRow(ctx, `INSERT INTO daily_performance_records(employee_id,work_date,product_id,orders_count,sales_amount,commission_amount,work_period)VALUES($1,$2,$3,0,0,0,$4)RETURNING id`, in.EmployeeID, in.WorkDate, in.ProductID, in.WorkPeriod).Scan(&perfID)
	}
	if err != nil {
		return nil, err
	}
	var raw []byte
	err = tx.QueryRow(ctx, `INSERT INTO daily_performance_order_details(performance_record_id,employee_id,work_date,product_id,parent_phone,order_id,notes,parent_added_teacher_wechat,wechat_pay_screenshot_url,wechat_chat_screenshot_url,wechat_contact_screenshot_url,approval_status,submitted_by)
		VALUES($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8,$9,$10,$11,'pending_approval',$2)RETURNING to_jsonb(daily_performance_order_details.*)`, perfID, in.EmployeeID, in.WorkDate, in.ProductID, in.ParentPhone, in.OrderID, in.Notes, in.ParentAddedTeacherWechat, in.PayURL, in.ChatURL, in.ContactURL).Scan(&raw)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return decodeObject(raw)
}

func DeleteOrderDetail(ctx context.Context, id int) error {
	tx, err := Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var perfID, empID int
	var date, status string
	var productID *int
	err = tx.QueryRow(ctx, `SELECT performance_record_id,employee_id,TO_CHAR(work_date,'YYYY-MM-DD'),product_id,approval_status FROM daily_performance_order_details WHERE id=$1 FOR UPDATE`, id).Scan(&perfID, &empID, &date, &productID, &status)
	if err != nil {
		return err
	}
	if err = lockPerformance(ctx, tx, empID, date, productID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM daily_performance_order_details WHERE id=$1`, id); err != nil {
		return err
	}
	if status == "approved" {
		var count int
		var sales float64
		if err = tx.QueryRow(ctx, `SELECT GREATEST(COALESCE(orders_count,0)-1,0),COALESCE(sales_amount,0) FROM daily_performance_records WHERE id=$1 FOR UPDATE`, perfID).Scan(&count, &sales); err != nil {
			return err
		}
		commission := float64(domain.DailyCommission(count))
		if productID != nil {
			tiers, err := ProductCommissionTiers(ctx, *productID)
			if err != nil {
				return err
			}
			price, err := ProductPrice(ctx, *productID)
			if err != nil {
				return err
			}
			sales -= price
			commission = 0
			for n := 1; n <= count; n++ {
				commission += float64(CommissionForIndex(tiers, n))
			}
		}
		if sales < 0 {
			sales = 0
		}
		if _, err = tx.Exec(ctx, `UPDATE daily_performance_records SET orders_count=$2,sales_amount=$3,commission_amount=$4,updated_at=CURRENT_TIMESTAMP WHERE id=$1`, perfID, count, sales, commission); err != nil {
			return err
		}
		d := DailyPerformance{ID: perfID, EmployeeID: empID, WorkDate: date, ProductID: productID, OrdersCount: count, SalesAmount: sales, CommissionAmount: commission}
		if err = syncDailyFinance(ctx, tx, &d); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func UpdatePerformance(ctx context.Context, id, orders int, period string) error {
	if orders < 0 {
		return fmt.Errorf("订单数不能为负数")
	}
	tx, err := Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var d DailyPerformance
	if err = tx.QueryRow(ctx, `SELECT id,employee_id,TO_CHAR(work_date,'YYYY-MM-DD'),product_id FROM daily_performance_records WHERE id=$1`, id).Scan(&d.ID, &d.EmployeeID, &d.WorkDate, &d.ProductID); err != nil {
		return err
	}
	if err = lockPerformance(ctx, tx, d.EmployeeID, d.WorkDate, d.ProductID); err != nil {
		return err
	}
	if err = tx.QueryRow(ctx, `SELECT COALESCE(sales_amount,0) FROM daily_performance_records WHERE id=$1 FOR UPDATE`, id).Scan(&d.SalesAmount); err != nil {
		return err
	}
	var details int
	if err = tx.QueryRow(ctx, `SELECT COUNT(*) FROM daily_performance_order_details WHERE performance_record_id=$1`, id).Scan(&details); err != nil {
		return err
	}
	if details > 0 {
		return fmt.Errorf("存在订单明细，请通过明细审核或删除调整业绩")
	}
	d.OrdersCount = orders
	d.CommissionAmount = float64(domain.DailyCommission(orders))
	if d.ProductID != nil {
		tiers, err := ProductCommissionTiers(ctx, *d.ProductID)
		if err != nil {
			return err
		}
		price, err := ProductPrice(ctx, *d.ProductID)
		if err != nil {
			return err
		}
		d.SalesAmount = price * float64(orders)
		d.CommissionAmount = 0
		for n := 1; n <= orders; n++ {
			d.CommissionAmount += float64(CommissionForIndex(tiers, n))
		}
	}
	if period == "" {
		period = "full_day"
	}
	if _, err = tx.Exec(ctx, `UPDATE daily_performance_records SET orders_count=$2,sales_amount=$3,commission_amount=$4,work_period=$5,updated_at=CURRENT_TIMESTAMP WHERE id=$1`, id, orders, d.SalesAmount, d.CommissionAmount, period); err != nil {
		return err
	}
	if err = syncDailyFinance(ctx, tx, &d); err != nil {
		return err
	}
	if err = syncPerformanceAttendance(ctx, tx, d.EmployeeID, d.WorkDate, period); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
