# Testing Phase 6 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move from coverage visibility to staged enforcement after baseline data is available.

**Architecture:** Use observed native coverage reports to set realistic per-stack floor thresholds in GitHub Actions first. Clean frontend coverage scope before enforcing it, avoid combined coverage gates for now, and keep backend component coverage advisory until its harness history is mature.

**Tech Stack:** Go coverage, Vitest coverage, GitHub Actions, Codecov

---

## Status

- Status: Complete
- Last updated: 2026-05-19
- Owner: Unassigned
- Execution note: Implemented GitHub Actions-only floor thresholds, not delta rules. Enforcement covers backend unit coverage (`45.0%`) and cleaned frontend app coverage (`22.5%`) using native artifacts and lightweight parsing scripts. Backend component coverage and combined coverage remain visibility-only in this phase.

## Files

- Modify: `.github/workflows/ci.yml`
- Modify: `Taskfile.yml`
- Modify: `docs/superpowers/specs/2026-05-19-frontend-testing.md`
- Modify: `frontend/vitest.config.ts`
- Modify: `frontend/package.json`
- Create: `scripts/coverage/`

## Deliverables

- [x] Coverage baseline review
- [x] Frontend coverage-scope cleanup for app-only enforcement
- [x] Proposed per-stack floor thresholds
- [x] CI enforcement changes
- [x] Documented exception/update process

## Execution Chunks

- [x] Gather baseline history from previous phases
- [x] Clean frontend coverage includes/excludes and emit a machine-readable summary
- [x] Propose conservative backend/frontend floor thresholds with rationale
- [x] Add CI threshold checks in GitHub Actions
- [x] Document how thresholds are raised or exceptions are handled later

## Exit Criteria

- coverage enforcement exists and is documented
- thresholds are based on observed repo history, not guesswork
- frontend coverage gate applies to app code rather than repo config files

## Planning Notes

- Use GitHub Actions as the only enforcement surface in this phase.
- Enforce per-stack floors only:
  - backend unit coverage
  - frontend app coverage
- Do not enforce:
  - backend component coverage
  - combined repo coverage
  - patch/project delta rules
- Conservative floors should be set slightly below currently observed stable baselines to absorb normal variance.
- Current local reference point:
  - backend unit coverage artifact reports `46.0%` statements from a fresh `backend/coverage.out`
  - cleaned frontend app coverage reports `23.22%` statements from `frontend/coverage/coverage-summary.json`
- Initial conservative floor proposal:
  - backend unit coverage: `45.0%`
  - frontend app coverage: `22.5%`

## Threshold Maintenance

- Raise floors only after multiple stable CI runs show consistent headroom above the current minimums.
- Treat backend component coverage and combined repo coverage as visibility metrics until their historical variance is better understood.
- If a legitimate refactor temporarily drops below a floor, update tests first where practical; lower the floor only with an explicit plan note and spec/program sync.
