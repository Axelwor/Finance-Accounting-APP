# UI Contract

**Version:** 0.4.0  
**Status:** Active  
**Owner:** Product + Frontend  
**Language:** English  
**Last synced:** 2026-08-08 (SPEC-001)

## Design Direction

- **Accurate Online-inspired corporate workbench.** Calm, dense, professional — the instrument the user opens many times a day. The interface earns trust through consistency, legibility, and speed, not through decoration.
- **White workspace, cool slate panels, deep navy ink.** A single blue accent (`#2f80ed`) is the only color carrying meaning beyond ink/surface; green is reserved exclusively for positive financial status.
- **One professional sans family** (Inter) for headings, body, buttons, navigation, and field labels. **One monospace family** (IBM Plex Mono) for amounts, dates, codes, and compact metadata. No serif display layer.
- **Native controls and structured layouts** over decorative component stacks. Optimize for daily-work density on desktop and mobile.
- **Authoritative token source:** `web/src/styles.css` (`:root`). Every spec value below must match it; the CSS file is the source of truth.

## Tokens

| Token | Value | Role |
|---|---|---|
| `--canvas` | `#f5f7fa` | Page workspace background |
| `--paper` | `#ffffff` | Form cards, table surfaces, flyouts |
| `--paper-elev` | `#ffffff` | Elevated cards and popup surfaces |
| `--panel` | `#eef2f6` | Sidebar, tab strip, secondary surfaces |
| `--panel-hover` | `#e6edf5` | Hover states on panels |
| `--ink-deep` | `#18324b` | Primary headings, strongest text, active nav |
| `--ink` | `#263d55` | Body text, controls |
| `--ink-secondary` | `#5c6f82` | Supporting text |
| `--ink-muted` | `#8191a0` | Metadata, hints |
| `--ink-faint` | `#aebbc7` | Disabled, faint borders |
| `--rule` | `#d8e0e8` | Standard borders |
| `--rule-soft` | `#e9eef3` | Subtle dividers |
| `--rule-strong` | `#b8c5d2` | Input borders, control borders |
| `--accent` | `#2f80ed` | Brand blue — links, primary buttons, active nav, positive amounts |
| `--accent-hover` | `#236dcc` | Primary button hover |
| `--accent-soft` | `rgba(47,128,237,0.10)` | Selected backgrounds, active-tab fills |
| `--accent-rule` | `rgba(47,128,237,0.25)` | Tinted rules, focus rings |
| `--positive` | `#27966f` | Profit, credit, balanced status (semantic only) |
| `--negative` | `#c64b4b` | Losses, errors |
| `--warning` | `#d18b2c` | Alerts, attention states |
| Font — sans | `Inter` | All UI |
| Font — mono | `IBM Plex Mono` | Amounts, dates, codes, status, journal numbers |
| Radii | `2 / 4 / 6px` + `999px` pill | Cards, inputs, buttons, kind-mark pills |
| Spacing | `4 / 8 / 12 / 16 / 24 / 32 / 48 / 64px` | One rhythm, no in-between values |
| Shadow | `0 1px 2px + 0 8px 24px -12px` | Card elevation; popup shadow stronger |

**No Wave teal (`#0d7370`). No cream canvas (`#f7f5f0`). No Source Serif display layer.** These tokens are superseded by SPEC-001 (2026-08-08).

## Layout

- **Top bar (56px):** white paper surface; Ledgerly wordmark, business name, session context, and Sign out. No clock, no console tokens.
- **Fixed left sidebar (64px icon rail):** icon-only by default; module label appears below the icon on hover, and a white elevated sub-menu flies out to the right. On screens `< 960px` the rail hides behind a slide-over (`260px`) opened by a top-bar toggle.
- **Browser-style tab strip** sits below the top bar. The Dashboard tab is pinned (no close button). Module groups open nested child tabs (list + entry) inside a sub-strip in the work area.
- **Work area** uses a centered max-width of approximately `1320px` with responsive table and form layouts. Compact density: 16px page padding, 12px section gap, 32px input height.

## UX Rules

- Beginner UI uses "Money In", "Money Out", "Transfer", "Profit / Loss", "Balance Sheet", "Cash Flow", and "Trial Balance".
- Do not expose debit/credit in beginner flows except inside the accountant-oriented multi-line journal grid (header `Dr / Cr / Diff`).
- Every loading, empty, success, and error state is explicit and uses the live copy style ("Stamping the day…", "The ledger is empty — open it with your first money in or money out").
- Keyboard navigation, visible focus (`outline: 2px solid var(--accent); outline-offset: 2px`), semantic labels, and responsive layouts are required.
- Accounting confirmation and error copy must be understandable in English.
- List tabs provide filter pills, action toolbar (Reload, Export, Print, Settings, Search), row preview/open, and visible totals.
- Entry tabs provide a 2-column header (account/date on the left, auto-number/document on the right), a "Cari/Pilih Akun…" search bar, a multi-line account grid, a bottom-right running total, a right-side vertical action rail (Save brand-blue, Save & New warning-amber, Document, Attach, More), and unsaved-change protection.

## Module Map (sidebar)

| Group | Sub-items |
|---|---|
| Cash & Bank | Other Receipt, Other Payment, Bank Transfer |
| Sales | Sales Invoice, Receive Payment |
| Purchases | Purchase Invoice, Pay Bills, Check |
| Inventory | Items, Stock Movements |
| Fixed Assets | Asset Register, Depreciation |
| Reports | Trial Balance, Profit & Loss, Balance Sheet, Cash Flow |

Sales / Purchases / Inventory / Fixed Assets use a "Demo Data" badge until the corresponding backend endpoints ship (see TASK_LEDGER follow-ups).

## Skill

- Taste Skill: `design-taste-frontend` from `https://github.com/Leonxlnx/taste-skill`.
- Installed project path: `.agents/skills/design-taste-frontend`.
- Skill is guidance, not authority over accounting behavior.
