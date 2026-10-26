# UI Library Selector Badcase 打磨报告

打磨日期：2026-07-13

本轮目标：修复“公开个人网站 + 独立后台”被压成一个全局 UI 选型的问题，并把效果源实证、三层选型与渲染验收固化为可回归规则。

## 1. 验料结果（Skill 前提挑战）

挑战 1 - 真实问题：**成立**。AI 常把仓库或品牌误当成选型边界，导致后台的稳定性权重覆盖公开站的视觉目标。

挑战 2 - 独特角度：唯一性来自**工作流与验证资产**。它不是另一个 design Skill，而是按产品表面在现有组件、npm 系统、headless primitives、effect registries 与 animation orchestrators 之间做路由。

挑战 3 - 安装理由：临时问 Agent 容易得到“选一个最稳妥的全局库”；持久 Skill 可以固定 surface gate、目标权重、三层输出、live inspection、dry-run 与 rendered evidence gate。

挑战 4 - 公共传播性：钩子是“**不要给整个仓库选一个库，要给每个产品表面选对三层材料**”。可展示产物是 surface map、权重表、三层选型表、候选证据表与 before/after 回放。

验料结论：**好料，继续打磨**。问题不是定位不成立，而是原工作流的决策单元和验证门不够强。

## 2. 访行记录（同类 Skill 横向对标）

| 同类 Skill | 链接 | 类型 | 一句话定位 | 它为什么容易被理解/安装/传播 | 可学的手艺 | 不能照搬的点 |
|---|---|---|---|---|---|---|
| UI/UX Pro Max | https://github.com/nextlevelbuilder/ui-ux-pro-max-skill | 直接/间接 | 按产品类型检索设计规则并生成设计系统建议 | 名字清楚、数据与 CLI 形成可见产物 | 产品类型到视觉目标的显式映射、结果卡 | 不负责真实组件源码、依赖与 dry-run 对账 |
| shadcn 官方 Skill | https://github.com/shadcn-ui/ui/blob/main/skills/shadcn/SKILL.md | 直接，单生态执行层 | 项目感知地搜索、查看、安装与更新 registry 组件 | 官方权威，命令链具体 | `info -> search -> docs/view -> dry-run/diff -> install -> review` | 预设已经选择 shadcn，不能替代跨库决策 |
| Anthropic Frontend Design | https://github.com/anthropics/skills/tree/main/skills/frontend-design | 间接 | 建立有辨识度的视觉方向并避免模板化 AI 审美 | 一句话价值强、约束简洁 | 一个主导效果、先定视觉 thesis | 不搜索组件、不检查依赖和安装写入 |
| Taste Skill | https://github.com/Leonxlnx/taste-skill | 手艺 | 面向 landing/portfolio 的 anti-slop 设计框架 | surface 边界清楚，README 与示例完整 | 明确排除 dashboard、用目标旋钮表达差异 | “one system per project”若不先拆 surface 会复现本 badcase |
| Ultimate Frontend | https://clawhub.ai/nemesis0017/skills/ultimate-frontend | 手艺/间接 | 聚合研究、设计、taste 与 critique 的前端工作流 | all-in-one 定位易懂，完整 Skill 可直接浏览 | before/after/why 展示与分层协作 | 聚合面广，但缺少组件级实证链 |
| Storybook MCP | https://storybook.js.org/docs/ai/mcp/overview | 间接，项目内复用 | 让 Agent 发现现有组件、props 与 stories | 真实项目组件可见，复用路径短 | 先查内部设计系统再引外部库 | 不负责外部组件源比较或效果源决策 |

访行结论：没有找到完整覆盖“**surface decomposition -> weighted objectives -> three layers -> exact candidate evidence -> rendered acceptance**”的公开同行。最值得组合的手艺是 Taste 的 surface 边界、UI/UX Pro Max 的显式目标、shadcn 的 live command chain，以及 Frontend Design 的单一主导效果。

## 3. 生态位判断

纵向结论：这个 Skill 从“跨库目录”演进为“证据驱动的材料路由器”。下一阶段不应继续堆组件名，而应提高 surface 判断、候选实证和渲染验收的可靠性。

横向结论：设计 Skill 强在视觉方向，library Skill/MCP 强在单一生态执行，Storybook 强在项目内复用；中间仍缺一个能防止跨 surface 权重污染的选择层。

交叉洞察：真正该抢的生态位不是“最全 React 组件库推荐”，而是“**按产品表面独立优化，并用 live evidence 证明材料选择**”。

一句话新定位：**把一个 React 项目拆成正确的产品表面，为每个表面选择行为基础、视觉效果与动画编排，并在安装和渲染证据齐全前保持结论可证伪。**

## 4. 过尺结果（活体检查 + 质量评分）

### 活体证据

- 冻结版静态检查全绿，但对混合描述的独立回放仍给出“全站唯一 Mantine”，证明结构检查没有覆盖真实决策失败。
- 冻结版回放识别了两个 surface，却没有数值权重、独立动画编排层、精确候选证据或 rendered evidence gate。
- 更新后 multi-surface 回放通过 8 个 must 与 5 个 must_not；显式 wow 回放 7/7；admin guardrail 回放 8/8，合计 **28/28**。
- React Bits 官方 registry manifest 实查：`Threads` 依赖 `ogl`；`LiquidEther` 依赖 `three`；`Dither` 依赖 React Three Fiber、postprocessing 与 `three`。三者的降级和运行成本不能从预览名推断。
- 分发包活体对账首次发现 5/5 文件仍是冻结版；新增 archive parity gate 后先红灯，再重打包，最终 5/5 一致。
- 当前项目没有可用于实现该 UI 的目标 React 应用，因此 registry dry-run、真实桌面/移动渲染和 FPS/trace 仍明确标为 pending，没有伪造“哇塞”证据。

| 维度 | 权重 | 打磨前 | 打磨后 | 主要证据 | 当前最大短板 | 优先级 |
|---|---:|---:|---:|---|---|---|
| Frontmatter 与触发条件 | 7 | 7 | 7 | 正负触发、中文口语、中途触发、metadata 长度门 | 无 | P2 |
| 工作流清晰度 | 12 | 9 | 12 | 新增 Product Surface Gate、权重与三层顺序 | 真实项目演示未录制 | P1 |
| 失败模式编码 | 12 | 7 | 11 | 全局库捷径、wow+none、未实证 final 均编码为失败 | 尚无更多真实边界案例 | P1 |
| 检查点设计 | 6 | 4 | 6 | search/view/dry-run 与 Rendered Evidence Gate | 无目标项目执行证据 | P1 |
| 可执行具体性 | 17 | 13 | 16 | 三候选证据字段、Motion 路由、exact command chain | 未跑项目级 dry-run | P1 |
| 资源整合度 | 4 | 4 | 4 | decision matrix、search index、install commands、catalog 同步 | 无 | P2 |
| 整体架构 | 12 | 10 | 11 | README、tests、examples、report、archive parity | 缺公开 demo/marketplace 元数据 | P1 |
| 实测表现 | 23 | 13 | 18 | 前后独立回放、28/28、官方 manifest 活检、分发包对账 | 无真实 UI 渲染/FPS | P0（发布前） |
| 反例与黑名单 | 7 | 6 | 7 | admin 不过触发效果、不得用后台权重压公开站 | 无 | P2 |
| **总分** | **100** | **73** | **92** | **模型回放实测；真实 UI 渲染待目标项目** |  |  |

## 5. 差距清单

### P0：不补就不能把视觉效果宣称为已验证

- 在一个真实 React 项目中完成 `Threads`、`LiquidEther`、`Dither` 的 `search`、`view`、`add --dry-run` 与源码对账。
- 对同一首屏录制桌面与移动端前三秒，记录设备/视口/节流条件、performance trace、reduced-motion 和 static fallback。

### P1：补上后明显提升安装率/传播率

- 录制可复现 demo，并把录制脚本和产物一起入库。
- 有公开 owner/repo 后补公网 `npx skills add` 实测、skills.sh 徽章与 marketplace 元数据。
- 给结果表增加可截图的真实 rendered before/after，而不是只展示文字回放。

### P2：锦上添花

- 增加官方命令漂移的定期远程检查。
- 收集更多“同仓库、不同 surface”案例，例如 marketing + SaaS、login + data workspace。

### 与同行相比，我们最缺的 3 件事

1. 真实 UI 项目端到端录像。
2. 可复查的移动性能数据。
3. 公网安装和市场展示信号。

### 与同行相比，我们最有机会打穿的 3 件事

1. 以 product surface 而非 repository 为选型单元。
2. 强制输出 behavior/effect/orchestration 三层，避免把不同性质的库当竞品。
3. 用 source/dependency/dry-run/render evidence 阻止保守默认值和预览即结论。

## 6. 三个打磨方向

### 方案 A：细修——只补 multi-surface 规则

定位：保持原 Skill，只增加拆分提醒。改动小、风险低，但模型仍可能给一个全局库，也无法证明效果候选真的更好。

### 方案 B：精雕——把 surface-aware 选型做成可验证工作流（本轮采用）

定位：surface map + 目标权重 + 三层输出 + effect gate + rendered gate + 回归测试。优点是直接修根因并留下验证资产；风险是 planning 输出略长，需要 response contract 控制。

### 方案 C：开套件——拆成 selector、effect-audit、install-review、visual-acceptance

定位：每个 Skill 更窄。优点是触发精准；风险是安装、协调和维护复杂度显著增加，当前证据不足以支持拆套件。

推荐选择：**方案 B**。用户已明确给出 badcase、正确分层与优化指令，本轮方向已经锁定，无需停在概念建议。

## 7. 候选改写方案

本轮只刨：**选型决策架构**。

改动边界：修改 surface 拆分、权重、三层来源、效果实证与渲染验收；不扩成通用 design Skill，不自动安装组件，不创建高风险脚本，不发布或部署。

预期提升：从“识别 surface 但仍做全局折中”变为“每个 surface 独立决策，视觉结论必须有证据”。

验证方式：冻结版与更新版独立回放、admin 负向 guardrail、官方 registry manifest 活检、静态规则检查、分发包逐文件一致性。

### 建议文件变更

| 文件 | 操作 | 原因 |
|---|---|---|
| `ui-library-selector/SKILL.md` | 修改 | 增加 surface/effect/render gates 与三层 response contract |
| `references/decision-matrix.md` | 修改 | 增加权重模板、三层矩阵和 badcase 示例 |
| `references/component-search-index.md` | 修改 | 增加三候选证据字段和 motion orchestration |
| `references/install-commands.md` | 修改 | 固化 React Bits search/view/dry-run 链与 Motion 当前安装入口 |
| `references/library-catalog.md` | 修改 | 区分 Motion orchestrator 与 UI/effect source |
| `tests/test-prompts.json` | 修改 | 增加 multi-surface 与 explicit-wow 回归契约 |
| `scripts/check-skill.sh` | 修改 | 固化新规则与 archive parity 检查 |
| `README.md` | 修改 | 用多产品表面示例解释新定位 |
| `examples/multi-surface-regression-results.md` | 新增 | 保存真实前后回放与官方源码活检 |
| `ui-library-selector-skill.tar.gz` | 重建 | 防止分发物继续携带冻结版 |

### 关键改写片段

> Before selecting any library, split the project into product surfaces by audience, permission boundary, primary job, and visual objective. Do not force a project-wide primary library conclusion before evaluating every surface independently.

> Resolve three layers per surface: behavior foundation, visual effect source, and motion orchestration.

> When the user explicitly prioritizes wow, memorable motion, high impact, technological atmosphere, or showcase quality for a public surface, effect-source evaluation is mandatory. `None` is valid only when live evidence shows that candidates fail a performance, accessibility, or brand-fit gate.

## 8. README 与 Showcase 升级

README 已完成：

- 从“判断 UI 类型”升级为“先拆产品表面”；
- 把一个主库/可选动效改成三层来源；
- 增加公开 Portfolio + Admin CMS 的权重与选型示例；
- 明确无目标项目时不输出安装命令；
- 明确缺少 rendered evidence 时不得宣称“哇塞”。

Showcase 已新增文字版真实回放与 source manifest 对账。没有伪造 GIF、FPS 或 before/after 截图；这些必须来自下一步真实 React 项目。

## 9. 执行计划

### 本轮已完成

- [x] 冻结旧版并记录 hash
- [x] 联网覆盖直接、间接、手艺同行
- [x] 复现“识别两个 surface 仍选全局 Mantine”
- [x] 写入 surface gate、权重、三层选型、effect gate、rendered gate
- [x] 增加两个正向回归和一个 admin guardrail
- [x] 检查 React Bits 三个官方 registry manifest
- [x] 固化 archive parity gate 并重建分发包
- [x] 独立回放 28/28 通过

### 3 天内可完成

- [ ] 在真实 React 项目执行三个候选的完整命令链与源码 diff
- [ ] 记录桌面/移动性能、reduced-motion 与 static fallback

### 7 天内可完成

- [ ] 录制可复现 demo
- [ ] 有公开仓库后验证公网安装与 README 渲染

### 本轮不做

- 不创建 GitHub 仓库，不 commit/push，不 merge，不打 tag，不发布 registry。
- 当前目录不是 Git 仓库，因此也不存在可安全提交的本地 commit。
- 不为缺失的目标 React 项目伪造 dry-run、FPS、截图或录像。

## 10. 出师证书

```text
┌──────────────────────────────────────────────────┐
│  出师证书 · 鲁班工坊                             │
│                                                  │
│  作品：UI Library Selector                       │
│  过尺：73 分 -> 92 分（模型回放实测）            │
│  定位：按产品表面选择三层 UI 材料并用证据验收    │
│  绝活：阻止 Admin 权重压制公开站的视觉目标       │
│  实证：3 组回放 28/28；分发包 5/5 对账通过       │
│  下一步：真实项目三候选渲染与移动性能回放        │
│                                                  │
│  验收师傅：鲁班                                  │
└──────────────────────────────────────────────────┘
```

## 11. 回炉清单

- 观察 shadcn CLI/Skill 的 `search`、`view`、`dry-run` 与 registry namespace 变化。
- 观察 React Bits 的依赖、offscreen pause、reduced-motion 与 fallback 改进。
- 观察 Motion for React 的安装、React 前置条件、reduced-motion 与 View Transitions 集成。
- 每次修改 surface/effect 规则，至少回放 multi-surface、explicit-wow、admin-no-effect 三类 prompt。
- 每次改 distributable Skill 文件，必须运行 archive parity gate。
- 下一轮从真实项目的三个候选 dry-run 和渲染证据开始，不重新争论“是否该拆 surface”。

## 12. 需要用户确认的问题

本轮没有阻塞问题。发布、创建远程仓库、merge、tag 和 registry 发布均未获授权，也未执行。

## 13. 附录：参考来源

- https://github.com/nextlevelbuilder/ui-ux-pro-max-skill
- https://github.com/shadcn-ui/ui/blob/main/skills/shadcn/SKILL.md
- https://github.com/anthropics/skills/tree/main/skills/frontend-design
- https://github.com/Leonxlnx/taste-skill
- https://clawhub.ai/nemesis0017/skills/ultimate-frontend
- https://storybook.js.org/docs/ai/mcp/overview
- https://github.com/DavidHDev/react-bits
- https://github.com/DavidHDev/react-bits/blob/main/src/ts-tailwind/Backgrounds/Threads/Threads.tsx
- https://github.com/DavidHDev/react-bits/blob/main/src/ts-tailwind/Backgrounds/LiquidEther/LiquidEther.tsx
- https://github.com/DavidHDev/react-bits/blob/main/src/ts-tailwind/Backgrounds/Dither/Dither.tsx
- https://motion.dev/docs/react
- https://motion.dev/docs/react-installation
