/**
 * 设计令牌 —— cc4 呼叫中心管理台
 *
 * 设计立场（frontend-design skill 两遍法第一遍的结论）：
 * - 主题：电销呼叫中心。视觉锚点是"话务台"——密集、清晰、看一眼知道今天打了几通、成了几单。
 * - 刻意避开（skill 列出的 AI 模板脸）：奶油底+赤陶橙、近黑底+荧光绿、SaaS 圆角卡片阵列、
 *   ALL-CAPS eyebrow、'A · B · C' 中点串、每卡同款阴影。
 * - 用色：深墨青做主色（话务台夜班的沉稳），账目红/入账绿只给钱相关数字——
 *   钱是这套系统最重要的事实，颜色只讲这一件事。
 * - 字体：系统栈（内部工具不赌网络字体），层级靠字重与大小对比，不靠装饰。
 * - 布局：左窄导航 + 内容区最大宽度约束；表格是主角，密度优先。
 */

export const tokens = {
  color: {
    // 基底 4-6 色
    ink:      '#16302B', // 主墨：深墨青
    inkSoft:  '#3E5A54', // 次级文字
    paper:    '#F7F6F2', // 纸面：微暖白（非奶油色，灰度更高）
    line:     '#D8D5CC', // 分隔线
    accent:   '#0F6B5C', // 动作色：青绿（按钮/激活态）
    moneyIn:  '#1D7A4F', // 入账绿：只用于提成/入账
    moneyOut: '#B4432F', // 扣减红：只用于扣款/驳回
    warn:     '#B07D2B', // 待办琥珀
  },
  radius: { sm: '4px', md: '8px', lg: '14px' },
  space: [0, 4, 8, 12, 16, 24, 32, 48, 64],
  type: {
    family: `-apple-system, 'PingFang SC', 'Microsoft YaHei', 'Segoe UI', sans-serif`,
    mono: `'SF Mono', 'JetBrains Mono', Menlo, monospace`,
    scale: {
      display: '28px / 1.2 700',
      title:   '18px / 1.3 600',
      body:    '14px / 1.6 400',
      small:   '12px / 1.5 400',
      money:   '22px / 1.2 650',
    },
  },
} as const;

/** 金额展示规则（业务规则 12.4：无小数不显示 .00） */
export function money(n: number | string | null | undefined): string {
  if (n === null || n === undefined) return '—';
  const num = typeof n === 'string' ? parseFloat(n) : n;
  if (Number.isNaN(num)) return '—';
  return num % 1 === 0 ? `¥${num.toLocaleString('zh-CN')}` : `¥${num.toFixed(2)}`;
}

/** 等级中文名映射（业务规则：前端全中文） */
export const LEVEL_ZH: Record<string, string> = {
  trainee: '实习生', c_level: 'C级', b_level: 'B级', a_level: 'A级',
  suggested_elimination: '建议淘汰', eliminated: '已淘汰',
};

/** 状态中文名 */
export const STATUS_ZH: Record<string, string> = {
  active: '在职', inactive: '停用', eliminated: '已淘汰',
  pending: '待处理', approved: '已通过', rejected: '已驳回',
  pool: '线索池', assigned: '已分配', following: '跟进中', paid: '已成交', invalid: '无效',
};
