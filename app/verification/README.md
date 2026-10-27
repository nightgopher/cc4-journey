# 学员验收包

学员交付时要能演示业务、重跑验证，并解释预期结果的依据。本目录提供统一入口，复用真实业务测试，不依赖老师的私有账号、云端服务器或数据库。

## 1. 从仓库根目录运行

需要 Docker（已启动）、Python 3.9+、Go（版本见 `app/go.mod`，当前1.26；race测试需要CGO与C编译器）、Node.js24/npm、Git、Bash、curl和Poppler的`pdfinfo`（macOS：`brew install poppler`；Ubuntu/WSL：`sudo apt-get install poppler-utils`）。macOS、Linux或Windows WSL2均使用下面同一条命令；首次运行需要下载依赖与PostgreSQL14镜像。无需另外安装psql或创建业务数据库。

```sh
python3 app/verification/verify.py
```

入口会清除继承的业务连接配置，新建自己的PostgreSQL14容器，只把随机端口绑定到127.0.0.1；创建 `cc4_test_*` 测试库，启动临时Go API和独立外呼模拟服务。正常完成或测试失败后删除临时容器、数据库、上传目录；证据目录保留。不会连接老师的云端环境，也不要求先启动工作台。

依次执行Go race/vet、发布包检查、运行器反例测试、验收器反例测试、真实数据库迁移、HTTP/MCP业务及授权/报销集成、连续7日经营与重跑/故障注入、前端格式/日期/TypeScript/生产构建。Go基础测试中未配置数据库而跳过的集成，随后会在独立阶段实际运行；缺少19组HTTP业务集成、MCP业务及授权集成或11组报销集成不会被视为通过。

报销验收使用本地HTTP供应商合同模拟，覆盖异步识别、权限、查重、旧版本操作、故障恢复和并发付款，无需PaddleOCR密钥、不消耗云端额度；执行器会清除继承的`PADDLEOCR_*`。这不能代替真实模型验证。学员接入自己的服务端密钥后，应另用合成票据展示真实识别及审批付款流程，保留独立证据。

成功时退出码为0，最后输出 `[PASSED] .../REPORT.md`；任何阶段失败退出码为1，缺失前置依赖也不会伪装成通过。已有证据目录不覆盖；需要指定新位置时运行：

```sh
python3 app/verification/verify.py --output /tmp/my-cc4-acceptance
```

## 2. 查看与提交证据

默认输出 `app/verification-results/<UTC时间>-<编号>/`，已加入gitignore。主要文件：

| 文件 | 用途 |
|---|---|
| `REPORT.md` | 适合老师查看的阶段结果、日志链接和业务结论 |
| `summary.json` | 退出结果、源commit、工作区是否有未提交内容、每阶段命令/耗时、清理结果 |
| `api-integration.jsonl` | Go实际执行的HTTP/MCP测试事件，能区分pass、fail和skip |
| `reimbursement-report.json` | 报销11组集成结论、合同模拟标识、6次并发付款仅一笔¥125.50、旧版本审批/付款拦截 |
| `simulation/first-report.json` | 第一次业务演练，每项预期/实际、事件和资金结果 |
| `simulation/repeat-report.json` | 完整重跑后的报告 |
| `simulation/counts-before.json`、`counts-after.json` | 7类业务记录数量对比，证明没有重复录单/回款 |
| `simulation/unrelated-month-*.json` | 非模拟团队月记录前后对比 |
| `simulation/negative-report.json` | 故意改错1元后保存的失败断言 |
| `simulation/server.log`、`mock.log` | 临时Go API与模拟上游日志，用于定位业务或联通失败 |
| `*.log` | 各阶段完整输出；失败时先看对应日志 |

`negative-report.json` 必须失败，且失败断言的实际收入必须比预期多1元。只有它符合这个预期，整套验收才能通过；连接失败或任意报错不能冒充“反例验证有效”。资金汇总只取最终“账务复核”明细，避免把操作过程中的重复快照累计两次。

2026-10-27的空测试库基线示例为7日、12位模拟员工、105个事件、831项核对，销售¥7,623、流入¥8,323、流出¥528.50、净流入¥7,794.50。已有工作日配置的演示库可能少1个配置事件；后续持续经营天数与检查项会增长，因此不能把固定831当作唯一通过条件。程序检查连续日期、场景覆盖、断言一致性、资金明细和重复记录等实际条件。

已有commit但包含未提交代码时，`summary.json` 会标记 `dirty=true`。最终提交应提交源码后重跑，或使用GitHub Actions产出的确定commit证据；仅提交一张绿色截图不够。

## 3. 在学员自己的GitHub仓库运行

打开 Actions → **Student acceptance** → Run workflow。该工作流无需任何Secrets，不部署、不调用老师服务器。PR修改相关代码也会执行。结束后下载 `cc4-acceptance-<SHA>` artifact；失败运行同样保留日志与已有证据，保留14天，交作业前自行下载保存。

这与 **Business simulation** 工作流分工不同：前者在隔离库判定源码是否满足验收；后者会向明确启用模拟模式的演示环境写入业务。课堂多人同时操作时，各自使用自己的环境。

## 4. 验收已经部署的演示报告

已有管理员会话时，从进程环境提供 `CC4_VERIFY_TOKEN`，然后执行：

```sh
python3 app/verification/check_report.py \
  --base-url http://127.0.0.1:3101/api/v1 \
  --output /tmp/my-cc4-remote-report.json
```

也可用 `CC4_VERIFY_ADMIN_PHONE` 和 `CC4_VERIFY_ADMIN_PASSWORD` 环境变量登录。凭据不放进命令参数、源码或报告。该命令仅登录并GET报告，不推进/重置业务、不申请执行锁、不重新发工资或收款。接口示例见 [api.http](api.http)。

默认要求报告已推进到北京时间今天；未完成、过期、缺少场景、摘要与明细不符、通过标记与预期/实际矛盾或资金不平都返回非0。它验证报告内容与日期，不能证明报告生成后数据库没有被人工改动；需要重新核对实际业务时，在自己的演示环境执行经营工作流，或重新跑本地隔离验收。

离线复核某次保存的原始报告，不需要账号或网络：

```sh
python3 app/verification/check_report.py \
  --input /path/to/first-report.json --date 2026-10-27
```

`--date`填写该次报告的业务日期；这是历史证据复核，不表示当前服务器仍通过。

## 5. 课堂讲解与边界

按 [CASES.md](CASES.md) 讲清“规则 → 操作 → 预期 → 回读 → 证据”。按 [SUBMISSION.md](SUBMISSION.md) 提交作业与说明。前端自动检查涵盖代码、日期和构建，页面交互与视觉仍按模板中的人工检查项展示。

当前收付款为模拟记账，未接真实银行/ERP；工资保留月末有效职级口径，同月多职级分段规则未在本次新增。通过证明列出的场景，不证明所有业务组合、真实供应商兼容性、性能容量或全部安全性质。

如果进程被强制终止或机器断电，自动清理可能未完成；可按 `database-create.log` 中的本次容器编号检查并清理对应 `cc4-verify-*` 容器，不删除其他容器。
