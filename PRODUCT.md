# 呼叫中心 4.0

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

教育体验课电销团队的销售、团队经理、职场经理、人力、财务与管理员。不同角色操作自己的业务范围。

## Product Purpose

把线索跟进、客户、订单、员工出勤、业绩审核和月度工资串在一个工作台里。本仓库同时用于演示从客户需求到 Go 实现的研发交付过程。

## Operating Context

来源为 `journey/00-original-brief.md`、`journey/01-requirements-clarify.md`、`PROGRESS.md`，以及作为领域对照的 `nightgopher/cc4-callcenter`。本次工作是补全现有应用，用户已明确授权按仓库 `skills/external` 的前端技能直接实施，无需再次询问选型与设计偏好。

## Capabilities and Constraints

- Go、PostgreSQL、React、TypeScript 与 Vite；现有组件位于 `app/web/src/components/kit.tsx`。
- 中文业务界面；薪资采用既定 D1、D3、D4、D6 裁决，具体见金标准文档。
- 列表、表单、筛选、分页和业务反馈必须连接真实 API；演示数据明确来自测试库。
- 财务、薪资、审批和员工资料按角色与所属团队控制访问。
- 外部呼叫数据同步需要部署方提供服务配置；不在源码中内置凭据。

## Brand Commitments

保留“呼叫中心 4.0”名称与中文业务用语。用户最新要求完整重写前端，原界面仅作为业务功能参考；视觉与页面结构重新设计。实现与检查使用本仓库收录的 frontend-design、ui-library-selector、Apple Design、Web Interface Guidelines 和 Impeccable，设计与选型由本次实施直接决策。

## Evidence on Hand

现有 Go 领域测试、`app/test/golden/` 金标准、数据库基线 `app/migrations/000_baseline_schema.sql`，以及现有前端令牌和组件。现有进度声明必须由本次编译、测试与实跑结果重新核对。

## Product Principles

- 每个页面先服务一个明确的业务任务。
- 金额、等级规则和归属关系可追溯到业务口径。
- 写入完成后立即反映最新状态；错误给出可执行的恢复方式。
- 权限通过服务端执行，前端同时提供相应的导航和操作入口。
