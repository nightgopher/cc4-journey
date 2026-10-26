# UI Library Selector Skill 打磨报告

打磨日期：2026-07-13

目标：把一个能用的跨组件库选型 Skill，打磨成可维护、可安装、可验证，并能和 design/frontend Skill 及组件库官方工具协作的公共 Skill 资产。

## 1. 验料结果（Skill 前提挑战）

挑战 1 - 真实问题：成立。设计 Skill 能给方法论，但通常不维护跨库组件对象池、检索入口和安装机制。

挑战 2 - 独特角度：唯一性来自“跨库路由工作流”。它同时处理 npm 主 UI 系统、headless primitives、shadcn registries 和动效库，并要求比较现有组件是否能达到质量目标。

挑战 3 - 安装理由：成立。临时问 Agent 容易遗漏项目现状、混用多个主库、全站抓取或输出过时命令；持久 Skill 能固定检查顺序和安全门。

挑战 4 - 公共传播性：钩子是“先选对组件材料，再让设计 Skill 把它做成真正的产品”。可见产物是组件来源、定向检索范围、预览命令、安装命令和拒绝项。

验料结论：好料，继续打磨。定位不需要改成另一个通用 design Skill。

## 2. 访行记录（同类 Skill 横向对标）

| 同类项目 | 链接 | 类型 | 一句话定位 | 可学的手艺 | 不能照搬的点 |
|---|---|---|---|---|---|
| component-craft | https://github.com/snqb/my-skills/blob/main/component-craft/SKILL.md | 直接同行，动效子域 | 在多个 React/Tailwind 视觉组件源中排序并给安装方式 | 视觉组件源排序 | 不覆盖 MUI、Ant、Mantine 等主系统 |
| shadcn-component-discovery | https://github.com/mattbx/shadcn-skills/blob/main/skills/shadcn-component-discovery/SKILL.md | 直接同行，registry 子域 | 在 shadcn-compatible registries 中发现组件 | 定向发现和 registry catalog | 不做 npm 主 UI 系统选型 |
| 官方 shadcn Skill + MCP | https://github.com/shadcn-ui/ui/blob/main/skills/shadcn/SKILL.md | 执行层近邻 | 项目感知地搜索、预览、安装 shadcn registry 资源 | `info/search/view/docs/dry-run/diff` 闭环 | 预设已经选择 shadcn |
| UI UX Pro Max | https://github.com/nextlevelbuilder/ui-ux-pro-max-skill | 设计决策近邻 | 按行业和页面类型推荐样式、颜色、字体与 UX 规则 | 数据化设计方向 | 不检索真实组件或生成组件安装路径 |
| Impeccable | https://github.com/pbakaus/impeccable | 设计方法近邻 | 用设计词汇、反模式和审查流程提升前端质量 | 清晰的 design skill 协作边界 | 不承担跨组件库选型 |
| Anthropic frontend-design | https://github.com/anthropics/claude-code/blob/main/plugins/frontend-design/skills/frontend-design/SKILL.md | 方法论近邻 | 强制明确视觉方向并避免通用 AI 审美 | 简洁、强约束的设计原则 | 缺少组件供给与安装机制 |
| 21st.dev MCP | https://21st.dev/mcp | 供给平台近邻 | 语义搜索并安装一个组件市场中的资源 | live search 和可见对象池 | 单一市场，不治理跨 UI 系统组合 |
| Storybook MCP | https://storybook.js.org/docs/ai/mcp/overview | 项目内复用近邻 | 让 Agent 搜索现有组件、props 和 stories | 先复用内部设计系统 | 不负责外部库比较和安装 |

访行结论：没有发现完整覆盖“页面分类 -> npm 主库选择 -> 动效来源选择 -> 定向搜索 -> 预览 -> 安装”的完全直接同行。

## 3. 生态位判断

纵向结论：该 Skill 从“组件库说明文档”演进为跨库选型路由器；下一阶段不应继续堆静态组件源码，而应强化 live tooling handoff 和回放验证。

横向结论：设计 Skill 强在审美和 critique，library-specific Skill/MCP 强在当前 API 和安装，市场工具强在单一供给池。

交叉洞察：真正该抢的生态位不是“最懂设计”或“组件最多”，而是“在设计意图、项目现状和多个官方组件工具之间做正确路由”。

一句话新定位：为 AI 前端工作选择最合适的组件来源，并把执行交给当前项目和官方 live tooling。

## 4. 过尺结果（活体检查 + 质量评分）

### 活体证据

- 旧版 `quick_validate.py`：通过，但这只能证明基础 YAML 结构有效。
- 旧版鲁班结构尺：6 PASS、7 WARN；项目中只有 README 和压缩包，没有展开源码、测试或维护脚本。
- 新版鲁班结构尺：9 PASS、4 WARN；LICENSE、源码、脚本和一行安装已补齐。
- 新版项目检查：全部通过，覆盖 metadata、口语触发、负触发、中途触发、过时 URL、私有路径和测试 JSON。
- 通用安装实测：`npx skills add <local-path> -a codex -y --copy` 在临时目录发现并安装 1 个 Skill，完整复制 SKILL.md 和 4 个 references。
- 用户级更新活体检查发现：通用 CLI 更新 canonical Skill 目录后，预先存在的客户端专属手工副本仍可能保持旧内容。最终通过实际加载路径同步、逐文件 diff 和 SHA-256 对账确认三份源码一致。
- 前向回放：数据后台和已有 shadcn 项目两例通过；营销页选型正确，但第一次仍输出 React Bits 旧裸 URL，证明“只把命令放 reference”不够。随后把命令门移到 SKILL.md 顶部并禁止无项目上下文时输出安装命令。
- 营销页修复后的全新独立回放保留 `shadcn/ui + 单一 React Bits` 路由，并在无项目、仅规划条件下不再输出安装命令，命令门通过。

| 维度 | 权重 | 打磨前 | 打磨后 | 主要证据 |
|---|---:|---:|---:|---|
| Frontmatter 与触发条件 | 7 | 5 | 7 | 正负触发、中文口语、中途触发、1024 字符校验 |
| 工作流清晰度 | 12 | 10 | 11 | intake、selection、handoff、response contract |
| 失败模式编码 | 12 | 3 | 10 | 无项目、文档冲突、混库、网络阻塞、版本门 |
| 检查点设计 | 6 | 4 | 6 | inspect、better-than-existing、preview、verify |
| 可执行具体性 | 17 | 12 | 15 | live search/view/docs/dry-run/diff 和 npm 版本分流 |
| 资源整合度 | 4 | 3 | 4 | 4 个 reference 按 heading 路由 |
| 整体架构 | 12 | 9 | 11 | 可维护源码、README、tests、scripts、LICENSE |
| 实测表现 | 23 | 11 | 16 | 3 类前向例通过，营销页问题经历失败、修复和独立重放 |
| 反例与黑名单 | 7 | 4 | 7 | 负触发、旧 URL、无预览安装、图标假设、多主库 |
| **总分** | **100** | **61** | **87** | 打磨后为实测分 |

## 5. 差距清单

### P0：公开发布前必须补

- 确定 GitHub owner/repo，替换 README 中的发布占位符。
- 从公网仓库实测一次 `npx skills add owner/repo`。
- 发布动作前确认许可证署名。

### P1：明显提升传播和可信度

- 在真实 React 项目中录制 30 秒演示：识别已有系统 -> 选择库 -> registry preview -> 安装 -> 验证。
- 把真实演示产物和可复现录制脚本放入仓库。
- 在真实项目上下文中再验证一次 namespaced search、dry-run 和 install 命令链。

### P2：后续增强

- 评估 21st.dev、Storybook MCP、Magic UI MCP、MUI MCP 的可选路由。
- 按季度核验 HeroUI、PrimeReact 等高变化库的版本门。

### 与同行相比，我们最缺的 3 件事

1. 真实 UI 项目端到端 demo。
2. 发布后的安装量和真实用户反馈。
3. 可自动检查官方命令漂移的远程验证。

### 与同行相比，我们最有机会打穿的 3 件事

1. 跨 npm 主 UI 系统与动效 registry 的统一决策。
2. “现有组件能不能更好”的质量比较，而不是只判断能不能做。
3. 与 design Skill、library Skill、MCP 的清晰分工和 handoff。

## 6. 三个打磨方向

### 方案 A：细修路由可靠性（本轮采用）

不改变定位，修触发边界、协作顺序、命令门、版本门和验证资产。优点是收益直接、风险低；缺点是 showcase 仍需真实项目。

### 方案 B：精雕 live discovery

增加 shadcn、Magic UI、MUI、Mantine、HeroUI 等官方 Skill/MCP 的能力探测和路由。优点是时效性更高；风险是客户端差异和配置成本上升。

### 方案 C：升级为 UI Skill 套件

拆成 selector、component-discovery、install-review、ui-audit 等多个 Skill。优点是每个触发更窄；风险是触发冲突、维护和安装复杂度显著增加。

推荐选择：方案 A。当前核心问题是可靠执行，不是功能数量不足。

## 7. 候选改写方案

本轮只刨：跨 Skill 路由和命令可靠性。

改动边界：不下载组件源码，不扩成通用 design Skill，不新增高风险自动安装脚本。

| 文件 | 操作 | 原因 |
|---|---|---|
| `ui-library-selector/SKILL.md` | 重写 | 压缩 metadata，增加正负触发、中途触发、handoff、命令门和失败模式 |
| `references/*.md` | 更新 | 采用 namespace、live discovery、版本门和官方 source map |
| `README.md` | 更新 | 说明生态位、协作顺序、一行安装、安全边界和验证方式 |
| `tests/test-prompts.json` | 新增 | 固化 11 个正负触发与选型回放场景 |
| `scripts/check-skill.sh` | 新增 | 固化 frontmatter、引用、旧 URL、私有路径和测试 JSON 检查 |
| `LICENSE` | 新增 | 补齐公共发布基础件 |

验证门结果：结构校验通过、通用 CLI 临时安装通过；3 类典型前向回放通过，其中营销页命令回放经历一次失败、规则修复和全新独立重放。

## 8. README 与 Showcase 升级

README 已补：一句话钩子、与 design/library Skill 的差异、触发边界、本地和 GitHub 安装、安全边界、验证命令。

本轮未伪造 GIF 或截图。公开发布前应使用真实项目录制，不用摆拍样例。

## 9. 执行计划

### 本轮已完成

- [x] 展开长期维护源码并冻结旧压缩包基线
- [x] 更新 SKILL.md 与全部 references
- [x] 增加测试样例、检查脚本和 LICENSE
- [x] 通过 `quick_validate.py` 和项目检查
- [x] 使用通用 skills CLI 在临时目录真实安装
- [x] 完成同行调研和 3 个前向回放，记录活体失败

### 发布前完成

- [ ] 确定 GitHub 仓库和许可证署名
- [ ] 从公网地址重跑安装
- [ ] 真实项目演示和可复现录制脚本
- [ ] 在真实 React 项目中回放 namespaced preview/install 命令链

### 本轮不做

- 不创建 GitHub 仓库、不 merge、不 tag、不发布 registry。
- 不安装所有组件库或缓存第三方源码。
- 不加入单一客户端专属 frontmatter。

## 10. 出师证书

```text
┌──────────────────────────────────────────────┐
│  出师证书 · 鲁班工坊                         │
│                                              │
│  作品：UI Library Selector                   │
│  过尺：61 分 -> 87 分（实测）                │
│  定位：设计意图与官方组件工具之间的选型路由  │
│  绝活：统一治理主 UI 系统与动效组件来源       │
│  下一步：真实项目 demo + 公网安装回放         │
│                                              │
│  验收师傅：鲁班                              │
└──────────────────────────────────────────────┘
```

## 11. 回炉清单

- 观察 `shadcn-ui/ui` 的 CLI、Skill、MCP 和 registry index 变化。
- 观察 `mattbx/shadcn-skills` 与 `component-craft` 的发现策略。
- 每次修改触发词或命令时，先更新测试样例，再运行项目检查和至少两个独立前向回放。
- 每季度核验 HeroUI、PrimeReact、Mantine 和 React Aria 的安装与 AI tooling。
- release notes 必须解释为什么改，不只列文件。

## 12. 需要用户确认的问题

1. 公开仓库准备使用哪个 GitHub owner/repo？
2. LICENSE 的版权署名使用个人名、团队名还是 `contributors`？
3. 发布前是否要选择一个真实 React 项目制作 30 秒 demo？

## 13. 附录：主要来源

- https://ui.shadcn.com/docs/skills
- https://ui.shadcn.com/docs/mcp
- https://ui.shadcn.com/docs/cli
- https://github.com/vercel-labs/skills
- https://github.com/DavidHDev/react-bits
- https://magicui.design/docs/installation
- https://ui.aceternity.com/docs/cli
- https://github.com/mattbx/shadcn-skills
- https://github.com/snqb/my-skills/blob/main/component-craft/SKILL.md
- https://github.com/nextlevelbuilder/ui-ux-pro-max-skill
- https://github.com/pbakaus/impeccable
- https://github.com/anthropics/claude-code/blob/main/plugins/frontend-design/skills/frontend-design/SKILL.md
- https://21st.dev/mcp
- https://storybook.js.org/docs/ai/mcp/overview
