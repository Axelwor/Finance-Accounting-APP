# ADR 0001: MVP 1 Boundary and Parallel Session Ownership

**Status:** Accepted for MVP 1  
**Date:** 2026-08-06  
**Owner:** Engineering + Product

## Decision

MVP 1 implements only foundation, authentication/tenant onboarding, COA/category mapping, opening balances, cash in/out, transfers, and basic reports. Sales, purchasing, inventory, production, fixed assets, tax automation, recurring, bank feeds, and advanced workflows are separate feature tracks after the MVP baseline.

## Reasons

- Keeps the first baseline small enough to validate the accounting invariant and transaction boundary.
- Stabilizes API, migration, tenant, journal, idempotency, reversal, and UI contracts before parallel feature work.
- Prevents independent AI sessions from editing overlapping domain paths.

## Consequences

- MVP uses a deliberately small schema subset.
- Later features extend contracts through migrations and dedicated tasks.
- Shared files require coordination tasks.
- Every feature session must add tests, changelog entries, migration notes, and user-story traceability.
