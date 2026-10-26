# UI Library Selector Agent Skill

> 先选对组件材料，再让设计 Skill 把它做成真正的产品。

一个通用的 Agent Skill，用于帮助 AI 编程助手选择、定向搜索、预览并安装合适的 React UI 组件库。

它不绑定某一个具体客户端。只要客户端支持 Agent Skills / `SKILL.md` 目录格式，就可以使用同一套内容，例如 VS Code Copilot Agent、Claude Code、或其他兼容 Agent Skills 的工具。

它的目标不是替代设计判断，也不是复制每个组件库的文档。它负责跨库路由：先拆分公开站、登录后台、数据工作台等产品表面，分别确定目标权重和三层来源，再只查相关组件类别，最后把具体实现交给项目现有组件、官方 library Skill、MCP、registry CLI 或 npm。

核心定位：这个 Skill 是 design/frontend Skill 与组件库官方工具之间的“选型路由层”。很多 design/frontend Skill 能提升审美判断，但如果没有成熟组件、交互模式和安装入口，最终仍然容易变成手搓 UI。这个 Skill 先解决“用什么材料、从哪里取”，再让设计类 Skill 解决“如何组合和打磨”。

## 解决的问题

AI 做 UI 时常见问题：

- 看到太多组件库后随机混用，页面风格不一致。
- 每次都全量搜索组件库网站，浪费 token 和时间。
- 只问“现有组件能不能做”，但不判断“有没有明显更好的实现”。
- 不区分基础 UI、生产级业务组件、动效组件、无样式交互原语。
- 把 shadcn/ui、React Bits、MUI、Ant Design、Magic UI 等放在同一层比较。
- 把公开作品集和独立后台压成一个 UI，用后台的稳定性权重稀释公开站的视觉目标。
- 为了一个小视觉效果引入过重依赖。

这个 Skill 把 UI 选型流程固定下来：

```text
识别项目现有技术栈
-> 必要时追问关键问题
-> 按用户、权限、任务和视觉目标拆分产品表面
-> 为每个表面给出总计 100% 的目标权重
-> 分别选择行为基础层、视觉效果层、动画编排层
-> 只查相关组件分类
-> 搜索、查看源码与依赖、执行 dry-run
-> 用 shadcn CLI 或 npm 安装
-> 构建/类型检查/桌面与移动渲染验收
```

## 和设计类 Skill 的区别

很多 design/frontend 类 Skill 主要解决的是“如何让界面更好看、更像高质量产品”：视觉层级、排版、留白、颜色、动效节制、响应式、组件结构等。

它们的常见短板是：方法论很好，但没有稳定的组件选择对象。AI 知道“应该高级、克制、有视觉 thesis”，却不知道当前任务应该优先查看哪些成熟组件、从哪个库安装、用 CLI 还是 npm、哪些组件已经内置状态/可访问性/响应式行为。结果经常是原则正确，但实现仍然手搓，成品质量不稳定。

这个 Skill 更窄，也更工程化。它专门解决：

- 当前 UI 任务属于什么类型；
- 是否应该继续用现有组件；
- 是否值得引入更好的组件库；
- 应该查哪个组件库的哪个类别；
- 应该用 shadcn registry CLI 还是 npm 包安装；
- 如何避免全站爬取和多库混用。

它应该和通用 frontend/design Skill 配合使用，而不是替代它：

```text
ui-library-selector
-> 拆分产品表面和目标权重
-> 分别选择行为基础、视觉效果、动画编排
-> 定向查找组件分类
-> search / view / dry-run 后再安装
-> 用 CLI/npm 安装或引入组件

frontend/design skill
-> 确定视觉 thesis
-> 控制品牌感、层级、排版、留白、动效
-> 避免模板感和 AI 默认审美
-> 做最终视觉 critique
```

一句话：这个 Skill 负责“拿到好材料”，design/frontend Skill 负责“把材料做成好设计”。

它也不替代组件库自己的专用 Skill。例如官方 shadcn Skill 负责 shadcn 当前 API、组合规则、registry 搜索和安装；本 Skill 负责先判断这个项目到底该不该选 shadcn，还是应该选 Mantine、Ant Design、MUI、React Aria、React Bits 等其他来源。

推荐协作顺序：

```text
design/frontend skill 确定用户、视觉方向和质量目标
-> ui-library-selector 拆分产品表面并选择三层来源
-> 官方 library Skill / MCP / CLI 获取当前组件
-> design/frontend skill 完成组合、响应式、reduced-motion 和视觉 critique
```

## 适用场景

- 创建 UI 页面
- 修改当前页面布局效果
- 优化页面视觉质量
- 美化 UI / 美化界面
- 页面不好看，需要提升质感
- 页面效果一般，需要改布局
- 优化当前页面或当前界面
- 做产品原型
- 做 landing page
- 做 SaaS / dashboard / admin panel
- 做 portfolio / showcase
- 选择 React 组件库
- 安装 shadcn/ui、React Bits、Magic UI、Aceternity UI、MUI、Ant Design、Mantine、HeroUI 等组件

也覆盖更口语化的表达，例如：

```text
美化一下这个 UI
这个页面不好看，帮我改高级一点
优化当前页面布局效果
做一个 UI 页面
做一个产品原型
让这个界面更有质感
交互效果更好一点
```

这些请求通常不只是 CSS 调整，而是需要先判断有没有更合适的组件、页面区块、交互模式或动效组件。

## 不适用场景

- 非 React 项目，除非只是做 UI 选型分析。
- 已有强约束设计系统，且明确禁止引入外部 UI 库。
- 只需要改一两个 CSS 值，不需要组件选型。
- 只是提到 `UI`、阅读 UI 文档或让 Agent 等待下一步，没有要求实施、选型或改进。

## 触发机制说明

这个 Skill 不是只能在对话开头触发。只要当前会话已经加载了这个 Skill 的 metadata，后续任意一轮用户提出“做 UI / 改 UI / 美化页面 / 推荐组件库 / 安装组件”等任务，都应该可以触发。

但有三个边界：

- Skill 触发看的是当前用户请求的任务意图，不是上下文里只要出现 `ui` 这个词就触发。
- 如果旧线程创建时还没有安装这个 Skill，该线程可能不会自动刷新新安装的 Skill metadata；新开线程或重新加载后更可靠。
- 如果用户只是让 AI “阅读文档，等待指令”，即使文档里有 UI 内容，也通常不应触发这个 Skill；等用户明确要求做界面、改页面、选组件或安装组件时再触发。
- 如果用户在对话中途提出新的 UI 实施或优化任务，应该在那一轮触发，不需要回到对话开头。

## 设计原则

### 0. 先给 AI 可选择对象，再谈设计质量

只给 AI 设计原则，通常不够。真实 UI 开发里，设计师和前端工程师会先知道有哪些组件、模式和交互可以借用，再进行组合、改造和视觉表达。

因此这个 Skill 的第一优先级不是生成漂亮描述，而是建立明确的选择面：

- 基础 UI：shadcn/ui、Mantine、HeroUI、Chakra UI
- 企业/后台/数据密集：Ant Design、MUI、PrimeReact、Fluent UI
- 自定义无障碍原语：Radix UI、React Aria、Headless UI
- 视觉/动效/营销增强：React Bits、Magic UI、Aceternity UI

设计方法论用于后续筛选和打磨，不应该替代组件对象池。

### 1. 先拆产品表面，再判断 UI 类型

仓库、品牌或后端相同，不代表 UI 目标相同。公开 Portfolio、登录后台、认证流程和数据工作台只要用户、权限、任务或视觉目标不同，就必须独立选型。只有某个来源在每个表面都独立胜出时，才允许共享；不能先决定“一套库全站通用”再倒推理由。

同样是“卡片”，后台、SaaS、作品集、landing page 的最佳选择不同：

- 后台：Ant Design / MUI / Mantine 的稳定组件更重要。
- SaaS：shadcn/ui / Mantine 更适合。
- 作品集：React Bits / Aceternity 的视觉效果更有价值。
- landing page：Magic UI / Aceternity / React Bits 适合做少量高光效果。

### 2. 不是“能不能做”，而是“能不能达到质量目标”

Skill 会比较现有组件与候选组件：

- 交互质量
- 可访问性
- 响应式
- 视觉质量
- 维护性
- 性能
- 开发速度

如果现有组件能做但质量一般，Skill 可以推荐更好的组件库；如果只是小修小改，则不会为了轻微收益引入新依赖。

### 3. 只允许有限组合

每个产品表面默认只选三层：

- 一个行为基础层：现有系统、shadcn/ui、Radix、React Aria、Ant Design 等
- 一个视觉效果层：React Bits、Magic UI、Aceternity UI、local/static，或有证据的 none
- 一个动画编排层：现有方案、Motion for React（原 Framer Motion）、CSS/View Transitions，或 none

避免同一表面混用 MUI、Ant、Mantine、shadcn、HeroUI 或多个主导动效。用户明确要求“哇塞、科技感、视觉冲击”时，视觉效果层必须实测候选；只有性能、无障碍或品牌不匹配有证据时，才能选择 none。

### 4. 减少 token 和爬取成本

Skill 内置了 `component-search-index.md`，把常见库按类别索引：

- shadcn/ui: forms, overlays, navigation, data display, feedback
- React Bits: text animations, animations, components, backgrounds
- Magic UI: marketing, bento, marquee, text reveal, device mockups
- Aceternity UI: hero, backgrounds, cards, sticky scroll, testimonials
- MUI / Ant / Mantine / HeroUI / PrimeReact 等生产级组件分类

AI 不应该每次全站爬取，而是根据当前任务只查相关类别。

## Skill 结构

```text
ui-library-selector-skill/
├── README.md
├── LICENSE
├── examples/
│   ├── forward-test-results.md
│   └── multi-surface-regression-results.md
├── reports/
│   ├── luban-skill-report.md
│   └── luban-badcase-report.md
├── scripts/
│   └── check-skill.sh
├── tests/
│   └── test-prompts.json
└── ui-library-selector/
    ├── SKILL.md
    └── references/
        ├── component-search-index.md
        ├── decision-matrix.md
        ├── install-commands.md
        └── library-catalog.md
```

### `SKILL.md`

主工作流：

- 什么时候追问
- 如何判断 UI 类型
- 如何评估现有组件是否足够好
- 如何为每个表面选择行为基础、视觉效果和动画编排
- 如何控制安装和验证

### `references/decision-matrix.md`

库选择决策矩阵：

- 如何拆分公开站、后台、认证和数据工作台
- 如何给每个表面设置总计 100% 的目标权重
- 如何分别选择行为基础、视觉效果和动画编排
- landing page 用什么
- 后台用什么
- 数据密集页用什么
- 自定义设计系统用什么
- 什么时候保留现有组件
- 什么时候引入更好的组件库

### `references/component-search-index.md`

高频组件类别索引，用来减少全站爬取。

### `references/install-commands.md`

安装命令：

- shadcn/ui
- React Bits
- Magic UI
- Aceternity UI
- MUI
- Ant Design
- Mantine
- Chakra UI
- HeroUI
- Radix UI
- React Aria Components
- Headless UI
- PrimeReact
- Fluent UI
- lucide-react

### `references/library-catalog.md`

每个组件库的定位、优点、缺点。

## 安装

在本项目目录中，可以用跨 Agent 的 `skills` CLI 安装展开源码：

```bash
npx skills add ./ui-library-selector
```

安装为用户级 Skill：

```bash
npx skills add ./ui-library-selector -g
```

从 GitHub 仓库安装：

```bash
npx skills add wangwei-wade/ui-library-selector-skill
```

当前仓库为私有仓库，安装端需要先取得该 GitHub 仓库的读取权限。

也可以手工复制到兼容客户端支持的位置。

项目级安装通常可以使用：

```bash
mkdir -p .agents/skills
cp -R ui-library-selector .agents/skills/ui-library-selector
```

安装后，新会话或重新加载 skills 后，兼容客户端可以通过 `description` 自动判断何时激活这个 Skill。

如果同名 Skill 曾被手工安装到多个目录，通用 CLI 可能只更新其 canonical copy，而客户端仍读取旧副本。升级后应运行客户端的 Skill 列表或直接比较实际加载路径，避免 `~/.agents/skills`、客户端专属目录和项目级目录同时存在不同版本。

## 使用示例

```text
做一个 AI SaaS 首页，想要高级一点，但不要太花。
```

预期行为：

```text
分类：landing / SaaS marketing page
行为基础：shadcn/ui
视觉效果：Magic UI 或 React Bits，先列候选，未检查前不宣布组件赢家
动画编排：现有方案；需要页面级联动时再评估 Motion for React
搜索范围：hero、bento、marquee、background、CTA
状态：没有目标项目，只做规划；不输出安装命令，search/view/dry-run 待项目检查
```

多产品表面示例：

```text
为我的个人品牌做 React 网站：公开作品集要独特、科技感和哇塞动效；我登录后的独立后台要稳定、无障碍、表单和 CRUD 高效。现在只做方案。
```

预期行为：

```text
先拆成公开 Portfolio 与登录 Admin 两个 surface，不给“全站唯一主库”。
公开站权重：视觉辨识度 35 / 品牌契合 25 / 动效 20 / 性能与降级 15 / 成本 5
  行为基础：shadcn/ui 或现有 local primitives
  视觉效果：React Bits；Threads / LiquidEther / Dither 为待实证短名单
  动画编排：需要页面和滚动联动时评估 Motion for React
后台权重：可靠性 30 / 无障碍 25 / 操作效率 25 / 维护性 15 / 视觉 5
  行为基础：Radix UI 或现有 accessible system
  视觉效果：none
  动画编排：现有克制方案或 none
所有组件结论在 search / view / dry-run 和真实渲染前标记为 provisional。
```

后台单表面示例：

```text
优化当前后台用户管理页面，表格筛选和批量操作太弱。
```

预期行为：

```text
分类：admin / data-heavy interface
优先检查现有 UI 系统
推荐：Ant Design / MUI X / PrimeReact
不推荐：React Bits / Magic UI / Aceternity
原因：这里需要稳定表格、筛选、分页、批量操作，不是视觉动效。
```

## 维护建议

- 每隔一段时间更新 `component-search-index.md`，加入新高频组件。
- 不要把完整组件源码放进 Skill，避免过期和上下文膨胀。
- 保留安装命令和分类索引，让 AI 按需拉取组件。
- 如果某个团队固定使用某套设计系统，可以在 `decision-matrix.md` 里提高它的优先级。
- 修改触发条件或选型规则后，运行 `bash scripts/check-skill.sh`，并回放 `tests/test-prompts.json`。

## 安全边界

- 不会因为用户只提到 `UI` 就自动安装依赖。
- 不会默认全站抓取组件库，也不会把整套组件源码塞进 Skill。
- 不会把公开站与后台压成一个全局主库结论，也不会在同一表面随意混用多个行为基础。
- 安装第三方 registry 组件前完成 `search`、`view` 和 `--dry-run`，安装后读取新增源码和依赖。
- 不会在缺少桌面、移动、性能和 reduced-motion 证据时宣称页面已经达到“哇塞”目标。
- 更新本地已修改的 copy-to-code 组件时，不会未经确认直接 `--overwrite`。

## 验证

```bash
bash scripts/check-skill.sh
```

验证包括 frontmatter、必需引用文件、多产品表面门、三层选型契约、效果源门、渲染验收门、过时命令模式和测试样例 JSON。模型级触发与选型回放样例位于 `tests/test-prompts.json`。

## 当前支持的库

- shadcn/ui
- React Bits
- Magic UI
- Aceternity UI
- Motion for React（原 Framer Motion）
- MUI
- Ant Design
- Mantine
- Chakra UI
- HeroUI
- Radix UI
- React Aria Components
- Headless UI
- PrimeReact
- Fluent UI
- lucide-react

## License

This repository contains workflow instructions and reference indexes only. Third-party component libraries keep their own licenses.
