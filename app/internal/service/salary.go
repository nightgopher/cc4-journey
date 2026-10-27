// Package service 用例编排层：把 domain 纯函数与 repo 数据访问组装成业务动作。
// 与存量 routes 的逻辑逐条对齐；差异（D1/D3/D4/D6 修复）显式标注。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nightgopher/cc4-journey/app/internal/domain"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
)

// SalaryDetail 对齐旧 calculateSalaryByLevel 的返回结构
// （baseSalary / commissionAmount / bonusAmount / overtimePay / deductions / netSalary）。
type SalaryDetail struct {
	BaseSalary       float64 `json:"baseSalary"`
	CommissionAmount float64 `json:"commissionAmount"`
	BonusAmount      float64 `json:"bonusAmount"`
	OvertimePay      float64 `json:"overtimePay"`
	Deductions       float64 `json:"deductions"`
	NetSalary        float64 `json:"netSalary"`
}

// CalculateSalary 复刻存量 utils/salaryCalculator.js 的按等级分发逻辑。
// 与旧引擎的差异（均为文档口径修复，见 GOLDEN.md）：
//   - B 级 88 元：总量顺延（旧引擎每月重置 → D1）
//   - A 级出勤：按去重日期（旧引擎按记录行数 → D3）
func CalculateSalary(ctx context.Context, employeeID, year, month int) (*SalaryDetail, string, error) {
	monthEnd := time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC)
	emp, err := repo.GetEmployeeByID(ctx, employeeID)
	if err != nil {
		return nil, "", fmt.Errorf("员工不存在: %w", err)
	}
	if emp.JoinDate != nil && emp.JoinDate.Format("2006-01-02") > monthEnd.Format("2006-01-02") {
		return &SalaryDetail{}, emp.Level, nil
	}
	level, err := repo.EmployeeLevelAsOf(ctx, employeeID, monthEnd.Format("2006-01-02"))
	if err != nil {
		return nil, "", fmt.Errorf("无法确定工资月份的有效职级: %w", err)
	}

	var detail SalaryDetail
	switch level {
	case domain.LevelTrainee:
		detail, err = traineeSalary(ctx, employeeID, year, month)
	case domain.LevelC:
		detail, err = cSalary(ctx, employeeID, year, month)
	case domain.LevelB:
		detail, err = bSalary(ctx, employeeID, year, month)
	case domain.LevelA:
		detail, err = aSalary(ctx, employeeID, year, month)
	default:
		return nil, level, fmt.Errorf("未知员工等级: %s", level)
	}
	if err != nil {
		return nil, level, err
	}
	return &detail, level, nil
}

// monthlyCommission 月度提成：按日订单序列累加（模型一），对齐旧 calculateMonthlyCommission。
func monthlyCommission(ctx context.Context, employeeID, year, month int) (float64, error) {
	totals, err := repo.MonthlyOrderTotals(ctx, employeeID, year, month)
	if err != nil {
		return 0, err
	}
	// 按日期排序累加（金额与顺序无关，但保持确定性）。
	dates := make([]string, 0, len(totals))
	for d := range totals {
		dates = append(dates, d)
	}
	sortStrings(dates)
	total := 0
	for _, d := range dates {
		total += domain.DailyCommission(totals[d])
	}
	return float64(total), nil
}

// traineeSalary：实习生 = 纯提成。
func traineeSalary(ctx context.Context, employeeID, year, month int) (SalaryDetail, error) {
	commission, err := monthlyCommission(ctx, employeeID, year, month)
	if err != nil {
		return SalaryDetail{}, err
	}
	return SalaryDetail{CommissionAmount: commission, NetSalary: commission}, nil
}

// cSalary：C 级 = 前 3 个工作日 × 30（跨月补足）+ 提成。
func cSalary(ctx context.Context, employeeID, year, month int) (SalaryDetail, error) {
	// C 级起始日：最近一条 to_level='c_level'；无历史则回退 join_date（对齐旧引擎）。
	monthEnd := time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC)
	hist, err := repo.LatestLevelChangeOnOrBefore(ctx, employeeID, domain.LevelC, monthEnd.Format("2006-01-02"))
	if err != nil {
		emp, e := repo.GetEmployeeByID(ctx, employeeID)
		if e != nil || emp.JoinDate == nil {
			return SalaryDetail{}, fmt.Errorf("无法确定C级起始日: %w", err)
		}
		hist = &repo.LevelHistory{ChangeDate: *emp.JoinDate}
	}

	workdays, err := repo.WorkdaysUpTo(ctx, hist.ChangeDate, monthEnd)
	if err != nil {
		return SalaryDetail{}, err
	}

	// 截至上月底的 C 级工作日数（额度扣减项）。
	monthStart := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	before := 0
	inMonth := 0
	for _, d := range workdays {
		if d.Before(monthStart) {
			before++
		} else {
			inMonth++
		}
	}

	commission, err := monthlyCommission(ctx, employeeID, year, month)
	if err != nil {
		return SalaryDetail{}, err
	}
	base := domain.CSalary(before, inMonth)
	return SalaryDetail{
		BaseSalary:       float64(base),
		CommissionAmount: commission,
		NetSalary:        float64(base) + commission,
	}, nil
}

// bSalary：B 级 = 晋升后 88 元/天（6 天总量、跨月顺延，D1 文档口径）+ 提成。
func bSalary(ctx context.Context, employeeID, year, month int) (SalaryDetail, error) {
	monthEnd := time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC)
	hist, err := repo.LatestLevelChangeOnOrBefore(ctx, employeeID, domain.LevelB, monthEnd.Format("2006-01-02"))
	if err != nil {
		return SalaryDetail{}, fmt.Errorf("无B级晋升记录: %w", err)
	}

	workdays, err := repo.WorkdaysUpTo(ctx, hist.ChangeDate, monthEnd)
	if err != nil {
		return SalaryDetail{}, err
	}
	monthStart := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	before, inMonth := 0, 0
	for _, d := range workdays {
		if d.Before(monthStart) {
			before++
		} else {
			inMonth++
		}
	}

	commission, err := monthlyCommission(ctx, employeeID, year, month)
	if err != nil {
		return SalaryDetail{}, err
	}
	base := domain.BSalary(before, inMonth)
	return SalaryDetail{
		BaseSalary:       float64(base),
		CommissionAmount: commission,
		NetSalary:        float64(base) + commission,
	}, nil
}

// aSalary：A 级 = 底薪比例 + 全勤奖 + 绩效 + 提成。
func aSalary(ctx context.Context, employeeID, year, month int) (SalaryDetail, error) {
	workdays, err := repo.WorkdaysOfMonth(ctx, year, month)
	if err != nil {
		return SalaryDetail{}, err
	}
	totalWorkdays := len(workdays)

	// D3 修复：按去重日期统计出勤（旧引擎按记录行数，同日多产品会重复计）。
	attendance, err := repo.AttendanceUnitsInMonth(ctx, employeeID, year, month, workdays)
	if err != nil {
		return SalaryDetail{}, err
	}

	totals, err := repo.MonthlyOrderTotals(ctx, employeeID, year, month)
	if err != nil {
		return SalaryDetail{}, err
	}
	monthlyOrders := 0
	dates := make([]string, 0, len(totals))
	for d := range totals {
		dates = append(dates, d)
	}
	sortStrings(dates)
	daily := make([]int, 0, len(dates))
	for _, d := range dates {
		monthlyOrders += totals[d]
		daily = append(daily, totals[d])
	}

	commission, err := monthlyCommission(ctx, employeeID, year, month)
	if err != nil {
		return SalaryDetail{}, err
	}

	base := domain.ABaseSalaryForUnits(attendance, totalWorkdays)
	full := domain.AFullAttendanceBonus(int(attendance))
	perf := domain.APerformanceBonus(monthlyOrders)
	bonus := float64(full + perf)
	net := float64(base) + bonus + commission
	return SalaryDetail{
		BaseSalary:       float64(base),
		CommissionAmount: commission,
		BonusAmount:      bonus,
		NetSalary:        net,
	}, nil
}

// ---- 等级流转（dry-run + cron 共用）----

// CheckLevelTransition 对存量 employeeLevelTransition.js 判定链的对齐封装。
// 返回 domain 的判定结果；RecentOrders 由 repo 按「最近 N 个工作日」窗口取数。
func CheckLevelTransition(ctx context.Context, employeeID int, today time.Time) (*domain.LevelTransitionResult, error) {
	emp, err := repo.GetEmployeeByID(ctx, employeeID)
	if err != nil {
		return nil, err
	}

	in := domain.LevelTransitionInput{Level: emp.Level}

	switch emp.Level {
	case domain.LevelC:
		in.RecentOrders, err = recentOrdersInWorkdays(ctx, employeeID, domain.CPromotionWindow, today)
		in.WorkdaysInLevel, err = workdaysSince(ctx, employeeID, domain.LevelC, today)
	case domain.LevelB:
		in.RecentOrders, err = recentOrdersInWorkdays(ctx, employeeID, domain.BPromotionWindow, today)
		in.WorkdaysInLevel, err = workdaysSince(ctx, employeeID, domain.LevelB, today)
	case domain.LevelA:
		in.RecentOrders, err = recentOrdersInWorkdays(ctx, employeeID, domain.ADemotionWindow, today)
		// D4：nil = 无任何业绩记录，domain 层转人工核查。
		if total, e := repo.MonthlyOrderTotals(ctx, employeeID, today.Year(), int(today.Month())); e == nil && len(total) == 0 {
			in.RecentOrders = nil
		}
	default:
		r := domain.EvaluateLevelTransition(in)
		return &r, nil
	}
	if err != nil {
		return nil, err
	}

	r := domain.EvaluateLevelTransition(in)
	return &r, nil
}

// recentOrdersInWorkdays 取截止 today（含）最近 n 个工作日的订单序列（旧 getPerformanceInWorkdays 对齐）。
func recentOrdersInWorkdays(ctx context.Context, employeeID, n int, today time.Time) ([]int, error) {
	start := today.AddDate(0, -3, 0) // 往前查 3 个月（对齐旧引擎回溯范围）
	workdays, err := repo.WorkdaysUpTo(ctx, start, today)
	if err != nil {
		return nil, err
	}
	if len(workdays) > n {
		workdays = workdays[len(workdays)-n:]
	}
	if len(workdays) == 0 {
		return []int{}, nil
	}

	// 逐日取订单数（有记录取值，无记录为 0），保持工作日顺序。
	out := make([]int, 0, len(workdays))
	for _, d := range workdays {
		key := d.Format("2006-01-02")
		totals, err := repo.MonthlyOrderTotals(ctx, employeeID, d.Year(), int(d.Month()))
		if err != nil {
			return nil, err
		}
		out = append(out, totals[key])
	}
	return out, nil
}

// workdaysSince 员工自 levelChange 起至 today 的工作日数。
func workdaysSince(ctx context.Context, employeeID int, toLevel string, today time.Time) (int, error) {
	hist, err := repo.LatestLevelChange(ctx, employeeID, toLevel)
	if err != nil {
		return 0, err
	}
	days, err := repo.WorkdaysUpTo(ctx, hist.ChangeDate, today)
	if err != nil {
		return 0, err
	}
	return len(days), nil
}

// SaveSalary 计算并持久化薪资记录（对齐旧 POST /salary/calculate 落库行为）。
func SaveSalary(ctx context.Context, employeeID, year, month int) (*SalaryDetail, error) {
	var paid bool
	if err := repo.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM salary_records WHERE employee_id=$1 AND period_year=$2 AND period_month=$3 AND payment_status='paid')`, employeeID, year, month).Scan(&paid); err != nil {
		return nil, err
	}
	if paid {
		return nil, fmt.Errorf("已发放的薪资不能重新计算，请先撤销发放")
	}
	detail, level, err := CalculateSalary(ctx, employeeID, year, month)
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(detail)
	_, err = repo.UpsertSalary(ctx, &repo.SalaryRecord{
		EmployeeID:       employeeID,
		PeriodYear:       year,
		PeriodMonth:      month,
		BaseSalary:       detail.BaseSalary,
		CommissionAmount: detail.CommissionAmount,
		BonusAmount:      detail.BonusAmount,
		NetSalary:        detail.NetSalary,
		CalcDetails:      strPtr(string(raw)),
	})
	if err != nil {
		return nil, err
	}
	repo.WriteAudit(ctx, "system", "salary_calculate",
		fmt.Sprintf("employee=%d period=%d-%d level=%s net=%.2f", employeeID, year, month, level, detail.NetSalary))
	return detail, nil
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func strPtr(s string) *string { return &s }
