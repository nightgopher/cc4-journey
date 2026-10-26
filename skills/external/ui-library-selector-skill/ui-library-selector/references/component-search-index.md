# Component Search Index

Use this file to avoid full-site crawling. Search only the category that matches the current UI task.

## Contents

- Discovery order and page patterns
- Copy-to-code: shadcn/ui, React Bits, Magic UI, Aceternity UI
- Motion orchestration: Motion for React, CSS/View Transitions, GSAP
- Production libraries: MUI, Ant Design, Mantine, Chakra UI, HeroUI
- Headless primitives: Radix UI, React Aria Components
- Enterprise specialists: PrimeReact, Fluent UI

## Discovery Order

1. Search installed project components and dependencies.
2. If the project uses shadcn-compatible registries, use the official shadcn Skill or MCP when available.
3. Otherwise use `shadcn search`, `view`, and `docs` for structured registry discovery.
4. Use the relevant library's official documentation only when no structured registry route exists.

Do not load every section below. Read the selected library and the categories needed by the task.

For multi-surface work, run the page-pattern map independently for each surface. Do not merge a public portfolio row and an admin row into one search scope.

## Page Pattern Map

| Page or flow | Search categories |
|---|---|
| Authentication | form, field, input, password, OTP, alert, button |
| Settings/profile | tabs, form, field, select, switch, avatar, dialog |
| Admin list | sidebar, data grid/table, filter, pagination, bulk action, drawer |
| Analytics | app shell, stat card, chart, date range, table, skeleton, empty state |
| Landing page | hero, logo cloud, bento/features, testimonial, pricing, FAQ, CTA |
| Portfolio | profile/project card, masonry, carousel, text animation, detail dialog |
| Commerce | product grid, filters, gallery, cart drawer, checkout form, toast |
| Onboarding | stepper, progress, form, selection cards, confirmation, empty state |

## shadcn/ui

Use for foundational app UI. Install via `npx shadcn@latest add ...`.

- Project context: `info --json`
- Registry discovery: `search @shadcn -q "<need>"`
- Item inspection: `view @shadcn/<item>`
- Current docs: `docs <item>`

- Actions: `button`, `button-group`, `toggle`, `toggle-group`
- Forms: `form`, `input`, `input-group`, `textarea`, `select`, `native-select`, `checkbox`, `radio-group`, `switch`, `slider`, `calendar`, `input-otp`, `field`, `label`
- Overlays: `dialog`, `alert-dialog`, `drawer`, `sheet`, `popover`, `hover-card`, `tooltip`, `context-menu`, `dropdown-menu`
- Navigation: `tabs`, `navigation-menu`, `menubar`, `breadcrumb`, `pagination`, `sidebar`
- Data display: `table`, `card`, `badge`, `avatar`, `accordion`, `chart`, `item`, `empty`, `scroll-area`, `resizable`
- Feedback: `alert`, `progress`, `skeleton`, `sonner`, `spinner`

## React Bits

Use for visual/interactive polish. Install from the `@react-bits` shadcn registry with the matching `JS/TS` and `CSS/TW` suffix.

- Text animation: `SplitText`, `BlurText`, `GradientText`, `ShinyText`, `RotatingText`, `TextType`, `CountUp`, `ScrollReveal`, `DecryptedText`
- General animation: `AnimatedContent`, `FadeContent`, `GlareHover`, `ClickSpark`, `Magnet`, `Noise`, `StarBorder`, `PixelTransition`
- Components: `ProfileCard`, `TiltedCard`, `SpotlightCard`, `MagicBento`, `Dock`, `Carousel`, `Masonry`, `Stepper`, `ModelViewer`
- Backgrounds: `Aurora`, `Dither`, `Threads`, `Silk`, `Particles`, `DotGrid`, `Galaxy`, `LiquidEther`, `Iridescence`, `Waves`

Inspect dependencies before choosing. Prefer CSS/Motion components over WebGL or shader components when the effect is secondary, mobile performance matters, or reduced-motion support is weak.

### Effect Candidate Evidence

When a public surface explicitly requires a memorable or high-impact visual, do not select React Bits at library level and stop. Search at least three relevant candidates, then compare exact registry items.

For a technological hero, `Threads`, `LiquidEther`, and `Dither` are a useful starting shortlist, not a preselected winner. For every candidate record:

| Evidence | What to inspect |
|---|---|
| Exact source | Rendering technique, animation loop, observers, event handlers, cleanup, SSR/client assumptions |
| Dependencies | Direct and transitive packages, existing overlap, bundle and maintenance cost |
| Dry-run | Files written, aliases, CSS/Tailwind assumptions, peer dependencies, local collisions |
| Mobile/render cost | DPR or resolution controls, touch behavior, offscreen pause, representative performance trace |
| Reduced motion | Built-in media-query support, disable/pause control, required adapter, static fallback |
| Brand fit | Whether the effect supports the visual thesis without becoming a generic demo |

Do not infer performance or accessibility from the component name or preview. If planning without a target project, mark source inspection, dry-run, render, and fallback evidence as pending and keep the component choice provisional.

## Motion Orchestration

Use the project's installed animation setup first. Motion orchestration is a separate selection layer from visual effects.

- **Motion for React (formerly Framer Motion)**: page/layout transitions, scroll-linked motion, gestures, sequencing, and centralized reduced-motion handling.
- **CSS/View Transitions**: small self-contained transitions with low dependency cost.
- **GSAP**: advanced timelines or effects that materially require it.

Do not add an orchestrator merely because an effect component has an internal animation loop. Add one when multiple page elements or routes require coordinated timing and policy.

## Magic UI

Use for landing pages and marketing sections. Install from the `@magicui` shadcn registry.

- Hero/marketing: `bento-grid`, `marquee`, `hero-video-dialog`, `terminal`, `tweet-card`, `code-comparison`
- Backgrounds: `animated-grid-pattern`, `grid-pattern`, `dot-pattern`, `retro-grid`, `particles`, `warp-background`
- Buttons/cards: `shimmer-button`, `rainbow-button`, `magic-card`, `neon-gradient-card`, `shine-border`
- Text: `animated-gradient-text`, `text-reveal`, `typing-animation`, `word-rotate`, `sparkles-text`
- Device/mockups: `iphone`, `android`, `safari`

## Aceternity UI

Use sparingly for strong visual moments. Install from the `@aceternity` shadcn registry.

- Hero/background: `background-beams`, `background-gradient-animation`, `aurora-background`, `spotlight`, `vortex`, `shooting-stars`
- Cards: `3d-card`, `glare-card`, `wobble-card`, `card-spotlight`, `card-hover-effect`
- Marketing sections: `hero-parallax`, `sticky-scroll-reveal`, `bento-grid`, `animated-testimonials`, `timeline`
- Navigation: `floating-navbar`, `navbar-menu`, `resizable-navbar`, `sidebar`
- Text/effects: `typewriter-effect`, `text-generate-effect`, `flip-words`, `encrypted-text`, `colourful-text`

Use one visually dominant pattern per viewport by default. Do not stack animated background, 3D card, cursor effect, and text effect without a deliberate art direction.

## MUI

Use package docs for exact imports.

- Core app UI: Button, TextField, Select, Checkbox, Radio, Switch, Dialog, Drawer, Menu, Tabs, Card
- Data: Table, List, Chip, Badge, Avatar
- Layout: Box, Container, Grid, Stack, Paper
- Feedback: Alert, Snackbar, Skeleton, Progress
- Advanced data: MUI X Data Grid, Date/Time Pickers

## Ant Design

- General: Button, Typography, Icon
- Layout/navigation: Layout, Menu, Breadcrumb, Pagination, Steps, Tabs
- Data entry: Form, Input, Select, DatePicker, TimePicker, Upload, Transfer, TreeSelect
- Data display: Table, Descriptions, List, Card, Calendar, Tree, Statistic, Timeline, Tag
- Feedback: Modal, Drawer, Message, Notification, Popconfirm, Progress, Skeleton, Spin, Result

## Mantine

- Layout: AppShell, Container, Grid, Group, Stack, Flex, SimpleGrid
- Inputs: TextInput, NumberInput, PasswordInput, Select, MultiSelect, TagsInput, Date inputs, FileInput
- Overlays: Modal, Drawer, Popover, HoverCard, Tooltip, Menu
- Feedback: Alert, Notification, Progress, Loader, Skeleton
- Display: Card, Badge, Avatar, Timeline, DataList, Table, Code, Typography

## Chakra UI

- Layout: Box, Flex, Grid, Stack, Container
- Forms: Field, Input, Select, Checkbox, Radio, Switch, Slider, TagsInput
- Overlays: Dialog, Drawer, Popover, Tooltip, Menu, HoverCard
- Feedback/display: Alert, Toast, Skeleton, Spinner, Progress, Card, Badge, Table, Timeline

## HeroUI

- Core: Button, ButtonGroup, Card, Modal, Drawer, Dropdown, Popover, Tooltip
- Forms: Input, TextArea, Select, Checkbox, RadioGroup, Switch, Slider, Form
- Advanced inputs: Calendar, DatePicker, DateRangePicker, ColorPicker, Autocomplete, ComboBox
- Data/display: Table, Pagination, Avatar, Badge, Chip, Toast, Tabs, TagGroup, Toolbar

## Radix UI

Use only for custom primitives:

- Dialog, AlertDialog, Popover, Tooltip, DropdownMenu, ContextMenu, Menubar
- Select, Checkbox, RadioGroup, Slider, Switch
- Tabs, Accordion, Collapsible, NavigationMenu, Toast

## React Aria Components

Use for accessibility-critical custom UI:

- Button, Dialog, Modal, Popover, Tooltip
- ComboBox, Select, ListBox, Menu, Tabs
- Calendar, DatePicker, DateRangePicker, TimeField
- Table, Tree, GridList, Virtualizer
- Drag and drop hooks, form controls

## PrimeReact

Use for data-heavy enterprise widgets:

- DataTable, TreeTable, VirtualScroller, Paginator
- FileUpload, Calendar, Dropdown, MultiSelect
- PickList, OrderList, OrganizationChart, Tree
- Chart, Galleria, Carousel, Timeline

## Fluent UI

Use for Microsoft-style internal tools:

- Button, Input, Combobox, Dropdown, Select
- Dialog, Drawer, Popover, Tooltip, Toast
- DataGrid, Table, Tree, List
- Persona, Avatar, Toolbar, Nav, Menu
