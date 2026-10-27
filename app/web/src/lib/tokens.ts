/**
 * 设计令牌 —— cc4 呼叫中心管理台
 *
 * 方向：教育电销团队的教务周计划；蓝白分区与可操作的周日期条。
 * 业务表格、原生表单和稳定的数字对齐服务于全天办公录入。
 * 依据与技能选型见 journey/04-frontend-decisions.md。
 */

export const tokens = {
  color: {
    // 基底 4-6 色
    ink: "#182B49", // 主文字与海军蓝导航
    inkSoft: "#53647E", // 次级文字
    paper: "#F5F7FB", // 冷白工作区
    line: "#DCE3ED", // 分隔线
    accent: "#315CE0", // 主操作与选中
    moneyIn: "#1F7354", // 入账绿：只用于提成/入账
    moneyOut: "#B64843", // 扣减红：只用于扣款/驳回
    warn: "#805819", // 待办琥珀
  },
  radius: { sm: "4px", md: "8px", lg: "14px" },
  space: [0, 4, 8, 12, 16, 24, 32, 48, 64],
  type: {
    family: `-apple-system, 'PingFang SC', 'Microsoft YaHei', 'Segoe UI', sans-serif`,
    mono: `'SF Mono', 'JetBrains Mono', Menlo, monospace`,
    scale: {
      display: "28px / 1.2 700",
      title: "22.4px / 1.4 650",
      body: "14px / 1.6 400",
      small: "12px / 1.5 400",
      money: "22px / 1.2 650",
    },
  },
} as const;

/** 金额展示规则（业务规则 12.4：无小数不显示 .00） */
export function money(n: number | string | null | undefined): string {
  if (n === null || n === undefined) return "—";
  if (typeof n === "string" && n.trim() === "") return "—";
  const num = Number(n);
  if (!Number.isFinite(num)) return "—";
  return new Intl.NumberFormat("zh-CN", {
    style: "currency",
    currency: "CNY",
    minimumFractionDigits: num % 1 === 0 ? 0 : 2,
    maximumFractionDigits: 2,
  }).format(num);
}

/** 等级中文名映射（业务规则：前端全中文） */
export const LEVEL_ZH: Record<string, string> = {
  trainee: "实习生",
  c_level: "C级",
  b_level: "B级",
  a_level: "A级",
  suggested_elimination: "建议淘汰",
  eliminated: "已淘汰",
};

/** 状态中文名 */
export const STATUS_ZH: Record<string, string> = {
  active: "在职",
  inactive: "停用",
  eliminated: "已淘汰",
  pending: "待处理",
  approved: "已通过",
  rejected: "已驳回",
  pool: "线索池",
  assigned: "已分配",
  following: "跟进中",
  paid: "已成交",
  invalid: "无效",
  new: "新线索",
  contacted: "已联系",
  qualified: "有意向",
  converted: "已转化",
  lost: "已流失",
  duplicate: "重复线索",
  work: "出勤",
  partial: "部分完成",
  absent: "缺勤",
  completed: "已结清",
  confirmed: "已确认",
  cancelled: "已取消",
  suspended: "已暂停",
  pending_approval: "待审核",
  individual: "个人客户",
  business: "企业客户",
  vip: "重点客户",
  blacklisted: "黑名单",
  promotion: "晋升",
  level_review: "等级人工核查",
  demotion: "降级",
  elimination: "淘汰",
  leave: "请假",
  other: "其他",
  high: "高",
  medium: "中",
  low: "低",
  onetime: "一次性",
  recurring: "周期性",
  sales: "销售收入",
  receivable_payment: "应收回款",
  salary: "薪资",
  operation: "运营",
  product: "产品",
  marketing: "营销",
};
