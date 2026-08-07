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

## Visual Foundation (Tactile Operations Console — locked)

| Token | Value | Role |
|---|---|---|
| `--ink-deep` | `#0f1a17` | Display text, deepest body |
| `--ink-graphite` | `#1c2530` | Primary text, sidebar background, primary button |
| `--ink-muted` | `#5a6470` | Secondary text, hints |
| `--ink-faint` | `#8a929c` | Tertiary, captions |
| `--paper` | `#f5efe2` | Page background (with faint horizontal ruling) |
| `--paper-warm` | `#efe7d6` | Hover surface, balance summary |
| `--paper-card` | `#fbf6e9` | Elevated cards, forms |
| `--rule-soft` | `#ddd3bd` | Soft divider |
| `--rule-strong` | `#8a7f5c` | Heavy divider, button outline |
| `--ledger` | `#2f5d4a` | Accent — single role, ink underline, active marker |
| `--negative` | `#8a2e1f` | Money out, errors |
| `--amber` | `#a37226` | Reserved for warning states |
| `--rust` | `#8a4b2a` | Reserved for legacy / migration states |

Typography:
- Display: `Iowan Old Style → Source Serif Pro → Georgia, serif`. Used for headings, page titles, the today-stamp amount. Italic on the brand glyph "ly" and on emphasized nouns.
- Sans: system stack (`Segoe UI → system-ui → ...`). Body, buttons, navigation, field labels.
- Mono: `IBM Plex Mono → ui-monospace → SF Mono`. Currency amounts, dates, status codes, journal numbers, kind marks, all button labels (uppercase), all section meta and page-head meta. Tabular numerals enabled on amounts.

Spacing: `1 / 4 / 9 unit` rhythm — `--u-1: 4px`, `--u-2: 8px`, `--u-3: 12px`, `--u-4: 16px`, `--u-5: 24px`, `--u-6: 36px`, `--u-7: 56px`, `--u-8: 80px`. No in-between.

Edges: square. `--radius-xs: 2px`, `--radius-sm: 3px`, `--radius-pill: 999px` reserved for the kind mark badges and filter chips. The period card, form cards, auth card, onboarding card, ledger stamp, and KPI ledger rows all have square corners with single-pixel ink borders.

Depth: only the ledger-stamp proof object and form cards cast shadow (`--shadow-elev`). Everything else relies on rules and hairlines.

Surface texture: faint horizontal ruling on `body` and `app-main` (`repeating-linear-gradient` 28px) to evoke ruled ledger paper. This is the only decorative element allowed by default.

## Component Rules

- **Sidebar:** fixed left column, graphite background, brand at top, grouped sections (Overview / Record) with mono labels, each item with inline SVG icon and a 2px ledger-green left rule when active. Footer holds user identity + Sign out.
- **Page head:** date meta (mono uppercase) → display title (serif, with one italic emphasis word) → sub copy → page-head actions on the right. Bottom border in ink-graphite.
- **Ledger stamp:** bordered proof object on the dashboard, inner border (1px from outer 6px), mono label "Cash & Bank — Today's Standing" + mono date stamp, large serif amount, two-cell ledger sub-row.
- **Ledger rows:** ruled horizontal rows, label + mono note + value. Used for KPIs on the dashboard.
- **Transaction table:** 5-column ruled table (date, kind-mark + description + meta, category, amount, action). Mono amounts in tabular numerals with sign (`+`, `-`, or none). Kind marks are bordered pills (`IN`, `OUT`, `XFER`).
- **Forms:** hairline-only inputs (single bottom border in ink-graphite), mono uppercase labels, large mono suffix for currency. Two-column rows for related fields.
- **Buttons:** square, 1px ink border, uppercase mono labels, no rounded corners. Variants: primary (graphite fill), secondary (outline), ghost, danger, ink (ledger-green fill for standout).
- **Filters:** mono uppercase, no background, ink-graphite underline activates on selection. Bottom rule separates filter row from content.
- **Period card:** bordered stamp with mono header row, ledger green status line with mono journal code chip.

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
