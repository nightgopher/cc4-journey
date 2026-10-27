# CC4 Journey · 从 0 到 1 的 AI 研发全流程预演

本仓库记录一次完整的演练：**客户只给一段模糊的口述需求，AI 驱动的研发流程从需求澄清开始，一路走到设计、开发、部署**。每个阶段留下可审计的过程产物，最终沉淀出一套可复用的流程技能（Skill）。

## 背景

我们是做教育体验课电话销售的。客户跟进、员工出勤、每天业绩、月底工资，目前全靠表格和群消息管理，混乱且经常扯皮。我们希望有一个系统把这些事串起来——但客户自己也说不清楚具体要什么功能。

这正是本次演练的起点：**真实世界的需求从来不是清晰的**，看 AI 驱动的流程如何把模糊变成可交付。

## 旅程地图（Journey）

| 阶段 | 产物 | 状态 |
|-|-|-|
| 00 客户原始需求 | [journey/00-original-brief.md](journey/00-original-brief.md) | ✅ 逐字保存，未做任何加工 |
| 01 需求澄清 | [journey/01-requirements-clarify.md](journey/01-requirements-clarify.md) | ✅ AI 向客户提问，客户逐条回答 |
| 02 PRD | journey/02-prd.md | ⏳ 待开始 |
| 03 技术设计 | journey/03-design.md | ⏳ 待开始 |
| 04 开发 | [app/README.md](app/README.md)、[PROGRESS.md](PROGRESS.md) | ✅ Go 业务补全与 18 页 React 工作台，详见验证范围 |
| 05 部署与验收 | journey/05-deployment.md | ⏳ 待开始 |

> 演示规则：每个阶段产物**先独立成文、再进入下一阶段**，允许中途返工，返工过程同样留痕——这就是真实研发的样子。

## 流程技能（Skill）

本次演练结束后，把「从模糊需求到部署」的完整工作流沉淀为可复用技能，见 [SKILL.md](SKILL.md)。

## 仓库结构

```
cc4-journey/
├── README.md                # 本文件：旅程地图
├── SKILL.md                 # 沉淀出的可复用流程技能
├── journey/                 # 阶段产物（按序号）
│   ├── 00-original-brief.md     # 客户原始口述需求（逐字）
│   ├── 01-requirements-clarify.md
│   ├── 02-prd.md
│   ├── 03-design.md
│   └── 05-deployment.md
└── app/                     # 系统代码（阶段 04 产出）
```

## 与既有系统的关系

本仓库是全新从 0 开始的演练，不基于 `cc/`（Call Center 4.0 存量实现）。既有系统的业务规则可作为领域参考，但代码、决策与文档全部重新产生，用于演示「需求 → 交付」的完整路径。

## 相关仓库：nightgopher/cc4-callcenter（存量系统，脱敏开源）

存量系统 Call Center 4.0 已单独整理开源：**https://github.com/nightgopher/cc4-callcenter**

两仓库关系与区别，务必区分清楚：

| | cc4-journey（本仓库） | cc4-callcenter |
|-|-|-|
| 定位 | **从 0 到 1 的研发过程演练**：模糊需求 → 澄清 → PRD → 开发 → 部署，全程留痕 | **已有系统的成品代码**：一个功能完整的电销管理系统 |
| 起点 | 一段客户口述（见 journey/00） | 已运行的生产系统快照 |
| 代码来源 | 演练过程中新写 | 直接来自实际项目（Express + PostgreSQL + React） |
| 演示中的角色 | 主角：展示 AI 研发全流程 | 参照物：提供真实业务规则背景（等级体系、薪资计算、线索流转等，见其 design.md） |

**引用方式**：演练中做需求澄清、PRD 设计时，cc4-callcenter 的 `design.md`（1375 行业务全景文档）可作为领域知识参考——但仅作参考，本仓库的决策与产物一律独立产生，不回填、不依赖。

**脱敏说明**（cc4-callcenter 已做的处理）：

- 真实数据库/MinIO 密码、生产服务器 IP 全部替换为 `CHANGE_ME_PASSWORD` / `SERVER_IP_PLACEHOLDER` 占位符
- `.env`、`.env.production` 不提交，提供 `.env.example` 模板
- 含真实客户数据（手机号、员工名单）的 `backup.sql.gz` 与批量导入脚本已移除
- `node_modules`、IDE 配置、运行日志不入库
