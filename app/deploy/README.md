# CC4 单机发布

目标：Ubuntu 22.04 / x86_64，Nginx 对外提供 **8080 端口**，`/` 自动进入 `/cc4/`；Go API 监听 `127.0.0.1:3101`，PostgreSQL 14 仅本机连接。服务器无需安装 Go、npm 或运行 Vite；构建在 GitHub Actions 完成。

当前入口：`http://150.5.143.32:8080/`。同时保留 80 端口下的 `/cc4/` 兼容入口；80 的原项目根路径转发配置不变。

## 首次准备

1. 安装 `postgresql postgresql-contrib`，保留本机监听与 peer/SCRAM 默认认证。运行 `sudo bash app/deploy/install-reimbursement-support.sh` 安装用于单页PDF校验的`pdfinfo`；OCR在Paddle云端执行，本机无需GPU或识别模型。
2. 为此项目生成专用 Ed25519 发布密钥，将公钥和本目录文件通过已有管理员 SSH 上传。
3. root 执行 `bootstrap.sh <公钥文件>`。脚本针对已经检查过的 `/etc/nginx/sites-enabled/ai-video-studio` 追加自己的 include，不替换原根路径；数据库角色 `cc4` 无超级用户、建库或建角色权限。
4. `/etc/cc4.env` 为 root 0600，自动生成独立 DB/JWT 密钥。可追加 `GOOKI_HOST / GOOKI_USERNAME / GOOKI_PASSWORD` 与 `PADDLEOCR_API_KEY / PADDLEOCR_MODEL`，不要提交该文件。默认OCR模型为`PaddleOCR-VL-1.6`。
5. 配置仓库 Secrets：`CC4_APP_DEPLOY_SSH_KEY`（专用私钥）、`CC4_APP_KNOWN_HOSTS`（经已有可信 SSH 连接核对的主机公钥）。该发布公钥有 `restrict,command="/usr/local/sbin/cc4-deploy"` 限制，不能运行交互命令或转发端口。

空库首次发布会应用全量基线和增量。基线中的本机 `OWNER TO root` 在打包时移除，所有对象归 `cc4` 所有。既有完整数据库快照应先用 `pg_dump --no-owner --no-acl` 导入空库，再用同一打包逻辑生成规范化 SQL，明确执行 `migrate.py <规范化迁移目录> --adopt-existing`。采用记录仅跳过已核对的 `000 / 001` 快照基线并检查关键表/字段，后续增量仍实际执行；后续发布不自动采用未知库。

`002` 迁移对齐导入数据的自增序列：锁住对应表后读取最大主键，只向上推进落后的序列，保留已领先的序列及空表初始值。它解决历史显式 ID 已存在但新建记录撞号的问题，不改动业务记录。

`004` 增加报销、识别任务、版本与处理记录，保留原业务表。原图位于持久上传目录的`receipts/`，不随release替换。已安装旧发布校验器的服务器，应由管理员用本仓库最新版`app/deploy/verify_release.py`更新`/usr/local/lib/cc4/verify_release.py`；静态白名单仅新增两个明确的合成样例PNG/PDF路径，未放开任意上传文件。

同时将`nginx-location.conf`中新加的两个`/cc4/samples/invoice-demo.png`和`.pdf`精确路由并入已安装的`/etc/nginx/snippets/cc4.conf`，`nginx -t`通过后reload。否则SPA兜底会把下载链接返回成HTML。发布检查会逐字节比对两个公开合成样例，并检查报销API匿名401；其他报销原图仍只能通过认证API读取。

初始账号通过受保护的管理员通道配置。数据库快照、密码、截图和登录凭据均不进入 Git 仓库或 Actions 构建产物。

## 自动发布

`.github/workflows/deploy-cc4.yml` 在 main 的 `app/**` 变化时触发，也支持手工触发；PR 运行测试和构建。流程包含 Go race/vet、PostgreSQL 14 完整业务集成、发布包校验测试、前端格式与 TypeScript 检查，以及 `/cc4/` 基路径的生产构建。

构建包只含 Go Linux 二进制、前端静态文件、版本文件、SQL 迁移和 SHA256 清单。服务器拒绝超限解压、路径穿越、符号链接、重复成员和 PAX/GNU 扩展，只安装校验后的常规文件。上传内容中的程序与 SQL 分别以系统用户和数据库角色 `cc4` 运行。

发布会先备份数据库和上传目录，再校验/执行尚未应用的迁移，以软链接切换 release，重启 systemd 服务并检查健康端点的 commit SHA。失败会还原上一个程序与静态文件版本；**已经提交的数据库迁移保留**，因此增量迁移必须兼容上一版应用。数据库不会在健康检查失败时被自动覆盖，避免丢失并发业务写入。需要数据恢复时，先停止应用，核对备份后通过管理员操作恢复。

## 运行与持久化

- 应用：`systemctl status cc4`；日志：`journalctl -u cc4`。
- 程序：`/opt/cc4/releases/<commit>`；当前版本：`/opt/cc4/current`。
- 数据库：本机 PostgreSQL 的 `cc4` 库；不随 release 替换。
- 截图：`/var/lib/cc4/uploads`，仅由认证 API 读取，Nginx 不直接公开。
- 备份：`/var/lib/cc4/backups/<UTC时间>.<随机后缀>/`，仅含 `COMPLETE` 的目录表示完整备份。`database.dump` 为 pg_restore 格式，另有 `uploads.tar.gz`。
- 每次发布前备份，且 `cc4-backup.timer` 每天北京时间 03:30 后随机延迟 0–5 分钟执行，保留约 14 天。数据库与上传归档依次生成；需要严格一致的数据恢复点时应暂停业务写入再手工运行备份。
- 应用以非 root 运行，只能写上传目录，内存上限 512 MiB；Go 内存目标 320 MiB。数据库和 Nginx 的内存另计。

程序回退示例：先核对 `/opt/cc4/releases` 中的目标版本和迁移兼容性，原子替换 `current` 软链接，再 `systemctl restart cc4`；以 `/cc4/health` 与 `/cc4/version.json` 核对前后端版本。
