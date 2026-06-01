# Open Work Index

**Date:** 2026-05-19
**Last reconciled:** 2026-06-01
**Status:** Active; completed specs and plans removed

---

## Purpose

Track the non-testing specs and plans that are still pending so implementation can be selected from one queue instead of inferred from scattered docs.

The original feedback triage slices are complete and their specs/plans have been removed. The queue below tracks follow-up work that was discovered or intentionally deferred during those slices.

## Priority Queue

### Current Feedback Triage Queue

1. `docs/superpowers/plans/2026-05-19-search-followup.md`
   Status: Partially complete; ranking remains open after safe highlighting shipped.
   Notes: Saved searches and advanced query syntax were intentionally removed from scope.

### Parked Follow-Ups

- `docs/superpowers/plans/2026-05-19-diagnostics-followup.md`
  Status: Parked for later.
  Notes: The core diagnostics workflow is implemented and working. The follow-up would add Thunderbird body-hash dedupe, reprocess actions, and richer repair guidance, but this is not currently high priority unless duplicate diagnostics or repair friction becomes a recurring user-visible problem.
- `docs/superpowers/plans/2026-05-19-backup-restore-db-runtime-followup.md`
  Status: Blocked behind multi-tenant households.
  Notes: Runtime-state details in the backup design need to be updated eventually, but backup/restore implementation remains explicitly deferred until tenant scoping exists.

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

1. Search ranking follow-up.
2. Diagnostics follow-up only if duplicate diagnostics or repair friction becomes a real priority.
3. Backup/restore only after multi-tenant households are implemented.

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

- Testing work is intentionally excluded from this index because the testing program is complete.
- This file is a queue index, not a replacement for individual specs or implementation plans.
