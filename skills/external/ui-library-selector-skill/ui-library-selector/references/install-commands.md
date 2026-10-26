# Install and Discovery Commands

Last verified against official documentation: 2026-07-13.

Treat these as command patterns, not permanent API guarantees. If a command fails or this file is materially old, check the library's current official documentation and update this reference.

## Contents

- Project runner selection
- Optional shadcn Skill and MCP setup
- shadcn search, docs, preview, install, and update workflow
- Copy-to-code registries: React Bits, Magic UI, Aceternity UI
- Motion orchestration: Motion for React
- npm libraries: MUI, Ant Design, Mantine, Chakra UI, HeroUI, Radix UI, React Aria, Headless UI, PrimeReact, Fluent UI
- Verification checklist

## Select the Project Runner

| Lockfile | Add packages | Run one-off CLI |
|---|---|---|
| `pnpm-lock.yaml` | `pnpm add` | `pnpm dlx` |
| `yarn.lock` | `yarn add` | `yarn dlx` |
| `bun.lock` / `bun.lockb` | `bun add` | `bunx --bun` |
| `package-lock.json` or none | `npm install` | `npx` |

Prefer `package.json#packageManager` and library-specific project context. Use lockfiles as fallback. Examples below use `npx` and `npm install`; substitute the matching runner before executing.

## Optional AI Setup for shadcn Registries

Install the official shadcn Agent Skill when the user's agent supports portable Skills and recurring shadcn work justifies it:

```bash
npx skills add shadcn/ui
```

Connect the shadcn MCP server when the agent supports MCP and live multi-registry search is useful:

```bash
npx shadcn@latest mcp init
```

Client support differs. Follow the official MCP page for the target agent; do not assume `mcp init` can update every client's configuration. These commands configure the AI environment. Do not run them merely to add one component when the existing CLI route is sufficient.

Official sources:

- https://ui.shadcn.com/docs/new
- https://ui.shadcn.com/docs/mcp

## shadcn Project Workflow

Inspect the current project first:

```bash
npx shadcn@latest info --json
```

Search only selected registries and categories:

```bash
npx shadcn@latest search @shadcn -q "sidebar"
npx shadcn@latest search @shadcn @magicui -q "hero"
```

Read current component docs and inspect registry metadata:

```bash
npx shadcn@latest docs button dialog select
npx shadcn@latest view @shadcn/button
```

Preview project-specific changes before installation:

```bash
npx shadcn@latest add @shadcn/button --dry-run
npx shadcn@latest add @magicui/bento-grid --dry-run
```

Install only after inspection:

```bash
npx shadcn@latest add button dialog form
npx shadcn@latest add @magicui/bento-grid
```

When updating an existing local component, inspect differences and preserve local edits:

```bash
npx shadcn@latest add button --dry-run
npx shadcn@latest add button --diff button.tsx
```

Never use `--overwrite` without explicit user approval.

Official CLI source: https://ui.shadcn.com/docs/cli

## Copy-to-Code Registries

### React Bits

React Bits uses the `@react-bits` registry and includes a technology suffix. Select the variant that matches the project:

- `JS-CSS`
- `JS-TW`
- `TS-CSS`
- `TS-TW`

```bash
npx shadcn@latest search @react-bits -q "profile"
npx shadcn@latest view @react-bits/ProfileCard-TS-CSS
npx shadcn@latest add @react-bits/ProfileCard-TS-CSS --dry-run
npx shadcn@latest add @react-bits/ProfileCard-TS-CSS
```

For a technological hero shortlist, repeat the full inspection for each exact candidate before choosing one:

```bash
npx shadcn@latest search @react-bits -q "background"
npx shadcn@latest view @react-bits/Threads-TS-TW
npx shadcn@latest view @react-bits/LiquidEther-TS-TW
npx shadcn@latest view @react-bits/Dither-TS-TW
npx shadcn@latest add @react-bits/Threads-TS-TW --dry-run
npx shadcn@latest add @react-bits/LiquidEther-TS-TW --dry-run
npx shadcn@latest add @react-bits/Dither-TS-TW --dry-run
```

Substitute the `JS/TS` and `CSS/TW` suffix only after inspecting the target project. Prefer a Tailwind variant only when the project already uses compatible Tailwind styling. Read the source and dependencies from `view` and the project-specific writes from every dry-run; preview appearance alone does not establish mobile or reduced-motion quality.

Official source: https://github.com/DavidHDev/react-bits

### Magic UI

```bash
npx shadcn@latest search @magicui -q "bento"
npx shadcn@latest view @magicui/bento-grid
npx shadcn@latest add @magicui/bento-grid --dry-run
npx shadcn@latest add @magicui/bento-grid
```

Official source: https://magicui.design/docs/installation

For recurring Magic UI discovery in an MCP-capable client, check the current official MCP setup instead of embedding client-specific configuration here: https://magicui.design/docs/mcp

### Aceternity UI

```bash
npx shadcn@latest search @aceternity -q "card"
npx shadcn@latest view @aceternity/3d-card
npx shadcn@latest add @aceternity/3d-card --dry-run
npx shadcn@latest add @aceternity/3d-card
```

Current shadcn CLI releases can resolve public namespaces from the official registry index. Only add a manual registry entry if live CLI discovery reports that the namespace is unknown:

```json
{
  "registries": {
    "@aceternity": "https://ui.aceternity.com/registry/{name}.json"
  }
}
```

Official source: https://ui.aceternity.com/docs/cli

## Motion Orchestration

Prefer the existing animation setup. Add Motion for React (formerly Framer Motion) only when the surface needs coordinated page, layout, scroll, or gesture animation beyond self-contained CSS or component effects.

```bash
npm install motion
```

Use the matching project runner and verify the current React prerequisite before installation. Current imports use `motion/react`; do not reconstruct legacy package guidance from memory.

Official sources:

- https://motion.dev/docs/react
- https://motion.dev/docs/react-installation

## npm Component Libraries

Install only the chosen behavior foundation and the packages needed for the current feature.

### MUI

```bash
npm install @mui/material @emotion/react @emotion/styled
npm install @mui/x-data-grid
```

Install Data Grid only for a feature that needs it, and choose Community, Pro, or Premium deliberately. MUI also offers an official MCP server; prefer its current setup for repeated documentation lookups: https://mui.com/material-ui/getting-started/mcp/

Official source: https://mui.com/material-ui/getting-started/installation/

### Ant Design

```bash
npm install antd
```

Check the project's React version before choosing the current Ant Design major. Install `@ant-design/icons` only when explicitly needed.

Official source: https://ant.design/docs/react/introduce

### Mantine

```bash
npm install @mantine/core @mantine/hooks
```

After installation, follow the current framework guide for PostCSS, stylesheet imports, and `MantineProvider`. Mantine also publishes current AI/LLM integration guidance: https://mantine.dev/guides/llms/

Official source: https://mantine.dev/getting-started/

### Chakra UI

```bash
npm install @chakra-ui/react @emotion/react
```

Official source: https://chakra-ui.com/docs/get-started/installation

### HeroUI

```bash
npm install @heroui/styles @heroui/react
```

HeroUI v3 requires current React and Tailwind prerequisites and a styles import. Verify those prerequisites before selection. For recurring work, prefer HeroUI's official Agent Skill or MCP instead of duplicating current API details here.

Official source: https://heroui.com/en/docs/react/getting-started/quick-start

### Radix UI

Install only the primitives used by the feature:

```bash
npm install @radix-ui/react-dialog @radix-ui/react-popover
```

Official source: https://www.radix-ui.com/primitives/docs/overview/installation

### React Aria Components

```bash
npm install react-aria-components
```

React Aria also provides shadcn registry items, Agent Skills, MCP, and `llms.txt`; use the current official route when live discovery is available.

Official source: https://react-aria.adobe.com/getting-started

### Headless UI

```bash
npm install @headlessui/react
```

Official source: https://headlessui.com/react/menu

### PrimeReact

Do not use one unversioned install recipe for every project. For an existing PrimeReact v10 codebase:

```bash
npm install primereact primeicons
```

PrimeReact v11 uses a different package structure and was still pre-release at the last review. For a new production project, inspect the current stable documentation and pin the intended major before installing. Prefer Ant Design or MUI when the user needs a mature stable choice and does not specifically require PrimeReact.

Official sources:

- https://primereact.org/installation/
- https://primereact.dev/

### Fluent UI

```bash
npm install @fluentui/react-components
```

Official source: https://react.fluentui.dev/

### Icons

Use the project's existing icon library. Add Lucide only when no icon system exists and the selected component source expects it:

```bash
npm install lucide-react
```

## Verification

After any install:

1. Read every added or changed source file.
2. Confirm aliases and icon imports match project configuration.
3. Check required CSS, fonts, media, and peer dependencies.
4. Test keyboard, focus, reduced motion, mobile layout, loading, empty, and error states as applicable.
5. Run the project's typecheck, build, lint, and relevant UI tests.

If network or sandbox access is blocked, request approval for the exact command. Do not ask for broad shell or network access.
