# Skills 库

本目录收录本次演练要用的研发技能，分三部分：**自沉淀的全流程技能**、**mattpocock/skills 精选**、**培训文档引用的外部技能**。

## 1. 自有技能

- 根目录 [`SKILL.md`](../SKILL.md)：`zero-to-one-delivery`——本次演练自沉淀的全流程技能，定义"旅程路线"；下面两批外部技能提供各站点的"操作手法"。

## 2. mattpocock/skills（来源：https://github.com/mattpocock/skills）

> Matt Pocock 的 "Skills for Real Engineers"（30k+ stars），取自其 `.agents` 目录。以下为人工挑选的 11 个，保留原文，未做改写。

| 演练阶段 | 技能 | 作用 |
|-|-|-|
| 01 需求澄清 | `productivity_grilling` | 逐轮追问把模糊需求磨清：每轮提"当前可答"的全部问题并附推荐答案 |
| 01 需求澄清 | `productivity_to-questionnaire` | 把答不了的决策变成问卷，交给客户异步填写 |
| 02 PRD / 03 设计 | `engineering_to-spec` | 把对话收敛成规格，发布到 issue 跟踪器 |
| 03 设计 | `engineering_domain-modeling` | 建领域模型：术语表（GLOSSARY）+ 架构决策记录（ADR） |
| 04 开发 | `engineering_tdd` | 红-绿-重构循环，测试先行的纪律与反模式 |
| 04 开发 | `engineering_implement-spec` | 按规格 + 票据在集成分支上实施 |
| 04 开发 | `engineering_prototype` | 快速原型：先验证再固化（LOGIC/UI 两份参考） |
| 04 排错 | `engineering_diagnosing-bugs` | 疑难 bug 的分阶段诊断循环 |
| 05 交付 | `engineering_code-review` | 双轴评审：规范符合性 + 规格符合性，并行子代理执行 |
| 05 交付 | `engineering_pr` | PR 描述模板（源自 Humanlayer show-me） |
| 会话交接 | `productivity_handoff` | 把当前会话压缩成交接文档，供下一个 agent 续接 |

目录：`mattpocock/engineering_*` 与 `mattpocock/productivity_*`，对应上游 `skills/<category>/<name>/`。

## 3. 培训文档引用的外部技能（`external/`）

> 来源：《AI 全栈开发方法与实践》培训文档（飞书）3.1–3.2 节引用的开源 Skills。课程附带的 zip 包（forge-prd-entity v2.0、frontend-skill、ui-library-selector 完整包）因 Drive API 权限暂未能下载，其中两个已从其公开 GitHub 仓库获取，forge-prd-entity 本地已有安装副本。

| 技能 | 来源仓库 | 用途（培训文档 3.x 节） |
|-|-|-|
| `external/anthropic-official/skills/frontend-design` | anthropics/skills | 3.2.1 界面设计：为页面形成明确的设计方向，避免通用模板脸 |
| `external/anthropic-official/skills/canvas-design` | anthropics/skills | 3.2.1 静态视觉素材（海报/封面 PNG/PDF） |
| `external/anthropic-official/skills/web-artifacts-builder` | anthropics/skills | 3.2.3 在 Claude 中交付多组件交互演示 artifact |
| `external/web-design-guidelines-vercel` | vercel-labs/agent-skills | 3.2.4 按 Web Interface Guidelines 审查 UI 代码，输出 file:line |
| `external/ui-library-selector-skill` | wangwei-wade/ui-library-selector-skill | 3.2.2 组件选型：分层选行为/视觉/动画来源，定向查找并验证接入 |
| `external/apple-design-skill` | dickwu/apple-design-skill | 3.2.4 按 Apple HIG 检查体验：可访问性/平台习惯/交互五维审查 |
| `external/impeccable` | pbakaus/impeccable | 3.2.4 打磨已有页面：critique → audit → polish，有针对性地改进 |

培训文档同一场次还推荐了（未随本仓库收录，按需自取）：

- **ui-ux-pro-max**：https://github.com/nextlevelbuilder/ui-ux-pro-max-skill （3.2.1 建立可复用设计规则 MASTER.md）
- **self-evolution / grilling 等工程质量与思维工具集合**：课程内部 Skill 包（其中 grilling 已从 mattpocock/skills 收录，两者同源）
- **forge-prd-entity v2.0**：课程完整包，本地已安装（`~/.easycode-user/skills/forge-prd-entity`），如需入库可从本地目录复制

### 课程 zip 包下载状态

| 包 | 获取方式 | 状态 |
|-|-|-|
| forge-prd-entity v2.0.zip | 飞书附件 token BAI5b2pcAoWmsXxrll2cQm0vnrI | 未能下载（bot 缺 drive:file:download 权限）；本地已有安装副本 |
| frontend-skill.zip | 飞书附件 token AlBqbQXKZoV1mZxme40cZJXUnZj | 未能下载（同上）；功能与 anthropic frontend-design 重叠 |
| ui-library-selector.zip | 飞书附件 token VoSFbFDNfoIRbCxv2KmcISQCnSb | 未能下载（同上）；已从作者公开仓库获取 |

如需补齐：在飞书开发者后台为应用 `cli_a97aeade66f85cc6` 开通 `drive:file:download` 权限后重试 `lark-cli drive +download`。

## 使用与更新

- 所有技能保留原文与配套文件（scripts/references/agents），安装到工具的 Skill 目录即可使用（Claude Code: `.claude/skills/<name>/`，Codex: `.agents/skills/<name>/`）。
- 更新单个技能示例：
  ```bash
  gh api repos/anthropics/skills/contents/skills/frontend-design/SKILL.md --jq '.content' | base64 -d > skills/external/anthropic-official/skills/frontend-design/SKILL.md
  ```
- 收录原则：与本次「从 0 到 1」演练阶段直接相关；上游 Skill 依赖其仓库级配置（GLOSSARY / issue tracker）的，按本仓库 journey 语境适配，不强制照搬。
