# Wave Corporate Handoff Notes

**Date:** 2026-08-07  
**Status:** Deployed live to `https://accounting.tikuma.net` (commit `2b50177`)  
**Brand:** Ledgerly (English, M1, IDR)

## Design System

### Colors (Wave-inspired corporate)
| Token | Hex | Use |
|---|---|---|
| `--canvas` | `#f5f7fa` | Page workspace background |
| `--panel` | `#eef2f6` | Sidebar, tab strip, card backgrounds |
| `--panel-hover` | `#e6edf5` | Hover states on panels |
| `--paper` | `#ffffff` | Form cards, tables, flyouts |
| `--ink-deep` | `#18324b` | Primary text, active nav |
| `--ink` | `#263d55` | Body text |
| `--ink-secondary` | `#5c6f82` | Secondary labels |
| `--ink-muted` | `#8191a0` | Meta, captions |
| `--ink-faint` | `#aebbc7` | Disabled, borders |
| `--rule` | `#d8e0e8` | Standard borders |
| `--rule-soft` | `#e9eef3` | Subtle dividers |
| `--accent` | `#2f80ed` | Blue primary — links, buttons, active nav, positive amounts |
| `--positive` | `#27966f` | Profit, credit, positive balances |
| `--negative` | `#c64b4b` | Losses, red |
| `--warning` | `#d18b2c` | Alerts, low stock |

### Typography
| Token | Font | Usage |
|---|---|---|
| `--font-sans` | Inter | All buttons, nav, body labels, tabs — clean corporate sans |
| `--font-display` | Inter | Headings, titles, large numbers |
| `--font-mono` | IBM Plex Mono | Numeric data only (amounts, dates, status codes, KPI values) |

### Key Principles
- **No console language** — zero uppercase meta tags, no "SESSION", "CLOCK", "CONSOLE"
- **One accent color** — blue `#2f80ed` everywhere; green ONLY for positive financial values
- **Icon rail sidebar** — icon-only by default, label appears below on hover, submenu flys out
- **White surfaces** — panels and cards use white/very-light gray, not dark console panels

## Architecture

### File map (sidebar + workbench)
- `web/src/workbench/AppShell.tsx` — layout shell: Sidebar + AppShell grid
- `web/src/workbench/Sidebar.tsx` — icon rail + hover-expand labels + flyout submenu
- `web/src/workbench/TopBar.tsx` — brand + business name + sign out (no clock/dot)
- `web/src/workbench/TabStrip.tsx` — browser-style tabs
- `web/src/workbench/WorkArea.tsx` — tab dispatch to screens
- `web/src/workbench/state.tsx` — useReducer + sessionStorage persistence
- `web/src/workbench/modules.ts` — module registry (6 groups, sub-items)
- `web/src/workbench/types.ts` — tab/session types
- `web/src/styles.css` — **authoritative** token source (~2000 lines)

### Layout spec
```
.app-shell {
  display: grid;
  grid-template-columns: 64px 1fr;   /* sidebar rail | content */
}
```
- Sidebar: 64px icon rail, hover → 160px+ flyout (no width expansion of rail itself)
- `.app-main` is standalone (not grid child with template areas)
- Mobile `<960px`: sidebar hidden, slide-over via `.sidebar.is-open` (fixed, 260px wide)

### Sidebar hover behavior
1. Module icon button has `onMouseEnter`/`onMouseLeave`
2. `handleEnter`: opens flyout + shows label (120ms debounce via `setOpenModule`)
3. `handleLeave`: 200ms deferred close (allows pointer travel from icon to flyout)
4. `insideRef` tracks whether focus is inside module (prevents keyboard focus from closing)
5. Invisible bridge `.rail-flyout::before` covers the gap between icon and flyout

### Module groups (modules.ts)
| Group | Key | Sub-items |
|---|---|---|
| Cash & Bank | `cash` | Other Receipt, Other Payment, Bank Transfer |
| Sales | `sales` | Sales Invoice, Receive Payment |
| Purchases | `purchases` | Purchase Invoice, Pay Bills, Check |
| Inventory | `inventory` | Items, Stock Movements |
| Fixed Assets | `assets` | Asset Register, Depreciation |
| Reports | `reports` | Trial Balance, P&L, Balance Sheet, Cash Flow |

## Backend (already live)
- `GET /api/v1/cash-entries` — unified list of cash-in/cash-out/transfer entries
- Existing endpoints for dashboard summary, accounts, categories, transaction posting, period close/unlock
- `seed.go` already translated to English COA + category names

## Known issues / TODO
1. **CSS**: `.pos-dot` class is defined in AuthScreen but CSS rule is minimal — acceptable
2. **Mobile**: sidebar slide-open works but flyouts don't have touch fallbacks (acceptable for M1)
3. **Sales/Purchases/Inventory/Assets**: these modules use mock data — marked with "Demo Data" badge
4. **Tab restore**: sessionStorage persists tabs across reload — if a non-dashboard tab was active, dashboard won't auto-open. Fix by checking `tabs.length === 0` not just session state.
5. **Caddy cache**: may hold old CSS — hard refresh (Cmd+Shift+R) or wait 30s

## Deployment
```bash
cd /srv/finance-accounting-app
git pull origin main
docker compose --env-file .env.prod -f docker-compose.prod.yml up -d --build web
```
Caddy restarts automatically on container recreate.

## Verify
```bash
curl -s https://accounting.tikuma.net/ | grep -oE 'assets/index-[A-Za-z0-9_-]+\.css'
curl -s https://accounting.tikuma.net/assets/index-*.css | grep -oE '#[0-9a-f]{6}' | sort -u
# Expected present: #ffffff #f5f7fa #18324b #2f80ed #27966f
# Expected absent: #f7f5f0 #0d7370 #2f5d4a #0a0f14
```
