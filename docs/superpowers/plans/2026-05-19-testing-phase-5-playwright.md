# Testing Phase 5 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a small, stable Playwright suite for high-value browser flows once the frontend unit harness and mock infrastructure are in place.

**Architecture:** Split the rollout into two slices: preview/build-mode mocked browser flows first, then a thin real full-stack smoke layer. Reuse the frontend’s shared test fixtures for mocked flows and the existing seeded backend dataset for the real-stack slice.

**Tech Stack:** Playwright, MSW, Vite, Task, GitHub Actions

---

## Status

- Status: Complete
- Last updated: 2026-05-19
- Owner: Unassigned
- Execution note: Implemented Playwright against a built preview server for mocked browser regressions (`Transactions`, `Settings`, `Dashboard`, app-shell navigation) plus a separate non-blocking real-stack smoke project that reuses the existing seeded backend dataset via the component Compose stack.

## Files

- Create: `frontend/playwright/fixtures/`
- Create: `frontend/playwright/mocks/`
- Create: `frontend/playwright/utils/`
- Create: `frontend/playwright.config.ts`
- Create: selected `frontend/playwright/**/*.spec.ts`
- Create: `frontend/playwright.global.setup.ts`
- Modify: `frontend/package.json`
- Modify: `frontend/src/main.tsx`
- Modify: `Taskfile.yml`
- Modify: `.github/workflows/ci.yml`

## Deliverables

- [x] Playwright config and launch strategy
- [x] Shared browser fixtures and browser-mode mocks
- [x] Initial mocked browser suite for `Transactions`, `Settings`, `Dashboard`, and shell navigation
- [x] Separate real full-stack smoke project reusing the shared seeded backend dataset
- [x] Separate CI job, initially non-blocking with trace/screenshot/video artifacts

## Execution Chunks

- [x] Decide app boot mode for Playwright
- [x] Add config and task targets
- [x] Add first mocked flows from Phase 0 inventory
- [x] Add real-stack smoke flow using the shared seed
- [x] Add CI job and artifact capture
- [ ] Decide promotion criteria for required status later

## Exit Criteria

- `task test:fe:e2e` runs locally
- `task test:fe:e2e:smoke` runs locally against the real seeded stack
- initial browser suite is reliable enough to keep separate from unit failures
