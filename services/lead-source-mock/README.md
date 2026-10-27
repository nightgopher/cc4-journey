# 独立上游线索模拟服务

为 CC4 演示和联调提供 Gooki 风格的上游外呼 API，独立 Go module、进程和部署目录，不依赖 CC4 数据库，也不会拨打电话。

部署入口：`http://101.47.77.214/cc4-upstream`。

## 接口

| 路径 | 用途 |
|---|---|
| `GET /health`（另有 `/healthz`） | 服务状态、部署 commit、每日记录数、时区 |
| `GET /token?username=...&password=...` | 兼容现有 CC4 上游适配器的登录接口 |
| `POST /token` | JSON `{username,password}` 登录，返回 `data.token` |
| `GET /task/cdr/all2` | 按日期和分页返回外呼记录，需要 Authorization |
| `GET /scenarios` | 列出模拟场景 |
| `GET /openapi.json` | OpenAPI 3.1 契约 |

`Authorization` 接受原始 token 或 `Bearer token`；令牌默认有效 1 小时。分页入参为 `page`、`pageSize`、`startTime`、`endTime`，响应为 `code=200`、`mock=true`、`data.totalCount` 和 `data.data` 数组。

日期支持 `YYYY-MM-DD`、`YYYY-MM-DD HH:mm:ss`、RFC3339；结束时间包含在范围内，纯日期包含整天，每次最多 31 个自然日。默认业务时区为 Asia/Shanghai，默认每页 100 条、最多 500 条。

## 合成数据与场景

每个自然日默认 120 条记录，平均分为四种状态：

- `AF3加微信成功挂断（模拟未成交）`：CC4 会同步进微信线索池。
- `AF1接通未加微信（模拟）`：过滤。
- `AF2当场成交（模拟）`：过滤，不作为未成交线索导入。
- `AF0未接通（模拟）`：过滤。

所有客户名以“模拟家长”开头、手机号以 `000` 开头。ID 由业务日期与日内序号确定，重启、分页和重复请求不会改变；可验证重复同步去重。`callTime` 为 Unix 秒，录音地址留空，未伪造支付或录音凭证。CC4 适配器保留 `data_source=mock` 与模拟姓名。

带 `scenario` 查询参数可覆盖服务默认场景：`normal` 正常混合数据、`empty` 空列表、`error` HTTP 502、`slow` 延迟响应。延迟默认为 2 秒，可由 `MOCK_DELAY` 调整至 30 秒，用于测试超时和取消。模拟场景不改变任何真实业务数据。

## 本地运行

```sh
cd services/lead-source-mock
# 先把 .env.example 中的配置加载到进程环境，替换登录与签名密钥。
go run ./cmd/server
go test -race ./...
go vet ./...
```

应用不自动加载 env 文件。`MOCK_USERNAME` 必填，`MOCK_PASSWORD` 至少 16 字符，`MOCK_TOKEN_SECRET` 至少 32 字符；不存在可用于部署的默认密码。其他配置见 `.env.example`。

## GitHub Actions 发布

[部署 workflow](../../.github/workflows/deploy-lead-source-mock.yml) 在 main 的本服务或 workflow 发生变更时执行，也支持手工 `workflow_dispatch`；PR 只测试和构建。步骤为 `go test -race` / `go vet` → 构建静态 Linux amd64 二进制 → 校验和与压缩包 → 带 HMAC 签名的 HTTP 发布 → 公网版本、登录、分页与 30 条可导入线索检查。官方 Actions 固定到已核对的完整 commit SHA。

仓库配置：

| 名称 | 类型 | 说明 |
|---|---|---|
| `CC4_MOCK_DEPLOY_SIGNING_KEY` | Actions Secret | 仅用于本模拟服务发布的 HMAC 密钥 |
| `CC4_MOCK_USERNAME` / `CC4_MOCK_PASSWORD` | Actions Secret | 公网契约检查使用的模拟账号 |

首次准备服务器时，以现有管理 SSH 身份上传 `deploy/` 目录，然后运行 `bash deploy/bootstrap.sh`。脚本创建服务账号、systemd unit、随机账号密码及发布签名密钥。它不上传应用二进制。可选公钥参数仅用于启用受限 SSH 发布备用通道。

首次 Actions 运行已通过测试和构建，但 GitHub hosted runner 连接本服务器 TCP 22 超时；本机防火墙为 ACCEPT。实际部署改走现有 80 端口的 `/cc4-upstream-deploy/deploy`，不依赖新的防火墙开放。请求对 commit、时间、一次性 nonce 和发布包 SHA-256 做 HMAC-SHA256 签名；密钥不在请求中发送，接收端拒绝过期、重放和被修改的包，并签名返回部署回执。公开传输的发布包仅含公开项目的二进制和校验文件，不包含运行凭据。

本次服务器使用现有 Nginx HTTP 默认站点 `/etc/nginx/sites-available/orionstar`，只插入独立 include；添加 `/cc4-upstream/` 路径，转发至 `127.0.0.1:18081`。配置前备份、`nginx -t` 通过后 reload。该路径不记录含登录参数的访问日志。发布接收进程仅监听 `127.0.0.1:18082`，固定调用归档验证和版本切换程序，不接受任意命令；systemd 限制它只可写本模拟服务的版本目录。

运行配置为 `/etc/cc4-lead-source-mock.env`（root 0600），程序以 `cc4-lead-mock` 用户运行。程序与版本目录在 `/opt/cc4-lead-source-mock/releases/<commit>/`，`current` 为原子切换的软链接。发布包限制为压缩 32 MiB、解压 34 MiB，先限制解压总量，再逐个检查固定长度的普通文件头，拒绝扩展头、路径穿越与额外文件，并验证 SHA-256；健康检查失败时恢复上一个版本。服务关闭时等待在途请求排空，最长 35 秒。

```sh
systemctl status cc4-lead-source-mock
journalctl -u cc4-lead-source-mock -n 50
```

## CC4 接入

给 CC4 Go 服务配置：

```text
GOOKI_HOST=http://101.47.77.214/cc4-upstream
GOOKI_USERNAME=<模拟服务账号>
GOOKI_PASSWORD=<模拟服务密码>
```

重启 Go 服务后，在“微信线索 → 同步外呼数据”选择日期。一天首次同步应新增 30 条，再次同步应新增 0、重复 30。所有写入仍经过现有鉴权和团队范围控制。

`CC4_UPSTREAM_TEST_URL` 可让 `app/scripts/test-integration.sh` 的上游同步场景使用部署服务（同时提供 `GOOKI_USERNAME` / `GOOKI_PASSWORD`）；未设置时使用本地契约服务，测试无需公网。两种模式都只写入脚本创建的隔离 `cc4_test_*` 数据库。
