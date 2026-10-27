# 03 · 技术设计

> 输入：[02-prd.md](02-prd.md)。纪律：每个设计决策可回溯到 PRD 条目；PRD 未提出的复杂度一律不引入。
> 状态：已确认，进入阶段 04 开发。
> 范围说明：本文档对应 PRD 的电销管理主链。上线后的迭代（远程 MCP、学员验收包、OCR 报销）在设计上有增量（迁移 003/004、verification 体系、OCR 供应商适配），过程与决策记录在各迭代文档 journey/10–13 中，不回填本文档。

---

## 1. 技术栈选择及理由

| 层 | 选择 | 理由（对应 PRD 条目） |
|---|---|---|
| 后端 | Go 1.26 + chi 路由 + pgx/v5 直连 PostgreSQL + golang-jwt/v5 | R1 权限在服务端执行、R6 并发安全需要显式事务与行锁；静态类型 + 单二进制利于 systemd 部署；不用 ORM，SQL 全部可审计（工资公式 D1/D2 必须逐行可查） |
| 数据库 | PostgreSQL 14+ | 事务、行级锁（业绩序号并发）、JSONB（工资明细、提成档位）满足 R4 留痕与产品档位配置 |
| 前端 | React + TypeScript（strict）+ Vite，原生组件、无 UI 库 | 中文业务表格与表单为主（R2/R3），自建组件库可完全控制可访问性与键盘操作；Vite 构建产物由后端静态托管，减少部署面 |
| 定时任务 | 进程内调度（业务时区），支持关闭与并发去重 | 考勤默认生成、等级检查（F8/F9、系统设置开关） |
| 集成入口 | MCP（官方 Go SDK，7 个工具，stdio/HTTP） | 允许 AI 助理以受控方式查询业务数据（进阶能力，不改变权限模型） |

**否决项**：Java/Spring（部署重）、ORM（掩盖 SQL、工资公式审计难）、微服务拆分（单体规模足够，150–200 用户）、前端组件库（中文表格定制成本高于自建）。

## 2. 数据模型（关键表与关系）

```
users ──1:1── employees ──N:1── teams
                 │  └──N:1── employee_level_history（等级留痕）
                 │
products（price, commission_rules JSONB）
                 │
leads ──转化── customers ──1:N── customer_orders（订单号、金额、负责人）
wechat_leads（五态，独立流转）        │
                                      │
daily_performance_records（员工×日期×产品 唯一汇总）
        └──1:N── daily_performance_order_details（每单一行，3 张截图、序号）
                                      │
attendance_records / detailed_attendance_records / monthly_attendance_summary（来源标识）
workday_config(year, month, workdays[], total_workdays)
                                      │
approval_requests ──1:N── approval_records（转交/通过/驳回留痕）
                                      │
salary_records（月度工资，payment_status 三态，明细 JSONB）
finance_records（收入/支出流水，来源标识：自动/调整）
payment_records（应收回款，逐笔）
notifications · system_settings · position_module_permissions · order_edit_history
```

关键约束：

| 约束 | 支撑的 PRD 条目 |
|---|---|
| 汇总表 (employee, date, product) 唯一；明细行带当日序号，序号在事务锁内分配 | 3.3 并发序号验收 |
| 工资明细存 JSONB 逐项公式展开 | 3.7 明细可追溯 |
| finance_records 带 source（自动/调整），自动流水禁止手工改写 | 3.7 收入流水规则 |
| employee_level_history 带 condition_met_date（生效日=条件满足日） | 3.4 等级验收 |
| attendance 来源四值（手工/业绩/默认/请假） | 3.5 来源标识 |
| 一笔工资全程至多一笔有效支出（发放→撤销→再发放复用） | 3.7 工资支出验收 |

## 3. 状态机与定时任务

**状态机**（与 PRD 状态表一一对应）：

| 对象 | 状态与流转 |
|---|---|
| 微信线索 | pool → assigned → following → paid / invalid；分配转交不改状态 |
| 名单线索 | new → contacted → converted / lost |
| 业绩明细 | pending → approved / rejected（终态均不可再编辑，驳回重录新明细） |
| 等级 | trainee→C→B→A；C→淘汰观察→C/eliminated；B→C；A→C；除 trainee→C 外全部走审批 |
| 工资单 | 未计算 ⇄ 已计算 → 已发放 →（撤销）→ 已计算；已发放拒绝重算 |

**定时任务**（业务时区，默认北京时间）：

| 时间 | 任务 | 说明 |
|---|---|---|
| 01:00 | 默认考勤生成 | 为前一日无业绩且无手工标记的在职员工生成"默认缺勤"；可经系统设置关闭 |
| 02:00 | 等级检查 | 按 D6 口径判定（第 6/9 工作日结束后），条件满足日记为昨天；D4 零业绩走人工核查、重复不建单 |
| 02:00 | 月度考勤汇总 | 按日期去重、缺勤/请假优先（D3） |

两任务并发去重：同一日期重复触发不重复生成；任务可取消、有执行锁。

## 4. API 清单（概览）

共 153 条受认证保护的路由，按模块分组（含兼容路径）：

| 分组 | 覆盖能力（回溯 PRD） |
|---|---|
| auth / users | 登录、会话、资料与密码（3.1 个人中心） |
| leads / wechat | 名单线索 CRUD 与分配转交、微信线索导入去重/分配/统计/导出（3.2） |
| customers / orders | 客户档案、客户订单（3.2） |
| performance | 自录/代录/零业绩、审核通过/驳回、日汇总、月汇总（3.3） |
| products | 产品与提成档位 CRUD（3.3） |
| employees / teams / positions | 账号、团队、等级历史、职位权限矩阵（3.4、3.8） |
| attendance / leave | 考勤录入/批量/汇总/同步、请假审批（3.5） |
| approvals | 统一审批：转交/通过/驳回/代批（3.6） |
| salary | 单人与批量计算、发放/撤销、已发放禁重算（3.7） |
| finance / payments | 收支类型与流水、应收与逐笔回款、超额拦截（3.7） |
| dashboard / notifications | 经营概览、七日趋势、通知（3.1） |
| settings / lookups | 系统设置、基础下拉最小投影（3.8） |
| simulation | 连续经营演练入口（阶段 05 验收用，见 08 文档） |
| mcp | 7 个只读/受控工具（commission_preview 等） |

权限模型：JWT 会话 + 角色 + 职位模块权限 + 数据范围（本人/本团队/全公司）在服务端统一裁剪；未登录一律 401（R1）。

## 5. 关键工程决策

| # | 决策 | 回溯 |
|-|-|-|
| 1 | 工资月提成只用日总量模型（D2 裁决）：`domain.DailyCommission` 逐日独立、月内累加；产品明细档位仅在明细行展示，不进工资公式 | 3.3 每日/月度业绩验收 |
| 2 | B 级 88 元固定工资实现"总量跨月顺延"：按等级历史逐月扣减已领天数（D1）；C 级同理 3 天终身额度跨月补足 | 3.7 D1/C 级跨月验收 |
| 3 | 业绩明细序号在事务 + 行锁内分配，并发提交序号连续不重复 | 3.3 并发验收 |
| 4 | 连接池统一业务时区（timestamptz 读写按业务日期解释），杜绝 UTC 日期边界错账 | R5、3.5 考勤验收 |
| 5 | 已发放工资在服务层与数据库双层拒绝重算；撤销走独立流水，不删原记录 | 3.7 工资状态表 |
| 6 | 回款累计校验在事务内先锁应收行再累加，超额拦截 | 3.7 超额拦截验收 |
| 7 | 外部线索源做成可配置适配器（令牌 + 分页 + 稳定合成 ID），独立模拟服务与真实供应商同一接口 | 1.3 本期不做、3.2 微信线索 |
| 8 | 截图存本地受认证保护目录（本期不做对象存储），路径不公开、经鉴权接口读取 | 1.3 本期不做 |

## 6. 部署形态

```
[浏览器] → Nginx :8080（静态前端 + API 反代）
              └── systemd: cc4 Go 服务 :3101 ── 本机 PostgreSQL 14
                                                ├─ 每日备份（pg_dump，可隔离库恢复验证）
                                                └─ 持久化上传目录（截图）
[GitHub Actions] ──发布──▶ 版本校验 + 迁移 + 健康检查
[上游模拟服务 services/lead-source-mock] ──独立主机/独立库──▶ 主系统经令牌分页拉取
```

- 环境分层：本地开发（API 3101 / 前端 5173 / Docker PostgreSQL 5433）；云端演示（公网 8080 入口，`/cc4/health` 健康检查）；隔离测试库（`cc4_test_*`，集成测试自动建删）。
- 凭据管理：数据库、JWT、上游令牌均由环境/私有配置提供（0600，不入库）。

## 7. 交给阶段 04 的验收入口

设计的正确性由以下三层验证承接（详见阶段 04 与部署记录）：

1. **领域金标准**：15 个金标准场景（提成阶梯、四级薪资、等级边界），输入→期望输出独立于任何旧实现；
2. **数据库集成**：17 组隔离库场景覆盖并发、跨月、撤销、去重、越权；
3. **公网验收**：全路由 401、角色登录/权限抽样、浏览器逐页走查。

---

## 与 PRD 的回溯总表

| PRD 章节 | 设计落点 |
|---|---|
| 3.1 工作台 | dashboard API、React 工作台布局、七日日期条 |
| 3.2 线索与客户 | leads/wechat/customers/orders 表与 API、五态状态机、导入去重 |
| 3.3 业绩 | 汇总+明细双表、事务序号、审核事务一致性、产品档位 JSONB |
| 3.4 组织与员工 | employees/teams/level_history、等级定时任务、账号创建范围控制 |
| 3.5 考勤与工作日 | attendance 三表 + workday_config、默认考勤任务、D3 统计规则 |
| 3.6 审批 | approval_requests/records、转交模型、重复建单拦截 |
| 3.7 薪资与财务 | salary.go 领域层四等级公式、发放/撤销状态机、finance/payment 事务 |
| 3.8 系统管理 | position_module_permissions、system_settings、上游适配器 |
| 4 跨模块规则 | 中间件鉴权 401/403、业务时区、等宽数字展示、全链路留痕表 |
