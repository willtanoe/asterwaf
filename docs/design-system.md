# Design system

The dashboard is a dense operator console. Tokens are defined once in `web/src/styles/tokens.css`; components do not use raw colors, font sizes, or spacing values. Copy lives in `web/src/copy.ts`.

## Color tokens

| Token | Role |
| --- | --- |
| `--bg` | application background |
| `--bg-panel` | panels, cards, tables |
| `--bg-raised` | hover rows, inactive tracks |
| `--bg-input` | inputs and selects |
| `--border`, `--border-strong` | separators and control outlines |
| `--text` | primary text |
| `--text-dim` | secondary text, labels |
| `--text-faint` | tertiary metadata |
| `--accent`, `--accent-strong`, `--accent-soft` | one interaction accent |
| `--blocked`, `--blocked-soft` | blocked requests and errors |
| `--recorded`, `--recorded-soft` | shadow/recorded requests |
| `--ratelimited`, `--ratelimited-soft` | rate-limited requests |
| `--ok`, `--ok-soft` | healthy upstream |
| `--focus` | focus-visible outline |

Both themes are defined in the same file under `:root[data-theme="dark"]` and `:root[data-theme="light"]`. The theme is stored under the `asterwaf-theme` localStorage key and applied through `data-theme` on `<html>`.

Contrast was checked programmatically for every foreground/background pair used by the components; all pairs are at least 4.5:1. `--text-faint` is only used for metadata at 12px and still meets 4.5:1.

## Typography

System font stack only. Sizes are `--fs-12`, `--fs-13`, `--fs-14`, `--fs-16`, `--fs-20`, `--fs-24`. Monospace data (IPs, paths, rule IDs, timings) uses `--font-mono`.

## Spacing and shape

Only `--sp-4`, `--sp-8`, `--sp-12`, `--sp-16`, `--sp-24`, `--sp-32` are used. Panels have a 1px border, `--radius` corners, and no shadow. There are no gradients, glows, glass effects, or decorative color blocks.

## Components

| Component | File | States |
| --- | --- | --- |
| `AppShell` | `web/src/components/AppShell.vue` | skip link, brand logo, navigation, theme toggle, sign out |
| `Gauge` | `web/src/components/Gauge.vue` | SVG speedometer for the block rate: track arc, progress arc, ticks, animated needle |
| `StatusBar` | `web/src/components/StatusBar.vue` | loading, error, runtime status with site health |
| `KpiCard` | `web/src/components/KpiCard.vue` | loading, error, value with delta and sparkline |
| `Sparkline` | `web/src/components/Sparkline.vue` | inline trend, tooltip enabled, animations disabled |
| `TrafficChart` | `web/src/components/TrafficChart.vue` | loading/empty/error handled by the parent, two series |
| `BreakdownList` | `web/src/components/BreakdownList.vue` | loading, error, empty, ranked bars |
| `StateMessage` | `web/src/components/StateMessage.vue` | loading, empty, error with retry |
| `Panel` styling | `web/src/styles/base.css` | header, body, table and badge primitives |

Semantic states always carry a text label (`Blocked`, `Recorded`, `Rate limited`, `up`, `down`, `unknown`); color is never the only signal. Charts use the same tokens and disable animation on update so polling does not redraw point by point.

## Layout

The shell is a two-column grid (sidebar, content) above 900px and a single column with a horizontally scrollable navigation below that. The Overview hero pairs the block-rate gauge with four decision counters; KPI cards are 3 columns at desktop, 2 at tablet, 1 at 320px. Tables scroll inside their own container; the page itself does not scroll horizontally at 320, 768, or 1440.

## Verification

`scripts/screenshot.mjs` (`make shots`) seeds `data/asterwaf-seed.db`, boots a throwaway instance, and writes one PNG per page, theme, and width (6 pages x 2 themes x 1440/768/390) to `docs/screenshots/`, failing on horizontal overflow, unapplied themes, console errors, and failed network requests. `make contrast` enforces 4.5:1 for every token pair used for text. It also fails when the document scroll width exceeds the viewport or when the requested theme is not applied. Contrast ratios are computed from `tokens.css` during the phase 9 self-check and recorded in `docs/progress.md`.
