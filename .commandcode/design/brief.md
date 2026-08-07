# Ledgerly — Design Brief

Single source of truth for visual and interaction work on the Ledgerly web app (`web/`) and any future brand surfaces. Lives at `.commandcode/design/brief.md` so future `/design` commands have stable context without re-reading the repo.

**Visual direction: Accurate-inspired corporate** (SPEC-001, 2026-08-08). Wave-teal / cream / serif / ruled-sheet / stamped-card direction is **superseded** — do not use.

## Register

**Product.** Ledgerly is a daily-use accounting instrument for small-business operators. It is not a marketing surface. The interface earns trust through consistency and speed, not emotional arrival. Auth and onboarding count as product — they are the gates into the instrument.

## Domain Snapshot

- App: `web/` — React 19 + TypeScript 7 + Vite 8 SPA. No SSR, no CSS framework. Two runtime deps only (`react`, `react-router-dom`). Built into a static bundle served by nginx on the VPS.
- Backend: Go modular monolith at `backend/` exposing JSON over `/api/v1/*`. The UI is the source of truth for what the user sees; the backend is the source of truth for the books.
- Source of truth for accounting: `ACCOUNTING_ENGINE.md`, `DATA_MODEL.md`, `ARCHITECTURE.md`, `PRD.md`, `GLOSSARY.md`, `USER_STORIES.md`, `docs/UI_CONTRACT.md`.
- Source of truth for tokens: `web/src/styles.css` (`:root`). Every value in `UI_CONTRACT.md` must match it.
- Brand: **Ledgerly**. Wordmark uses `Ledger<em>ly</em>` with italic glyph on "ly". Logo is a small square mark in `--accent` blue.

## Primary User

A small-business owner or bookkeeper in Indonesia, working in IDR. Pressure: low — they open the app a few times a day to rule entries and check today's standing. They are not an accountant and do not want to be reminded of debit/credit. Their accountant reads the same data later, expects a consistent register.

## Dominant Work Patterns

| Surface | Pattern | Why |
|---|---|---|
| Dashboard (`/dashboard`) | **Monitor** | KPI rows + today-stamp proof object — read at a glance. |
| Record Money In / Out / Transfer | **Operate** | One entry at a time, fast input, immediate feedback. |
| Ledger (`/transactions`) | **Operate + Compare** | Ruled table; scan + filter + delete local cache. |
| Onboarding (3 steps) | **Configure** | Business details, period, opening balance — guided setup. |
| Auth | **Decide** | Single dominant action per tab (sign in / open ledger). |

Future surfaces (period close, reports) will lean **Monitor** + **Operate**. Composition must follow the work pattern, not the reverse — no centered-hero reflex.

## Voice and Copy

- Tone: craft ledger, not fintech brochure. Words: ruled, stamp, ledger, rule, open, balance, position. Never: "supercharge", "next-gen", "AI-powered".
- All UI copy, code identifiers, routes, comments, and seed data are English. The product speaks English; the user still communicates in Indonesian with the assistant — those are separate.
- One verb per button ("Rule money in", "Reopen period", "Sign in"). Loading copy names the actual work ("Stamping the day…", "Preparing the entry…", "Ruling…").
- Errors are recovery paths, never blame. Empty states teach the space ("The ledger is empty — open it with your first money in or money out").

## Visual Foundation (Accurate-inspired Corporate — locked)

| Token | Value | Role |
|---|---|---|
| `--canvas` | `#f5f7fa` | Cool light workspace background |
| `--paper` | `#ffffff` | Form cards, table surfaces |
| `--paper-elev` | `#ffffff` | Elevated cards and popups |
| `--panel` | `#eef2f6` | Sidebar, tab strip, secondary surfaces |
| `--rule-soft` | `#e9eef3` | Soft divider |
| `--rule-strong` | `#b8c5d2` | Input and control borders |
| `--ink-deep` | `#18324b` | Primary headings, strongest text, active nav |
| `--ink` | `#263d55` | Body text, controls |
| `--ink-secondary` | `#5c6f82` | Supporting text |
| `--ink-muted` | `#8191a0` | Metadata, hints |
| `--accent` | `#2f80ed` | Brand blue — links, primary buttons, active nav, positive amounts |
| `--positive` | `#27966f` | Profit, credit, balanced status (semantic only) |
| `--negative` | `#c64b4b` | Money out and errors |
| `--warning` | `#d18b2c` | Alerts, attention states |

Typography:

- Primary type: `Inter → Segoe UI → system-ui`. Headings, body, buttons, navigation, and field labels all use the same corporate sans family. No serif display layer.
- Mono: `IBM Plex Mono → ui-monospace → SF Mono`. Currency amounts, dates, status codes, journal numbers, kind marks, and compact metadata only. Tabular numerals enabled on amounts.
- Type scale is restrained: page titles around 18–20px, section titles around 15–16px, body 14–15px, metadata 11–12px. No oversized marketing headline.

Spacing: `4 / 8 / 12 / 16 / 24 / 32 / 48 / 64px` rhythm. No in-between.

Edges: `--radius-xs: 2px`, `--radius-sm: 4px`, `--radius-md: 6px`, `--radius-pill: 999px` for kind-mark badges and filter chips.

Depth: only elevated cards and popups cast shadow. Everything else relies on rules and hairlines.

## Component Rules

- **Sidebar:** icon-rail (64px) with paper panel background; hover reveals a label below the icon and flies out a white sub-menu with module sub-items. Active module uses blue left rule + accent text. Touch devices open the rail as a 260px slide-over.
- **Top bar:** white paper surface with Ledgerly mark, business name, session context, and Sign out. 56px maximum height. No clock, no "LIVE" pill, no console tokens.
- **Page head:** compact sans title + supporting copy + right-aligned actions. No editorial slogans, no decorative timestamp strips.
- **Dashboard proof:** four metric cards with restrained borders, followed by a ruled recent-entry table.
- **Entry tabs:** 2-column header grid (account + date on the left, auto-number toggle + document number + ambil button on the right), full-width description, account search bar, 3-column detail grid (Account | Account Name | Amount), bottom-right running total, and a right-side vertical action rail (Save brand-blue, Save & New warning-amber, Document, Attach, More).
- **List tabs:** filter pill row (Tanggal · Kas/Bank · more filters) + action toolbar (+ Tambah, Reload, Export, Print, Settings, Search) + ruled table + count badge.
- **Transaction table:** 6-column ruled table (Number | Date | Cash/Bank | Check No | Description | Amount). Mono only for dates/amounts/status. Kind marks use quiet outlined badges (`IN`, `OUT`, `XFER`).
- **Forms:** white elevated form panels on cool canvas, visible labels above inputs, modest 4–6px radii, two-column related fields, clear primary action footer.
- **Buttons:** sentence case sans labels, blue primary, white outlined secondary. Save button = brand blue (`#2f80ed`); Save & New = warning amber.
- **Filters:** compact filter pills in a toolbar; active filter uses accent-soft background + accent text, not a filled neon pill.
- **Period card:** white corporate card with navy heading, muted explanation, accent status line, and small journal code.

## Composition Lanes (allowed)

The surface is a craft instrument. Allowed lanes for new work:

- **Ruled table** — multi-column tables with full grid (outer border + right-rule cells, accent-soft header band with 2px accent underline, hover accent-soft).
- **Stamped card** — bordered proof object with mono header + accent amount.
- **Tabular register** — 4–6 column ruled table with mono amounts.
- **Editorial split** — auth uses a 1:1 split with the proof/narrative on one side and the form on the other.

**Refused** as the default answer: centered hero, boxed KPI cards in a grid, pill-shaped filter buttons with colored fills, gradient buttons, big "Sign up free!" CTAs, soft drop shadows on most elements, decorative photography, emoji in copy, three-equal-card feature rows, neon/AI purple accent, serif display fonts.

## Anti-References

The app must not look or feel like:

- **Tactile / ruled-sheet / cream Wave direction** (the previous direction before SPEC-001). No `#f7f5f0` cream canvas, no `#0d7370` teal accent, no Source Serif display layer, no stamped-card motif. These are explicitly superseded.
- **Generic SaaS dashboard** (Linear, Vercel, Stripe Dashboard lookalikes). No pure-black sidebars, no glassmorphism, no purple/violet/electric-blue CTAs, no gradient text.
- **Notion / Webflow / Framer marketing**. No emoji-led hero, no centered CTA stack, no infinite scroll sections.
- **Navy-blue fintech serif** (Revolut, Mercury, Brex). No serif body copy, no metallic feel, no logo lockup at top-center.
- **Old Indonesian SMB dashboards** (Jurnal, Kledo pre-rebrand). No giant white surfaces with playful icons, no soft pastel palette.

If a stranger can look at the page for 2 seconds and say "looks like [one of the above]", it failed.

## Accuracy Online as Layout Reference

**Accurate Online is the authoritative layout reference**, not a visual anti-reference. The current direction was driven by the user's preference for an Accurate Online-style workbench: blue primary, white/slate surfaces, compact density, icon-rail sidebar, Accurate-style 2-column entry header + 3-column detail grid + right-side action rail + bottom-right total, Accurate-style list with filter pills + action toolbar + 6-column ruled table.

The instruction "no rounded KPI cards in a grid, no blue SaaS dashboard" from the brief that came before SPEC-001 referred to the first-generation cream/teal design — not to Accurate Online. SPEC-001 reconciles that history.

## Accessibility Expectations

- WCAG AA contrast minimum on all text against its background. The current palette passes: ink `#263d55` on canvas `#f5f7fa` ≈ 12:1, muted `#8191a0` on paper `#ffffff` ≈ 3.4:1 (use for metadata only, not body), accent `#2f80ed` on white ≈ 4.6:1, positive `#27966f` on white ≈ 4.5:1.
- Keyboard navigation, visible focus rings (`outline: 2px solid var(--accent); outline-offset: 2px;`), semantic labels on every interactive control.
- Form inputs always have visible labels. Placeholders are hints only, never labels.
- Touch targets ≥ 44×44px on every interactive element. Primary actions inside forms use 38–56px height depending on context.
- `prefers-reduced-motion: reduce` disables the spinner fast-spin, the sidebar slide, and transitions globally. Already enforced in `styles.css`.
- iOS Safari input zoom: every input has min-height 32–42px and font-size ≥ 0.9375rem — already above the 16px threshold; form containers must remain at full width on < 640px so the inputs never trigger zoom.

## Motion

Page-wide motion personality: deliberate, not bouncy. Two beats:

1. Sidebar slide-over on mobile (200ms ease).
2. Loading spinner (800ms linear, slowed to 1.6s under reduced-motion).

No entrance choreography, no hover-elevate, no fade-in stagger. The instrument should feel still — the data is what moves.

## Process Rules (from `AGENTS.md` + taste)

- Every change to a frontend file must pass `make fmt && make lint && make test && make web-build` (per taste entry #48).
- The user audits English conversion claims themselves by opening backend files in the IDE (taste entry #60). A "clean" claim must survive a whole-repo grep, not just a frontend check.
- The user runs the deployed app on the VPS (`accounting.tikuma.net`). Deploy workflow: commit locally → push → VPS `git pull` → `docker compose --env-file .env.prod up -d --build web api` → restart Caddy if proxy state stalls.
- A new `/design` pass should read this file before touching anything. If a finding contradicts this brief, raise it; do not silently rewrite.
- When a visual reference changes, durable project docs that encode the visual direction must be updated in the same pass so they stop pointing at the superseded style (taste entry #63). This is exactly what SPEC-001 does.

## Out of Scope (intentional)

- No dark mode. The corporate direction is committed to the white/cool-slate surface.
- No i18n framework. Strings stay hardcoded English per current zero-dep frontend philosophy.
- No new design system dependency (no Tailwind, no shadcn). The single `styles.css` file is the system.
- No marketing/landing surface. Auth and onboarding are the only entry points.

## Superseded Tokens (do not use)

| Token | Old value | Replaced by |
|---|---|---|
| `--canvas` (legacy) | `#f7f5f0` (cream) | `#f5f7fa` (cool light) |
| `--paper` (legacy) | `#fbfaf6` (warm paper) | `#ffffff` |
| `--accent` (legacy) | `#0d7370` (teal) | `#2f80ed` (brand blue) |
| `--font-display` (legacy) | Source Serif 4 | Inter (no display serif layer) |

These legacy values are recorded in `.commandcode/taste/preferences/taste.md` line 71 as the user-correction history. They are not in `styles.css` and must not appear in any new spec, code, or copy.
