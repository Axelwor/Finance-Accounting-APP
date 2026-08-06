# UI Contract

**Version:** 0.1.0  
**Status:** Draft  
**Owner:** Product + Frontend  
**Language:** Bahasa Indonesia

## Design Direction

- Calm, clear, non-accountant-first interface.
- Use a restrained warm-neutral surface with one high-contrast accent.
- Prefer native controls and simple layouts over decorative component stacks.
- Optimize for readable dashboard density on desktop and mobile.

## Tokens

- Background: `#f4f1e9`
- Ink: `#17211f`
- Muted text: `#52615c`
- Accent: `#8a4b2a`
- Spacing base: `0.25rem`
- Radius: use modest radius; avoid excessive rounded cards.

## UX Rules

- Beginner UI uses “Uang Masuk”, “Uang Keluar”, “Pindah Uang”, “Untung/Rugi”, and “Tagihan”.
- Do not expose debit/credit in beginner flows.
- Every loading, empty, success, and error state is explicit.
- Keyboard navigation, visible focus, semantic labels, and responsive layouts are required.
- Accounting confirmation and error copy must be understandable in Indonesian.

## Skill

- Taste Skill: `design-taste-frontend` from `https://github.com/Leonxlnx/taste-skill`.
- Installed project path: `.agents/skills/design-taste-frontend`.
- Installation command: `npx skills add https://github.com/Leonxlnx/taste-skill --skill design-taste-frontend -y`.
- Skill is guidance, not authority over accounting behavior.
