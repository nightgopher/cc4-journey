# 云端 MCP：Streamable HTTP

线上工作台和 MCP 使用同一个 Go 应用、同一套云端业务服务及数据库。MCP 不需要使用者在本机安装业务服务或连接本地数据库。

- 工作台：`http://150.5.143.32:8080/`。
- **远程 MCP 地址：`https://150.5.143.32/mcp`**。
- 传输：Streamable HTTP，同一个地址接受协议 POST、可选 SSE GET、会话 DELETE。
- 鉴权：请求头 `Authorization: Bearer <个人接入 Token>`。

这是 Streamable HTTP；没有单独的旧 `/sse`、`/messages` 双端点。响应可通过 SSE 承载，SDK 客户端会处理。多个请求复用同一个会话注册表，客户端重连或服务发布时会重新初始化。

## Agent 接入

登录 CC4，进入**个人中心 → Agent 接入 → 生成接入配置**。默认仅查询；账号具备代录/审核权限时可选允许业务写入。复制配置到 Agent 即可持续使用，不需要定期重新登录或换 Token。

每个账号只保留一把有效 Token。**重置**会原子替换旧 Token；**断开 Agent**会立即撤销。明文只在生成弹窗展示一次，关闭后服务端不能找回，丢失时重置。网页退出或七天登录态到期不会撤销接入 Token。

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

**有效能力 = 当前 CC4 账号权限 ∩ 接入模式。** 查询与试算同样记录带用户、员工、凭据 ID 的审计。`tools/list` 仅显示当前允许工具，直接猜测隐藏工具名也会被 `tools/call` 拒绝。

- 仅查询 Token 最多提供 5 个工具，不能代录和审核。
- 员工可查询本人授权范围；经理按当前团队限制；其他角色沿用现有业务范围。职位模块权限同样生效。
- 代录/审核需可写 Token、相应角色与模块权限、有效员工档案，并检查目标员工/订单范围；普通员工不能通过 MCP 绕过自录审批。
- 每个请求及工具调用重新读取账号、职位和团队，已有会话不会保留旧权限。账号停用、员工离职、撤销/重置影响下一次请求。会话绑定凭据 ID，另一把有效 Token 也不能复用该会话。
- Token 使用 48 字节安全随机数，数据库只保留 SHA-256 哈希和短标识；不保存明文或过期时间。生成/撤销接口复用 CC4 网页 JWT 确认身份，普通用户仅能管理自己，管理员可指定目标账号。

凭据接口：`GET /api/v1/mcp/credentials` 读取元信息；`POST` 生成或重置（`accessMode=read_only|read_write`，管理员可传 `ownerUserId`）；`DELETE /api/v1/mcp/credentials/{id}` 撤销。完整设计见 [身份权限方案](../journey/11-mcp-demo-authorization.md)。这是自定义 Bearer Header 的演示接入，未实现 OAuth 授权服务器，适用于支持此配置的客户端。当前 MCP 封装上述 7 个工具。

## 运行与发布

主 Go 服务直接挂载 `/mcp`，复用已有数据库连接池、事务和业务逻辑。Nginx 将 `/mcp`、`/cc4/mcp` 代理到本机 `127.0.0.1:3101/mcp`，关闭缓冲并保留流式响应。HTTP 入口跳转 HTTPS；客户端应直接配置 HTTPS URL。

- 主应用 `/mcp` 只接受数据库中有效的个人接入 Token，旧全局 `MCP_TOKEN` 已停用，不存在绕过账号权限的兼容路径。未鉴权或凭据失效返回 401。
- `MCP_PUBLIC_URL` 是生成配置使用的可信服务端地址，默认 `https://150.5.143.32/mcp`，不会使用客户端 Host 头拼接。
- `MCP_ALLOWED_ORIGINS` 为逗号分隔的浏览器 Origin 白名单；线上为 `https://150.5.143.32`。普通 Agent 请求可不带 Origin。
- 主服务监听本机，不公开数据库或内部 Go 端口。Nginx 使用固定的本机 Host 头，保留 SDK 的 localhost 防护。
- IP 证书使用 Let's Encrypt 的 shortlived 配置；`cc4-certbot-renew.timer` 每六小时检查续期，签发后通过 hook 校验并 reload Nginx。
- 首次 HTTPS 准备见 `deploy/enable-tls.sh`，需要 Certbot >=5.4、8080/80 现有配置和开放的 443 端口。凭据与证书不属于 release 构建包。

每次 GitHub Actions 发布后会验证 HTTPS 证书、未鉴权 401，再用官方 Go SDK 和专用仅查询账号完成初始化、5 个查询工具发现、提成试算、工作日查询和同会话后续请求。CI 凭据与演示用户分开，仍使用同一套可撤销账号凭据机制。

手工调用验证：先将私有 `mcp.env` 中的 `CC4_MCP_TOKEN` 加入进程环境，再执行：

```sh
cd app
go run ./cmd/mcp-check -url https://150.5.143.32/mcp
# 如需核对某位已有云端员工：追加 -employee <员工ID>
# 持有管理员可写 Token 时：追加 -access-mode read_write；检查本身仍只查询
```

该检查只做查询和试算，输出不含凭据；HTTP 写入和审核在独立 `cc4_test_*` 数据库进行真实集成测试。独立 stdio 程序仍保留，构建方法见 README；独立 HTTP 启动方式为 `MCP_HTTP_ADDR=127.0.0.1:3102 ./bin/cc4-mcp`，从同一数据库验证个人 Token。stdio 进程需设置 `CC4_MCP_TOKEN` 为其连接数据库中已签发的个人凭据，并在每次调用重新校验权限；它操作哪套数据仍取决于该进程的数据库配置。云端演示使用上面的远程 HTTPS 地址。
