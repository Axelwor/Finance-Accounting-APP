# Ledgerly — Design Brief

Single source of truth for visual and interaction work on the Ledgerly web app (`web/`) and any future brand surfaces. Lives at `.commandcode/design/brief.md` so future `/design` commands have stable context without re-reading the repo.

## Register

**Product.** Ledgerly is a daily-use accounting instrument for small-business operators. It is not a marketing surface. The interface earns trust through consistency and speed, not emotional arrival. Auth and onboarding count as product — they are the gates into the instrument.

## Domain Snapshot

- App: `web/` — React 19 + TypeScript 7 + Vite 8 SPA. No SSR, no CSS framework. Two runtime deps only (`react`, `react-router-dom`). Built into a static bundle served by nginx on the VPS.
- Backend: Go modular monolith at `backend/` exposing JSON over `/api/v1/*`. The UI is the source of truth for what the user sees; the backend is the source of truth for the books.
- Source of truth for accounting: `ACCOUNTING_ENGINE.md`, `DATA_MODEL.md`, `ARCHITECTURE.md`, `PRD.md`, `GLOSSARY.md`, `USER_STORIES.md`, `docs/UI_CONTRACT.md`.
- Brand: **Ledgerly**. Wordmark uses `Ledger<em>ly</em>` with italic glyph on "ly". Logo is a small square mark in `--ledger` green.

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
- One verb per button ("Rule money in", "Reopen period", "Sign in"). Loading copy names the actual work ("Stamping the day...", "Preparing the entry...", "Ruling...").
- Errors are recovery paths, never blame. Empty states teach the space ("The ledger is empty — open it with your first money in or money out").

## Visual Foundation (Wave-inspired Corporate Accounting Console — locked)

| Token | Value | Role |
|---|---|---|
| `--ink-deep` | `#142036` | Primary headings and strongest text |
| `--ink` | `#1f2c46` | Body text and controls |
| `--ink-secondary` | `#4f5b71` | Supporting text |
| `--ink-muted` | `#7d8597` | Metadata and hints |
| `--canvas` | `#f7f5f0` | Warm off-white application canvas |
| `--paper` | `#fbfaf6` | Sidebar and top navigation surface |
| `--paper-elev` | `#ffffff` | Elevated cards and forms |
| `--panel` | `#f2efe8` | Hover and selected surfaces |
| `--rule-soft` | `#e6e2d8` | Soft divider |
| `--rule-strong` | `#b9b1a0` | Input and control borders |
| `--accent` | `#0d7370` | Single corporate teal accent |
| `--positive` | `#2d7a5c` | Money in and balanced status |
| `--negative` | `#a8443b` | Money out and errors |
| `--warning` | `#b87a2e` | Attention states |

Typography:
- Primary type: `Inter → Segoe UI → system-ui`. Headings, body, buttons, navigation, and field labels all use the same corporate sans family. No serif display layer.
- Mono: `IBM Plex Mono → ui-monospace → SF Mono`. Currency amounts, dates, status codes, journal numbers, kind marks, and compact metadata only. Tabular numerals enabled on amounts.
- Type scale is restrained: page titles around 30px, section titles around 18px, body 14-15px, metadata 11-12px. No oversized marketing headline.

Spacing: `1 / 4 / 9 unit` rhythm — `--u-1: 4px`, `--u-2: 8px`, `--u-3: 12px`, `--u-4: 16px`, `--u-5: 24px`, `--u-6: 36px`, `--u-7: 56px`, `--u-8: 80px`. No in-between.

Edges: square. `--radius-xs: 2px`, `--radius-sm: 3px`, `--radius-pill: 999px` reserved for the kind mark badges and filter chips. The period card, form cards, auth card, onboarding card, ledger stamp, and KPI ledger rows all have square corners with single-pixel ink borders.

Depth: only the ledger-stamp proof object and form cards cast shadow (`--shadow-elev`). Everything else relies on rules and hairlines.

Surface texture: faint horizontal ruling on `body` and `app-main` (`repeating-linear-gradient` 28px) to evoke ruled ledger paper. This is the only decorative element allowed by default.

## Component Rules

- **Sidebar:** light paper surface, deep navy text, small restrained icons, teal left rule for the active module. Hover submenu is a white elevated popup with a subtle shadow and clear sub-item hierarchy. It must remain usable by mouse, keyboard, and touch.
- **Top bar:** light paper surface with Ledgerly mark, LIVE indicator, company name, clock, session context, and Sign out. 56px maximum height.
- **Page head:** compact sans title + supporting copy + actions. Avoid editorial slogans and decorative timestamp strips.
- **Dashboard proof:** four corporate metric cards with restrained borders and minimal shadow, followed by a ruled recent-entry table. Metrics are the proof object.
- **Transaction table:** 5-column ruled table (date, kind mark + description + meta, category, amount, action). Mono only for dates/amounts/status. Kind marks use quiet outlined badges (`IN`, `OUT`, `XFER`).
- **Forms:** white elevated form panels on warm canvas, visible labels above inputs, quiet bottom-border inputs, two-column related fields, clear primary action footer.
- **Buttons:** modest 4px radius, sentence case sans labels, navy primary, white outlined secondary, teal only for key positive/action states.
- **Filters:** light outlined controls in a compact toolbar; active filter uses teal border/text, not a filled neon pill.
- **Period card:** white corporate card with navy heading, muted explanation, teal status line, and small journal code.

## Composition Lanes (allowed)

The surface is a craft instrument. Allowed lanes for new work:

- **Ruled sheet** — content lives between hairline rules, not in boxed cards.
- **Stamped card** — bordered proof object with optional inner border + mono header + serif amount.
- **Tabular register** — 4-5 column ruled table with mono amounts.
- **Editorial split** — auth and onboarding use a 1:1 split with the proof/narrative on one side and the form on the other.

**Refused** as the default answer: centered hero, boxed KPI cards in a grid, pill-shaped filter buttons with colored fills, gradient buttons, big "Sign up free!" CTAs, soft drop shadows on most elements, decorative photography, emoji in copy, three-equal-card feature rows, AI purple / neon blue accent, Inter as default.

## Anti-References

The app must not look or feel like:

- **Accurate Online** (the original brief referenced it for layout inspiration; we deliberately moved away). No blue SaaS dashboard, no rounded KPI cards, no left-rounded sidebar with subtle shadows.
- **Generic SaaS dashboard** (Linear, Vercel, Stripe Dashboard lookalikes). No pure-black sidebars, no glassmorphism, no purple/violet/electric-blue CTAs, no gradient text.
- **Notion / Webflow / Framer marketing**. No emoji-led hero, no centered CTA stack, no infinite scroll sections.
- **Navy-blue fintech serif** (Revolut, Mercury, Brex). No serif body copy, no metallic feel, no logo lockup at top-center.
- **Old Indonesian SMB dashboards** (Accurate, Jurnal, Kledo pre-rebrand). No giant white surfaces with playful icons, no soft pastel palette.

If a stranger can look at the page for 2 seconds and say "looks like [one of the above]", it failed.

## Accessibility Expectations

- WCAG AA contrast minimum on all text against its background. The current palette passes: graphite `#1c2530` on paper `#f5efe2` ≈ 13:1, muted `#5a6470` on paper ≈ 7:1, ledger `#2f5d4a` on paper-card `#fbf6e9` ≈ 7.5:1.
- Keyboard navigation, visible focus rings (`outline: 2px solid var(--ledger); outline-offset: 2px;`), semantic labels on every interactive control.
- Form inputs always have visible labels (mono uppercase). Placeholders are hints only, never labels.
- Touch targets ≥ 44×44px on every interactive element. The toggle button on the sidebar is 36×36 — reserved as "always visible" affordance; primary actions inside forms use 38px height + 16px horizontal padding.
- `prefers-reduced-motion: reduce` disables the spinner fast-spin, the sidebar slide, and transitions globally. Already enforced in `styles.css`.
- iOS Safari input zoom: every input has min-height 42px and font-size ≥ 0.9375rem — already above the 16px threshold; form containers must remain at full width on < 640px so the inputs never trigger zoom.

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

## Out of Scope (intentional)

- No dark mode. The Tactile direction is committed to the paper surface.
- No i18n framework. Strings stay hardcoded English per current zero-dep frontend philosophy.
- No new design system dependency (no Tailwind, no shadcn). The single `styles.css` file is the system.
- No marketing/landing surface. Auth and onboarding are the only entry points.
