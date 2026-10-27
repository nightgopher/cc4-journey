# 云端 MCP 发布与实际调用验收

日期：2026-10-27（America/Los_Angeles）。用户要求 Agent 通过云端接口调用已部署的 CC4 业务，明确使用 Streamable HTTP。

## 已发布入口

- MCP：**<https://150.5.143.32/mcp>**。
- 兼容路径：`https://150.5.143.32/cc4/mcp`。
- 工作台继续使用 `http://150.5.143.32:8080/`，也可通过 HTTPS 的 `/cc4/` 访问。
- MCP 使用独立的 Bearer 集成凭据，与工作台连接同一套云端 PostgreSQL 数据库。
- 本次发布、验收版本：`7c208003012ff0c1418451d297f256db7e883e30`，后续版本以 `/cc4/health` 为准。
- [GitHub Actions 成功发布与远程 MCP 验证](https://github.com/nightgopher/cc4-journey/actions/runs/40485582828)。

客户端接入、7 个工具及权限说明见 [app/MCP.md](../app/MCP.md)。操作者本机私有凭据位于 `~/.config/cc4-journey/deploy-150/mcp-access.md`，可复制配置为同目录的 `mcp-agent.json`；均为 0600，未提交仓库。

## 本次变更

主 Go 服务直接挂载 `/mcp`，复用现有业务服务、事务和数据库连接池，无需另启本地业务服务。修复原 HTTPHandler 在每次请求内重建 Streamable HTTP handler 的问题；现在同一 handler 保留会话注册表，初始化、通知、工具发现、调用及后续请求可在同一会话连续进行。

鉴权为空时拒绝启用 HTTP MCP。未授权请求返回 401；浏览器 Origin 按白名单检查，避免继承 REST 接口的通配 CORS。Nginx 将 MCP 请求转至本机 Go 端口，关闭响应缓冲，保留 SDK 的 localhost 防护。MCP 端点使用 HTTPS，HTTP 请求跳转 HTTPS。

为服务器 IP 签发了公开受信任的 Let's Encrypt IP 证书（SAN 为 `150.5.143.32`），通过 Certbot 5.4 webroot 验证，无需停止现有 Nginx。`cc4-certbot-renew.timer` 每六小时检查续期，签发后校验并 reload Nginx。已执行 `certbot renew --dry-run`，模拟续期成功。

参考：[MCP 传输规范](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports)、[Let's Encrypt IP 证书与 Certbot webroot 说明](https://letsencrypt.org/2026/03/11/shorter-certs-certbot.html)。

## 验收证据

| 检查 | 实测 |
|---|---|
| HTTP 会话回归 | 官方 Go SDK 完成初始化、重复 tools/list、两个并存会话；关闭一个会话不影响另一个，兼测有/无独立 SSE GET |
| 认证/Origin | 空配置拒绝启用，缺失/错误 Token 为 401；非法 Origin 为 403；合法预检成功且无通配 CORS |
| 7 个工具 | 隔离 `cc4_test_*` 数据库中通过实际 Streamable HTTP 调用全部 7 个工具 |
| 写操作 | 隔离库真实写入 2 单业绩、提成 20；真实通过订单审核，并验证数据库状态变化；重复审核被拒绝 |
| 查询与试算 | 薪资试算不生成薪资业务记录；提成 `[3,5,8]` 返回 `[30,70,160]`、合计 260 |
| 现有业务 | 19 组业务集成、7 天真实 HTTP API 演练、Go race/vet、前端格式/日期/TypeScript/构建和发布脚本校验通过 |
| 公网 HTTPS | 标准证书验证通过；匿名 401、错误 Token 401、非法 Origin 403 |
| 云端实际调用 | 本机仅运行 MCP 客户端，通过 HTTPS 发现 7 个工具、完成试算、查询云端工作日与已有员工 ID=1，并在调用后再次 tools/list 成功 |

公网客户端输出：

```json
{
  "commissionTotal": 260,
  "employeeVerified": true,
  "endpoint": "https://150.5.143.32/mcp",
  "status": "ok",
  "tools": [
    "commission_preview", "employee_get", "level_transition_check",
    "performance_approve", "performance_submit", "salary_preview", "workday_get"
  ]
}
```

两项管理写工具在隔离库完成真实验收；公网只执行查询、试算和调用审计，未为了验收新增或修改线上业绩、薪资或审批记录。此次没有更改另一会话正在编辑的业务演练文档和实现。
