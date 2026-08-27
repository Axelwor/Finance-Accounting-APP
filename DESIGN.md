---
name: Trexo
description: Accounting-first bookkeeping — a dense, precise accountant's desk where every number earns trust.
colors:
  ledger-blue: "#2563eb"
  ledger-blue-deep: "#1d4ed8"
  ledger-blue-tint: "#eff6ff"
  cleared-emerald: "#059669"
  overdue-red: "#dc2626"
  due-amber: "#d97706"
  memo-sky: "#0284c7"
  canvas-mist: "#f8fafc"
  paper-white: "#ffffff"
  slate-wash: "#f1f5f9"
  slate-line: "#e2e8f0"
  graphite-rule: "#cbd5e1"
  ink: "#0f172a"
  ink-soft: "#475569"
  ink-muted: "#64748b"
typography:
  display:
    fontFamily: "JetBrains Mono, IBM Plex Mono, SF Mono, Menlo, Consolas, monospace"
    fontSize: "16px"
    fontWeight: 800
    lineHeight: 1.2
    fontFeature: "\"tnum\" 1"
  headline:
    fontFamily: "Inter, Geist Sans, -apple-system, BlinkMacSystemFont, Segoe UI, Roboto, sans-serif"
    fontSize: "15px"
    fontWeight: 700
    lineHeight: 1.3
    letterSpacing: "-0.2px"
  title:
    fontFamily: "Inter, Geist Sans, -apple-system, BlinkMacSystemFont, Segoe UI, Roboto, sans-serif"
    fontSize: "13.5px"
    fontWeight: 700
    lineHeight: 1.35
  body:
    fontFamily: "Inter, Geist Sans, -apple-system, BlinkMacSystemFont, Segoe UI, Roboto, sans-serif"
    fontSize: "13.5px"
    fontWeight: 400
    lineHeight: 1.45
  label:
    fontFamily: "Inter, Geist Sans, -apple-system, BlinkMacSystemFont, Segoe UI, Roboto, sans-serif"
    fontSize: "10.5px"
    fontWeight: 600
    lineHeight: 1.3
    letterSpacing: "0.4px"
rounded:
  xs: "4px"
  sm: "6px"
  md: "8px"
  lg: "12px"
  full: "9999px"
spacing:
  2xs: "4px"
  xs: "6px"
  sm: "8px"
  md: "12px"
  lg: "16px"
  xl: "20px"
  2xl: "28px"
components:
  button-primary:
    backgroundColor: "{colors.ledger-blue}"
    textColor: "#ffffff"
    rounded: "{rounded.xs}"
    padding: "5px 10px"
    height: "26px"
  button-secondary:
    backgroundColor: "{colors.paper-white}"
    textColor: "{colors.ink}"
    rounded: "{rounded.xs}"
    padding: "5px 10px"
    height: "26px"
  input-text:
    backgroundColor: "{colors.paper-white}"
    textColor: "{colors.ink}"
    rounded: "{rounded.xs}"
    padding: "5px 8px"
  card:
    backgroundColor: "{colors.paper-white}"
    textColor: "{colors.ink}"
    rounded: "{rounded.md}"
    padding: "16px"
  form-card:
    backgroundColor: "{colors.paper-white}"
    textColor: "{colors.ink}"
    rounded: "{rounded.sm}"
    padding: "10px 12px"
---

# Design System: Trexo

## Overview

**Creative North Star: "The Accountant's Desk"**

Trexo looks like the desk of a meticulous professional accountant: white working papers laid on cool slate, ruled lines instead of ornament, instruments arranged within reach and nothing out of place. The voice is **sober, exact, tireless** — a tool that works as long as the user does and is never louder than the numbers. Density is a feature: the screen is a workspace where an accountant scans dozens of rows without scrolling fatigue, so type stays compact (13.5px base, 12px inside tables), surfaces stay flat and bordered, and every visual decision serves scanability and audit-grade trust.

Color behaves like ink stamps on a ledger: blue marks action and identity, while emerald, red, and amber are *earned* by document state — posted, void, pending — never applied as decoration. Depth exists only as whispers: hairline shadows under sticky bars, real elevation reserved for things that actually float (menus, dialogs, flyouts).

**Key Characteristics:**
- High-density corporate layout: 64px icon-rail shell, 52px topbar, tab strips, full-width content
- Flat white cards defined by 1px slate borders; shadows only as state response or floating chrome
- All money set in JetBrains Mono with tabular numerals, right-aligned
- Strict semantic status palette (draft/posted/void/pending) shared across every module
- 3-Zone Enterprise Form architecture: sticky header bar, tabbed body, sticky totals footer
- Light and dark themes driven by the same token names (`[data-theme="dark"]` flips values in `web/src/styles/m3-tokens.css`; the hex values above are the light-theme canon)

## Colors

A restrained working palette: one decisive blue over four semantic state colors, all sitting on a slate neutral ramp.

### Primary
- **Ledger Blue** (#2563eb): The single action color. Primary buttons, active navigation rail items, links, focused inputs, the running total in a form footer. Hover deepens to **Ledger Blue Deep** (#1d4ed8); selected/active backgrounds wash with **Ledger Blue Tint** (#eff6ff) bordered by #bfdbfe.

### Secondary
- **Memo Sky** (#0284c7): Informational notices and neutral-positive hints (bg #f0f9ff, border #bae6fd, text #075985). Never competes with primary actions.

### Tertiary
- **Cleared Emerald** (#059669): Money in, posted documents, open periods, success states (bg #ecfdf5, border #a7f3d0, text #065f46).
- **Overdue Red** (#dc2626): Money out, errors, voided documents — paired with strikethrough when a document is void (bg #fef2f2, border #fecaca, text #991b1b).
- **Due Amber** (#d97706): Pending approvals, upcoming deadlines, warnings (bg #fffbeb, border #fde68a, text #92400e).

### Neutral
- **Ink** (#0f172a): Primary text; also the double-rule under accounting totals.
- **Ink Soft** (#475569): Secondary text, table headers, labels.
- **Ink Muted** (#64748b): Placeholder text, meta captions.
- **Graphite Rule** (#cbd5e1): Strong borders — input outlines, table head separators.
- **Slate Line** (#e2e8f0): Default card and divider borders.
- **Slate Wash** (#f1f5f9): Table head fills, footers, disabled/computed fields.
- **Canvas Mist** (#f8fafc): App background beneath the white papers; row-hover tint.
- **Paper White** (#ffffff): Every card, form surface, dialog, and the topbar.

### Named Rules
**The Earned Color Rule.** Emerald, red, and amber appear only when a document state or amount direction justifies them. A screen with no financial state on it shows blue and slate only.

## Typography

**Display Font:** JetBrains Mono (with IBM Plex Mono, SF Mono, Menlo fallbacks)
**Body Font:** Inter (with Geist Sans, system-ui fallbacks)
**Label/Mono Font:** JetBrains Mono for every figure, badge count, document number, and keyboard hint

**Character:** The pairing splits labor cleanly — Inter carries language quietly and legibly; JetBrains Mono carries numbers with fixed-width authority so columns of amounts align to the rupiah regardless of magnitude.

### Hierarchy
- **Display** (JetBrains Mono 800, 16px, `tnum`): The grand total in a form footer and hero KPI values. The loudest thing on any screen is always a number.
- **Headline** (Inter 700, 15px, −0.2px tracking): Form header titles ("Invoice", "Sales Order") in the sticky Zone 1 bar.
- **Title** (Inter 700, 13.5px): Card and section titles.
- **Body** (Inter 400, 13.5px, 1.45 line-height): Default reading size. Dense contexts (tables, inputs, list rows) drop to 12px.
- **Label** (Inter 600–700, 10.5–11px, +0.4px tracking, uppercase): Table column heads, field labels (11px sentence case), footer summary labels, rail item captions (10px).

### Named Rules
**The Tabular Truth Rule.** Every monetary figure renders in JetBrains Mono with `font-feature-settings: "tnum" 1`, right-aligned. Proportional digits never touch money.

## Layout

An application desk, not a marketing page. The shell is a full-viewport flex row: a **64px icon-rail sidebar** (expanding to 240px), a **52px topbar**, and a **40px nested tab strip** for open workspaces. Content areas claim full available width — pages must never clip on the right; multi-column grids use `minmax(0, 1fr)` tracks so children can't force overflow.

- **Page rhythm:** dashboard containers pad `20px 28px`; form bodies pad `10px 14px`; card-to-card gaps 8–16px.
- **Enterprise Form (3 zones):** Zone 1 is a sticky document header (icon chip, title, mono document number pill, status badge) with `padding: 8px 24px`; Zone 2 scrolls the body — a 42px icon side-rail for Header/Items/Info tabs beside form cards; Zone 3 is a sticky footer carrying the uppercase total label, the mono grand total, and primary actions.
- **Header field grid:** `repeat(4, minmax(0, 1fr))`, collapsing to 2 columns at ≤1100px; two-column form grids gap `12px 16px`.
- **Data tables:** 12px cells (`4px 8px` padding), 10.5px uppercase heads on Slate Wash with sticky positioning, hover row tint, optional 480px max-height scroll region, and a footer strip for counts and pagination.

## Elevation & Depth

Flat by default, lifted only in response. White papers separate from the misty canvas through 1px borders first; shadow is a whisper that confirms interaction or floatation.

### Shadow Vocabulary
- **shadow-xs** (`0 1px 2px 0 rgba(0,0,0,0.05)`): Cards at rest.
- **shadow-sm** (`0 1px 3px rgba(0,0,0,0.1), 0 1px 2px -1px rgba(0,0,0,0.1)`): Brand badge, slightly raised elements.
- **shadow-md** (`0 4px 6px -1px rgba(0,0,0,0.1), 0 2px 4px -2px rgba(0,0,0,0.1)`): Small popovers.
- **shadow-lg** (`0 10px 15px -3px rgba(0,0,0,0.1), 0 4px 6px -4px rgba(0,0,0,0.1)`): Menus, dropdown panels.
- **shadow-xl** (`0 20px 25px -5px rgba(0,0,0,0.1), 0 8px 10px -6px rgba(0,0,0,0.1)`): Flyouts and modal dialogs.
- **Sticky-bar whispers** (`0 1px 2px rgba(15,23,42,0.04)` downward, `0 -2px 6px rgba(15,23,42,0.04)` upward): Zone 1 and Zone 3 form bars; scrolled-table head gets `0 2px 4px rgba(15,23,42,0.06)`.

Focus is structural, not shadowed: a keyboard-first double ring `box-shadow: 0 0 0 2px var(--bg-surface), 0 0 0 4px var(--brand-primary)`.

### Named Rules
**The Floating Chrome Rule.** Only detached surfaces (flyout menus, dialogs, autocomplete panels) may cast shadow-lg/xl. Anything docked to the page stays at xs/sm or uses the sticky-bar whisper.

## Shapes

Rectangular with softened shoulders. Inputs, buttons, badges, and icon tiles take **4px**; form cards and nested containers take **6px**; standalone cards and modals take **8px**; 12px appears only on large overlay surfaces. Status chips alone use the full pill (**9999px**) — roundness signals "stamp," so it never spreads to structural elements. Borders are uniformly 1px solid; dashed 1px divides meta rows inside a card; the only heavy rule in the system is the accounting **double rule** — `border-bottom: 3px double` under final totals, borrowed straight from a paper ledger.

## Components

The component language is instrument-panel corporate: small, bordered, state-explicit.

### Buttons
- **Shape:** Near-square corners (4px), inline-flex, 11.5px Inter 600, compact padding (`5px 10px`), ~26px tall.
- **Primary:** Ledger Blue fill, white text, 1px Ledger Blue Deep border; hover darkens fill; disabled drops to 50% opacity.
- **Secondary:** Paper White fill, Ink text, Graphite Rule border; hover washes Slate Wash.
- **Ghost/link:** Transparent, Ledger Blue text, 600 weight — used for card-level actions ("View all →").
- **Keyboard hint:** Buttons may embed a `.btn-kbd` mono chip (9.5px, translucent white on primary) showing shortcuts like Ctrl+S.

### Badges / Status Chips
- **Style:** Pill (9999px) or square (4px) micro-chips, 10–10.5px Inter 700, uppercase for document status, tinted bg + colored text + matching border.
- **State:** Draft = slate; Posted/Open = Cleared Emerald; Void = Overdue Red **with line-through**; Pending = Due Amber. Counts on icon rails use mono 8.5px counter bubbles.

### Cards / Containers
- **Corner Style:** 8px for dashboard cards, 6px for form cards and table wrappers.
- **Background:** Paper White on Canvas Mist.
- **Shadow Strategy:** shadow-xs at rest (see Elevation).
- **Border:** 1px Slate Line.
- **Internal Padding:** 16px dashboard cards; `10px 12px` form cards; card headers underline with 1px subtle border.

### Inputs / Fields
- **Style:** Paper White fill, 1px Graphite Rule border, 4px radius, 12px text, `5px 8px` padding; labels sit above at 11px/600.
- **Focus:** Border shifts to Ledger Blue with a `0 0 0 2px` Ledger Blue Tint halo; global keyboard focus adds the two-layer ring.
- **Computed/Disabled:** Slate Wash background, muted text, not-allowed cursor — computed fields look locked because they are.
- **Amount fields:** Mono, right-aligned, with an attached currency suffix ("Rp").
- **Error:** Field container gains invalid styling; message renders below in role="alert".

### Navigation
- **Icon rail:** 52×50px hit targets, muted icons with 10px captions; active state = Ledger Blue Tint fill, Ledger Blue icon/text, tinted border. Submenu flyouts (220px, shadow-xl, 150ms slide-in) hover off the rail.
- **Topbar:** 52px Paper White with breadcrumb/context on the left and quick actions on the right; 1px Slate Line underneath.
- **Tabs:** Text tabs in 40px strips; nested strips run 38px; the active tab claims Ink text against muted siblings.

### Data Table Engine (signature)
The heart of the product. Uppercase 10.5px tracked heads on Slate Wash stick to the viewport edge while 12px rows scroll beneath; numeric columns are mono and right-aligned; debit cells tint faintly emerald and credit cells faintly slate; totals rows sit on Slate Wash sealed with the 3px double rule; a footer strip carries row counts and pagination. Hover is a whole-row Canvas Mist tint.

### Enterprise Form (signature)
Every commercial document (Quotation, SO, Invoice, Purchase Order…) shares one skeleton: sticky Zone 1 header with document identity and status stamp, Zone 2 tabbed body (icon rail → Header / Items / Additional Info cards), sticky Zone 3 footer with the mono grand total in Ledger Blue and exactly one primary action plus a close affordance in the header. One print/cetak button lives in the header; footers never duplicate dismiss controls.

## Do's and Don'ts

### Do:
- **Do** render every monetary figure in JetBrains Mono, `"tnum" 1`, right-aligned — including KPI cards, table cells, and form totals.
- **Do** reuse the four status stamps (draft/posted/void/pending) verbatim across modules; void always strikes through its label.
- **Do** build commercial forms on the 3-Zone skeleton with `minmax(0, 1fr)` grids so nothing ever clips at the right edge.
- **Do** mark computed fields visually locked (Slate Wash fill, muted text) the moment they're derived.
- **Do** keep interactive feedback fast and quiet: `0.12–0.15s ease` transitions, no bounce, no spring.

### Don't:
- **Don't** use consumer-gradient decoration, glassmorphism/backdrop-blur surfaces, playful bubble-fintech shapes, or decorative italic serif accents — all four are binding rejections.
- **Don't** apply emerald/red/amber as decoration; they exist only for financial state (see The Earned Color Rule).
- **Don't** center or proportionally-space numbers in tables.
- **Don't** introduce new accent hues; the palette is Ledger Blue plus the semantic quartet on slate neutrals.
- **Don't** loosen the density contract (13.5px base, 12px table text, 4px-radius controls) without an explicit accessibility-driven reason.
