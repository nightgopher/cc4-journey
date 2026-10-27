// Package api exposes the Go business API and preserves the established response envelopes.
package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"

	businessaccess "github.com/nightgopher/cc4-journey/app/internal/access"
	ccmcp "github.com/nightgopher/cc4-journey/app/internal/mcp"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
	"github.com/nightgopher/cc4-journey/app/internal/service"
)

// AuthClaims 对齐存量 generateToken({ userId })。
type AuthClaims struct {
	UserID int `json:"userId"`
	jwt.RegisteredClaims
}

type apiHandler struct {
	jwtSecret []byte
}

// cors 对齐存量 cors() 中间件行为：全放开 + 预检放行。
// （生产环境应收紧 CORS_ORIGIN；对齐期保持与 JS 后端一致。）
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/mcp" {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Router 构建对齐版路由树。
// 存量挂载点：/api/v1/auth|employees|daily-performance|salary|workdays|approvals|...
// Business groups share authentication, operation permissions and module access.
func Router(jwtSecret string) http.Handler {
	h := &apiHandler{jwtSecret: []byte(jwtSecret)}
	r := chi.NewRouter()
	r.Use(cors)
	r.Use(middleware.Recoverer)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, 16<<20)
			next.ServeHTTP(w, r)
		})
	})

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "OK", "timestamp": time.Now().UTC().Format(time.RFC3339Nano), "revision": os.Getenv("CC4_RELEASE_SHA")})
	})
	r.Handle("/mcp", ccmcp.HTTPHandler())

	r.Route("/api/v1", func(r chi.Router) {
		// 公开端点
		r.Post("/auth/login", h.login)
		r.Get("/settings/public", h.publicSettings)

		// 认证区
		r.Group(func(r chi.Router) {
			r.Use(h.auth)
			r.Use(h.authorize)
			r.Get("/auth/session", h.session)

			r.Get("/employees", h.listEmployees)
			r.Get("/employees/{id}", h.getEmployee)
			r.Get("/employees/stats/overview", h.employeeStats)

			r.Get("/daily-performance", h.listPerformance)
			r.Get("/daily-performance/calendar", h.performanceCalendar)
			r.Post("/daily-performance", h.submitPerformance)
			r.Post("/daily-performance/order-details/{id}/approve", h.approveOrderDetail)

			r.Get("/salary", h.listSalary)
			r.Post("/salary/calculate", h.calculateSalary)
			r.Post("/salary/calculate-batch", h.calculateBatch)

			r.Get("/workdays/{year}/{month}", h.getWorkdays)
			r.Post("/workdays", h.upsertWorkdays)

			r.Post("/employees/check-level-transitions", h.checkLevelTransitions)
			r.Post("/employees/trigger-level-check", h.triggerLevelCheck)

			// 其余业务组（leads/customers/products/attendance/teams/notifications/settings）
			h.RegisterExtraRoutes(r)
			h.registerFinance(r)
			h.registerPermissions(r)
			h.registerStaff(r)
			h.registerApprovals(r)
			h.registerOperations(r)
			h.registerReimbursements(r)
			h.registerOrders(r)
			h.registerWechat(r)
			h.registerAttendance(r)
			h.registerLookups(r)
			h.registerMCPCredentials(r)
			r.Get("/dashboard/today", h.dashboardToday)
			r.Get("/dashboard/trend", h.dashboardTrend)
		})
	})
	r.Route("/api", func(r chi.Router) { r.Use(h.auth); r.Use(h.authorize); h.registerWechat(r) })
	return r
}

// ---- helpers ----

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func pageParams(r *http.Request) (page, limit, offset int) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if page > 1_000_000 {
		page = 1_000_000
	}
	return page, limit, (page - 1) * limit
}

func queryInt(r *http.Request, key string) *int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return nil
	}
	return &n
}

func pagination(page, limit, total int) map[string]any {
	tp := 0
	if limit > 0 {
		tp = (total + limit - 1) / limit
	}
	return map[string]any{"page": page, "limit": limit, "total": total, "totalPages": tp}
}

// ---- auth ----

func (h *apiHandler) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Phone    string `json:"phone"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Phone == "" {
		writeErr(w, http.StatusBadRequest, "请求参数错误")
		return
	}

	var u struct {
		ID             int     `db:"id"`
		Email          *string `db:"email"`
		PasswordHash   string  `db:"password_hash"`
		Role           string  `db:"role"`
		Status         string  `db:"status"`
		EmployeeID     *int    `db:"employee_id"`
		Name           *string `db:"name"`
		EmployeeCode   *string `db:"employee_code"`
		Department     *string `db:"department"`
		Level          *string `db:"level"`
		EmployeeStatus *string `db:"employee_status"`
	}
	err := repo.Pool.QueryRow(r.Context(), `
		SELECT u.id, u.email, u.password_hash, u.role, u.status,
		       e.id AS employee_id, e.name, e.employee_code, e.department, e.level, e.status AS employee_status
		FROM users u LEFT JOIN employees e ON u.id = e.user_id
		WHERE u.phone = $1`, body.Phone).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.Status,
		&u.EmployeeID, &u.Name, &u.EmployeeCode, &u.Department, &u.Level, &u.EmployeeStatus)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "手机号或密码错误")
		return
	}
	if u.Status != "active" {
		writeErr(w, http.StatusUnauthorized, "账户已被停用，请联系管理员")
		return
	}
	if u.EmployeeID != nil && (u.EmployeeStatus == nil || *u.EmployeeStatus != "active") {
		writeErr(w, http.StatusUnauthorized, "员工已离职或被暂停，无法登录")
		return
	}
	if !checkPassword(body.Password, u.PasswordHash) {
		writeErr(w, http.StatusUnauthorized, "手机号或密码错误")
		return
	}

	_, _ = repo.Pool.Exec(r.Context(), `UPDATE users SET last_login = CURRENT_TIMESTAMP WHERE id = $1`, u.ID)

	token, err := signToken(u.ID, h.jwtSecret)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "令牌生成失败")
		return
	}
	// 响应字段与旧 POST /auth/login 逐一对应（token 在 session.access_token）。
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "登录成功",
		"user": map[string]any{
			"id": u.ID, "email": u.Email, "role": u.Role,
			"employee_id": u.EmployeeID, "name": u.Name,
			"employee_code": u.EmployeeCode, "department": u.Department, "level": u.Level,
		},
		"session": map[string]any{
			"access_token": token,
			"expires_in":   "7d",
		},
	})
}

// auth 对齐存量 authenticateToken：Bearer JWT → 查 users → 校验 active。
func (h *apiHandler) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authz := r.Header.Get("Authorization")
		if len(authz) < 8 || authz[:7] != "Bearer " {
			writeErr(w, http.StatusUnauthorized, "访问令牌缺失")
			return
		}
		tok, err := jwt.ParseWithClaims(authz[7:], &AuthClaims{}, func(t *jwt.Token) (any, error) {
			return h.jwtSecret, nil
		}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
		if err != nil || !tok.Valid {
			writeErr(w, http.StatusUnauthorized, "无效的访问令牌")
			return
		}
		claims := tok.Claims.(*AuthClaims)

		principal, err := businessaccess.Load(r.Context(), claims.UserID)
		if err != nil {
			writeErr(w, 401, "账户已停用或员工已离职")
			return
		}
		// 注入上下文（对齐旧 req.userId / req.userRole）。
		ctx := businessaccess.WithPrincipal(withUser(r.Context(), claims.UserID, principal.Role), principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *apiHandler) session(w http.ResponseWriter, r *http.Request) {
	uid, _ := r.Context().Value(ctxUserID).(int)
	row, err := repo.One(r.Context(), repo.Pool, `SELECT u.id,u.phone,u.email,u.role,e.id AS employee_id,e.name,e.level,e.position,e.team_id FROM users u LEFT JOIN employees e ON e.user_id=u.id WHERE u.id=$1`, uid)
	if err != nil {
		dbError(w, err)
		return
	}
	row["demo_mode"] = os.Getenv("BUSINESS_SIMULATION_ENABLED") == "true"
	writeJSON(w, http.StatusOK, map[string]any{"userId": uid, "user": row})
}

// ---- employees ----

func (h *apiHandler) listEmployees(w http.ResponseWriter, r *http.Request) {
	uid, role := mustUser(r.Context())
	page, limit, offset := pageParams(r)

	p := repo.EmployeeListParams{
		Search:     r.URL.Query().Get("search"),
		Department: r.URL.Query().Get("department"),
		Level:      r.URL.Query().Get("level"),
		Status:     r.URL.Query().Get("status"),
		Limit:      limit, Offset: offset,
	}
	if role == "employee" {
		e, err := repo.GetEmployeeByUserID(r.Context(), uid)
		if err != nil {
			writeErr(w, 403, "员工档案不存在")
			return
		}
		p.EmployeeID = &e.ID
	}
	// manager/workplace_manager 数据隔离（对齐旧 getCurrentUserTeamContext）。
	if role == "manager" || role == "workplace_manager" {
		emp, err := repo.GetEmployeeByUserID(r.Context(), uid)
		if err != nil || emp.TeamID == nil {
			writeJSON(w, http.StatusOK, map[string]any{"employees": []any{}, "pagination": pagination(page, limit, 0)})
			return
		}
		p.TeamID = emp.TeamID
	}
	list, total, err := repo.ListEmployees(r.Context(), p)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"employees": list, "pagination": pagination(page, limit, total)})
}

func (h *apiHandler) getEmployee(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "无效的员工ID")
		return
	}
	if !canAccessEmployee(w, r, id) {
		return
	}
	emp, err := repo.GetEmployeeByID(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "员工不存在")
		return
	}
	writeJSON(w, http.StatusOK, emp)
}

func (h *apiHandler) employeeStats(w http.ResponseWriter, r *http.Request) {
	var out struct {
		Total      int `json:"total"`
		Active     int `json:"active"`
		Trainees   int `json:"trainees"`
		CLevel     int `json:"c_level"`
		BLevel     int `json:"b_level"`
		ALevel     int `json:"a_level"`
		Eliminated int `json:"eliminated"`
	}
	err := repo.Pool.QueryRow(r.Context(), `
		SELECT COUNT(*),
			COUNT(*) FILTER (WHERE status='active'),
			COUNT(*) FILTER (WHERE level='trainee' AND status='active'),
			COUNT(*) FILTER (WHERE level='c_level' AND status='active'),
			COUNT(*) FILTER (WHERE level='b_level' AND status='active'),
			COUNT(*) FILTER (WHERE level='a_level' AND status='active'),
			COUNT(*) FILTER (WHERE status='eliminated')
		FROM employees`).Scan(&out.Total, &out.Active, &out.Trainees, &out.CLevel, &out.BLevel, &out.ALevel, &out.Eliminated)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "统计失败")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// ---- daily performance ----

func (h *apiHandler) listPerformance(w http.ResponseWriter, r *http.Request) {
	uid, role := mustUser(r.Context())
	page, limit, offset := pageParams(r)

	q := repo.PerformanceQuery{
		StartDate: r.URL.Query().Get("start_date"),
		EndDate:   r.URL.Query().Get("end_date"),
		WorkDate:  r.URL.Query().Get("work_date"),
		Limit:     limit, Offset: offset,
	}
	if q.WorkDate == "" {
		q.WorkDate = r.URL.Query().Get("work_date")
	}
	switch role {
	case "employee":
		emp, err := repo.GetEmployeeByUserID(r.Context(), uid)
		if err != nil {
			writeErr(w, http.StatusNotFound, "员工档案不存在")
			return
		}
		q.EmployeeID = &emp.ID
	case "manager", "workplace_manager":
		emp, err := repo.GetEmployeeByUserID(r.Context(), uid)
		if err != nil || emp.TeamID == nil {
			writeErr(w, 403, "员工档案或团队不存在")
			return
		}
		q.TeamID = emp.TeamID
	default:
		q.EmployeeID = queryInt(r, "employee_id")
	}

	list, total, err := repo.ListPerformance(r.Context(), q)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"performances": list, "dailyPerformance": list, "pagination": pagination(page, limit, total)})
}

func (h *apiHandler) performanceCalendar(w http.ResponseWriter, r *http.Request) {
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	month, _ := strconv.Atoi(r.URL.Query().Get("month"))
	if !validPeriod(year, month) {
		writeErr(w, http.StatusBadRequest, "年份和月份是必填项")
		return
	}
	employeeID := queryInt(r, "employee_id")
	self, teamID, scopeErr := employeeScope(r)
	if scopeErr != nil {
		writeErr(w, 403, "员工档案或团队不存在")
		return
	}
	if self != nil {
		employeeID = self
	}

	rows, err := repo.Pool.Query(r.Context(), `
		SELECT TO_CHAR(dpr.work_date,'YYYY-MM-DD') AS day,
		       SUM(COALESCE(dpr.orders_count,0)) AS orders,
		       SUM(COALESCE(dpr.commission_amount,0)) AS commission
		FROM daily_performance_records dpr
		JOIN employees e ON e.id = dpr.employee_id
		WHERE EXTRACT(YEAR FROM dpr.work_date)=$1 AND EXTRACT(MONTH FROM dpr.work_date)=$2
		  AND ($3::int IS NULL OR dpr.employee_id = $3)
		  AND ($4::int IS NULL OR e.team_id = $4)
		GROUP BY 1 ORDER BY 1`, year, month, employeeID, teamID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	defer rows.Close()

	type dayRow struct {
		Day        string  `json:"day"`
		Orders     int     `json:"orders"`
		Commission float64 `json:"commission"`
	}
	out := []dayRow{}
	for rows.Next() {
		var d dayRow
		if err := rows.Scan(&d.Day, &d.Orders, &d.Commission); err != nil {
			writeErr(w, http.StatusInternalServerError, "扫描失败")
			return
		}
		out = append(out, d)
	}
	writeJSON(w, http.StatusOK, map[string]any{"calendar": out})
}

// ---- salary ----

func (h *apiHandler) listSalary(w http.ResponseWriter, r *http.Request) {
	uid, role := mustUser(r.Context())
	page, limit, offset := pageParams(r)
	employeeID := queryInt(r, "employee_id")
	year := queryInt(r, "year")
	month := queryInt(r, "month")

	where := []string{"1=1"}
	args := []any{}
	add := func(v any) string {
		args = append(args, v)
		return strconv.Itoa(len(args))
	}
	if role == "manager" || role == "workplace_manager" {
		if emp, err := repo.GetEmployeeByUserID(r.Context(), uid); err == nil && emp.TeamID != nil {
			where = append(where, `e.team_id = $`+add(*emp.TeamID))
		} else {
			writeErr(w, 403, "员工档案或团队不存在")
			return
		}
	} else if role == "employee" {
		if emp, err := repo.GetEmployeeByUserID(r.Context(), uid); err == nil {
			where = append(where, `sr.employee_id = $`+add(emp.ID))
		} else {
			writeErr(w, 403, "员工档案不存在")
			return
		}
	}
	if employeeID != nil {
		where = append(where, `sr.employee_id = $`+add(*employeeID))
	}
	if year != nil {
		where = append(where, `sr.period_year = $`+add(*year))
	}
	if month != nil {
		where = append(where, `sr.period_month = $`+add(*month))
	}
	whereSQL := "WHERE " + where[0]
	for i := 1; i < len(where); i++ {
		whereSQL += " AND " + where[i]
	}

	var total int
	if err := repo.Pool.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM salary_records sr JOIN employees e ON e.id = sr.employee_id `+whereSQL, args...).Scan(&total); err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	rows, err := repo.Pool.Query(r.Context(), `
		SELECT sr.id, sr.employee_id, sr.period_year, sr.period_month,
		       COALESCE(sr.base_salary,0), COALESCE(sr.commission_amount,0), COALESCE(sr.bonus_amount,0),
		       COALESCE(sr.overtime_pay,0), COALESCE(sr.deductions,0), COALESCE(sr.net_salary,0),
		       TO_CHAR(sr.payment_date,'YYYY-MM-DD'), COALESCE(sr.payment_status,'pending'),
		       sr.calculation_details, sr.notes
		FROM salary_records sr JOIN employees e ON e.id = sr.employee_id
		`+whereSQL+` ORDER BY sr.period_year DESC, sr.period_month DESC, sr.employee_id
		LIMIT $`+add(limit)+` OFFSET $`+add(offset), args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	defer rows.Close()

	out := []repo.SalaryRecord{}
	for rows.Next() {
		var s repo.SalaryRecord
		if err := rows.Scan(&s.ID, &s.EmployeeID, &s.PeriodYear, &s.PeriodMonth,
			&s.BaseSalary, &s.CommissionAmount, &s.BonusAmount, &s.OvertimePay,
			&s.Deductions, &s.NetSalary, &s.PaymentDate, &s.PaymentStatus, &s.CalcDetails, &s.Notes); err != nil {
			writeErr(w, http.StatusInternalServerError, "扫描失败")
			return
		}
		out = append(out, s)
	}
	writeJSON(w, http.StatusOK, map[string]any{"salaries": out, "pagination": pagination(page, limit, total)})
}

func (h *apiHandler) calculateSalary(w http.ResponseWriter, r *http.Request) {
	if _, role := mustUser(r.Context()); role != "admin" && role != "hr" && role != "finance" && role != "manager" {
		writeErr(w, http.StatusForbidden, "无权限执行薪资计算")
		return
	}
	var body struct {
		EmployeeID int `json:"employeeId"`
		Year       int `json:"periodYear"`
		Month      int `json:"periodMonth"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.EmployeeID == 0 {
		writeErr(w, http.StatusBadRequest, "缺少必要参数：employeeId")
		return
	}
	now := time.Now()
	if body.Year == 0 {
		body.Year = now.Year()
	}
	if body.Month == 0 {
		body.Month = int(now.Month())
	}
	if !validPeriod(body.Year, body.Month) {
		writeErr(w, 400, "年份或月份无效")
		return
	}
	if !canAccessEmployee(w, r, body.EmployeeID) {
		return
	}
	detail, level, err := service.CalculateSalary(r.Context(), body.EmployeeID, body.Year, body.Month)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "薪资计算失败: "+err.Error())
		return
	}
	if _, err := service.SaveSalary(r.Context(), body.EmployeeID, body.Year, body.Month); err != nil {
		dbError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"employeeId":       body.EmployeeID,
		"periodYear":       body.Year,
		"periodMonth":      body.Month,
		"level":            level,
		"baseSalary":       detail.BaseSalary,
		"commissionAmount": detail.CommissionAmount,
		"bonusAmount":      detail.BonusAmount,
		"overtimePay":      detail.OvertimePay,
		"deductions":       detail.Deductions,
		"netSalary":        detail.NetSalary,
	})
}

func (h *apiHandler) calculateBatch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Year  int `json:"periodYear"`
		Month int `json:"periodMonth"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !validPeriod(body.Year, body.Month) {
		writeErr(w, http.StatusBadRequest, "缺少必要参数：periodYear/periodMonth")
		return
	}
	ids, err := service.ActiveEmployeeIDs(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	results := []map[string]any{}
	failed := []map[string]string{}
	for _, id := range ids {
		_, role := mustUser(r.Context())
		if hasRole(role, "manager", "workplace_manager") {
			_, team, err := employeeScope(r)
			if err != nil {
				writeErr(w, 403, "员工档案或团队不存在")
				return
			}
			target, err := repo.TeamIDOfEmployee(r.Context(), id)
			if err != nil || target == nil || *target != *team {
				continue
			}
		}
		detail, err := service.SaveSalary(r.Context(), id, body.Year, body.Month)
		if err != nil {
			failed = append(failed, map[string]string{"employeeId": strconv.Itoa(id), "error": err.Error()})
			continue
		}
		results = append(results, map[string]any{"employeeId": id, "netSalary": detail.NetSalary})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"message": fmt2("批量计算完成：成功 %d，失败 %d", len(results), len(failed)),
		"results": results, "failed": failed,
	})
}

// ---- workdays ----

func (h *apiHandler) getWorkdays(w http.ResponseWriter, r *http.Request) {
	year, err1 := strconv.Atoi(chi.URLParam(r, "year"))
	month, err2 := strconv.Atoi(chi.URLParam(r, "month"))
	if err1 != nil || err2 != nil || !validPeriod(year, month) {
		writeErr(w, http.StatusBadRequest, "无效的年月参数")
		return
	}
	wcfg, err := repo.GetWorkdayConfig(r.Context(), year, month)
	if err != nil {
		writeErr(w, http.StatusNotFound, "未找到指定月份的工作日配置")
		return
	}
	writeJSON(w, http.StatusOK, wcfg)
}

func (h *apiHandler) upsertWorkdays(w http.ResponseWriter, r *http.Request) {
	if _, role := mustUser(r.Context()); role != "admin" && role != "workplace_manager" {
		writeErr(w, http.StatusForbidden, "无权限修改工作日配置")
		return
	}
	var body struct {
		Year     int   `json:"year"`
		Month    int   `json:"month"`
		Workdays []int `json:"workdays"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "请求参数错误")
		return
	}
	if !validPeriod(body.Year, body.Month) || body.Workdays == nil {
		writeErr(w, http.StatusBadRequest, "缺少必要参数")
		return
	}
	if body.Month < 1 || body.Month > 12 {
		writeErr(w, http.StatusBadRequest, "月份必须在1-12之间")
		return
	}
	seen := map[int]bool{}
	maxDay := time.Date(body.Year, time.Month(body.Month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
	for _, d := range body.Workdays {
		if d < 1 || d > maxDay || seen[d] {
			writeErr(w, http.StatusBadRequest, "工作日须为当月有效且不重复的日期")
			return
		}
		seen[d] = true
	}
	wcfg, err := repo.UpsertWorkdayConfig(r.Context(), body.Year, body.Month, body.Workdays)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "保存失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "工作日配置保存成功", "workdayConfig": wcfg})
}

// ---- level check（cron 触发器，对齐旧 trigger-level-check / check-level-transitions）----

func (h *apiHandler) checkLevelTransitions(w http.ResponseWriter, r *http.Request) {
	results, err := service.RunDailyLevelCheck(r.Context(), service.BusinessNow())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "检查失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "等级检查完成", "results": results})
}

func (h *apiHandler) triggerLevelCheck(w http.ResponseWriter, r *http.Request) {
	h.checkLevelTransitions(w, r)
}

// ---- 业绩录入与审核（P0 写路径）----

func (h *apiHandler) submitPerformance(w http.ResponseWriter, r *http.Request) {
	uid, role := mustUser(r.Context())
	if role == "employee" {
		h.submitSelfOrder(w, r)
		return
	}
	// 对齐旧通道权限：admin/hr/manager（employee 自录走 pending_approval 明细，此处对齐管理员分支）
	if !businessaccess.CanManagePerformance(role) {
		writeErr(w, http.StatusForbidden, "无权限代录业绩（employee 请走自录通道）")
		return
	}
	var body struct {
		EmployeeID  int     `json:"employeeId"`
		WorkDate    string  `json:"workDate"`
		ProductID   *int    `json:"productId,omitempty"`
		Orders      int     `json:"orders"`
		OrdersCount *int    `json:"ordersCount"`
		SalesAmount float64 `json:"salesAmount,omitempty"`
		WorkPeriod  string  `json:"workPeriod,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.EmployeeID == 0 || body.WorkDate == "" {
		writeErr(w, http.StatusBadRequest, "缺少必要参数：employeeId/workDate/orders")
		return
	}
	if body.OrdersCount != nil {
		body.Orders = *body.OrdersCount
	}
	if !canAccessEmployee(w, r, body.EmployeeID) {
		return
	}
	result, err := service.SubmitPerformance(r.Context(), service.SubmitPerformanceInput{
		EmployeeID: body.EmployeeID, WorkDate: body.WorkDate,
		ProductID: body.ProductID, Orders: body.Orders,
		SalesAmount: body.SalesAmount, WorkPeriod: body.WorkPeriod,
		SubmittedBy: uid, Channel: "api",
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"message": "每日业绩记录创建成功", "dailyPerformance": result, "needsApproval": false,
	})
}

func (h *apiHandler) approveOrderDetail(w http.ResponseWriter, r *http.Request) {
	uid, role := mustUser(r.Context())
	if !businessaccess.CanManagePerformance(role) {
		writeErr(w, http.StatusForbidden, "无权限审核业绩")
		return
	}
	detailID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "无效的明细ID")
		return
	}
	detail, err := repo.GetOrderDetail(r.Context(), detailID)
	if err != nil {
		dbError(w, err)
		return
	}
	if !canAccessEmployee(w, r, detail.EmployeeID) {
		return
	}
	var body struct {
		Approved        bool   `json:"approved"`
		RejectionReason string `json:"rejectionReason,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "请求参数错误")
		return
	}
	if !body.Approved && body.RejectionReason == "" {
		writeErr(w, http.StatusBadRequest, "驳回必须提供 rejectionReason")
		return
	}
	approverEmp := 0
	if emp, e := repo.GetEmployeeByUserID(r.Context(), uid); e == nil {
		approverEmp = emp.ID
	}
	result, err := service.ApproveOrderDetail(r.Context(), detailID, body.Approved, body.RejectionReason, approverEmp)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"message": map[bool]string{true: "订单审批已通过", false: "订单审批已拒绝"}[body.Approved],
		"result":  result,
	})
}
