# Go 应用与 React 工作台

业务与验证记录见根目录 [PROGRESS.md](../PROGRESS.md)。界面设计与所用技能见 [前端决策](../journey/04-frontend-decisions.md)。

## 本地运行

需要 Go（版本见 go.mod）、Node.js/npm 与 PostgreSQL。将 `.env.example` 中的值配置到启动进程的环境；应用不会自动读取 `.env`。

在空数据库中先导入基线，再应用增量迁移。已有基线的数据库只执行增量：

```sh
cd app
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/000_baseline_schema.sql
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/001_go_completion.sql
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
| UPLOAD_DIR | 默认 `uploads`，保存订单截图，需持久化 |
| BUSINESS_TIMEZONE | Asia/Shanghai，决定定时任务的业务日期 |
| ENABLE_CRON | 默认开启，设为 `false` 关闭；启动时不补跑历史任务 |
| GOOKI_HOST / GOOKI_USERNAME / GOOKI_PASSWORD | 可选外呼同步服务配置；缺失时同步接口明确提示未配置 |
| GOOKI_FILTER_PREFIX | 可选呼叫结果前缀筛选 |
| VITE_API_BASE_URL | 前端构建/开发使用的 API 地址 |
| MCP_HTTP_ADDR / CC4_MCP_TOKEN | 独立 MCP HTTP 监听地址；stdio 使用所连数据库签发的个人 Token |
| MCP_PUBLIC_URL / MCP_ALLOWED_ORIGINS | 生成配置使用的可信远程 URL / 浏览器 Origin 白名单 |

每天业务时区 01:00 生成前一日默认考勤，02:00 执行等级检查和月汇总；手工、请假考勤不会被业绩自动同步覆盖。应用提供手工触发接口，权限与页面一致。

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

云端 Agent 使用 **Streamable HTTP**：`https://150.5.143.32/mcp`，与工作台连接同一套云端业务数据库，需在个人中心生成绑定账号的长期、可撤销 Bearer Token。主应用直接挂载该端点，工具与数据范围跟随账号当前权限，旧全局 `MCP_TOKEN` 不再生效。7 个工具、客户端配置、权限范围与验证命令见 [MCP 接入说明](MCP.md)。

以下保留本地 stdio 程序的构建方式：

```sh
cd app
go build -o bin/cc4-mcp ./cmd/cc4-mcp
CC4_MCP_TOKEN=<该数据库签发的个人Token> ./bin/cc4-mcp
```

以 MCP 客户端启动该进程，stdout 专用于协议。HTTP 模式使用环境变量 `MCP_HTTP_ADDR`，工具包括薪资预览、提成预览、等级判断、员工/工作日查询、业绩录入与订单审核。写工具沿用业务事务与审计。

## 当前边界

独立上游模拟服务已上线并完成公网同步与去重验收，见 [发布记录](../journey/06-upstream-mock-deployment.md)。本机可将 `~/.config/cc4-journey/mock-source.env` 加载到进程环境后启动 Go；连接凭据不入库。真实供应商账号仍待提供后独立验收。

截图为本地认证文件存储，未接对象存储或压缩。旧系统的历史端点并未逐项声明兼容；已实现和实测范围以 PROGRESS.md 为准。
