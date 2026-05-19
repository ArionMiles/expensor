# Testing Phase 4 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add contract validation under `tests/contract` using the generated OpenAPI spec and a live backend service.

**Architecture:** This phase verifies boundary conformance rather than business correctness. The contract suite should be isolated from the component suite so failures clearly indicate schema drift versus data/logic regressions.

**Tech Stack:** Schemathesis, OpenAPI, Docker Compose, Task, GitHub Actions

---

## Status

- Status: Complete
- Last updated: 2026-05-19
- Owner: Unassigned
- Execution note: Added a conservative read-only Schemathesis baseline against the current generated OpenAPI scope. The original plan named `schemathesis.yaml`; implementation uses explicit Schemathesis CLI flags plus a flat allowlist file, with the runner executed from the official Schemathesis container rather than committed Python helper code. The workstream also fixed the initial contract drift set in backend handlers and the generated OpenAPI artifact, and `task test:be:contract` now passes.

## Files

- Create: `tests/contract/docker-compose.yml`
- Create: `tests/contract/README.md`
- Create: `tests/contract/allowlist.tsv`
- Modify: `Taskfile.yml`
- Modify: `.github/workflows/ci.yml`

## Deliverables

- [x] Contract runner container
- [x] Configured Schemathesis execution
- [x] Skip/allowlist rules for unstable or unsupported endpoints
- [x] CI and local task targets

## Execution Chunks

- [x] Define contract scope from the generated spec
- [x] Add Compose harness
- [x] Add Schemathesis config and rules
- [x] Add `task test:be:contract`
- [x] Add CI job and artifacts

## Exit Criteria

- `tests/contract` runs against the generated spec and a live backend
- failures cleanly indicate spec/implementation mismatch
