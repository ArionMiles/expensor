# Feedback Triage Design

**Date:** 2026-05-19
**Status:** Complete; Slices 1-4 implemented

## Purpose

This document turns the feedback from `/Users/kanishk/Obsidian/Personal/Expensor/2026-05-19 - Ideas & More feedback.md` into a separate product triage queue.

This queue is separate from the existing open-work index. It can temporarily overtake already-approved specs when feedback items protect data correctness, prevent missed scans, or fix broken user-visible flows.

## Baseline

The feedback note says to complete pending Testing Enhancement phases first. The repository currently marks Testing Phases 0 through 6 as complete in:

- `docs/superpowers/specs/2026-05-19-frontend-testing.md`
- `docs/superpowers/plans/2026-05-19-testing-program.md`

Before implementing this triage queue, do a quick verification pass over those tracking docs. If they are still accurate, no testing-program work blocks this queue.

## Prioritization Rule

Use balanced release slices, with reliability promoted above ordinary polish:

1. Fix data correctness and missed-scan risks before convenience improvements.
2. Pair one larger reliability or workflow item with smaller polish items where that keeps releases shippable.
3. Keep platform rewrites separate from urgent product fixes.
4. Require tests for all backend behavior changes and for UI regressions that can be reproduced in component or Playwright tests.

## Slice 1: Stop Missed Data And Broken Screens

**Status:** Completed.

**Goal:** remove the highest-risk failure modes and visible crashes before broader workflow work.

Items:

1. Gmail token and network failures must not advance reader checkpoints.
2. Expired or revoked OAuth tokens should trigger one refresh attempt before giving up.
3. Token expiry should be logged at `ERROR` level, not `WARN`.
4. Force rescan must ignore the existing checkpoint and scan the configured lookback window.
5. A newly introduced rule should ignore the existing checkpoint on its first run so it can find matching emails in the lookback period.
6. Fix the Spend Breakdown hook-order crash seen after fresh onboarding when category or label data is initially absent.
7. Fix the clipped mute tooltip using the established fixed-position portal pattern.

Notes:

- The checkpoint behavior should distinguish successful scan completion from partial or failed rule evaluation.
- Network or auth failures should leave the previous checkpoint intact.
- The Spend Breakdown crash likely comes from a hook called after an early return in `BreakdownTimeline`; the fix should keep hook order stable even for empty series.

## Slice 2: Extraction Diagnostics Workflow

**Status:** Completed.

**Goal:** make failed extraction cases visible and actionable so rules can be improved from real emails.

Items:

1. When extraction produces an empty merchant or amount `0`, persist a diagnostic row.
2. Store enough context to improve the rule: sender email, subject, original email body, rule ID or rule name, and the amount, merchant, and currency regexes active at the time.
3. Add an API for listing and resolving diagnostic rows.
4. Add a diagnostics page with a table of failed emails and the corresponding failed rule.
5. Add a button that opens the rule editor with relevant fields pre-filled and the email body loaded as a test email sample.

Notes:

- This slice needs a dedicated implementation plan because it crosses reader behavior, migrations, store methods, API handlers, frontend routing, and rule-editor URL state.
- Diagnostics should not block normal daemon processing; failures to write diagnostics should be logged and surfaced without dropping the daemon.
- The UI should make it clear whether a diagnostic has been resolved, ignored, or still needs rule work.

## Slice 3: High-Value UX Polish

**Status:** Completed.

**Goal:** improve daily use without mixing these smaller UI tasks into deeper architecture work.

Items:

1. Make the reader setup guide wider or responsive without shifting the main onboarding forms.
2. Reorder Spend Breakdown toggles from `Labels, Categories, Buckets` to `Categories, Buckets, Labels`.
3. Improve the command palette:
   - Replace URL path display with short descriptions.
   - Make descriptions searchable.
   - Prefer breadcrumb-style display over the current title/subtitle split, unless a tab-first layout reads better in mockups.
4. Redesign the date picker so it matches the custom dark-themed design language and avoids browser-native controls.
5. Show the sum of selected transactions in place of the total sum when multi-select is active.
6. Improve transaction search beyond exact-word and prefix-style full-text behavior.

Notes:

- The date picker currently uses native `<select>` controls for time, which violates the frontend design rules. Replace those with project-style controls.
- Search may become backend-heavy if it moves to trigram, web-search tsquery, or hybrid full-text plus substring matching. Treat it as the largest item in this slice.
- Any filter, tab, or navigation state touched in these pages should follow the URL-state persistence rule.

## Slice 4: Platform Foundations

**Status:** Completed.

**Goal:** plan foundational investments separately so they do not delay urgent reliability fixes.

Items:

1. Move disk-backed runtime state and reader configuration into the DB:
   - `state.json`
   - active reader
   - Gmail client secret
   - Gmail OAuth token
   - Thunderbird persisted config, if currently file-backed
2. Add a temporary file-to-DB importer for backward compatibility.
3. Run an accessibility audit and remediate high-impact issues.
4. Add an i18n foundation so languages can be supported later.
5. Plan the backend repository-pattern rewrite with query templates and instrumentation middleware.

Temporary migration constraint:

- The file-to-DB importer must be isolated from the new DB-backed runtime path.
- The importer should live in a small, clearly named compatibility package or adapter.
- The normal runtime should read and write DB-backed state directly after startup migration completes.
- The importer should log when it migrates a legacy file and should not keep syncing back to disk.
- The importer should be easy to remove in the next release once the two known existing users have updated their Docker images.
- The implementation plan should include an explicit follow-up deletion task that removes the compatibility importer, its tests, and any temporary comments.

Repository-pattern notes:

- Start with an architecture spike before implementation.
- Inventory current query shapes and pick a bounded first migration area.
- Use templates only for structural query variation, never for raw user input.
- Parameterized query arguments remain mandatory.
- Instrumentation middleware should wrap major interfaces while business implementations return well-wrapped errors.

## Suggested Priority Order

Completed implementation order:

1. Slice 1 reliability and broken-screen fixes.
2. Slice 2 extraction diagnostics workflow.
3. Slice 3 UX polish, with search scoped carefully.
4. Slice 4 DB-backed runtime state migration.
5. Slice 4 accessibility audit.
6. Slice 4 i18n foundation.
7. Slice 4 repository-pattern and instrumentation spike.

Remaining follow-up work is tracked outside this triage spec:

- `docs/superpowers/plans/2026-05-19-remove-runtime-compat-importer.md`
- `docs/superpowers/plans/2026-05-19-backup-restore-db-runtime-followup.md`
- `docs/superpowers/plans/2026-05-19-diagnostics-followup.md`
- `docs/superpowers/plans/2026-05-19-search-followup.md`

## Testing Strategy

Slice 1:

- Backend unit tests for checkpoint persistence rules around successful scans, network failures, and OAuth failures.
- Gmail reader tests for force full scan and first-run rule behavior.
- Frontend component tests for Spend Breakdown empty data and tooltip rendering behavior where practical.

Slice 2:

- Store migration tests or integration tests for diagnostic row persistence.
- Handler tests with `mockStore` for diagnostic list and resolve endpoints.
- Frontend component tests for diagnostics table and rule-editor prefill.

Slice 3:

- Component tests for command palette search and rendering.
- Component or Playwright tests for date picker interactions.
- Transaction page tests for selected-sum behavior and URL state preservation.
- Store tests for improved search behavior.

Slice 4:

- Store tests for DB-backed state/config CRUD.
- Migration tests for legacy file import.
- Tests proving the runtime path works without legacy files.
- A follow-up removal plan for the temporary importer.

## Out Of Scope For This Triage Spec

- Implementing any feedback item directly.
- Reordering the existing open-work index.
- A full repository rewrite before the bounded architecture spike.
- Pixel-perfect date picker redesign; that should be handled during Slice 3 planning or visual design.
