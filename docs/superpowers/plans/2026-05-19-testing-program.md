# Testing Enhancement Program Implementation Index

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver the testing enhancement proposal through bounded, independently shippable workstreams instead of one monolithic effort.

**Architecture:** Treat the testing initiative as a program with one portfolio spec and multiple execution plans. Each execution cycle should select one workstream, update status in both the spec and the chosen plan, then complete only that bounded slice.

**Tech Stack:** Go, Vitest, React Testing Library, MSW, Playwright, Docker Compose, Swaggo, Schemathesis, GitHub Actions, Codecov

---

## Program Status

- Spec: `docs/superpowers/specs/2026-05-19-frontend-testing.md`
- Index status: Complete
- Last updated: 2026-05-19

## Workstreams

- [x] Phase 0: Test inventory and risk evaluation
  Status: Complete
  Plan: `docs/superpowers/plans/2026-05-19-testing-phase-0-inventory.md`
- [x] Phase 1: Coverage plumbing and frontend unit harness
  Status: Complete
  Note: Frontend harness, tests, Task targets, CI wiring, and backend coverage reporting are implemented.
  Plan: `docs/superpowers/plans/2026-05-19-testing-phase-1-coverage-frontend-unit.md`
- [x] Phase 2: OpenAPI generation and drift checks
  Status: Complete
  Note: Generated OpenAPI baseline, Task targets, README, CI drift check, and initial handler annotations are implemented.
  Plan: `docs/superpowers/plans/2026-05-19-testing-phase-2-openapi.md`
- [x] Phase 3: Backend component harness
  Status: Complete
  Note: Component Compose harness, seeded dataset, table-driven suites, task target, CI job, failure-log diagnostics, and runtime backend coverage artifact are implemented.
  Plan: `docs/superpowers/plans/2026-05-19-testing-phase-3-backend-component.md`
- [x] Phase 4: Backend contract harness
  Status: Complete
  Note: Containerized Schemathesis harness, read-only allowlist, task target, CI job, failure-log diagnostics, and backend/OpenAPI fixes for the initial contract drift set are implemented.
  Plan: `docs/superpowers/plans/2026-05-19-testing-phase-4-backend-contract.md`
- [x] Phase 5: Playwright readiness and first flows
  Status: Complete
  Note: Playwright now runs against the built preview app for mocked browser flows (`Transactions`, `Settings`, `Dashboard`, shell navigation) plus a separate real full-stack smoke slice against the shared seeded backend. CI remains non-blocking while the harness gathers stability data.
  Plan: `docs/superpowers/plans/2026-05-19-testing-phase-5-playwright.md`
- [x] Phase 6: Coverage enforcement rollout
  Status: Complete
  Note: GitHub Actions now enforces conservative floor thresholds for backend unit coverage (`45.0%`) and cleaned frontend app coverage (`22.5%`). Backend component coverage remains advisory and combined coverage stays visibility-only for now.
  Plan: `docs/superpowers/plans/2026-05-19-testing-phase-6-coverage-thresholds.md`

## Operating Rules

- [ ] Before starting any workstream, update the spec’s `Program Status` table to mark that workstream `In Progress`.
- [ ] Update the chosen plan’s `Status` block with the current date and execution note.
- [ ] Keep unrelated workstreams unchanged unless the scope boundary itself changes.
- [ ] When a workstream finishes, update:
  - the spec `Program Status` table
  - this index checklist
  - the chosen plan’s `Status` block and exit criteria
- [ ] If scope changes, record it in the spec before implementation starts.

## Suggested Delivery Order

1. Phase 0
2. Phase 1
3. Phase 2
4. Phase 3
5. Phase 4
6. Phase 5
7. Phase 6

## Exit Criteria

- Each workstream has a matching plan doc
- The spec points to each workstream plan
- Future implementation work can pick one bounded plan without redoing decomposition
