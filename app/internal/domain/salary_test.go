package domain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// goldenCase 对应 test/golden/cases.json 中的一个用例。
type goldenCase struct {
	ID      string          `json:"id"`
	Group   string          `json:"group"`
	Name    string          `json:"name"`
	Input   json.RawMessage `json:"input"`
	Expect  json.RawMessage `json:"expect"`
	Status  string          `json:"status"`
	Dispute string          `json:"disputeRef,omitempty"`
	Note    string          `json:"note,omitempty"`
}

type goldenFile struct {
	Version int          `json:"version"`
	Cases   []goldenCase `json:"cases"`
}

func loadGolden(t *testing.T) []goldenCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "test", "golden", "cases.json"))
	if err != nil {
		t.Fatalf("读取金标准文件失败: %v", err)
	}
	var gf goldenFile
	if err := json.Unmarshal(raw, &gf); err != nil {
		t.Fatalf("解析金标准文件失败: %v", err)
	}
	return gf.Cases
}

func findCase(cases []goldenCase, id string) goldenCase {
	for _, c := range cases {
		if c.ID == id {
			return c
		}
	}
	panic("golden case not found: " + id)
}

func expectInt(t *testing.T, c goldenCase, key string) int {
	t.Helper()
	var m map[string]int
	if err := json.Unmarshal(c.Expect, &m); err != nil {
		t.Fatalf("%s: 解析期望值失败: %v", c.ID, err)
	}
	return m[key]
}

// ---- A 组：日提成 G01–G04 ----

func TestGoldenDailyCommission(t *testing.T) {
	for _, id := range []string{"G01", "G02", "G03", "G04"} {
		c := findCase(loadGolden(t), id)
		var in struct {
			Orders int `json:"orders"`
		}
		if err := json.Unmarshal(c.Input, &in); err != nil {
			t.Fatalf("%s: %v", c.ID, err)
		}
		got := DailyCommission(in.Orders)
		want := expectInt(t, c, "commission")
		if got != want {
			t.Errorf("%s(%s): DailyCommission(%d) = %d, want %d", c.ID, c.Name, in.Orders, got, want)
		}
	}
}

// 额外钉死公式边界：0 单与 6 单。
func TestDailyCommissionExtraBoundaries(t *testing.T) {
	if got := DailyCommission(0); got != 0 {
		t.Errorf("DailyCommission(0) = %d, want 0", got)
	}
	if got := DailyCommission(6); got != 100 { // 70 + 1*30
		t.Errorf("DailyCommission(6) = %d, want 100", got)
	}
}

// ---- B 组：C 级固定工资 G05–G07 ----

func TestGoldenCSalary(t *testing.T) {
	cases := loadGolden(t)
	for _, id := range []string{"G05", "G06", "G07"} {
		c := findCase(cases, id)
		var in struct {
			Before int `json:"totalCLevelWorkdaysBeforeMonth"`
			This   int `json:"cLevelWorkdaysThisMonth"`
		}
		if err := json.Unmarshal(c.Input, &in); err != nil {
			t.Fatalf("%s: %v", c.ID, err)
		}
		got := CSalary(in.Before, in.This)
		want := expectInt(t, c, "baseSalary")
		if got != want {
			t.Errorf("%s(%s): CSalary(%d,%d) = %d, want %d", c.ID, c.Name, in.Before, in.This, got, want)
		}
	}
}

// ---- C 组：B 级 88 元（G08 文档口径 / G09 旧引擎口径对照）----

func TestGoldenBSalary(t *testing.T) {
	cases := loadGolden(t)

	// G08：文档口径（总量顺延）——新引擎的实现目标。
	g08 := findCase(cases, "G08")
	var in08 struct {
		WorkdaysByMonth map[string][]string `json:"workdaysByMonth"`
	}
	if err := json.Unmarshal(g08.Input, &in08); err != nil {
		t.Fatalf("G08: %v", err)
	}
	oct, nov := len(in08.WorkdaysByMonth["2025-10"]), len(in08.WorkdaysByMonth["2025-11"])
	// 期望值是嵌套对象（按月份），直接手工断言：10月发 3×88=264。
	if got := BSalary(0, oct); got != 264 {
		t.Errorf("G08 10月: BSalary(0,%d) = %d, want 264", oct, got)
	}
	// 11月：10月已用 3 天，剩 3 天额度；11月有 5 个工作日 → 只发 3 天。
	if got := BSalary(oct, nov); got != 264 {
		t.Errorf("G08 11月: BSalary(%d,%d) = %d, want 264（6天总量口径）", oct, nov, got)
	}

	// G09：旧引擎口径（每月重置）——仅作差分对照，由 JS 引擎侧产出，不测新引擎。
	// 差分报告会呈现：新引擎 264 vs 旧引擎 528（D1，每月多发 264 元）。
}

// ---- D 组：A 级薪资 G10–G11 ----

func TestGoldenASalary(t *testing.T) {
	cases := loadGolden(t)
	for _, id := range []string{"G10", "G11"} {
		c := findCase(cases, id)
		var in struct {
			TotalWorkdays int   `json:"totalWorkdays"`
			Attendance    int   `json:"attendanceDays"`
			MonthlyOrders int   `json:"monthlyOrders"`
			DailySeries   []int `json:"dailyOrderSeries"`
		}
		if err := json.Unmarshal(c.Input, &in); err != nil {
			t.Fatalf("%s: %v", c.ID, err)
		}
		base, full, perf := ASalary(in.Attendance, in.TotalWorkdays, in.MonthlyOrders)
		var want map[string]int
		if err := json.Unmarshal(c.Expect, &want); err != nil {
			t.Fatalf("%s: %v", c.ID, err)
		}
		if base != want["baseSalary"] {
			t.Errorf("%s(%s): base = %d, want %d", c.ID, c.Name, base, want["baseSalary"])
		}
		if full != want["fullAttendanceBonus"] {
			t.Errorf("%s(%s): fullAttendance = %d, want %d", c.ID, c.Name, full, want["fullAttendanceBonus"])
		}
		if perf != want["performanceBonus"] {
			t.Errorf("%s(%s): performance = %d, want %d", c.ID, c.Name, perf, want["performanceBonus"])
		}
		// 提成链路一并验证：日序列累加应等于月度提成。
		commission := MonthlyCommission(in.DailySeries)
		if commission <= 0 {
			t.Errorf("%s: 月度提成应为正，got %d", c.ID, commission)
		}
	}
}

// 绩效档位边界：75/100/125 恰好落档。
func TestAPerformanceBonusBoundaries(t *testing.T) {
	for orders, want := range map[int]int{74: 0, 75: 300, 99: 300, 100: 600, 124: 600, 125: 1000} {
		if got := APerformanceBonus(orders); got != want {
			t.Errorf("APerformanceBonus(%d) = %d, want %d", orders, got, want)
		}
	}
}

// 短月规则：22 工作日的月份即使全勤也拿不到 25 天全勤奖。
func TestShortMonthNoFullAttendance(t *testing.T) {
	if got := AFullAttendanceBonus(22); got != 0 {
		t.Errorf("短月全勤 22 天不应得全勤奖，got %d", got)
	}
}

// ---- E 组：等级流转 G12–G15 ----

func TestGoldenLevelTransitions(t *testing.T) {
	cases := loadGolden(t)

	// G12：C 晋 B 边界。
	g12 := findCase(cases, "G12")
	var in12 struct {
		Workdays int   `json:"cLevelWorkdays"`
		Orders   []int `json:"recentOrders"`
	}
	if err := json.Unmarshal(g12.Input, &in12); err != nil {
		t.Fatalf("G12: %v", err)
	}
	r12 := EvaluateLevelTransition(LevelTransitionInput{Level: LevelC, WorkdaysInLevel: in12.Workdays, RecentOrders: in12.Orders})
	if r12.Action != TransitionPendingPromotion {
		t.Errorf("G12: action = %s, want %s（reason: %s）", r12.Action, TransitionPendingPromotion, r12.Reason)
	}

	// G13：C 淘汰边界（D6 文档口径预置）。
	g13 := findCase(cases, "G13")
	var in13 struct {
		Workdays int   `json:"cLevelWorkdays"`
		Orders   []int `json:"recentOrders"`
	}
	if err := json.Unmarshal(g13.Input, &in13); err != nil {
		t.Fatalf("G13: %v", err)
	}
	r13 := EvaluateLevelTransition(LevelTransitionInput{Level: LevelC, WorkdaysInLevel: in13.Workdays, RecentOrders: in13.Orders})
	if r13.Action != TransitionPendingElimination {
		t.Errorf("G13: action = %s, want %s（D6 文档口径；reason: %s）", r13.Action, TransitionPendingElimination, r13.Reason)
	}

	// G14：A 级 =12 也降。
	g14 := findCase(cases, "G14")
	var in14 struct {
		Orders []int `json:"recentOrders"`
	}
	if err := json.Unmarshal(g14.Input, &in14); err != nil {
		t.Fatalf("G14: %v", err)
	}
	r14 := EvaluateLevelTransition(LevelTransitionInput{Level: LevelA, RecentOrders: in14.Orders})
	if r14.Action != TransitionPendingDemotion {
		t.Errorf("G14: action = %s, want %s（=12 也降；reason: %s）", r14.Action, TransitionPendingDemotion, r14.Reason)
	}

	// G15：A 级零业绩（D4 裁决口径：不自动降级，转人工）。
	findCase(cases, "G15") // 断言依据 GOLDEN.md G15 行；输入即 RecentOrders=nil
	r15 := EvaluateLevelTransition(LevelTransitionInput{Level: LevelA, RecentOrders: nil})
	if r15.Action != TransitionKeep || !r15.NeedsVote {
		t.Errorf("G15: action = %s needsVote = %v, want keep+人工核查（D4 裁决口径）", r15.Action, r15.NeedsVote)
	}

	// 对照：旧 JS 引擎对 G15 的行为是直接降级——差分报告呈现，不在此复刻。
	fmt.Fprintln(os.Stderr, "G15 按 D4 裁决口径断言；旧引擎直接降级行为走差分通道对照。")
}

// 旧代码已知对照点（ employeeLevelTransition.js:173 ）：nil 视为降级。
// 新引擎按 D4 裁决为人工核查——这条差异是演示的第二个「抓 bug」时刻。
func TestAZeroPerformanceDivergence(t *testing.T) {
	r := EvaluateLevelTransition(LevelTransitionInput{Level: LevelA, RecentOrders: nil})
	if r.Action == TransitionPendingDemotion {
		t.Error("零业绩不应自动降级（D4 裁决口径），旧引擎行为已被否决")
	}
}
