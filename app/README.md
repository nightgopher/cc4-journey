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
| MCP_HTTP_ADDR / MCP_TOKEN | MCP HTTP 监听地址与 Bearer 凭据；未设置监听地址时走 stdio |

每天业务时区 01:00 生成前一日默认考勤，02:00 执行等级检查和月汇总；手工、请假考勤不会被业绩自动同步覆盖。应用提供手工触发接口，权限与页面一致。

## 验证

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

## MCP

```sh
cd app
go build -o bin/cc4-mcp ./cmd/cc4-mcp
./bin/cc4-mcp
```

以 MCP 客户端启动该进程，stdout 专用于协议。HTTP 模式使用环境变量 `MCP_HTTP_ADDR`，工具包括薪资预览、提成预览、等级判断、员工/工作日查询、业绩录入与订单审核。写工具沿用业务事务与审计。

## 当前边界

外呼服务同步尚未使用线上账号验收；截图为本地认证文件存储，未接对象存储或压缩。旧系统的历史端点并未逐项声明兼容；已实现和实测范围以 PROGRESS.md 为准。
