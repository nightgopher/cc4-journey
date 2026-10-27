# CC4 Journey · 从 0 到 1 的 AI 研发全流程预演

本仓库记录一次完整的演练：**管理层立项，只给一段模糊的口述需求，AI 驱动的研发流程从需求澄清开始，一路走到设计、开发、部署**。每个阶段留下可审计的过程产物，最终沉淀出一套可复用的流程技能（Skill）。

运行入口：**[CC4 工作台 · 8080 端口](http://150.5.143.32:8080/)**。GitHub Actions 自动发布、运行结构与实测结果见 [部署记录](journey/07-main-application-deployment.md)。

## 背景

这是一家青少年教育体验课电销机构的真实管理场景：**管理层立项**，要把客户跟进、员工出勤、每日业绩、月底工资从表格和群消息搬进一个系统。研发拿到的只有负责人的一段口述——痛点说了三件，功能一个没定。

这正是本次演练的起点：**真实世界的需求从来不是清晰的**，看 AI 驱动的流程如何把模糊变成可交付。

> 案例性质：本仓库取材于真实项目改编（真实业务规则与上线裁决 D1–D7 是事实基准），00/01 阶段的口述与问答按真实规则以管理层视角重演，各文档内均有诚信标注。

## 旅程地图（Journey）

| 阶段 | 产物 | 状态 |
|-|-|-|
| 00 原始需求 | [journey/00-original-brief.md](journey/00-original-brief.md) | ✅ 管理层口述逐字留痕，未做任何加工 |
| 01 需求澄清 | [journey/01-requirements-clarify.md](journey/01-requirements-clarify.md) | ✅ 两轮问答留痕 + 边界追问 D1–D7 裁决（AI 提案、管理层拍板）+ 事实/假设清单 |
| 02 PRD | [journey/02-prd.md](journey/02-prd.md) | ✅ 背景 → 整体架构 → 产品设计，关键对象带状态×操作表与验收 |
| 03 技术设计 | [journey/03-design.md](journey/03-design.md) | ✅ 技术栈、数据模型、状态机与定时任务、API 概览、工程决策回溯表 |
| 04 开发 | [app/README.md](app/README.md)、[PROGRESS.md](PROGRESS.md) | ✅ Go 业务补全与 20 页 React 工作台（含智能报销），详见验证范围 |
| 05 部署与验收 | [主系统部署](journey/07-main-application-deployment.md)、[上游模拟服务](journey/06-upstream-mock-deployment.md) | ✅ 两个服务均由 GitHub Actions 发布；主系统独立 8080 入口、持久化数据库与上传、每日备份 |
| 06 连续经营 | [演练范围](journey/08-business-simulation.md)、[独立复核](journey/09-business-simulation-review.md) | ✅ 真实API覆盖入离职、调级、订单审批、回款和工资；7日105个事件、831项核对及1元故障注入 |
| 07 学员验收 | [一键验收包](app/verification/README.md)、[用例与依据](app/verification/CASES.md)、[提交模板](app/verification/SUBMISSION.md) | ✅ 本机与GitHub Actions实跑16阶段，保留逐项断言、幂等与1元反例证据；见[实测记录](journey/10-student-acceptance.md) |
| 08 智能报销 | [实现与验收](journey/13-ocr-reimbursement.md) | PaddleOCR异步识别、人工核对、审批、付款与唯一支出；11组报销集成纳入一键验收，真实模型与部署证据见记录 |

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
│   ├── 06-upstream-mock-deployment.md
│   └── 07-main-application-deployment.md
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
