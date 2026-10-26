# Decision Matrix

Use this file only after inspecting the target project. The matrix is a starting point, not a reason to replace a coherent existing system.

## Decompose Product Surfaces First

The unit of selection is a product surface, not automatically the repository or brand. Split surfaces when audience, permission boundary, primary job, or visual objective differs materially.

| Combined request | Required decomposition |
|---|---|
| Public portfolio plus authenticated CMS | Public showcase and private admin |
| Marketing site plus signed-in SaaS app | Acquisition surface and product workflow |
| Login plus dense data workspace | Authentication flow and operational workspace |
| One monorepo with multiple frontends | Evaluate each independently; share a source only if it wins independently |

Do not let the stability needs of an admin surface reduce the visual target of a public surface. Do not let public-site effects become defaults for forms, CRUD, settings, or data work.

## Objective Weight Profiles

Use explicit weights totaling 100%. These are starting profiles, not universal constants; replace them when the user or project provides stronger evidence. Framework compatibility, legal requirements, and accessibility minimums are hard gates, not negotiable score offsets.

| Surface profile | Weighted objectives |
|---|---|
| Public portfolio/showcase | Visual distinction 35%, brand fit 25%, interaction and motion 20%, performance plus accessible fallback 15%, delivery cost 5% |
| Admin/CRUD | Reliability 30%, accessibility 25%, task efficiency 25%, maintainability 15%, visual impact 5% |
| Marketing/landing | Message clarity and conversion 30%, brand fit 25%, visual distinction 20%, performance and accessibility 20%, delivery cost 5% |
| Data workspace | Task efficiency 30%, reliability 25%, accessibility 20%, performance 15%, maintainability 10% |

Score serious candidates against the profile with evidence-backed values. A weighted total cannot rescue a failed hard gate, and a two-point decimal is not meaningful without measured evidence.

## Three-Layer Selection

Resolve all three rows for every surface. They are complementary roles, not direct competitors.

| Layer | Question | Typical sources |
|---|---|---|
| Behavior foundation | Which primitives or system own state, keyboard behavior, forms, CRUD, and accessibility? | Existing system, shadcn/ui, Radix UI, React Aria, Mantine, Ant Design, MUI |
| Visual effect source | Which source supplies the single memorable visual material, if the objective requires one? | React Bits, Magic UI, Aceternity UI, local/static visual, or evidence-backed none |
| Motion orchestration | What coordinates page transitions, scroll sequences, layout motion, and reduced-motion policy? | Existing setup, Motion for React (formerly Framer Motion), CSS/View Transitions, GSAP, or none |

Do not count a motion orchestrator as a second component system. Do not use an effect source as the behavior foundation for operational UI.

## Route Before Choosing

| Situation | Route |
|---|---|
| Existing component already meets the quality target | Reuse it; use the design skill for composition and polish |
| Existing component is close but weak in layout or styling | Improve it locally; do not add a library |
| Existing component is weak in behavior, accessibility, responsiveness, or complex state | Compare a mature replacement |
| A surface already uses a behavior foundation | Search that system before introducing another |
| Project has `components.json` or uses shadcn registries | Use the official shadcn Skill/MCP/CLI after this skill selects the source |
| No structured registry or library-specific tool exists | Search only the relevant section of official documentation |

## Behavior Foundation

| UI Type | First Choice | Alternatives | Avoid |
|---|---|---|---|
| Modern app or prototype | shadcn/ui | Mantine, HeroUI | Mixing MUI + Ant + shadcn |
| SaaS/product app | shadcn/ui or Mantine | Chakra UI, HeroUI | Heavy animation libraries for core flows |
| Admin dashboard | Ant Design | MUI, Mantine, version-checked PrimeReact | React Bits/Magic UI as behavior foundation |
| Data-heavy page | MUI X, Ant Design, version-checked PrimeReact | Mantine | Hand-rolled data grid |
| Portfolio/showcase | shadcn/ui or existing local primitives | Radix UI or React Aria for bespoke behavior | Ant Design/MUI as the public visual base |
| Landing/marketing page | shadcn/ui or existing app primitives | Radix UI, React Aria, Mantine | Enterprise UI libraries as the public visual style |
| Custom design system | Radix UI or React Aria Components | Headless UI | Styled monolith libraries if full control is required |
| Accessibility-critical custom UI | React Aria Components | Radix UI | Visual-only component snippets |
| Microsoft/internal enterprise | Fluent UI | MUI, Ant Design | Highly branded animation libraries |

## Effect Library Choice

Pick only one effect library by default:

- **React Bits**: best for advanced animated backgrounds, shader/WebGL visuals, text animations, interactive cards, profile/showcase effects.
- **Magic UI**: best for landing page building blocks, device mockups, bento, marquee, text reveal, border beam, terminal/code visuals.
- **Aceternity UI**: best for high-impact hero sections, strong animated marketing patterns, 3D cards, sticky scroll, beams, testimonials.

Use no effect library for admin, CRUD, settings, data entry, or dense operational screens unless there is a very specific interaction need.

An effect library is optional only when visual impact is not a product requirement. `None` is the default for ordinary product and operational surfaces.

When the user explicitly asks for `wow`, memorable motion, high impact, technological atmosphere, showcase quality, `炫酷`, `科技感`, `视觉冲击`, or `哇塞`, evaluating an effect source is mandatory. Inspect at least three relevant component candidates when available. Select `none` only when live source, dependency, mobile, reduced-motion, or brand-fit evidence disqualifies the candidates, and record the evidence.

## Motion Orchestration Choice

- Keep the existing motion setup when it already coordinates page, scroll, and layout transitions coherently.
- Prefer Motion for React (formerly Framer Motion) when a React surface needs shared page transitions, scroll-linked sequences, gestures, layout animation, or a centralized reduced-motion policy.
- Prefer CSS or View Transitions for simple, self-contained transitions where another dependency adds little value.
- Use GSAP only when its timeline or advanced animation capabilities materially beat the existing React/CSS path.
- Choose `none` when the effect components are self-contained and no page-level coordination is needed.

## Multi-Surface Example

For a public personal site plus an independently shipped admin CMS, a valid starting hypothesis is:

| Surface | Behavior foundation | Visual effect source | Motion orchestration |
|---|---|---|---|
| Public portfolio | shadcn/ui or local primitives | React Bits; inspect `Threads`, `LiquidEther`, and `Dither` before choosing | Motion for React when page and scroll choreography is required |
| Custom admin CMS | Radix UI or the existing accessible system | None | Existing restrained CSS/motion, or none |

This is not a universal final answer. The surface map, existing project, weights, live component inspection, dry-run output, and rendered evidence must still support it.

## Quality Bias

- If the UI must be stable and accessible: prefer production npm libraries.
- If the UI must be highly custom and AI-editable: prefer shadcn/copy-to-code.
- If the UI must feel memorable: add one visual/effect library.
- If the UI is a workflow tool used repeatedly: keep animation restrained.

## Existing vs Better Component Decision

Do not ask only "can the current component solve it?" Ask "can the current component solve it at the desired quality level?"

Keep the current components when:

- the requested change is mostly layout, spacing, copy, color, or responsive adjustment;
- the project already has a coherent component system;
- a new library would duplicate existing primitives;
- the quality gain is minor.

Introduce a better component/library when:

- the existing implementation lacks robust keyboard/focus/accessibility behavior;
- the UI needs complex behavior such as date range picking, virtualized table, drag/drop, command palette, carousel gestures, or advanced filtering;
- the desired visual effect is a key part of the experience and would be fragile to hand-roll;
- the candidate component is more maintainable than custom code;
- the project type rewards the improvement, for example a landing page hero, portfolio showcase, or data-heavy dashboard.

When uncertainty would change the dependency strategy, present both options:

1. Use existing components for minimal dependency cost.
2. Add a candidate library/component for higher quality.

Then ask the user which tradeoff they prefer before installing.

## Handoff Boundaries

- This skill owns cross-library selection and retrieval strategy.
- A design/frontend skill owns art direction, hierarchy, typography, spacing, motion restraint, and critique.
- A library-specific skill owns current APIs, component composition rules, and exact live registry commands.
- The target project's tests and design system remain the source of truth for acceptance.
