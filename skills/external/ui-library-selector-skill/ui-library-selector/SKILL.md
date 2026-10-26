---
name: ui-library-selector
description: >-
  Select, search, preview, and install the right React UI component sources. Use in any turn when
  asked to build or modify UI, redesign a layout, improve visual or interaction quality, compare
  current components, choose a component library, or install components. Trigger for "make this
  page look better", "fix this layout", "polish the UI", "UI美化",
  "页面不好看", "改页面", "改布局", "优化界面", "做UI页面", "做原型", "提升页面质感", and
  "交互效果更好". Decompose multi-surface products, then route each surface between existing
  components, shadcn registries, npm libraries, headless primitives, and effect sources such as
  React Bits, Magic UI, and Aceternity UI. Use alongside design/frontend skills: this retrieves materials;
  design skills set art direction and composition. Do not trigger for a mere UI mention, a request
  only to read or wait, a non-React implementation, or a trivial CSS value change where component
  choice cannot materially improve the result.
---

# UI Library Selector

## Purpose

Act as the routing layer between design intent and component implementation. Supply a small, high-quality set of component candidates and the correct retrieval mechanism. Do not replace a design skill, a library-specific skill, or the project's design system.

Activate when a qualifying UI request appears at any point in a conversation. Do not require the request to be the first turn.

## Product Surface Gate

Before selecting any library, split the project into product surfaces by audience, permission boundary, primary job, and visual objective. A public site, authenticated admin, login flow, and data workspace are separate surfaces when those dimensions differ, even when they share a repository, framework, brand, or backend.

- Do not force a project-wide primary library conclusion before evaluating every surface independently.
- Sharing a source is allowed only when it wins the comparison for each surface; shared code is not evidence of shared objectives.
- For each surface, rank the decision criteria, assign explicit weights totaling 100%, and keep hard constraints such as accessibility or framework compatibility as gates rather than hiding them in the score.
- Resolve three layers per surface: **behavior foundation**, **visual effect source**, and **motion orchestration**. A layer may reuse the project, select a source, or explicitly choose none.
- Admin reliability requirements must not suppress a public surface's visual objective, and public effects must not leak into operational flows.

## Non-Negotiable Command Rules

- If no target project is available, or the user requests planning only, do not emit install commands. Report the selected provider and components, then mark commands as deferred until project inspection.
- Before emitting any command, read the selected provider section in `references/install-commands.md`. Never reconstruct registry commands from memory.
- Use namespaced forms such as `@react-bits/...`, `@magicui/...`, and `@aceternity/...`; never emit raw `reactbits.dev/r/...` or `magicui.design/r/...` install URLs.
- For third-party registry items, complete `search`, `view`, and `add --dry-run` before the mutating `add` command.
- Read project configuration before adding an icon package. Do not assume `lucide-react`.
- If current official docs disagree with a reference, use the official command and flag the reference as stale.

## Coordinate With Other Skills

When a design/frontend skill is available, use both in this order:

1. Let the design skill establish audience, visual direction, hierarchy, density, motion restraint, and quality bar.
2. Use this skill to select the three source layers independently for each product surface.
3. Route component-level work to an installed library-specific skill or live registry tool when available.
4. Return to the design skill for composition, responsive refinement, accessibility review, and visual critique.

When shadcn/ui is selected, prefer the official shadcn Skill or shadcn MCP server for live component documentation, registry search, and installation. This skill decides whether shadcn is the right source; the official shadcn tooling handles current component details.

## Intake

Inspect the project before recommending a new system:

- `package.json` including `packageManager`, then lockfiles as fallback
- `components.json`
- existing `components/ui`, `src/components`, and `app/components`
- Tailwind, global CSS, theme, token, and design-system files
- installed UI, animation, icon, table, chart, and form dependencies

Resolve these dimensions for each product surface before selecting a library:

- audience, permission boundary, primary job, and whether the surface can ship independently
- UI type: marketing, product, admin, data-heavy, portfolio, internal tool, or design system
- weighted priorities totaling 100%: visual distinction, brand fit, motion, reliability, accessibility, task efficiency, performance, maintainability, or delivery cost
- existing stack and component system
- desired design language or reference
- animation, mobile, bundle, and performance constraints

Infer answers from the project and conversation first. Ask only unresolved questions that would change the recommendation. Ask no more than three concise questions at once; do not repeat questions already answered.

## Selection Workflow

1. Produce a surface map. If the request mixes public, authenticated, admin, or data-heavy work, classify them separately before any library conclusion.
2. Produce an objective-weight profile for each surface. Use `references/decision-matrix.md` as a starting point and adapt it to explicit user priorities.
3. Check whether existing components can meet each surface's target, not merely whether they can technically render it.
4. Select the behavior foundation, visual effect source, and motion orchestration independently for each surface. Use at most one behavior foundation and one dominant effect source per surface unless the project already has a justified boundary.
5. Apply the Effect Source Gate below before allowing `none` for a high-impact public experience.
6. Use the table of contents or heading search in `references/component-search-index.md` to read only the selected sources and relevant page-pattern sections.
7. Use the least expensive live discovery route:
   - installed project components and local source first;
   - an installed library-specific Skill or MCP next;
   - registry CLI `search`, `view`, and `docs` next;
   - official documentation search only when no structured discovery route exists.
8. For a material effect source, search at least three relevant components when three exist. Inspect exact source and dependencies, run project-specific dry-runs, and compare brand fit, mobile/render cost, offscreen behavior, reduced-motion support, and static fallback before naming the winner.
9. Preview every exact component or registry item before installation. Read only the selected provider sections in `references/install-commands.md` for current command patterns.
10. Install only the selected items with the project's package runner.
11. Read added source files. Fix incompatible aliases, icon imports, styling assumptions, missing dependencies, accessibility gaps, and missing motion fallbacks.
12. Run the project's normal typecheck, build, lint, relevant UI tests, and the Rendered Evidence Gate below.

Never crawl an entire component-library website by default. Never download a full component library into the Skill as a cache. Keep live source code in the target project and keep only routing knowledge in this Skill.

## Effect Source Gate

When the user explicitly prioritizes `wow`, memorable motion, high impact, technological atmosphere, showcase quality, `炫酷`, `科技感`, `视觉冲击`, `哇塞`, or an equivalent goal for a public surface, effect-source evaluation is mandatory.

- Inspect one effect source and at least three relevant component candidates when available; do not stop at a library label.
- Select one dominant effect candidate only after live source, dependency, dry-run, mobile, and reduced-motion comparison.
- `None` is valid only when live evidence shows that available candidates fail a performance, accessibility, or brand-fit gate. Record that evidence and propose a local or static alternative.
- For ordinary product, CRUD, settings, admin, and dense operational surfaces, `none` remains the default unless a specific interaction requires otherwise.
- An effect component and a motion orchestrator solve different problems. React Bits can supply a hero visual while Motion for React, CSS, or View Transitions coordinates page- and scroll-level motion.

If there is no target project or the user requests planning only, report the shortlist and evidence still required. Do not present an uninspected component as the final winner.

## Rendered Evidence Gate

Build and typecheck success do not prove a visual objective. Before claiming a rendered implementation is better or "wow":

- capture comparable desktop and mobile renders, plus the first three seconds of the public entry experience when motion is a primary goal;
- verify that one effect dominates rather than stacking unrelated animated backgrounds, cursors, cards, and text effects;
- record a performance trace or measured frame behavior on representative desktop and mobile conditions, naming the viewport, device/emulation, and throttling instead of inventing an FPS claim;
- test `prefers-reduced-motion` and a static fallback; content, hierarchy, and navigation must remain complete;
- disable the effect and confirm the typography, composition, and brand structure still work;
- compare before and after evidence against the surface-specific weighted objectives.

For planning-only work, mark every unrun item as pending. Do not claim rendered quality without rendered evidence.

## Better-Than-Existing Test

Compare the current implementation and the candidate on:

- interaction behavior: keyboard, focus, gestures, transitions, loading, empty, and error states
- accessibility: semantics, ARIA, screen readers, contrast, reduced motion, and focus visibility
- responsive behavior: touch targets, overflow, density, reflow, and breakpoint behavior
- visual fit: hierarchy, spacing, motion taste, brand fit, and consistency
- maintainability: local editability, API clarity, dependency health, and team familiarity
- performance: bundle impact, animation or WebGL cost, and virtualization needs
- delivery speed: whether a proven component avoids fragile custom code

Score serious alternatives against the surface's weighted objectives with evidence-backed values. The weighted score supports judgment; it does not override a failed hard gate or justify invented precision.

Keep the current system when the gain is minor or the request is primarily layout, spacing, copy, color, or a small responsive fix. Introduce a new source when at least one gain is material and worth the dependency and styling cost.

## Library Defaults

Apply these defaults per product surface, not once per repository:

- Modern app or prototype: `shadcn/ui`, with `Mantine` or `HeroUI` as alternatives.
- SaaS/product app: `shadcn/ui` or `Mantine`.
- Admin/internal tool: `Ant Design`, `MUI`, `Mantine`, or `PrimeReact`.
- Data-heavy view: `MUI X`, `Ant Design`, or `PrimeReact`.
- Custom design system: `Radix UI` or `React Aria Components`.
- Accessibility-critical custom UI: `React Aria Components`, then `Radix UI`.
- Microsoft ecosystem: `Fluent UI`.
- Landing page: primary app primitives plus one of `Magic UI`, `React Bits`, or `Aceternity UI`.
- Portfolio/showcase: primary app primitives plus `React Bits` or `Aceternity UI`.

Do not use React Bits, Magic UI, or Aceternity UI as the behavior foundation for CRUD, settings, admin, or data-entry flows. Do not combine multiple behavior foundations in one surface unless the project already has an explicit boundary.

## Search Scope

Map the request before searching:

- forms: input, select, checkbox, radio, switch, date, upload, validation
- overlays: dialog, drawer, sheet, popover, tooltip, confirmation
- navigation: tabs, sidebar, command menu, breadcrumbs, pagination
- data: table, data grid, filtering, bulk actions, card, chart, list, virtualization
- feedback: toast, alert, progress, skeleton, spinner, empty and error states
- marketing: hero, bento, logo cloud, marquee, testimonial, pricing, CTA, device mockup
- effects: text animation, hover, cursor, animated background, 3D card, shader/WebGL

Search only the matching categories in `references/component-search-index.md`.

## Install Safety

Detect the package runner from `package.json#packageManager`, library-specific project context, and then lockfiles instead of assuming npm. For third-party shadcn-compatible registries, complete `search`, `view`, and `add --dry-run` before writing. When updating an existing local component, review a diff and never overwrite local changes without explicit user approval.

Treat third-party registry items as source code, not trusted binaries. Review what was added and its transitive dependencies. If network or sandbox access is blocked, request approval for the exact command rather than broad access.

## Failure Modes

- If the project type is unsupported, provide analysis only and say that installation guidance is React-specific.
- If live docs disagree with a bundled reference, trust current official docs and report the reference as stale.
- If no candidate is materially better, keep the existing system and explain why.
- If two behavior foundations are already mixed within one surface, avoid adding a third; propose consolidation or isolate the change.
- If a request contains multiple product surfaces, reject any global-library shortcut until each surface has its own weights and three-layer selection.
- If an explicit high-impact public goal returns `none` without live candidate evidence, treat the selection as incomplete and run the Effect Source Gate.
- If exact effect candidates were not inspected, label the recommendation provisional rather than claiming a final component choice.
- If the user has not supplied enough information to choose between materially different systems, ask the smallest decision-changing question before installing.

## Response Contract

Report:

1. Surface map, including audience, permission boundary, primary job, and why surfaces are separate or shared.
2. A weighted objective profile totaling 100% for every surface.
3. Existing-system assessment per surface.
4. A per-surface table for behavior foundation, visual effect source, and motion orchestration; explain every `none`.
5. Exact categories and component candidates, including a live evidence comparison for material effects.
6. Discovery route, completed `search`/`view`/`dry-run` evidence, and exact install commands only when a target project exists.
7. Weighted comparison, rejected alternatives, and why admin and public objectives did not contaminate each other.
8. Rendered desktop/mobile, performance, reduced-motion, static-fallback, and before/after evidence; mark unrun checks pending.
9. Verification completed, remaining risks, and whether each selection is final or provisional.
