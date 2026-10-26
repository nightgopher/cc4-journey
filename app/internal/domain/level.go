package domain

// 等级与状态常量（与存量系统 employee_status 枚举对齐）。
const (
	LevelTrainee    = "trainee"
	LevelC          = "c_level"
	LevelB          = "b_level"
	LevelA          = "a_level"
	LevelEliminated = "eliminated"
)

// 裁定结果（dry-run 输出，不落库）。
const (
	TransitionKeep               = "keep"
	TransitionPendingPromotion   = "pending_promotion"     // 待晋升（需两级审批）
	TransitionPendingElimination = "suggested_elimination" // 建议淘汰（需两级审批）
	TransitionPendingDemotion    = "pending_demotion"      // 待降级（需两级审批）
)

// ---- 考核参数（文档口径；D6 边界以文档为准，见 GOLDEN.md 裁决表）----

const (
	// C 级：最近 3 个工作日订单 >= 3 晋升；考核期 6 个工作日（3 正常 + 3 宽限）。
	CPromotionWindow    = 3
	CPromotionThreshold = 3
	CMaxWorkdays        = 6

	// B 级：最近 6 个工作日订单 >= 12 晋升；缓冲期共 9 个工作日。
	BPromotionWindow    = 6
	BPromotionThreshold = 12
	BMaxWorkdays        = 9

	// A 级：唯一降级条件——最近 6 个工作日订单 <= 12。
	ADemotionWindow    = 6
	ADemotionThreshold = 12
)

// LevelTransitionInput 一次流转判定的输入。
// RecentOrders 为 nil 表示无任何业绩记录（D4 裁决项）。
type LevelTransitionInput struct {
	Level           string // 当前等级
	WorkdaysInLevel int    // 在当前等级已工作的工作日数
	RecentOrders    []int  // 最近 N 个工作日的订单序列（N 依等级而定）
}

// LevelTransitionResult 一次流转判定的输出。
type LevelTransitionResult struct {
	Action    string // TransitionKeep / PendingPromotion / PendingElimination / PendingDemotion
	Target    string // 目标等级（keep 时为当前等级）
	Reason    string // 判定依据摘要，用于审批单与演示展示
	NeedsVote bool   // 是否需要两级审批（所有非 keep 的流转均需要）
}

// EvaluateLevelTransition 纯函数判定：给定当前等级与业绩窗口，输出流转裁定。
// 该函数是 MCP level_transition_check 工具与每日 cron 检查共用的同一份逻辑。
func EvaluateLevelTransition(in LevelTransitionInput) LevelTransitionResult {
	switch in.Level {
	case LevelC:
		return evaluateC(in)
	case LevelB:
		return evaluateB(in)
	case LevelA:
		return evaluateA(in)
	default:
		return LevelTransitionResult{Action: TransitionKeep, Target: in.Level, Reason: "等级无自动流转"}
	}
}

func sum(orders []int) int {
	total := 0
	for _, n := range orders {
		total += n
	}
	return total
}

// evaluateC：近 3 工作日 >= 3 单 → 待晋升；超过 6 个工作日未达标 → 建议淘汰。
func evaluateC(in LevelTransitionInput) LevelTransitionResult {
	orders := in.RecentOrders
	if len(orders) > CPromotionWindow {
		orders = orders[len(orders)-CPromotionWindow:]
	}
	recent := sum(orders)

	if recent >= CPromotionThreshold {
		return LevelTransitionResult{
			Action: TransitionPendingPromotion, Target: LevelB, NeedsVote: true,
			Reason: "近" + itoa(len(orders)) + "个工作日" + itoa(recent) + "单，达到晋升线" + itoa(CPromotionThreshold),
		}
	}
	// D6 文档口径：第 6 天结束时仍未达标即判淘汰（WorkdaysInLevel >= 6）。
	if in.WorkdaysInLevel >= CMaxWorkdays {
		return LevelTransitionResult{
			Action: TransitionPendingElimination, Target: LevelC, NeedsVote: true,
			Reason: "C级已满" + itoa(CMaxWorkdays) + "个工作日，近3个工作日" + itoa(recent) + "单未达标",
		}
	}
	return LevelTransitionResult{Action: TransitionKeep, Target: LevelC, Reason: "考核期内，继续观察"}
}

// evaluateB：近 6 工作日 >= 12 单 → 待晋升；满 9 个工作日仍未达标 → 待降级。
func evaluateB(in LevelTransitionInput) LevelTransitionResult {
	orders := in.RecentOrders
	if len(orders) > BPromotionWindow {
		orders = orders[len(orders)-BPromotionWindow:]
	}
	recent := sum(orders)

	if recent >= BPromotionThreshold {
		return LevelTransitionResult{
			Action: TransitionPendingPromotion, Target: LevelA, NeedsVote: true,
			Reason: "近" + itoa(len(orders)) + "个工作日" + itoa(recent) + "单，达到晋升线" + itoa(BPromotionThreshold),
		}
	}
	if in.WorkdaysInLevel >= BMaxWorkdays {
		return LevelTransitionResult{
			Action: TransitionPendingDemotion, Target: LevelC, NeedsVote: true,
			Reason: "B级已满" + itoa(BMaxWorkdays) + "个工作日，近6个工作日" + itoa(recent) + "单未达标",
		}
	}
	return LevelTransitionResult{Action: TransitionKeep, Target: LevelB, Reason: "缓冲期内，继续观察"}
}

// evaluateA：唯一降级条件——近 6 工作日订单 <= 12。
// D4 裁决口径：无任何业绩记录（RecentOrders == nil）不自动降级，转人工审批。
func evaluateA(in LevelTransitionInput) LevelTransitionResult {
	if in.RecentOrders == nil {
		return LevelTransitionResult{
			Action: TransitionKeep, Target: LevelA, NeedsVote: true,
			Reason: "无业绩记录（疑似休假/停工），转人工核查，不自动降级",
		}
	}
	orders := in.RecentOrders
	if len(orders) > ADemotionWindow {
		orders = orders[len(orders)-ADemotionWindow:]
	}
	recent := sum(orders)

	if recent <= ADemotionThreshold {
		return LevelTransitionResult{
			Action: TransitionPendingDemotion, Target: LevelC, NeedsVote: true,
			Reason: "近6个工作日" + itoa(recent) + "单，未超过降级线" + itoa(ADemotionThreshold),
		}
	}
	return LevelTransitionResult{Action: TransitionKeep, Target: LevelA, Reason: "业绩达标，保持A级"}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
