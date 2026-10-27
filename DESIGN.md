---
name: "呼叫中心 4.0"
description: "服务教育电销日常核对与录入的蓝白业务工作台"
colors:
  accent: "#315ce0"
  accent-hover: "#2147bd"
  ink: "#182b49"
  ink-soft: "#53647e"
  paper: "#f5f7fb"
  surface: "#ffffff"
  line: "#dce3ed"
  filter-surface: "#edf1f8"
  table-heading: "#f4f6fb"
  hover-surface: "#eef2fb"
  field-stroke: "#a9b2a8"
  ghost-hover-stroke: "#aab8d2"
  income: "#1f7354"
  outflow: "#b64843"
  outflow-hover: "#8f2f20"
  status-neutral-surface: "#ecefe8"
  status-neutral-ink: "#42564d"
  status-good-surface: "#e9f3ec"
  status-good-ink: "#24623f"
  status-bad-surface: "#faeee8"
  status-bad-ink: "#963d2b"
  error-surface: "#fbeee9"
  error-ink: "#922e20"
  nav-label: "#d0d9e9"
  nav-group: "#a8b9d4"
typography:
  headline:
    fontFamily: "-apple-system, \"PingFang SC\", \"Microsoft YaHei\", \"Segoe UI\", sans-serif"
    fontSize: "1.4rem"
    fontWeight: 650
    lineHeight: 1.4
  title:
    fontFamily: "-apple-system, \"PingFang SC\", \"Microsoft YaHei\", \"Segoe UI\", sans-serif"
    fontSize: "1.05rem"
    fontWeight: 600
    lineHeight: 1.6
  body:
    fontFamily: "-apple-system, \"PingFang SC\", \"Microsoft YaHei\", \"Segoe UI\", sans-serif"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: 1.6
  label:
    fontFamily: "-apple-system, \"PingFang SC\", \"Microsoft YaHei\", \"Segoe UI\", sans-serif"
    fontSize: "0.8125rem"
    fontWeight: 500
    lineHeight: 1.6
  caption:
    fontFamily: "-apple-system, \"PingFang SC\", \"Microsoft YaHei\", \"Segoe UI\", sans-serif"
    fontSize: "0.75rem"
    fontWeight: 400
    lineHeight: 1.6
  metric:
    fontFamily: "-apple-system, \"PingFang SC\", \"Microsoft YaHei\", \"Segoe UI\", sans-serif"
    fontSize: "1.6rem"
    fontWeight: 600
    lineHeight: 1.6
rounded:
  status: "4px"
  control: "5px"
  button: "6px"
  date: "7px"
  container: "8px"
  panel: "10px"
  dialog: "12px"
spacing:
  xs: "4px"
  sm: "8px"
  md: "12px"
  form-gap: "14px"
  lg: "16px"
  panel-inset: "20px"
  xl: "24px"
  2xl: "32px"
  3xl: "48px"
components:
  button-primary:
    backgroundColor: "{colors.accent}"
    textColor: "{colors.surface}"
    rounded: "{rounded.button}"
    padding: "7px 13px"
  button-primary-hover:
    backgroundColor: "{colors.accent-hover}"
  button-ghost:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.button}"
    padding: "7px 13px"
  button-ghost-hover:
    backgroundColor: "{colors.hover-surface}"
  button-danger:
    backgroundColor: "{colors.outflow}"
    textColor: "{colors.surface}"
    rounded: "{rounded.button}"
    padding: "7px 13px"
  button-danger-hover:
    backgroundColor: "{colors.outflow-hover}"
  input:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    padding: "8px 10px"
  navigation-active:
    backgroundColor: "{colors.accent}"
    textColor: "{colors.surface}"
    rounded: "{rounded.button}"
    padding: "9px 11px"
  status-neutral:
    backgroundColor: "{colors.status-neutral-surface}"
    textColor: "{colors.status-neutral-ink}"
    rounded: "{rounded.status}"
    padding: "2px 7px"
  status-good:
    backgroundColor: "{colors.status-good-surface}"
    textColor: "{colors.status-good-ink}"
    rounded: "{rounded.status}"
    padding: "2px 7px"
  status-bad:
    backgroundColor: "{colors.status-bad-surface}"
    textColor: "{colors.status-bad-ink}"
    rounded: "{rounded.status}"
    padding: "2px 7px"
  panel:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.panel}"
    padding: "20px"
  day:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.date}"
    padding: "12px 16px"
  day-selected:
    backgroundColor: "{colors.accent}"
    textColor: "{colors.surface}"
  table-heading:
    backgroundColor: "{colors.table-heading}"
    textColor: "{colors.ink-soft}"
    padding: "11px 16px"
  dialog:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.dialog}"
    width: "560px"
    padding: "24px"
---

# Design System: 呼叫中心 4.0

## Overview

**Creative North Star: "教务周计划"**

以教务周计划的清楚分区与日期节奏组织教育电销工作。海军蓝导航提供稳定归属，冷白工作区承载列表、表单与对比数据，蓝色承担操作和当前选择。界面紧凑、克制，适合在办公屏幕上连续核对与录入。

“教务周计划”提供色彩、密度与可操作七日日期条这一处特征。业务页面继续使用熟悉的标题、筛选、表格与操作顺序；中文系统字体、等宽数字和原生控件优先保障辨认、输入与键盘操作。

**Key Characteristics:**

- 海军蓝导航、白色内容容器与冷白工作区。
- 紧凑中文信息层级，金额与数量采用等宽数字。
- 稳定业务表格、原生表单与有未保存保护的短任务弹窗。
- 七日日期条保持区间，日期输入负责改变区间。

提取依据：`app/web/src/styles.css` 的最终层叠结果、`lib/tokens.ts`、`components/kit.tsx` 与 `pages/shared.tsx`、`pages/core.tsx`、`pages/business.tsx`。已对照桌面、手机及长表单首尾截图。前言令牌为机器可读规范；同名值以最终 CSS 为准，备用但未渲染的 JS 令牌不自动成为规范。

## Colors

主色清晰，底色偏冷；低饱和语义底色帮助长时间核对。色值以 frontmatter 为准，以下说明用途。

### Primary

- **操作蓝（accent）**：主要按钮、活动导航、选中日期、链接、图表当前日期、文本选择和键盘焦点。
- **深操作蓝（accent-hover）**：主按钮悬停，保持白色文字。

### Neutral

- **海军蓝（ink）**：正文、标题与整块侧栏底色。
- **蓝灰（ink-soft）**：说明、标签、表头、次级数字和辅助文字。
- **冷白（paper）与纯白（surface）**：前者是工作区，后者是表格、面板、控件和弹窗。
- **浅分隔线（line）**：容器边界、表头与工作区分隔。
- **筛选浅蓝（filter-surface）与表头冷白（table-heading）**：区分筛选区与数据标题行。
- **操作浅蓝（hover-surface）与悬停描边（ghost-hover-stroke）**：次按钮悬停反馈。
- **控件描边（field-stroke）**：原生输入、选择与多行输入的静态边界。
- **导航浅字（nav-label）与分组灰蓝（nav-group）**：海军蓝背景上的业务入口与分组名称。

### 业务语义

- **入账绿（income）与扣减红（outflow）**：金额的收入、提成与扣减语义；危险操作使用扣减红，悬停使用 outflow-hover。
- **状态中性、通过、异常（status-*）**：成对使用浅底与深字；具体中文状态由业务映射提供。
- **错误浅底与错误深字（error-*）**：字段提交、请求失败等可恢复错误，旁边可放“重试”。

**The Semantic Color Rule.** 业务状态必须同时给出中文文字；绿色和红色只表达业务语义，蓝色表达操作与选择。

高对比偏好会加强分隔线并加深次级文字；新增样式应继续使用 CSS 变量，保留这一覆盖能力。阴影在 sidecar 扩展中记录，遮罩透明色保留在弹窗组件片段中，不作为主调色板使用。

## Typography

**Body Font:** 中文优先的系统字体栈，按 frontmatter 的 `fontFamily` 使用。页面标题、标签和数据沿用同一字体，不建立独立展示字体规范。

**Character:** 字号紧凑，依靠字重、对齐和留白建立业务层级。文字说明最长按现有段落宽度控制（72ch）；表格以核对为目标，不为装饰改变数字排列。

### Hierarchy

- **Headline**：业务页标题，尺寸与行高见 headline；保持短而明确。
- **Title**：面板与弹窗标题，使用 title。
- **Body**：正文与控件输入，使用 body。
- **Label**：字段标签使用 label；按钮、导航和表格正文同尺寸，其中按钮为半粗（600），导航默认常规、活动项为半粗（600）。
- **Caption**：表头、状态和辅助说明以 caption 尺寸为基准；表头为半粗（600），状态为中等（500）。侧栏分组与日期星期是更小的局部辅助文字，不用于正文。
- **Metric**：摘要数值使用 metric；窄屏降至 headline 的尺寸，字重保持半粗（600）。货币与数字使用比例一致的等宽数字特性，不切换为独立等宽字体。

**The Numeric Alignment Rule.** 可比较的金额与数量启用 tabular-nums；表格数值列右对齐，金额不换行。

## Layout

工作台为持续可见的侧栏与可收缩内容区。桌面侧栏固定宽度（220px），吸附视口顶部并占满视口高度（100dvh）；导航自身滚动，用户区留在底部。内容区保持 `min-width: 0`，主区最大宽度（1510px），常规内边距为上（30px）、左右（32px）、下（48px）。顶部工作栏最小高度（65px）。

页头把任务标题与主要操作分放两侧。筛选可换行，数据容器独立滚动，分页在下。通用间距以 frontmatter 的 spacing 为准；大区域常用（24px），操作组（8px），表单纵向与双列表单间距（14px）。面板内边距（20px）。摘要通过上下分隔线连接，不把每个指标做成浮起卡片。

经营概览的两个对比面板使用约（1.65 : 1）的列关系，右列最低（260px）；在（1100px）及以下合并为单列。这个断点同时把侧栏收为（196px），主区与工作栏左右内边距收为（24px）。

在（760px）及以下，侧栏改为可开合抽屉（236px），工作栏隐藏日期，主内容左右内边距（16px），摘要变为两列，页头和操作纵向排列。筛选换行，业务表格保持列语义并在容器内横向滚动。双列表单变单列，面板内边距（16px）。底部留白包含 safe-area inset。按钮与图标操作通常扩为触摸高度（44px）；密集表格行操作保留较小高度（36px）。

手机抽屉及遮罩仅覆盖实际物理视口；全页截图中视口以下的页面仍属于正常文档流。打开抽屉时锁定页面滚动、使工作区 inert、循环导航焦点；Escape、遮罩和关闭按钮均可收起，随后恢复触发按钮焦点。

## Elevation & Depth

工作区采用底色、边界与间距建立层级。表格、筛选、日期条和常规面板没有外投影。遮罩明确把注意力切换到短任务，固定反馈与弹窗才需要浮起层次。

### Shadow Vocabulary

- **短任务弹窗**：柔和大范围投影，值见 sidecar `dialog`；配合半透明背景遮罩。
- **固定反馈**：较轻投影，值见 sidecar `notice`；固定在页面右下方，窄屏保留安全边距。

**The Flat Workspace Rule.** 常规工作区、筛选和数据容器保持平面；仅弹窗与固定反馈使用外投影。

## Shapes

使用小幅圆角和细描边，按组件而非统一大圆角处理：状态最紧，输入略圆，按钮与导航同形，数据容器、面板、弹窗逐步放宽；对应值均在 rounded 中。日期条保留自身的小圆角。普通边框为单线（1px），数据行以细分隔线对齐，图表条形的圆角仅为局部绘制细节。

图标使用内联 SVG，沿用同一画布（24 × 24）、细笔画（1.6）、圆端与圆角连接，通常显示为（18px）。仅装饰图标隐藏于辅助技术，图标按钮有中文可访问名称。

## Components

### Buttons

按钮短小、清楚，以文字动作命名。主按钮为蓝底白字，次按钮为白底海军蓝字和边界，危险按钮为红底白字。圆角、内边距与颜色使用 frontmatter 中对应变体；常规最小高度（36px），字号与 label 同级，字重（600）。

悬停分别使用已记录的变体。禁用时降低不透明度（0.5），不执行提交。全局可见焦点使用主色描边（2px）与外偏移（3px），保留在按钮、链接、表格滚动区域及原生控件上。按钮与链接仅在背景、文字、边框颜色间短暂过渡（160ms，ease-out）；不增加入场或位移动画。reduced-motion 关闭动画和过渡。

### Chips

状态标签使用小圆角、浅底深字与紧凑内边距；中性、通过、异常的三组颜色在 frontmatter 中成对定义。标签文字写明实际状态，不把任意非空状态都染成成功色。缺失状态显示破折号。

### Cards / Containers

常规面板为白底、细线、小圆角；默认内边距与窄屏变化见 Layout。嵌入面板的表格移除重复外框，仍保留表头与行分隔。筛选使用单独浅底，摘要使用分隔线，使控制、数值和记录容易区分。

### Inputs / Fields

使用原生 `input`、`select`、`textarea`，标签始终可见并与控件关联。默认白底、控件描边、内边距与圆角见 input 令牌，最小高度（38px）。多行文本允许纵向调整，日期保留浏览器原生日期输入。复选与单选使用主色 accent，控制本体为（17px）。焦点进入字段时标签改为主色，同时保留全局焦点环。

错误通过中文 alert 说明；当前实现通过原生校验处理必填、类型与数值范围。登录字段提供用户名与当前密码 autocomplete，业务字段保留明确 name。

### Navigation

侧栏按业务分组；普通入口使用导航浅字与线性 SVG。悬停轻微提亮背景，活动入口直接使用当前主色蓝底、白色半粗文字和 `aria-current="page"`，不沿用被覆盖的旧色值或内投影。入口是带真实目标的链接，保留修改键打开行为。跳至主要内容链接在键盘焦点时可见；窄屏行为见 Layout。

### Tables and feedback

表格使用语义 `table`、列标题和稳定列顺序。表头采用浅底、辅助字色与半粗文字；数据行按现有密度保留纵向内边距（13px），表头为（11px），左右均为（16px）。行悬停轻微提亮。普通文字列左对齐，可比较数值右对齐；长说明在自己的单元格中换行，不撑开页面。

通用列表以每页（20条）载入，分页显示总数和当前页。加载时显示中文状态和静态占位条，区域标记 busy；失败显示中文 alert 与重试，隐藏失败请求的数据；空结果保留数据区域，显示图标与原因文字。保存成功反馈明确，失败保留表单内容。请求参数变化时立即隐藏旧结果，等待对应结果或错误，避免把旧数据标在新日期或筛选下。

### 七日日期条

七个等宽日期按钮连成可操作时间索引，保留星期、短日期、订单数三层信息。选中项使用蓝底白字和 `aria-pressed`；普通项白底细线，悬停边界变蓝。桌面与窄屏都保留七列，窄屏缩小内边距和辅助字。

选择条内某一天只切换当天汇总，七日区间不移动；使用上方原生日期输入时，同时改变当天和区间结束日。日期与区间进入 URL。趋势图保持真实数据比例，并提供可展开的数据表与可访问文字；选中日之外的柱形使用较浅蓝，状态以实际请求参数为准。

### Dialogs

短任务使用原生 modal dialog，桌面宽度（560px），视口左右至少留（16px），最大高度（90dvh）。内容区域独立滚动，标题与 footer 不参与压缩；默认内边距（24px），窄屏为（20px）。底部操作区保留分隔线，取消和保存始终可达。sidecar 中的弹窗仅演示静态外观，线上行为来自 React 与原生 dialog。

打开时聚焦第一个可用字段，关闭时恢复触发控件焦点。Escape、取消、关闭按钮和背景点击都通过同一关闭逻辑；有未保存内容时显示“继续编辑 / 放弃更改”，刷新页面也受保护。保存中显示“保存中…”并禁用重复操作和关闭；失败在弹窗内显示错误，成功关闭并刷新对应数据。

## Do's and Don'ts

### Do:

- **Do** 复用主色、海军蓝文字、冷白底与现有语义状态；新增状态同时写明中文含义。
- **Do** 保持页面标题、筛选、数据和分页的稳定阅读顺序，并让表格容器独立横向滚动。
- **Do** 为金额和数量保留等宽数字、数值列右对齐及清楚的单位。
- **Do** 使用带可见标签的原生控件、可见焦点、中文加载与可重试错误。
- **Do** 在窄屏检查弹窗内容滚动和底部操作可达，并保留未保存退出保护。

### Don't:

- **Don't** 用颜色单独区分审批、启用或财务状态。
- **Don't** 在切换业务日期后用上一请求的结果充当新日期数据。
- **Don't** 把密集表格变成页面级横向溢出，或为了手机布局隐藏关键业务列。
- **Don't** 给常规列表、筛选和摘要叠加装饰投影、入场动画或无关图像。
- **Don't** 把登录宣传标题的尺寸、未使用的备用令牌或旧版被覆盖的样式推广为新页面规范。
