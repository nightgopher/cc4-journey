package repo

import (
	"context"
	"time"
)

// ---- 线索 leads（真实表结构：customer_name/phone/priority/estimated_value/next_follow_up）----

// Lead 对齐 leads 表。
type Lead struct {
	ID             int       `json:"id"`
	CustomerName   string    `json:"customer_name"`
	Phone          string    `json:"phone"`
	Email          *string   `json:"email"`
	Company        *string   `json:"company"`
	Industry       *string   `json:"industry"`
	Source         *string   `json:"source"`
	Status         string    `json:"status"`
	Priority       *string   `json:"priority"`
	AssignedTo     *int      `json:"assigned_to"`
	EstimatedValue *float64  `json:"estimated_value"`
	Probability    *int      `json:"probability"`
	NextFollowUp   *string   `json:"next_follow_up"`
	Notes          *string   `json:"notes"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	AssigneeName   *string   `json:"assignee_name,omitempty"`
}

const leadCols = `l.id, l.customer_name, COALESCE(l.phone,''), l.email, l.company, l.industry, l.source, COALESCE(l.status,'new'),
	l.priority, l.assigned_to, l.estimated_value, l.probability, TO_CHAR(l.next_follow_up,'YYYY-MM-DD'), l.notes, l.created_at, l.updated_at`

// ListLeads 线索列表（状态/负责人过滤）。
func ListLeads(ctx context.Context, status, search string, assignedTo, teamID *int, limit, offset int) ([]Lead, int, error) {
	where := "WHERE 1=1"
	args := []any{}
	add := func(v any) string { args = append(args, v); return itoa(len(args)) }
	if search != "" {
		n := add("%" + search + "%")
		where += ` AND(l.customer_name ILIKE $` + n + ` OR l.phone ILIKE $` + n + `)`
	}
	if status != "" {
		where += ` AND l.status = $` + add(status)
	}
	if assignedTo != nil {
		where += ` AND l.assigned_to = $` + add(*assignedTo)
	}
	if teamID != nil {
		where += ` AND l.assigned_to IN (SELECT id FROM employees WHERE team_id=$` + add(*teamID) + `)`
	}
	var total int
	if err := Pool.QueryRow(ctx, `SELECT COUNT(*) FROM leads l `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := Pool.Query(ctx, `
		SELECT `+leadCols+`, e.name FROM leads l
		LEFT JOIN employees e ON e.id = l.assigned_to `+where+`
		ORDER BY l.created_at DESC LIMIT $`+add(limit)+` OFFSET $`+add(offset), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Lead{}
	for rows.Next() {
		var l Lead
		if err := rows.Scan(&l.ID, &l.CustomerName, &l.Phone, &l.Email, &l.Company, &l.Industry,
			&l.Source, &l.Status, &l.Priority, &l.AssignedTo, &l.EstimatedValue, &l.Probability,
			&l.NextFollowUp, &l.Notes, &l.CreatedAt, &l.UpdatedAt, &l.AssigneeName); err != nil {
			return nil, 0, err
		}
		out = append(out, l)
	}
	return out, total, rows.Err()
}

// CreateLead 新建线索（默认 status='new'）。
func CreateLead(ctx context.Context, l *Lead) (int, error) {
	var id int
	status := l.Status
	if status == "" {
		status = "new"
	}
	err := Pool.QueryRow(ctx, `
		INSERT INTO leads (customer_name, phone, email, company, industry, source, status, priority, assigned_to, estimated_value, probability, next_follow_up, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,COALESCE($8,'medium'),$9,$10,$11,NULLIF($12::text,'')::date,$13) RETURNING id`,
		l.CustomerName, l.Phone, l.Email, l.Company, l.Industry, l.Source, status,
		l.Priority, l.AssignedTo, l.EstimatedValue, l.Probability, l.NextFollowUp, l.Notes).Scan(&id)
	return id, err
}

// UpdateLead 部分更新（白名单字段）。
var leadUpdatable = map[string]bool{"customer_name": true, "phone": true, "email": true, "company": true,
	"industry": true, "source": true, "status": true, "priority": true, "assigned_to": true,
	"estimated_value": true, "probability": true, "next_follow_up": true, "notes": true}

func UpdateLead(ctx context.Context, id int, fields map[string]any) error {
	set, args := "", []any{id}
	i := 1
	for k, v := range fields {
		if !leadUpdatable[k] {
			continue
		}
		if k == "next_follow_up" {
			set += k + " = NULLIF($" + itoa(i+1) + "::text,'')::date, "
		} else {
			set += k + " = $" + itoa(i+1) + ", "
		}
		args = append(args, v)
		i++
	}
	if set == "" {
		return nil
	}
	tag, err := Pool.Exec(ctx, `UPDATE leads SET `+set+` updated_at = CURRENT_TIMESTAMP WHERE id = $1`, args...)
	return affected(tag, err)
}

// DeleteLead 删除线索。
func DeleteLead(ctx context.Context, id int) error {
	tag, err := Pool.Exec(ctx, `DELETE FROM leads WHERE id = $1`, id)
	return affected(tag, err)
}

// AssignLeadsBatch 分配只改变归属，保留跟进状态，兼容数据库的五态约束。
func AssignLeadsBatch(ctx context.Context, leadIDs []int, assigneeID int) (int64, error) {
	tag, err := Pool.Exec(ctx, `
		UPDATE leads SET assigned_to = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = ANY($1) AND EXISTS (SELECT 1 FROM employees WHERE id=$2 AND status='active')`, leadIDs, assigneeID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ---- 客户 customers（真实表结构：customer_type/credit_limit/balance）----

// Customer 对齐 customers 表。
type Customer struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Phone        string    `json:"phone"`
	Email        *string   `json:"email"`
	Company      *string   `json:"company"`
	Address      *string   `json:"address"`
	CustomerType *string   `json:"customer_type"`
	Status       string    `json:"status"`
	CreditLimit  *float64  `json:"credit_limit"`
	Balance      *float64  `json:"balance"`
	CreatedAt    time.Time `json:"created_at"`
}

// ListCustomers 客户列表（搜索）。
func ListCustomers(ctx context.Context, search string, limit, offset int) ([]Customer, int, error) {
	where := "WHERE 1=1"
	args := []any{}
	add := func(v any) string { args = append(args, v); return itoa(len(args)) }
	if search != "" {
		n := add("%" + search + "%")
		where += ` AND (name ILIKE $` + n + ` OR phone ILIKE $` + n + `)`
	}
	var total int
	if err := Pool.QueryRow(ctx, `SELECT COUNT(*) FROM customers `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := Pool.Query(ctx, `
		SELECT id, name, phone, email, company, address, customer_type, status, credit_limit, balance, created_at
		FROM customers `+where+` ORDER BY created_at DESC LIMIT $`+add(limit)+` OFFSET $`+add(offset), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Customer{}
	for rows.Next() {
		var c Customer
		if err := rows.Scan(&c.ID, &c.Name, &c.Phone, &c.Email, &c.Company, &c.Address,
			&c.CustomerType, &c.Status, &c.CreditLimit, &c.Balance, &c.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

// CreateCustomer 新建客户。
func CreateCustomer(ctx context.Context, c *Customer) (int, error) {
	var id int
	err := Pool.QueryRow(ctx, `
		INSERT INTO customers (name, phone, email, company, address, customer_type, status, credit_limit, balance)
		VALUES ($1,$2,$3,$4,$5, COALESCE($6,'individual'), COALESCE(NULLIF($7,''),'active'), COALESCE($8,0), COALESCE($9,0)) RETURNING id`,
		c.Name, c.Phone, c.Email, c.Company, c.Address, c.CustomerType, c.Status, c.CreditLimit, c.Balance).Scan(&id)
	return id, err
}

// UpdateCustomer 部分更新。
var customerUpdatable = map[string]bool{"name": true, "phone": true, "email": true, "company": true,
	"address": true, "customer_type": true, "status": true, "credit_limit": true, "balance": true}

func UpdateCustomer(ctx context.Context, id int, fields map[string]any) error {
	set, args := "", []any{id}
	i := 1
	for k, v := range fields {
		if !customerUpdatable[k] {
			continue
		}
		set += k + " = $" + itoa(i+1) + ", "
		args = append(args, v)
		i++
	}
	if set == "" {
		return nil
	}
	_, err := Pool.Exec(ctx, `UPDATE customers SET `+set+` updated_at = CURRENT_TIMESTAMP WHERE id = $1`, args...)
	return err
}

// DeleteCustomer 删除客户。
func DeleteCustomer(ctx context.Context, id int) error {
	_, err := Pool.Exec(ctx, `DELETE FROM customers WHERE id = $1`, id)
	return err
}

// ---- 客户订单 customer_orders（真实列：order_number/total_amount/employee_id）----

// CustomerOrder 客户订单。
type CustomerOrder struct {
	ID          int     `json:"id"`
	CustomerID  int     `json:"customer_id"`
	EmployeeID  *int    `json:"employee_id"`
	OrderNumber string  `json:"order_number"`
	TotalAmount float64 `json:"total_amount"`
	Status      string  `json:"status"`
	OrderDate   string  `json:"order_date"`
}

// ListCustomerOrders 某客户订单。
func ListCustomerOrders(ctx context.Context, customerID int) ([]CustomerOrder, error) {
	rows, err := Pool.Query(ctx, `
		SELECT id, customer_id, employee_id, order_number, total_amount, status, TO_CHAR(order_date,'YYYY-MM-DD')
		FROM customer_orders WHERE customer_id = $1 ORDER BY order_date DESC`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CustomerOrder{}
	for rows.Next() {
		var o CustomerOrder
		if err := rows.Scan(&o.ID, &o.CustomerID, &o.EmployeeID, &o.OrderNumber, &o.TotalAmount, &o.Status, &o.OrderDate); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// CreateCustomerOrder 新建订单（order_number 自动生成 CO-yyyymmdd-序号）。
func CreateCustomerOrder(ctx context.Context, o *CustomerOrder) (int, error) {
	var id int
	err := Pool.QueryRow(ctx, `
		INSERT INTO customer_orders (customer_id, employee_id, order_number, total_amount, status, order_date)
		VALUES ($1,$2, 'CO-'||TO_CHAR(CURRENT_DATE,'YYYYMMDD')||'-'||NEXTVAL('customer_orders_id_seq')::TEXT, $3, COALESCE(NULLIF($4,''),'pending'), COALESCE(NULLIF($5::text,'')::date, CURRENT_DATE))
		RETURNING id`,
		o.CustomerID, o.EmployeeID, o.TotalAmount, o.Status, o.OrderDate).Scan(&id)
	return id, err
}

// ---- 产品 products（真实列无 provider 单列变更；保持全列）----

// Product 对齐 products 表。
type Product struct {
	ID              int     `json:"id"`
	Name            string  `json:"name"`
	Code            *string `json:"code"`
	Description     *string `json:"description"`
	Price           float64 `json:"price"`
	Category        *string `json:"category"`
	Status          string  `json:"status"`
	CommissionRules *string `json:"commission_rules"`
}

// ListProducts 产品列表。
func ListProducts(ctx context.Context) ([]Product, error) {
	rows, err := Pool.Query(ctx, `
		SELECT id, name, code, description, price, category, status,
		       CAST(commission_rules AS TEXT)
		FROM products ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Product{}
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Code, &p.Description, &p.Price,
			&p.Category, &p.Status, &p.CommissionRules); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// UpsertProduct 新建/更新产品。
func UpsertProduct(ctx context.Context, p *Product) (int, error) {
	if p.Code == nil || *p.Code == "" {
		code := "P" + time.Now().Format("20060102150405.000000000")
		p.Code = &code
	}
	if p.ID > 0 {
		var id int
		err := Pool.QueryRow(ctx, `UPDATE products SET name=$2, code=$3, description=$4, price=$5,
			category=$6, status=COALESCE(NULLIF($7,''),'active'), commission_rules=$8::jsonb,
			updated_at=CURRENT_TIMESTAMP WHERE id=$1 RETURNING id`,
			p.ID, p.Name, p.Code, p.Description, p.Price, p.Category, p.Status, p.CommissionRules).Scan(&id)
		return id, err
	}
	var id int
	err := Pool.QueryRow(ctx, `
		INSERT INTO products (name, code, description, price, category, status, commission_rules)
		VALUES ($1,$2,$3,$4,$5, COALESCE(NULLIF($6,''),'active'), $7::jsonb)
		RETURNING id`,
		p.Name, p.Code, p.Description, p.Price, p.Category, p.Status, p.CommissionRules).Scan(&id)
	return id, err
}

// DeleteProduct 删除产品。
func DeleteProduct(ctx context.Context, id int) error {
	_, err := Pool.Exec(ctx, `DELETE FROM products WHERE id = $1`, id)
	return err
}
