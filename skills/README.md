# Skills 库

本目录收录本次演练要用的研发技能，按来源分目录存放。

## mattpocock/skills（来源：https://github.com/mattpocock/skills）

> Matt Pocock 的 "Skills for Real Engineers"，取自其 `.agents` 目录。以下为人工挑选的 11 个，均保留原文（英文），未做改写。挑选标准：与本次「从 0 到 1」演练的阶段直接相关。

### 与演练阶段的对应关系

| 演练阶段 | 用到的技能 | 作用 |
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

### 目录说明

- 每个 `engineering_*` / `productivity_*` 目录对应上游 `skills/<category>/<name>/`，`SKILL.md` 为入口，其余为随附参考文件。
- 上游 skill 部分依赖其仓库级的 `GLOSSARY.md` / issue tracker 配置（`/setup-matt-pocock-skills`）；在本地使用时按我们自己的 journey 语境适配即可，不强制照搬。

### 更新方式

```bash
# 重新拉取某个 skill 的最新版（示例：tdd）
gh api repos/mattpocock/skills/contents/skills/engineering/tdd/SKILL.md --jq '.content' | base64 -d > skills/mattpocock/engineering_tdd/SKILL.md
```

## 自有技能

- 根目录 [`SKILL.md`](../SKILL.md)：`zero-to-one-delivery`——本次演练自沉淀的全流程技能，与上面 11 个是互补关系：自有技能定义"旅程路线"，外部技能提供各站点的"操作手法"。
