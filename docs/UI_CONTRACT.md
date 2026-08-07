# UI Contract

**Version:** 0.3.0  
**Status:** Draft  
**Owner:** Product + Frontend  
**Language:** English

## Design Direction

- Calm, clear, non-accountant-first accounting workbench.
- Wave-inspired corporate product language: warm off-white canvas, deep navy ink, restrained teal accent, quiet borders, and modest elevation.
- Use a professional sans-serif hierarchy for headings and body copy; reserve monospaced type for amounts, dates, codes, and compact metadata.
- Prefer native controls and structured layouts over decorative component stacks.
- Optimize for readable daily-work density on desktop and mobile.

## Tokens

- Canvas: `#f7f5f0`
- Paper: `#fbfaf6`
- Elevated paper: `#ffffff`
- Panel: `#f2efe8`
- Deep ink: `#142036`
- Body ink: `#1f2c46`
- Secondary ink: `#4f5b71`
- Muted ink: `#7d8597`
- Accent: `#0d7370`
- Accent hover: `#0a5b58`
- Positive: `#2d7a5c`
- Negative: `#a8443b`
- Warning: `#b87a2e`
- Border: `#d8d3c8`
- Border strong: `#b9b1a0`
- Spacing base: `0.25rem`
- Radius: 4-6px for cards, inputs, buttons, and popup surfaces.
- Shadows: subtle, tinted shadows only for elevated cards and popup menus.

## Layout

- Corporate top bar (56px) contains the Ledgerly brand, live indicator, business name, session context, clock, and sign out.
- Fixed left sidebar (240px) contains accounting modules. Hovering or focusing a module opens a white sub-menu popup; on mobile it expands inside the slide-over sidebar.
- A browser-style tab strip sits below the top bar. Dashboard is the first default tab. Additional module and entry tabs can be opened, activated, and closed.
- Main work area uses a centered max-width of approximately 1320px with responsive table and form layouts.

## UX Rules

- Beginner UI uses "Money In", "Money Out", "Transfer", "Profit/Loss", and "Bills".
- Do not expose debit/credit in beginner flows except inside the accountant-oriented multi-line journal grid.
- Every loading, empty, success, and error state is explicit.
- Keyboard navigation, visible focus, semantic labels, and responsive layouts are required.
- Accounting confirmation and error copy must be understandable in English.
- List tabs provide search, filters, reload, add, row preview/open, and visible totals.
- Entry tabs provide a header, multi-line account grid, debit/credit balance check, memo/footer, and unsaved-change protection.

## Skill

- Taste Skill: `design-taste-frontend` from `https://github.com/Leonxlnx/taste-skill`.
- Installed project path: `.agents/skills/design-taste-frontend`.
- Skill is guidance, not authority over accounting behavior.
