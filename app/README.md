# Go 应用与 React 工作台

业务与验证记录见根目录 [PROGRESS.md](../PROGRESS.md)。界面设计与所用技能见 [前端决策](../journey/04-frontend-decisions.md)。

## 本地运行

需要 Go（版本见 go.mod）、Node.js/npm、PostgreSQL 和 Poppler 的 `pdfinfo`（用于校验PDF页数；macOS `brew install poppler`，Ubuntu `sudo apt-get install poppler-utils`）。将 `.env.example` 中的值配置到启动进程的环境；应用不会自动读取 `.env`。

在空数据库中先导入基线，再应用增量迁移。已有基线的数据库只执行增量：

```sh
cd app
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/000_baseline_schema.sql
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/001_go_completion.sql
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/002_align_imported_sequences.sql
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/004_reimbursement_ocr.sql
go build -o bin/cc4-server ./cmd/server
./bin/cc4-server
```

默认 API 为 `http://localhost:3101/api/v1`，`GET /health` 检查进程。已有本机演示数据库保留原账号；迁移不创建登录账号。

另一个终端启动前端：

```sh
cd app/web
npm ci
VITE_API_BASE_URL=http://localhost:3101/api/v1 npm run dev -- --host 127.0.0.1 --port 5173
```

生产静态文件用 `npm run build` 生成在 `app/web/dist/`（生成目录不入库，已移除历史跟踪的旧版构建产物）。构建内含严格 TypeScript 检查。静态托管与 API 地址由部署配置决定。

云端单机部署与 GitHub Actions 配置见 [部署说明](deploy/README.md)。线上前端构建使用 `/cc4/` 基路径和同源 `/cc4/api/v1`，数据库与截图独立持久化。

## 配置

| 变量 | 默认或用途 |
|---|---|
| DATABASE_URL | 完整 PostgreSQL URL；提供时优先于 DB_* |
| DB_HOST / DB_PORT / DB_NAME / DB_USER / DB_PASSWORD | localhost / 5433 / callcenter_db / root / 空；服务端与 MCP 共用 |
| HOST / PORT | 0.0.0.0 / 3101 |
| JWT_SECRET | 部署时设置为自己的签名密钥 |
| UPLOAD_DIR | 默认 `uploads`，保存订单截图和报销原始票据，需持久化 |
| BUSINESS_TIMEZONE | Asia/Shanghai，决定定时任务的业务日期 |
| ENABLE_CRON | 默认开启，设为 `false` 关闭；启动时不补跑历史任务 |
| GOOKI_HOST / GOOKI_USERNAME / GOOKI_PASSWORD | 可选外呼同步服务配置；缺失时同步接口明确提示未配置 |
| GOOKI_FILTER_PREFIX | 可选呼叫结果前缀筛选 |
| PADDLEOCR_API_KEY | 可选，PaddleOCR服务端密钥；未配置仍可上传并人工填写，识别会明确显示未配置 |
| PADDLEOCR_MODEL | 默认 `PaddleOCR-VL-1.6`；另支持 `PP-OCRv6`、`PP-StructureV3`，仅管理员通过进程环境切换 |
| VITE_API_BASE_URL | 前端构建/开发使用的 API 地址 |
| MCP_HTTP_ADDR / MCP_TOKEN | MCP HTTP 监听地址与 Bearer 凭据；未设置监听地址时走 stdio |

每天业务时区 01:00 生成前一日默认考勤，02:00 执行等级检查和月汇总；手工、请假考勤不会被业绩自动同步覆盖。应用提供手工触发接口，权限与页面一致。

## 智能报销演示

工作台“智能报销”可下载合成示例票据（明确标记非税务发票）。员工上传PNG/JPEG或单页PDF，单文件最多8MiB；Go后台创建PaddleOCR异步任务并保存jobId，刷新页面或服务重启后继续查询已保存任务。密钥不进入前端，识别内容及原图按报销权限读取。创建请求结果不确定时会停止自动重发，需用户明确重新识别。

演示顺序：员工上传 → 等待识别 → 对照原票据核对商家、票号、日期、人民币金额并填写用途 → 勾选确认后提交 → 其他主管/财务/管理员批准 → 财务/管理员登记付款。识别和批准均不入账；付款在事务中生成唯一自动支出。申请人不能自审或支付自己的报销。驳回可修改重提，待审批可撤回；已批准金额不可直接修改，已付款保留历史。

相同文件或票据编码/号码会查重。修改、审批和付款携带页面所见的 `version`；记录变更后，旧页面操作返回409，刷新并重新核对后再操作。付款重放仍返回已有结果，不重复支出。自动报销流水不能从通用财务页面改删。

接口示例见 [api.http](verification/api.http)，实现范围、真实供应商与浏览器验证记录见 [OCR验收记录](../journey/13-ocr-reimbursement.md)。默认模型已做真实服务验证，其余两个模型做了结果格式适配与合同测试。OCR仅辅助录入，不做税务真伪认证；当前付款为演示记账，未连接银行。

## 验证

学员最终交付统一从仓库根目录运行 `python3 app/verification/verify.py`。它自建临时PostgreSQL14，执行完整验收并输出报告、日志、幂等对比和1元反例证据，无需老师账号。操作、接口示例、判断依据和提交模板见 [学员验收包](verification/README.md)。以下命令保留用于开发时单独验证。

```sh
cd app
go test ./...
go build ./...
bash scripts/test-integration.sh
cd web
npm run format:check
npm run build
```

集成脚本默认使用 Docker 容器 `cc-postgres`、本机端口 5433，自动读取容器数据库密码，建立独立 `cc4_test_*` 数据库并在退出时删除。可通过 `CC4_TEST_CONTAINER`、`DB_HOST`、`DB_PORT`、`DB_USER`、`DB_PASSWORD` 覆盖连接参数。测试本身拒绝对非 `cc4_test_*` 数据库写入。

`CC4_KEEP_TEST_DB=1 bash scripts/test-integration.sh` 可保留合成数据供页面验收，库名写入 `/tmp/cc4-preview-db-name`。验收后删除对应测试库。测试不操作主业务数据库。

连续经营演练见 [simulation/README.md](simulation/README.md)。`bash scripts/test-simulation.sh` 通过真实 HTTP API 回放最近7天，在独立库检查入离职、调级、审核、回款、工资支出与重复运行，并注入1元差异证明能够报错。工作台“业务演练”页展示逐步核对报告。

## MCP

云端 Agent 使用 **Streamable HTTP**：`https://150.5.143.32/mcp`，与工作台连接同一套云端业务数据库，需独立 Bearer 集成凭据。主应用直接挂载该端点，未配置 `MCP_TOKEN` 时不开放。7 个工具、客户端配置、权限范围与验证命令见 [MCP 接入说明](MCP.md)。

以下保留本地 stdio 程序的构建方式：

```sh
cd app
go build -o bin/cc4-mcp ./cmd/cc4-mcp
./bin/cc4-mcp
```

以 MCP 客户端启动该进程，stdout 专用于协议。HTTP 模式使用环境变量 `MCP_HTTP_ADDR`，工具包括薪资预览、提成预览、等级判断、员工/工作日查询、业绩录入与订单审核。写工具沿用业务事务与审计。

## 当前边界

独立上游模拟服务已上线并完成公网同步与去重验收，见 [发布记录](../journey/06-upstream-mock-deployment.md)。本机可将 `~/.config/cc4-journey/mock-source.env` 加载到进程环境后启动 Go；连接凭据不入库。真实供应商账号仍待提供后独立验收。

截图为本地认证文件存储，未接对象存储或压缩。旧系统的历史端点并未逐项声明兼容；已实现和实测范围以 PROGRESS.md 为准。
