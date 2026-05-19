# Open Work Index

**Date:** 2026-05-19
**Last reconciled:** 2026-05-19
**Status:** Active; feedback triage slices complete, follow-up queue narrowed

---

## Purpose

Track the non-testing specs and plans that are still pending so implementation can be selected from one queue instead of inferred from scattered docs.

The original feedback triage slices are complete. The queue below now tracks follow-up work that was discovered or intentionally deferred during those slices.

## Priority Queue

### Current Feedback Triage Queue

1. `docs/superpowers/plans/2026-05-19-search-followup.md`
   Status: Partially complete; ranking remains open after safe highlighting shipped.
   Notes: Saved searches and advanced query syntax were intentionally removed from scope.
2. `docs/superpowers/plans/2026-05-19-remove-runtime-compat-importer.md`
   Status: Pending external rollout; execute only after the two known existing users have updated Docker images with DB-backed runtime state.

### Parked Follow-Ups

- `docs/superpowers/plans/2026-05-19-diagnostics-followup.md`
  Status: Parked for later.
  Notes: The core diagnostics workflow is implemented and working. The follow-up would add Thunderbird body-hash dedupe, reprocess actions, and richer repair guidance, but this is not currently high priority unless duplicate diagnostics or repair friction becomes a recurring user-visible problem.
- `docs/superpowers/plans/2026-05-19-backup-restore-db-runtime-followup.md`
  Status: Blocked behind multi-tenant households.
  Notes: Runtime-state details in the backup design need to be updated eventually, but backup/restore implementation remains explicitly deferred until tenant scoping exists.

### Completed Feedback Triage Work

- `docs/superpowers/plans/2026-05-19-repository-instrumentation-spike.md`
  Status: Implemented
  Notes: Query inventory, repository boundary note, instrumentation helper, label repository prototype, and spike results are complete. Results live in `docs/architecture/repository-spike-results-2026-05-19.md`.
- `docs/superpowers/plans/2026-05-19-i18n-foundation.md`
  Status: Implemented
  Notes: Lightweight English message catalog, `I18nProvider`, formatting helpers, navigation shell, command palette, settings shell, and Dashboard/Transactions formatting boundaries are implemented. Extraction guardrails live in `docs/i18n/string-extraction.md`.
- `docs/superpowers/plans/2026-05-19-feedback-slice-4-db-runtime-state.md`
  Status: Implemented and merged to `main`
  Notes: DB-backed runtime state/config implemented with migration `014_runtime_state.sql`. Temporary file-to-DB importer remains intentionally isolated with the explicit removal plan preserved for after the two known users update Docker images.
- `docs/superpowers/plans/2026-05-19-accessibility-audit-remediation.md`
  Status: Implemented and merged to `main`
  Notes: Accessibility audit harness, audit doc, semantic/focus fixes, URL-state fixes, axe smoke checks, and keyboard follow-ups for command palette, transaction labels, filters, and add-filter menu are merged.
- `docs/superpowers/plans/2026-05-19-feedback-slice-1-reliability.md`
  Status: Implemented
  Notes: Reliability, new-rule lookback, force rescan, Spend Breakdown crash/order, mute tooltip, and Gmail processRule complexity cleanup are complete.
- `docs/superpowers/plans/2026-05-19-extraction-diagnostics.md`
  Status: Implemented
  Spec: `docs/superpowers/specs/2026-05-19-extraction-diagnostics-design.md`
  Notes: Slice 2 diagnostics workflow is complete for both Gmail and Thunderbird.
- `docs/superpowers/plans/2026-05-19-feedback-slice-3-ux-polish.md`
  Status: Implemented
  Notes: Slice 3 setup guide width, command palette descriptions/search, custom date picker time controls, selected transaction sum, and improved transaction search are complete.
- `docs/superpowers/specs/2026-05-19-merchant-wide-categories-design.md`
  Status: Implemented
  Notes: Merchant-wide category/bucket application and future-scan persistence are complete.
- `docs/superpowers/specs/2026-05-19-dashboard-summary-polish-design.md`
  Status: Implemented
  Plan: `docs/superpowers/plans/2026-05-19-dashboard-summary-polish.md`
  Notes: Dashboard summary polish is complete.

### Older Approved Backlog

These remain approved but are behind the feedback triage queue unless a user explicitly selects one:

1. `docs/superpowers/specs/2026-05-19-multi-tenant-households.md`
   Status: Ready to implement — before original backup/restore design
2. `docs/superpowers/specs/2026-05-19-setup-wizard-design.md`
   Status: Approved; some setup-guide polish is also covered by Slice 3
3. `docs/superpowers/specs/2026-05-19-sse-daemon-events-design.md`
   Status: Approved
4. `docs/superpowers/specs/2026-05-19-currency-conversion-design.md`
   Status: Approved
5. `docs/superpowers/specs/2026-05-19-dashboard-transactions-polish-design.md`
   Status: Approved; some transaction polish is also covered by Slice 3 and search follow-up

## Sequencing And Dependencies

Implementation order:

1. Runtime importer removal follow-up after the two known users update their Docker images.
2. Search ranking and highlighting follow-up.
3. Diagnostics follow-up only if duplicate diagnostics or repair friction becomes a real priority.
4. Backup/restore only after multi-tenant households are implemented.

Migration numbering reservation:

- `012_extraction_diagnostics.sql`: Slice 2 diagnostics.
- `013_search_trigram.sql`: Slice 3 search.
- `014_runtime_state.sql`: Slice 4 DB runtime state.
- `015_saved_searches.sql`: Previously reserved for saved searches; no longer reserved while saved searches are out of scope.

If implementation happens out of order, use the next actual migration number and update the affected plans before coding.

## Proposed, Not Planned

- `docs/superpowers/specs/2026-05-19-webhooks.md`
- `docs/superpowers/specs/2026-05-19-monthly-signal-reports.md`
- `docs/superpowers/specs/2026-05-19-beancount-export.md`

## Explicitly Deferred Or Discretionary

- `docs/superpowers/specs/2026-05-19-backup-restore.md`
- `docs/superpowers/specs/2026-05-19-starlight-docs-site.md`

## Notes

- Testing work is intentionally excluded from this index because the testing program is complete and already tracked in its own closed program docs.
- This file is a queue index, not a replacement for individual specs or implementation plans.
- The feedback triage source spec is complete: `docs/superpowers/specs/2026-05-19-feedback-triage-design.md`.
