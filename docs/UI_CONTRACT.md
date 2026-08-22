# UI Contract

**Version:** 0.5.0  
**Status:** Active  
**Owner:** Product + Frontend  
**Language:** English / Indonesian Localization Support  
**Last synced:** 2026-08-22 (FE-OVR-001)

## Design Direction

- **Accounting-First Corporate Workbench.** High density, structured, robust data entry inspired by Accurate Online / SAP combined with the crisp aesthetic of modern enterprise software.
- **Zero-Glitch Pure SVG System:** Dependensi font ligature Material Symbols dihapus 100% dan digantikan oleh Pure SVG (`lucide-react`).
- **Standard 3-Zone Enterprise Form Architecture:**
  - **Zone 1 (Sticky Header):** Document Numbering, Status Badges, Toolbar Aksi (Cetak/Duplikasi/Tutup).
  - **Zone 2 (Dynamic Body):** 2-Column Header Fields, Line Items Grid Engine (Keyboard-driven), Live Journal Impact Preview.
  - **Zone 3 (Sticky Footer):** Real-time Debit=Credit Integrity Indicator, Grand Total Breakdown, Shortcut CTA Buttons.
- **Authoritative token source:** `web/src/styles/m3-tokens.css` dan `web/src/styles/base.css`.

## Tokens

| Token | Value | Role |
|---|---|---|
| `--brand-primary` | `#2563eb` | Primary Royal Blue accent |
| `--bg-canvas` | `#f8fafc` | Canvas / Page Background (Slate 50) |
| `--bg-surface` | `#ffffff` | Surface / Card / Table Background |
| `--border-color` | `#e2e8f0` | Standard slate border |
| `--color-success` | `#059669` | Balanced status, profit |
| `--color-danger` | `#dc2626` | Unbalanced warning, loss |
| `--color-warning` | `#d97706` | Pending, draft, warnings |
| Font — sans | `Inter` / `Geist Sans` | Primary UI, labels, text |
| Font — mono | `JetBrains Mono` / `IBM Plex Mono` | Tabular financial amounts, dates, IDs |

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
