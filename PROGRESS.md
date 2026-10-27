# CC4 项目进度交接文档

> 更新时间：2026-10-27 02:50
> 仓库：https://github.com/nightgopher/cc4-journey （全部已推送至 `d3d5cdc`+`ee335db`）
> 本文档用途：交给下一个 agent 继续推进。所有上下文都在这里，不需要翻聊天记录。

---

## 一、项目是什么

**演示项目**：客户（智研交付中心）痛点是"AI 研发全流程提效"。我们设计了一个从 0 到 1 的演练仓库 `cc4-journey`：

- **主线**：一段模糊口述需求（`journey/00-original-brief.md`）→ 需求澄清 → PRD → Go 重写 → 部署，全程留痕
- **基准系统**：`nightgopher/cc4-callcenter`（存量 JS 呼叫中心系统，已脱敏开源，作为业务规则出处与差分对照）
- **关键卖点**：用新引擎对账抓出旧系统真 bug（D1：B 级 88 元跨月，旧引擎每月重置多发钱）

## 二、已完成（全部经过验证代理 PASS）

### Go 后端（`app/`，~3000 行，对齐存量 12210 行 JS）

| 层 | 内容 | 状态 |
|-|-|-|
| `internal/domain/` | 薪资四套公式 + 等级流转纯函数，**15 个金标准场景全绿**（`app/test/golden/`） | ✅ |
| `internal/repo/` | pgx 数据访问：employees/leads/customers/products/attendance/teams/notifications/settings/审批/薪资 | ✅ |
| `internal/service/` | CalculateSalary 四等级分发、RunDailyLevelCheck（cron 语义）、SubmitPerformance 双模型、ApproveOrderDetail 单事务 | ✅ |
| `internal/api/` | chi 路由：P0 六组 + 新增 leads/customers/products/attendance/teams/notifications/settings（**13 组，覆盖存量 18 组的大部分**）| ✅ |
| `internal/mcp/` | **MCP server 7 工具**（5 读 2 写），官方 go-sdk v1.8.0，stdio + HTTP 双传输，审计落 `agent_tool_audit` | ✅ |
| `cmd/server` `cmd/cc4-mcp` | 两个入口进程 | ✅ |

### 前端（`app/web/`，705 行，零第三方 UI 库）

- 按**仓库内收录的 skills** 重写（frontend-design/web-design-guidelines/apple-design）
- 5 页面：概览/员工/业绩/薪资/工作日；`lib/tokens.ts` 设计令牌；`components/kit.tsx` 模板组件
- tsc strict + vite build 通过，`money()` 9 边界用例全对（无小数不显示 .00 业务规则）

### 测试与验证

- **金标准**：`app/test/golden/GOLDEN.md`（15 场景 + 差分协议 + D1/D6/D4 裁决清单）
- **深度测试**：双引擎实跑对账 18 项（报告同在 GOLDEN.md），**D1 实锤**：同员工同库，Go 176 元 vs JS 440 元
- 验证代理三轮全 PASS（MCP 彩排 7/7、后端对齐 13/13、sanitize 8/8）

## 三、当前运行状态（本机）

| 服务 | 地址 | 启动方式 |
|-|-|-|
| JS 旧版基准（对照组） | :3000 前端 / :3001 API | `docker compose up -d`（已运行） |
| **Go 新后端** | :3101 | `cd app && (DB_PORT=5433 DB_PASSWORD='15671040800q' PORT=3101 nohup ./bin/cc4-server > /tmp/cc4-server.log 2>&1 &)` |
| **新前端 dev** | :5173 | `cd app/web && (VITE_API_BASE_URL=http://localhost:3101/api/v1 nohup npm run dev -- --port 5173 --host > /tmp/cc4-web.log 2>&1 &)` |
| MCP | stdio | `app/bin/cc4-mcp`（env 同上）；HTTP 模式 `MCP_HTTP_ADDR=:3102` |

**数据库**：docker 容器 `cc-postgres`（:5433，callcenter_db，root/15671040800q）。种子数据：员工 901-904（TEST_B/C5/A/T 四等级）+ 2025-10/11 工作日配置 + 产品 id=1（体验课9元，分档 10/20/30）。

**登录账号**：13800138000 / admin123（admin 角色）。测试员工 13900000001-04 同密码。

## 四、⚠ 已知问题（下一个 agent 优先处理）

### P1 - 未完成项（必须修）

1. **`CreateLead` 编译/运行时问题**：`internal/repo/leads.go:81` 把 `l.NextFollowUp`（现在是 `*string`）直接传给 INSERT——需要适配 date 类型（传 `NULLIF($12,'')::date`）。当前 `GET /leads` 已修好能跑（返回 `{"leads":[...],"pagination":{...}}`），但 **POST /leads 建带 next_follow_up 的线索会失败**（不传该字段则正常，已实测成功创建 id=1）。
2. **`leads_status_check` 约束**：真实表约束是 `new/contacted/qualified/converted/lost`，与 design.md 的 `pool/assigned/paid` 状态机**不是一套**！`AssignLeadsBatch` 把 status 改成 `'assigned'` 会违反约束直接报错。**需要裁决**：按 DB 约束改代码（assigned_to 有值即视为已分配，status 用 contacted/converted/lost），或改表约束。我建议按 DB 来，别动表。

### P2 - 缺口（影响演示完整度）

3. **财务 finance 组**（income/expense/cost-types/receivable）未做——表 `finance_records` 存在，存量路由 12+ 端点。演示财务页需要。
4. **权限 permissions 组**（position_module_permissions）未做——演示优先级低。
5. **dashboard 组**（today/trend）未做——前端概览页目前用 employees/stats 兜底。
6. **wechatLeads 组**未做——演示可跳过。
7. **前端只有 5 个页面**——用户已指出"为啥只有 5 个 tab"。后端现在 13 组就绪，前端应补：线索/客户/产品/考勤/团队/通知/设置页面（全部可套 `components/kit.tsx` 的 DataTablePage 模板，每个页面 ~40 行）。
8. **写路径测试**：`CommissionForIndex` 边界（from/to/null/间隙）无单测；`UpsertDailyPerformance` 累加语义无自动化测试。

### P3 - 技术债

9. `app/bin/` 下两个二进制不入库（.gitignore 已配），每次重新 build。
10. MCP `level_check_live=1` 有历史审计残留，正式演示前 `DELETE FROM agent_tool_audit;` 清零。
11. Go 后端测试覆盖薄（只有 domain 有测试）。

## 五、关键口径备忘（改代码前必读）

| 编号 | 争议 | 采用口径 |
|-|-|-|
| D1 | B 级 88 元跨月 | **文档口径：6 天总量顺延**（旧引擎每月重置是 bug，正是演示卖点） |
| D3 | A 级出勤 | 按去重日期（旧引擎按记录行数是 bug） |
| D4 | A 级零业绩 | 转人工审批，不自动降级 |
| D6 | C/B 考核边界 | 文档口径：第 6/9 天结束时判定 |

**两套提成模型**：模型一=日汇总 10/20/30 公式（`domain.DailyCommission`）；模型二=产品分档×当日序号（`repo.CommissionForIndex`，tiers JSON `{"from","to","amount"}`，to=null 无上限，未匹配=0 元）。无 productId 走模型一，有 productId 走模型二。

## 六、给下一个 agent 的操作指南

```bash
# 1. 编译
cd ~/Desktop/ai-fde/cc4-journey/app && go build ./...

# 2. 重启 Go 后端（改代码后）
pkill -f 'bin/cc4-server'; sleep 0.5
cd ~/Desktop/ai-fde/cc4-journey/app && (DB_PORT=5433 DB_PASSWORD='15671040800q' PORT=3101 nohup ./bin/cc4-server > /tmp/cc4-server.log 2>&1 &)

# 3. 前端 dev（已运行，改代码热更新）
# http://localhost:5173

# 4. 拿 admin token 测 API
GT=$(curl -s -X POST http://localhost:3101/api/v1/auth/login -H 'Content-Type: application/json' -d '{"phone":"13800138000","password":"admin123"}' | python3 -c 'import json,sys; print(json.load(sys.stdin)["session"]["access_token"])')

# 5. 跑测试
go test ./internal/domain/ -v
```

**响应包约定**（新端点必须遵守）：列表 `{"<name>s": [...], "pagination": {page,limit,total,totalPages}}`；错误 `{"error": "中文消息"}`；登录 token 在 `session.access_token`。

**业务规则全文**：仓库根 `cc-business-rules.md` + 飞书文档（RxOCdvxT1oQiPixPv3Bcdbvbnjd）。存量代码在 `/tmp/cc-export/`（cc4-callcenter 的本地副本，JS 对照用）。

## 七、建议推进顺序

1. 修 `CreateLead` 的 next_follow_up 参数适配（P1.1）——10 分钟
2. 裁决 leads 状态机口径（P1.2）——问用户或按 DB 约束改代码
3. 前端补 7 个页面（线索/客户/产品/考勤/团队/通知/设置）——每页 40 行套模板
4. 财务组后端 + 前端（P2.3）
5. dashboard/trend 端点 + 概览图表
6. `CommissionForIndex` 单测补齐
7. 全部完成后跑一轮双引擎对账 + 验证代理复核
