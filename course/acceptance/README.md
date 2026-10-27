# 学员验收包（契约 0.1）

你的系统怎么设计都可以：表结构、API、页面、技术栈全由你定。验收只看一件事：**给定同样的业务局面，你的系统算出来的钱和规则卡一致**。

方案背景见 [../student-verification-plan.md](../student-verification-plan.md)。

## 包里有什么

| 文件 | 用途 |
|---|---|
| [RULES.md](RULES.md) | 规则卡：钱怎么算，R01–R10；D1 待裁决 |
| [contract.json](contract.json) | 验收契约：你的适配层要支持的动作 |
| [calendar.json](calendar.json) | 验收用工作日日历，验收器开跑时下发 |
| [scenarios/](scenarios/) | 剧本：L1 规则单测、L2 业务流程、L3 连续七日经营（见 [L3.md](L3.md)），每个期望值标规则编号 |
| [accept.py](accept.py) | 验收器，仅需 Python 3.9+，无第三方依赖 |
| [examples/toy_system.py](examples/toy_system.py) | 一个与讲师系统完全不同的内存版实现，演示「换实现也能接上」 |
| [adapters/cc4-reference/](adapters/cc4-reference/) | 讲师系统的参考适配，写自己的适配时照着看 |
| [HIDDEN.md](HIDDEN.md)、[tools/hidden_seeds.py](tools/hidden_seeds.py) | 讲师评分用的隐藏变体（学员不需要） |

## 你要交付的

1. 你的系统（能一键启动）。
2. 一层**薄适配**：一个 HTTP 入口 `POST /acceptance/act`，把契约动作翻译成对你系统的调用。
3. `adapter.json`：写明你的裁决和不支持的动作。
4. 验收报告目录（`REPORT.md` + `summary.json`）。

## 适配层怎么写

请求：

```json
{"action": "record_orders", "args": {"emp": 17, "date": "2026-01-05", "count": 3}}
```

回应只有三种：

```json
{"ok": true, "id": 17}                               // 写动作：创建了对象就返回你系统里的 ID
{"ok": true, "result": {"orders": 3, "sales": 297, "commission": 30}}   // 观测动作
{"ok": false, "error": "REJECTED"}                    // 业务拒绝：超收、重复审核……
{"ok": false, "error": "UNSUPPORTED"}                 // 你没做这个功能
```

要点：

- **ID 由验收器记住**。`hire` 返回你系统的员工 ID，之后的动作会把这个 ID 原样传回来。适配层不需要、也不应该保存任何映射。
- **日期都是业务日期**，随动作传入。你的系统要能按指定日期记录订单、考勤、回款，不能只用服务器当前时间。
- **适配层只做翻译**：调你的 API、改字段名、处理登录。提成、工资等任何计算都必须在你的系统里完成。验收器会在中途**重启适配进程**，还会扫描适配代码里的规则常量（88、2200 等）。
- 没做的功能就返回 `UNSUPPORTED`，或写进 `adapter.json` 的 `unsupported`。对应场景记为「不支持」，不算失败。
- 适配层建议让 AI 生成，人工审核。参考实现约 300 行。

`adapter.json` 示例：

```json
{
  "system": "张三的电销系统",
  "decisions": {"D1": "carry_over"},
  "unsupported": ["submit_order"]
}
```

## 运行

先启动你的系统和适配层，再运行：

```bash
python3 course/acceptance/accept.py --adapter http://127.0.0.1:9000/acceptance/act --manifest adapter.json --adapter-src path/to/adapter
```

如果由验收器替你启动适配进程，加上 `--adapter-cmd`，这样会额外启用中途重启检查（正式提交必须带上）：

```bash
python3 course/acceptance/accept.py --adapter http://127.0.0.1:9000/acceptance/act --adapter-cmd "python3 adapter.py --port 9000" --manifest adapter.json --adapter-src adapter.py
```

开发时可以只跑一部分，例如 `--layer L1`、`--rule R04`、`--only L1-R03-02`。L3 一个场景就用到全部动作，建议 L1/L2 全绿后再跑。

报告默认写到 `course/acceptance/results/<时间>-<编号>/`。失败项会指出是哪一步、期望多少、实际多少，以及违反了哪条规则，并附带提示。

## 通过条件

- 所有已支持的场景通过，没有契约错误；
- 验收器自检的「1 元反例」通过：把一个期望值改动 1 元后，验收器必须判为失败；
- 正式提交时，带 `--adapter-cmd` 运行，并且重启检查通过。

自动验收只证明**钱算对了**。页面、权限、流程设计另由评审和答辩评分。

## 先看一个例子跑通

```bash
python3 course/acceptance/examples/toy_system.py --port 9000 &
echo '{"decisions": {"D1": "carry_over"}}' > /tmp/toy-adapter.json
python3 course/acceptance/accept.py --manifest /tmp/toy-adapter.json
```

玩具实现把数据存在内存里，所以不能带 `--adapter-cmd`：重启会丢数据，验收器会因此判它失败。这正是重启检查要抓的「适配层自己攒账」的情况。

## 课程方：维护与自检

```bash
python3 -m unittest discover -s course/acceptance -p 'test_*.py'
```

这组自检确认三件事：玩具实现全部通过；8 种似是而非的错误实现（提成线性、待审计入、额度每月重置、历史按当前等级、请假被覆盖、允许超收、重复发放计两次流出等）各自被判失败，并且定位到正确的规则编号；裁决分支、`UNSUPPORTED`、重启检查都按预期工作。

```bash
bash course/acceptance/adapters/cc4-reference/run-reference.sh
```

这条命令用一次性的 PostgreSQL 启动讲师的 CC4 系统，通过参考适配跑全部剧本。2026-10-27 的运行结果：15 个场景（L1 10 个、L2 4 个、L3 1 个）、190 项检查全部通过，重启检查和 1 元反例也都通过。需要 Docker 和 Go。
