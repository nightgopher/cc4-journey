# 云端 MCP：Streamable HTTP

线上工作台和 MCP 使用同一个 Go 应用、同一套云端业务服务及数据库。MCP 不需要使用者在本机安装业务服务或连接本地数据库。

- 工作台：`http://150.5.143.32:8080/`。
- **远程 MCP 地址：`https://150.5.143.32/mcp`**。
- 传输：Streamable HTTP，同一个地址接受协议 POST、可选 SSE GET、会话 DELETE。
- 鉴权：请求头 `Authorization: Bearer <MCP integration token>`。

这是 Streamable HTTP；没有单独的旧 `/sse`、`/messages` 双端点。响应可通过 SSE 承载，SDK 客户端会处理。多个请求复用同一个会话注册表，客户端重连或服务发布时会重新初始化。

## Agent 接入

为 Agent 新增一个远程 HTTP MCP 连接，填写上述 URL 与 Authorization 请求头。客户端配置字段可能不同；支持 `.mcp.json` 的客户端通常采用以下结构，实际凭据应保存到私有配置：

```json
{
  "mcpServers": {
    "cc4-cloud": {
      "type": "http",
      "url": "https://150.5.143.32/mcp",
      "headers": {
        "Authorization": "Bearer REPLACE_WITH_PRIVATE_TOKEN"
      }
    }
  }
}
```

本机私有接入资料位于 `~/.config/cc4-journey/deploy-150/mcp-access.md`，可复制的客户端配置为同目录 `mcp-agent.json`，环境变量文件为 `mcp.env`；文件权限为 0600，不入库。仓库原有 `.mcp.json` 仍是独立的本地 stdio 配置，不会自动切换到云端。

## 工具与权限

| 工具 | 作用 | 业务写入 |
|---|---|---|
| `salary_preview` | 员工月薪资试算 | 否 |
| `commission_preview` | 每日提成与累计试算 | 否 |
| `level_transition_check` | 等级流转建议，支持真实数据或参数试算 | 否 |
| `employee_get` | 员工档案查询 | 否 |
| `workday_get` | 月度工作日查询 | 否 |
| `performance_submit` | 代录业绩并联动提成与流水 | **是** |
| `performance_approve` | 审核员工订单明细 | **是** |

查询与试算也会记录调用审计。集成凭据授予上述 7 个工具，两个写工具沿用管理员代录/审核语义；它不是网页员工的 JWT 会话，不自动套用员工本人/团队可见范围。只有受信 Agent 应持有此凭据。当前 MCP 只封装这 7 个工具，并非所有网页或 REST 功能。

## 运行与发布

主 Go 服务直接挂载 `/mcp`，复用已有数据库连接池、事务和业务逻辑。Nginx 将 `/mcp`、`/cc4/mcp` 代理到本机 `127.0.0.1:3101/mcp`，关闭缓冲并保留流式响应。HTTP 入口跳转 HTTPS；客户端应直接配置 HTTPS URL。

- `/etc/cc4.env` 的 `MCP_TOKEN` 为独立的集成凭据；未设置时 HTTP MCP 返回 503，禁止无鉴权启用。
- `MCP_ALLOWED_ORIGINS` 为逗号分隔的浏览器 Origin 白名单；线上为 `https://150.5.143.32`。普通 Agent 请求可不带 Origin。
- 主服务监听本机，不公开数据库或内部 Go 端口。Nginx 使用固定的本机 Host 头，保留 SDK 的 localhost 防护。
- IP 证书使用 Let's Encrypt 的 shortlived 配置；`cc4-certbot-renew.timer` 每六小时检查续期，签发后通过 hook 校验并 reload Nginx。
- 首次 HTTPS 准备见 `deploy/enable-tls.sh`，需要 Certbot >=5.4、8080/80 现有配置和开放的 443 端口。凭据与证书不属于 release 构建包。

每次 GitHub Actions 发布后会验证 HTTPS 证书、未鉴权 401，再用官方 Go SDK 完成初始化、7 工具发现、提成试算、工作日查询和同会话的后续请求。

手工调用验证：先将私有 `mcp.env` 中的 `CC4_MCP_TOKEN` 加入进程环境，再执行：

```sh
cd app
go run ./cmd/mcp-check -url https://150.5.143.32/mcp
# 如需核对某位已有云端员工：追加 -employee <员工ID>
```

该检查只做查询和试算，输出不含凭据；HTTP 写入和审核在独立 `cc4_test_*` 数据库进行真实集成测试。独立 stdio 程序仍保留，构建方法见 README；独立 HTTP 启动方式为 `MCP_HTTP_ADDR=127.0.0.1:3102 MCP_TOKEN=... ./bin/cc4-mcp`。
