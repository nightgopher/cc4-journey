// Package domain 承载 Call Center 4.0 的纯业务规则。
// 所有函数不触碰数据库与网络：输入输出均为显式结构体，保证可表驱动测试、
// 可被 MCP 工具 dry-run 调用。金额单位为元（整数分避免，与现有系统口径一致）。
package domain

// DailyCommission 按当日订单总数计算提成（模型一：日汇总公式）。
// 规则：n<=3 时 n*10；4-5 单时 30+(n-3)*20；>=6 单时 70+(n-5)*30。
// 每日独立计算，不跨日累进。
func DailyCommission(orders int) int {
	switch {
	case orders <= 0:
		return 0
	case orders <= 3:
		return orders * 10
	case orders <= 5:
		return 30 + (orders-3)*20
	default:
		return 70 + (orders-5)*30
	}
}

// MonthlyCommission 按日订单序列累加月度提成。
func MonthlyCommission(dailyOrders []int) int {
	total := 0
	for _, n := range dailyOrders {
		total += DailyCommission(n)
	}
	return total
}

// ---- C 级固定工资 ----

// C 级固定工资总额度：前 3 个工作日 × 30 元。
const (
	CAllowanceWorkdays = 3
	CAllowancePerDay   = 30
)

// CSalary 计算某月 C 级固定工资。
// 额度共 3 天，跨月可续：先扣减此前月份已领天数，剩余额度在本月内按实际 C 级工作日发放。
//   - cWorkdaysBeforeMonth: 截至上月底，该员工以 C 级身份已领取工资的工作日数
//   - cWorkdaysThisMonth:   本月内该员工以 C 级身份出勤的工作日数
func CSalary(totalCWorkdaysBeforeMonth, cWorkdaysThisMonth int) int {
	remaining := CAllowanceWorkdays - totalCWorkdaysBeforeMonth
	if remaining <= 0 {
		return 0
	}
	paid := cWorkdaysThisMonth
	if paid > remaining {
		paid = remaining
	}
	return paid * CAllowancePerDay
}

// ---- B 级 88 元补贴 ----

// B 级补贴：晋升后前 6 个工作日 × 88 元，总量口径（本月用不完顺延次月）。
const (
	BAllowanceWorkdays = 6
	BAllowancePerDay   = 88
)

// BSalaryPaidDays 按总量口径计算本月应发的 88 元天数。
//   - bWorkdaysBeforeMonth: 截至上月底已享受补贴的工作日数（总量扣减）
//   - bWorkdaysThisMonth:   本月晋升日之后的工作日数
//
// 注意：这是文档口径（total_allowance）。旧 JS 引擎为每月重置口径，
// 两者差异即 D1，由差分测试呈现，不由本函数复刻。
func BSalaryPaidDays(bWorkdaysBeforeMonth, bWorkdaysThisMonth int) int {
	remaining := BAllowanceWorkdays - bWorkdaysBeforeMonth
	if remaining <= 0 {
		return 0
	}
	paid := bWorkdaysThisMonth
	if paid > remaining {
		paid = remaining
	}
	return paid
}

func BSalary(bWorkdaysBeforeMonth, bWorkdaysThisMonth int) int {
	return BSalaryPaidDays(bWorkdaysBeforeMonth, bWorkdaysThisMonth) * BAllowancePerDay
}

// ---- A 级薪资 ----

const (
	ABaseSalary        = 2200 // 满勤底薪
	AFullAttendanceMin = 25   // 全勤奖门槛（有效出勤天数）
	AFullAttendancePay = 400
)

// ABaseSalaryProrated 底薪按出勤比例折算：2200 × attendance/totalWorkdays。
// 返回向下取整的元值；totalWorkdays<=0 视为无配置，返回 0。
func ABaseSalaryProrated(attendanceDays, totalWorkdays int) int {
	if totalWorkdays <= 0 || attendanceDays <= 0 {
		return 0
	}
	if attendanceDays > totalWorkdays {
		attendanceDays = totalWorkdays
	}
	return ABaseSalary * attendanceDays / totalWorkdays
}

// AFullAttendanceBonus 全勤奖：有效出勤 >= 25 天得 400，否则 0。
// 与订单数无关；短月（工作日 < 25）全月无人可得。
func AFullAttendanceBonus(attendanceDays int) int {
	if attendanceDays >= AFullAttendanceMin {
		return AFullAttendancePay
	}
	return 0
}

// APerformanceBonus 绩效奖金按月总订单分档：
// >=125 → 1000；>=100 → 600；>=75 → 300；否则 0。
func APerformanceBonus(monthlyOrders int) int {
	switch {
	case monthlyOrders >= 125:
		return 1000
	case monthlyOrders >= 100:
		return 600
	case monthlyOrders >= 75:
		return 300
	default:
		return 0
	}
}

// ASalary 汇总 A 级月薪的固定部分。
func ASalary(attendanceDays, totalWorkdays, monthlyOrders int) (baseSalary, fullAttendance, performance int) {
	return ABaseSalaryProrated(attendanceDays, totalWorkdays),
		AFullAttendanceBonus(attendanceDays),
		APerformanceBonus(monthlyOrders)
}
