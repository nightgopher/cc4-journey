# Multi-Surface Selection Regression

Run date: 2026-07-13

This regression was reconstructed from a real badcase: one React product contained a public personal site and an authenticated content-management backend, but the selector collapsed them into one library decision.

## Frozen Baseline

The pre-change Skill was copied to a temporary baseline before editing.

- `SKILL.md` SHA-256: `78bcbd41cf8ae1808602df81aaae0baf6bb0ba4790626080c8ebff204fa0399b`
- `decision-matrix.md` SHA-256: `4e7885908d24afa9671e5b73794962c7b672f54a2278fa9ed92a3099979f4f4e`
- `test-prompts.json` SHA-256: `08e88c7ddd46ba3965e3e531ebb5a6af4f8e1ef16898836a33e6d1f10e3e473e`

## Adversarial Prompt

```text
为我的个人品牌做一套 React 网站：访客能看个人介绍、项目和博客，我登录后还能管理文章、项目与媒体。整体希望很酷、有科技感和哇塞动效，但表单、内容编辑和日常维护必须稳定、易用、无障碍。请选最合适的 UI 组件体系并说明理由。现在只做方案，不安装。
```

## Baseline Result

An independent Agent read only the frozen baseline and returned:

- it recognized two product surfaces;
- it still selected **Mantine as the single global UI system**;
- it gave qualitative priorities rather than weights totaling 100%;
- it selected React Bits only as public polish, but had no motion-orchestration layer;
- it named `Aurora`, `SpotlightCard`, and `DecryptedText` without completing an exact source/dependency/dry-run comparison;
- it did not require rendered desktop/mobile evidence before claiming visual improvement.

This reproduced the failure mechanism: surface recognition alone was insufficient because the selection contract still demanded one project-level primary system.

| Regression criterion | Frozen baseline |
|---|---|
| Split public and admin surfaces | Pass |
| Independent library conclusion per surface | **Fail** |
| Numeric weights totaling 100% per surface | **Fail** |
| Behavior/effect/orchestration layers | **Fail** |
| Mandatory exact effect-candidate evidence | **Fail** |
| Rendered evidence gate | **Fail** |
| No install commands in planning-only work | Pass |

## Post-Change Multi-Surface Result

The same independent evaluation against the updated Skill returned:

| Surface | Objective weights | Behavior foundation | Visual effect source | Motion orchestration |
|---|---|---|---|---|
| Public portfolio | Visual distinction 35%, brand fit 25%, motion 20%, performance/fallback 15%, cost 5% | Existing primitives or shadcn/ui | React Bits; `Threads`, `LiquidEther`, `Dither` remain provisional until evidence | Motion for React only when page/scroll choreography is required |
| Authenticated admin | Reliability 30%, accessibility 25%, CRUD efficiency 25%, maintainability 15%, visual customization 5% | Existing accessible system or Radix UI | None | Existing restrained CSS or none |

The Agent explicitly:

- rejected a global primary-library shortcut;
- kept public and admin objectives independent;
- required `search`, `view`, and project-specific `add --dry-run` for all three React Bits candidates;
- marked every exact component decision and all render claims as provisional or pending;
- emitted no install command because no target project existed.

Outcome: all 8 `must` and all 5 `must_not` checks in `multi-surface-portfolio-admin` passed.

## Effect-Source Gate Result

Prompt:

```text
给我的 React 作品集做一个真正有视觉记忆点、科技感和哇塞感的首屏。现在只选材料，不安装。
```

The independent result treated effect-source evaluation as mandatory, compared `Threads`, `LiquidEther`, and `Dither` as provisional candidates, required source/dependency/mobile/reduced-motion/static-fallback/render evidence, and emitted no install command.

Outcome: 7/7 checks passed.

## Admin Guardrail Result

Prompt:

```text
重做 React 后台用户管理页，要高级筛选、批量操作、虚拟滚动和无障碍；这是日常高频操作工具，不需要炫酷视觉。现在只做选型方案，不安装。
```

The independent result evaluated the existing system, MUI X, Ant Design, and PrimeReact for the behavior layer; selected `none` for visual effects; kept motion restrained or absent; and did not introduce React Bits, Magic UI, or Aceternity UI.

Outcome: 8/8 checks passed. The new wow gate did not over-trigger on an operational surface.

## Live React Bits Source Inspection

Official registry manifests were inspected directly. This is source/dependency evidence, not a frame-rate benchmark and not a project-specific dry-run.

| Candidate | Official manifest dependencies | Source-level findings | Evidence still required |
|---|---|---|---|
| `Threads-TS-TW` | `ogl@^1.0.11` | WebGL shader; resolution cap; intersection and tab-visibility pause; no automatic `prefers-reduced-motion` adapter in the inspected source | Target-project dry-run, device trace, static fallback |
| `LiquidEther-TS-TW` | `three@^0.180.0` | Fluid-style WebGL simulation; configurable internal resolution; DPR cap; touch input; intersection and tab-visibility pause; no automatic reduced-motion adapter in the inspected source | Target-project dry-run, mobile trace, static fallback |
| `Dither-TS-TW` | `@react-three/fiber`, `@react-three/postprocessing`, `postprocessing`, `three` | React Three Fiber plus postprocessing; fixed DPR of 1; exposes `disableAnimation`; inspected source does not automatically bind that control to `prefers-reduced-motion` | Target-project dry-run, offscreen behavior, mobile trace, fallback adapter |

No final winner was declared. The correct component depends on the target brand, existing dependencies, rendered mobile cost, and fallback quality.

## Distribution Artifact Check

Before repackaging, the repository source had changed while all five files inside `ui-library-selector-skill.tar.gz` still matched the frozen baseline. The new archive-parity gate failed 5/5 packaged-file comparisons.

After rebuilding the archive, the same gate passed for:

- `ui-library-selector/SKILL.md`
- `ui-library-selector/references/decision-matrix.md`
- `ui-library-selector/references/component-search-index.md`
- `ui-library-selector/references/install-commands.md`
- `ui-library-selector/references/library-catalog.md`

This check now remains in `scripts/check-skill.sh` so future source-only updates cannot silently leave a stale distributable archive.
