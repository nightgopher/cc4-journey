# Library Catalog

Last reviewed: 2026-07-13. Use this catalog for positioning and tradeoffs; use live official documentation for current APIs and install details.

## Contents

- Copy-to-code foundations and effects: shadcn/ui, React Bits, Magic UI, Aceternity UI
- Motion orchestration: Motion for React
- Production UI systems: MUI, Ant Design, Mantine, Chakra UI, HeroUI
- Headless and accessibility foundations: Radix UI, React Aria Components, Headless UI
- Enterprise specialists: PrimeReact, Fluent UI

| Library | Official source |
|---|---|
| shadcn/ui | https://ui.shadcn.com/ |
| React Bits | https://reactbits.dev/ |
| Magic UI | https://magicui.design/ |
| Aceternity UI | https://ui.aceternity.com/ |
| Motion for React | https://motion.dev/docs/react |
| MUI | https://mui.com/ |
| Ant Design | https://ant.design/ |
| Mantine | https://mantine.dev/ |
| Chakra UI | https://chakra-ui.com/ |
| HeroUI | https://www.heroui.com/ |
| Radix UI | https://www.radix-ui.com/ |
| React Aria Components | https://react-spectrum.adobe.com/react-aria/components.html |
| Headless UI | https://headlessui.com/ |
| PrimeReact | https://primereact.org/ |
| Fluent UI | https://react.fluentui.dev/ |

## shadcn/ui

Best for modern React UI where AI should be able to edit local component code. It is a code distribution system, not a runtime component package. Components are copied into the app.

Strengths:

- local source code
- Tailwind-first
- good defaults
- easy for AI to modify
- strong for app UI basics
- official Agent Skill, MCP, and registry CLI provide structured live search and installation

Weaknesses:

- not a heavy enterprise widget suite
- complex tables/data workflows need extra libraries

## React Bits

Best for memorable visual interactions: backgrounds, text animation, hover effects, profile cards, 3D/showcase components.

Strengths:

- strong animated visuals
- many backgrounds and text effects
- shadcn-compatible registry
- official MCP is available for live component discovery in compatible clients
- variants for JS/TS and CSS/Tailwind

Weaknesses:

- not a base UI system
- some components may require GSAP, Motion, three.js, or OGL
- can hurt performance if overused

## Magic UI

Best for landing-page sections and lightweight marketing effects.

Strengths:

- strong landing page components
- many examples
- shadcn registry format
- useful for bento, marquee, terminal, device mockups

Weaknesses:

- less suited to admin/product workflows
- effects can feel generic if overused

## Aceternity UI

Best for high-impact hero/marketing visuals.

Strengths:

- many visual components and blocks
- strong animation style
- shadcn install support

Weaknesses:

- visually dominant
- many components depend on Motion or other effects
- use sparingly for consistency

## Motion for React

Formerly Framer Motion. Best for coordinating React page transitions, scroll-linked sequences, gestures, layout motion, and shared reduced-motion policy. It is an animation orchestrator, not a behavior foundation or a visual component catalog.

Strengths:

- declarative React animation and sequencing
- layout, gesture, scroll, enter/exit, and reduced-motion APIs
- complements Radix, shadcn/ui, React Bits, and custom components without becoming another UI system

Weaknesses:

- unnecessary for isolated CSS transitions or a self-contained effect component
- does not supply a visual thesis, accessible form behavior, or CRUD components

## MUI

Best for mature production apps and Material Design-compatible products.

Strengths:

- very mature
- broad ecosystem
- MUI X for advanced data grid
- strong documentation

Weaknesses:

- Material visual signature is strong
- deep brand customization can be work

## Ant Design

Best for enterprise admin and CRUD-heavy systems.

Strengths:

- excellent Table/Form/DataPicker/Upload/Tree coverage
- strong enterprise patterns
- good for Chinese/internal business apps

Weaknesses:

- Ant visual signature is strong
- less suitable for marketing/portfolio pages

## Mantine

Best for product apps needing broad coverage and flexible styling.

Strengths:

- many components and hooks
- good developer experience
- less visually rigid than MUI/Ant

Weaknesses:

- its styling system must be followed consistently

## Chakra UI

Best for fast SaaS/product UI with accessible component composition.

Strengths:

- friendly API
- good layout primitives
- accessible defaults

Weaknesses:

- default visuals may need polish for high-end marketing pages

## HeroUI

Best for modern npm-based UI with attractive defaults and React Aria influence.

Strengths:

- modern visual defaults
- good advanced inputs
- table/calendar/date/color components

Weaknesses:

- less universal adoption than MUI/Ant/shadcn
- current major versions can impose specific React, Tailwind, and styles-package prerequisites; verify before selection

## Radix UI

Best for building custom design systems from unstyled accessible primitives.

Strengths:

- excellent interaction primitives
- accessibility-focused
- useful foundation for custom components

Weaknesses:

- no visual design by default
- slower for AI to produce polished UI directly

## React Aria Components

Best for accessibility-critical custom components and complex interactions.

Strengths:

- excellent accessibility
- advanced widgets like date pickers, table, tree, virtualizer
- styleable components

Weaknesses:

- requires visual styling work
- more complex API surface

## Headless UI

Best for Tailwind projects needing a small set of accessible unstyled primitives.

Strengths:

- simple and Tailwind-friendly
- good core primitives

Weaknesses:

- smaller component set

## PrimeReact

Best for heavy enterprise widgets and data-heavy apps.

Strengths:

- large module surface
- DataTable, TreeTable, VirtualScroller, FileUpload, OrganizationChart
- useful when business widgets matter more than custom brand feel

Weaknesses:

- visual style often needs theme work
- current documentation spans different package structures across major versions; pin and verify the intended major before installation

## Fluent UI

Best for Microsoft-style internal tools.

Strengths:

- enterprise-ready
- DataGrid, Persona, Toolbar, Tree, Toast
- fits Microsoft ecosystem

Weaknesses:

- strong Fluent design language
- not ideal for expressive marketing pages
