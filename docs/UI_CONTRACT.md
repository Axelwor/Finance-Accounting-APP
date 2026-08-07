# UI Contract

**Version:** 0.2.0  
**Status:** Draft  
**Owner:** Product + Frontend  
**Language:** English

## Design Direction

- Calm, clear, non-accountant-first interface.
- Clean accounting-software look inspired by Accurate Online: cool light-gray surface with a single blue accent and a left sidebar.
- Prefer native controls and simple layouts over decorative component stacks.
- Optimize for readable dashboard density on desktop and mobile.

## Tokens

- Background: `#f5f7fa`
- Ink: `#1c2733`
- Muted text: `#5b6b7c`
- Accent: `#1e6fd9`
- Accent hover: `#185bb4`
- Surface: `#ffffff`, Surface 2: `#eef2f7`
- Border: `#dde3ea`, Border strong: `#c3ccd8`
- Positive: `#1e7e4e`, Negative: `#c0392b`
- Spacing base: `0.25rem`
- Radius: use modest radius (4-6px); avoid excessive rounded cards.

## Layout

- Fixed left sidebar (width ~232px) for primary navigation on desktop; collapses to a slide-over menu under 900px.
- Main content area keeps a single centered column, max-width ~1120px.

## UX Rules

- Beginner UI uses "Money In", "Money Out", "Transfer", "Profit/Loss", and "Bills".
- Do not expose debit/credit in beginner flows.
- Every loading, empty, success, and error state is explicit.
- Keyboard navigation, visible focus, semantic labels, and responsive layouts are required.
- Accounting confirmation and error copy must be understandable in English.

## Skill

- Taste Skill: `design-taste-frontend` from `https://github.com/Leonxlnx/taste-skill`.
- Installed project path: `.agents/skills/design-taste-frontend`.
- Installation command: `npx skills add https://github.com/Leonxlnx/taste-skill --skill design-taste-frontend -y`.
- Skill is guidance, not authority over accounting behavior.
