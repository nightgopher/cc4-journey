# 上游线索模拟服务发布记录

日期：2026-10-26（America/Los_Angeles）。需求：上游接口尚未提供，建立独立模拟服务，使用 GitHub Actions 发布至 101.47.77.214。

## 已上线版本与入口

- 运行版本：`46a7c666690be1dbfd8c90241237670f008e94c0`。
- [成功的 Actions 运行](https://github.com/nightgopher/cc4-journey/actions/runs/40457497030)：测试、Linux 构建、签名发布、公网契约验证全部成功。
- [服务入口](http://101.47.77.214/cc4-upstream/)、[健康检查](http://101.47.77.214/cc4-upstream/health)、[OpenAPI](http://101.47.77.214/cc4-upstream/openapi.json)。
- 源码：[独立 Go module](../services/lead-source-mock/README.md)；[发布 workflow](../.github/workflows/deploy-lead-source-mock.yml)。

后续仅修改本发布记录和应用测试的提交不会触发模拟服务重发，因此 GitHub main 的最新提交可能晚于上述运行版本。

## 接口与业务行为

兼容现有 Go 适配器：`GET /token` 查询参数登录（另支持 JSON POST），返回 `data.token`；`GET /task/cdr/all2` 接收 Authorization、分页、起止时间，返回 `data.totalCount`、`data.data`。

默认每天 120 条合成通话：30 条 AF3 加微未成交、30 条接通未加微、30 条当场成交、30 条未接通。CC4 只导入 AF3；当场成交不进入未成交线索池。手机号为不可拨打的 `000` 占位号，姓名带“模拟家长”，录音地址留空。

编号由业务日期与序号确定，分页、重启、重复请求保持一致。支持 normal / empty / error / slow 场景，日期含整日边界、最多 31 个自然日。数据不依赖数据库，也不进行实际外呼。

## 部署通道与隔离

首次 [SSH 发布运行](https://github.com/nightgopher/cc4-journey/actions/runs/40455447230) 的测试和构建成功，但 GitHub hosted runner 到目标 TCP 22 超时。服务器本机 UFW inactive、iptables INPUT ACCEPT；本机管理 SSH 可用。未扩大防火墙规则，改用已能公网访问的 80 端口。

Actions 向 `/cc4-upstream-deploy/deploy` 上传签名发布包。HMAC-SHA256 覆盖 commit、时间戳、一次性 nonce、压缩包 SHA-256，密钥不随请求发送。服务拒绝过期签名、重放、修改包与超时上传；完成上传后再次检查签名时效。返回的成功回执同样签名，Actions 验证回执后再检查公网版本与数据契约。

发布接收器监听 127.0.0.1:18082，经 Nginx 独立路径转发。它仅调用固定安装器；安装器限制压缩/解压大小、只接受两个规定的普通文件头、校验 SHA-256、原子切换版本，失败走回滚。运行程序以非 root 的 `cc4-lead-mock` 用户监听 127.0.0.1:18081。接收器的 systemd 沙箱只允许写本模拟服务的版本目录，程序与基础设施文件保持只读。

服务器路径：

| 路径或 unit | 用途 |
|---|---|
| `/opt/cc4-lead-source-mock/releases/<commit>` | 保留已发布程序与校验文件 |
| `/opt/cc4-lead-source-mock/current` | 当前版本软链接 |
| `/etc/cc4-lead-source-mock.env` | 模拟账号和令牌签名配置，root 0600 |
| `/etc/cc4-lead-source-deploy.env` | 发布 HMAC 密钥，root 0600 |
| `cc4-lead-source-mock.service` | 合成上游 API |
| `cc4-lead-source-deploy.service` | 签名发布接收器 |

已有站点在 bootstrap 前后均返回 200。新增配置以 include 方式接入 Nginx，语法检查后 reload。首次尝试建立的专用 SSH 发布 key、仓库 SSH secret 和 known-host variable 已撤销；原管理员 SSH 凭据保留。

当前 Actions 仅保存三项 Secret：`CC4_MOCK_DEPLOY_SIGNING_KEY`、`CC4_MOCK_USERNAME`、`CC4_MOCK_PASSWORD`。凭据值不在代码、文档或日志中保存。

日常修改 Go 服务或 workflow 后推送 main，即自动测试和发布。修改受控安装器、接收器、unit 或 Nginx 等基础设施时，需要先以管理员 SSH 重新运行 bootstrap；应用流水线只上传程序与校验文件，不替换 root 基础设施。

## 验证与复核

- 独立服务：7 个 Go 测试（含 `-race`）、`go vet` 通过；8 个 Python 发布测试通过。
- 发布测试覆盖签名绑定、重放、超时 408、上传中过期 401、回执、路径穿越、巨型扩展头、解压炸弹和非法校验和。
- workflow 通过 actionlint v1.7.12；官方 checkout/setup-go 固定到已核对的完整 SHA。
- 公网 Actions 检查：登录成功；一日分页 100 + 20 条，120 个不重复 ID，30 条 AF3，全部使用 000 号码，健康版本与发布 SHA 一致。
- 主系统针对实际公网服务运行 17 组隔离库 `-race` 集成，全部通过：处理 120 条、首次新增 30；再次新增 0、重复 30；姓名与 `data_source=mock` 保留。普通测试显式清除继承的 GOOKI_HOST，只有 CC4_UPSTREAM_TEST_URL 场景启用外部来源。
- 所有测试写入 `cc4_test_*` 临时库并已清理，未向本机主库添加模拟业务记录。

两轴复核分别检查需求契约与实现约束。Spec 未发现新缺口，并用本地独立服务验证 120 → 30 → 重复 30。Standards 发现归档解压限制、退出排空、安装子进程硬超时、上传总时限四项问题，均修复并针对性复核为 resolved；不把静态复核替代实际远端发布结果。

## 本机 Go 工作台接入

本机 Go API 已重启并接入公网模拟服务，健康检查通过；前端 5173 保留。进入“微信线索 → 同步外呼数据”，选择日期即可同步。

本机连接配置在 `~/.config/cc4-journey/mock-source.env`，文件权限 0600，不入库；重启 Go 进程时需要把它加载到环境。配置包括 GOOKI_HOST、GOOKI_USERNAME、GOOKI_PASSWORD，未在此记录凭据值。真实外呼供应商账号的兼容性验收仍独立于本模拟服务。
