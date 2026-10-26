# Forward-Test Results

Run date: 2026-07-13

These are condensed records from independent agent runs. The agents received the Skill and a user-like request without the expected answer.

## Case 1: Existing shadcn Settings Page

Prompt:

```text
项目已经有 components.json。给设置页加一个账号删除确认流程，优先复用现有组件。
```

Observed result:

- Classified the task as a high-risk SaaS settings flow.
- Kept the existing shadcn/ui system.
- Selected `AlertDialog`, `Button`, `Input`, `Label`, and existing toast/alert behavior.
- Rejected an effect library.
- Correctly emitted no install command because the existing components should be inspected first.

Outcome: pass.

## Case 2: Data-Heavy Admin Page

Prompt:

```text
重做后台用户管理页，要高级筛选、批量操作、虚拟滚动和可访问性。
```

Observed result:

- Classified the task as enterprise CRUD and data-heavy UI.
- Recommended Ant Design when no existing system is present.
- Said to retain MUI, PrimeReact, or Mantine when already established.
- Rejected visual effect libraries.
- Identified table, filters, bulk operations, virtual scrolling, failure states, and accessibility requirements.

Outcome: pass.

## Case 3: AI SaaS Landing Page

Prompt:

```text
做一个克制但有记忆点的 AI SaaS 首页，允许少量动效。
```

Observed result:

- Correctly selected shadcn/ui as the primary source and React Bits as the single effect source.
- Correctly limited effects and considered reduced motion and mobile fallback.
- Incorrectly emitted deprecated raw React Bits URLs and assumed `lucide-react` without project inspection.

Outcome: fail, useful live finding.

Resulting change:

- Moved command rules near the top of `SKILL.md`.
- Planning-only or no-project requests now forbid install commands.
- Real-project commands must read the selected provider section, use namespaced registries, preview before installation, and inspect the existing icon library.

Post-fix evidence:

- Static checks reject deprecated React Bits and Magic UI raw URL patterns in the Skill package.
- A fresh independent agent reran the same planning-only scenario after the command gate moved to the top of `SKILL.md`.
- It kept shadcn/ui as the primary source, one restrained React Bits effect as the optional visual source, and emitted no install commands because no target project was available.

Post-fix outcome: pass for the command gate.
