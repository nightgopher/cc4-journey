package repo

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
)

type FinanceQuery struct {
	TeamID                                     *int
	Type, StartDate, EndDate, Category, Status string
	Limit, Offset                              int
}

const financeProjection = `fr.*, fr.reference_id AS invoice_number,fr.category AS income_type,
	TO_CHAR(fr.transaction_date,'YYYY-MM-DD') AS due_date,
	COALESCE(fr.received_amount,0) AS paid_amount,
	COALESCE(fr.remaining_amount,fr.amount) AS outstanding_amount,
	fr.customer_name AS customer, fr.description AS notes, e.name AS recorder,
	CASE WHEN fr.type!='receivable' THEN fr.status WHEN COALESCE(fr.received_amount,0)>=fr.amount THEN 'completed'
	WHEN COALESCE(fr.received_amount,0)>0 THEN 'partial' ELSE 'pending' END AS payment_status`

func ListFinance(ctx context.Context, q FinanceQuery) ([]Object, int, error) {
	args := []any{q.Type}
	where := `fr.type=$1`
	add := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	if q.TeamID != nil {
		where += ` AND fr.recorded_by IN(SELECT id FROM employees WHERE team_id=` + add(*q.TeamID) + `)`
	}
	if q.StartDate != "" {
		where += ` AND fr.transaction_date>=` + add(q.StartDate) + `::date`
	}
	if q.EndDate != "" {
		where += ` AND fr.transaction_date<=` + add(q.EndDate) + `::date`
	}
	if q.Category != "" {
		where += ` AND fr.category=` + add(q.Category)
	}
	if q.Status == "partial" {
		where += ` AND COALESCE(fr.received_amount,0)>0 AND COALESCE(fr.received_amount,0)<fr.amount`
	} else if q.Status == "completed" {
		where += ` AND COALESCE(fr.received_amount,0)>=fr.amount`
	} else if q.Status != "" {
		where += ` AND fr.status=` + add(q.Status)
	}
	var total int
	if err := Pool.QueryRow(ctx, `SELECT COUNT(*) FROM finance_records fr WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := Objects(ctx, Pool, `SELECT `+financeProjection+` FROM finance_records fr LEFT JOIN employees e ON e.id=fr.recorded_by WHERE `+where+` ORDER BY fr.transaction_date DESC,fr.id DESC LIMIT `+add(q.Limit)+` OFFSET `+add(q.Offset), args...)
	return rows, total, err
}

func GetReceivable(ctx context.Context, id int) (Object, error) {
	return One(ctx, Pool, `SELECT `+financeProjection+` FROM finance_records fr LEFT JOIN employees e ON e.id=fr.recorded_by WHERE fr.id=$1 AND fr.type='receivable'`, id)
}

type FinanceInput struct {
	Category           string  `json:"category"`
	Amount             float64 `json:"amount"`
	TransactionDate    string  `json:"transactionDate"`
	Description        string  `json:"description"`
	InvoiceNumber      string  `json:"invoiceNumber"`
	CustomerName       string  `json:"customerName"`
	ProductName        string  `json:"productName"`
	OrderCount         int     `json:"orderCount"`
	CommissionPerOrder float64 `json:"commissionPerOrder"`
	CostTypeID         *int    `json:"costTypeId"`
	CostType           string  `json:"costType"`
	TotalAmount        float64 `json:"totalAmount"`
	Frequency          string  `json:"frequency"`
	StartDate          string  `json:"startDate"`
	EndDate            string  `json:"endDate"`
	AccountTitle       string  `json:"accountTitle"`
}

func ValidMoney(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v > 0 && v < 1e10 && math.Abs(v*100-math.Round(v*100)) < 0.0001
}

func CreateFinance(ctx context.Context, typ string, in FinanceInput, recorder *int) (Object, error) {
	if in.CostType == "" {
		in.CostType = "onetime"
	}
	if in.Category == "" {
		in.Category = map[string]string{"income": "sales", "expense": "other", "receivable": "receivable"}[typ]
	}
	status := "confirmed"
	if typ == "receivable" {
		status = "pending"
	}
	if in.InvoiceNumber == "" {
		in.InvoiceNumber = fmt.Sprintf("%s-%d", typ, time.Now().UnixNano())
	}
	var daily *float64
	if in.CostType == "recurring" {
		start, err := time.Parse("2006-01-02", in.StartDate)
		if err != nil {
			return nil, fmt.Errorf("周期起始日期无效")
		}
		end := start.AddDate(1, 0, 0)
		if in.EndDate != "" {
			end, err = time.Parse("2006-01-02", in.EndDate)
			if err != nil {
				return nil, fmt.Errorf("周期结束日期无效")
			}
		}
		if !end.After(start) || !ValidMoney(in.TotalAmount) {
			return nil, fmt.Errorf("周期金额和起止日期无效")
		}
		days := end.Sub(start).Hours() / 24
		switch in.Frequency {
		case "monthly":
			days = 30
		case "yearly":
			days = 365
		case "daily":
		default:
			return nil, fmt.Errorf("无效的周期类型")
		}
		v := math.Round(in.TotalAmount/days*100) / 100
		if v <= 0 {
			return nil, fmt.Errorf("每日均摊金额须至少为 0.01")
		}
		daily = &v
		in.Amount = v
		in.TransactionDate = in.StartDate
	}
	if !ValidMoney(in.Amount) {
		return nil, fmt.Errorf("金额须为大于 0 且最多两位小数的数字")
	}
	if _, err := time.Parse("2006-01-02", in.TransactionDate); err != nil {
		return nil, fmt.Errorf("交易日期格式须为 YYYY-MM-DD")
	}
	var raw []byte
	err := Pool.QueryRow(ctx, `INSERT INTO finance_records(type,category,amount,transaction_date,description,reference_id,reference_type,
		recorded_by,status,customer_name,product_name,order_count,commission_per_order,cost_type_id,cost_type,
		recurrence_frequency,recurrence_start_date,recurrence_end_date,total_amount,amortized_daily,
		total_receivable,received_amount,remaining_amount,account_title,source)
		VALUES($1::text,$2,$3::numeric,$4::text::date,$5,$6,$1::text,$7,$8,$9,$10,$11,$12,$13,$14,NULLIF($15,''),NULLIF($16::text,'')::date,
		NULLIF($17::text,'')::date,NULLIF($18::numeric,0),$19,CASE WHEN $1::text='receivable' THEN $3::numeric ELSE NULL END,0,
		CASE WHEN $1::text='receivable' THEN $3::numeric ELSE NULL END,$20,'manual') RETURNING to_jsonb(finance_records.*)`,
		typ, in.Category, in.Amount, in.TransactionDate, in.Description, in.InvoiceNumber, recorder, status, in.CustomerName, in.ProductName,
		in.OrderCount, in.CommissionPerOrder, in.CostTypeID, in.CostType, in.Frequency, in.StartDate, in.EndDate, in.TotalAmount, daily, in.AccountTitle).Scan(&raw)
	if err != nil {
		return nil, err
	}
	return decodeObject(raw)
}

var FinanceFields = map[string]string{"category": "", "amount": "", "transaction_date": "date", "description": "", "reference_id": "",
	"customer_name": "", "product_name": "", "order_count": "", "commission_per_order": "", "cost_type_id": "", "cost_type": "",
	"recurrence_frequency": "", "recurrence_start_date": "date", "recurrence_end_date": "date", "status": ""}

type PaymentInput struct {
	Amount        float64 `json:"amount"`
	PaymentDate   string  `json:"paymentDate"`
	PaymentMethod string  `json:"paymentMethod"`
	Note          string  `json:"note"`
}

// RecordPayment locks the receivable, rejects overpayment, and commits the payment,
// receivable balances and matching income together. No unsupported status is written.
func RecordPayment(ctx context.Context, id int, in PaymentInput, recorder *int) (Object, error) {
	if !ValidMoney(in.Amount) {
		return nil, fmt.Errorf("回款金额须为大于 0 且最多两位小数的数字")
	}
	if _, err := time.Parse("2006-01-02", in.PaymentDate); err != nil {
		return nil, fmt.Errorf("回款日期格式须为 YYYY-MM-DD")
	}
	tx, err := Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var remaining float64
	var title, customer, status string
	err = tx.QueryRow(ctx, `SELECT amount-COALESCE(received_amount,0),COALESCE(account_title,description,''),COALESCE(customer_name,''),status
		FROM finance_records WHERE id=$1 AND type='receivable' FOR UPDATE`, id).Scan(&remaining, &title, &customer, &status)
	if err != nil {
		return nil, err
	}
	if status == "cancelled" {
		return nil, fmt.Errorf("已取消的应收款不能回款")
	}
	if math.Round(in.Amount*100) > math.Round(remaining*100) {
		return nil, fmt.Errorf("回款金额不能超过剩余应收 %.2f", remaining)
	}
	var paymentID int
	err = tx.QueryRow(ctx, `INSERT INTO payment_records(receivable_id,amount,payment_date,payment_method,note,recorded_by)
		VALUES($1,$2,$3::text::date,$4,$5,$6) RETURNING id`, id, in.Amount, in.PaymentDate, in.PaymentMethod, in.Note, recorder).Scan(&paymentID)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE finance_records SET received_amount=COALESCE(received_amount,0)+$2::numeric,
		remaining_amount=amount-COALESCE(received_amount,0)-$2::numeric,
		status=CASE WHEN amount-COALESCE(received_amount,0)-$2::numeric=0 THEN 'confirmed' ELSE 'pending' END,
		updated_at=CURRENT_TIMESTAMP WHERE id=$1`, id, in.Amount)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO finance_records(type,category,amount,transaction_date,description,status,reference_type,reference_id,customer_name,recorded_by,source)
		VALUES('income','receivable_payment',$1,$2::text::date,$3,'confirmed','receivable_payment',$4,$5,$6,'auto')`,
		in.Amount, in.PaymentDate, "回款: "+title+" "+in.Note, fmt.Sprintf("payment-%d", paymentID), customer, recorder)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return GetReceivable(ctx, id)
}

func syncDailyFinance(ctx context.Context, tx pgx.Tx, d *DailyPerformance) error {
	product := "null"
	if d.ProductID != nil {
		product = itoa(*d.ProductID)
	}
	ref := fmt.Sprintf("daily-%d-%s-%s", d.EmployeeID, d.WorkDate, product)
	if _, err := tx.Exec(ctx, `DELETE FROM finance_records WHERE reference_type='daily_performance' AND reference_id=$1`, ref); err != nil {
		return err
	}
	if d.SalesAmount <= 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `INSERT INTO finance_records(type,category,amount,transaction_date,description,status,reference_type,reference_id,product_name,order_count,recorded_by,source)
		SELECT 'income','sales',$1,$2::text::date,e.name||' - '||COALESCE(p.name,'日汇总'),'confirmed','daily_performance',$3,p.name,$4,e.id,'auto'
		FROM employees e LEFT JOIN products p ON p.id=$5 WHERE e.id=$6`, d.SalesAmount, d.WorkDate, ref, d.OrdersCount, d.ProductID, d.EmployeeID)
	return err
}

func SyncMonthlyIncome(ctx context.Context, year, month int, teamID *int) (float64, error) {
	tx, err := Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	ref := fmt.Sprintf("perf_summary_%d_%d", year, month)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, ref); err != nil {
		return 0, err
	}
	// Daily sales are already booked. Synchronize their rows, rather than inserting a
	// second monthly income on top of the same daily revenue.
	rows, err := tx.Query(ctx, `SELECT id,employee_id,TO_CHAR(work_date,'YYYY-MM-DD'),COALESCE(orders_count,0),COALESCE(sales_amount,0),product_id FROM daily_performance_records WHERE EXTRACT(YEAR FROM work_date)=$1 AND EXTRACT(MONTH FROM work_date)=$2 AND($3::int IS NULL OR employee_id IN(SELECT id FROM employees WHERE team_id=$3))`, year, month, teamID)
	if err != nil {
		return 0, err
	}
	data := []DailyPerformance{}
	for rows.Next() {
		var d DailyPerformance
		if err = rows.Scan(&d.ID, &d.EmployeeID, &d.WorkDate, &d.OrdersCount, &d.SalesAmount, &d.ProductID); err != nil {
			rows.Close()
			return 0, err
		}
		data = append(data, d)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	var total float64
	for _, d := range data {
		if err = lockPerformance(ctx, tx, d.EmployeeID, d.WorkDate, d.ProductID); err != nil {
			return 0, err
		}
		if err = tx.QueryRow(ctx, `SELECT COALESCE(orders_count,0),COALESCE(sales_amount,0) FROM daily_performance_records WHERE id=$1 FOR UPDATE`, d.ID).Scan(&d.OrdersCount, &d.SalesAmount); err != nil {
			return 0, err
		}
		if err = syncDailyFinance(ctx, tx, &d); err != nil {
			return 0, err
		}
		total += d.SalesAmount
	}
	return total, tx.Commit(ctx)
}
