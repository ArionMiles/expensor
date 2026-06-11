# Plugin Adapter Consolidation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove one-to-one plugin adapter packages by colocating plugin metadata and construction with concrete implementations.

**Architecture:** Keep `internal/plugins` as the catalog and contract package. Make each Gmail, Thunderbird, and PostgreSQL implementation package directly provide its `Plugin` type, preserving all existing plugin behavior.

**Tech Stack:** Go 1.26, Task.

---

### Task 1: Consolidate reader plugins

**Files:**
- Move: `backend/pkg/plugins/readers/gmail/plugin.go` to `backend/pkg/reader/gmail/plugin.go`
- Move: `backend/pkg/plugins/readers/gmail/plugin_test.go` to `backend/pkg/reader/gmail/plugin_test.go`
- Move: `backend/pkg/plugins/readers/thunderbird/plugin.go` to `backend/pkg/reader/thunderbird/plugin.go`
- Move: `backend/pkg/plugins/readers/thunderbird/plugin_test.go` to `backend/pkg/reader/thunderbird/plugin_test.go`

- [ ] Move Gmail plugin code and tests into the Gmail reader package.
- [ ] Remove the self-import alias and call the local Gmail `New` constructor.
- [ ] Move Thunderbird plugin code and tests into the Thunderbird reader package.
- [ ] Remove the self-import alias and call the local Thunderbird `New` constructor.
- [ ] Update external test imports to the concrete reader package where required.
- [ ] Run the Gmail and Thunderbird package tests.

### Task 2: Consolidate the PostgreSQL writer plugin

**Files:**
- Move: `backend/pkg/plugins/writers/postgres/plugin.go` to `backend/pkg/writer/postgres/plugin.go`
- Move: `backend/pkg/plugins/writers/postgres/plugin_test.go` to `backend/pkg/writer/postgres/plugin_test.go`

- [ ] Move PostgreSQL plugin code and tests into the writer package.
- [ ] Remove the self-import alias and build the local `Config` before calling `New`.
- [ ] Update the external test import to the concrete writer package.
- [ ] Run the PostgreSQL writer package tests.

### Task 3: Update registration and remove adapters

**Files:**
- Modify: `backend/cmd/server/main.go`
- Delete: `backend/pkg/plugins/readers/gmail/guide.json`
- Delete: `backend/pkg/plugins/readers/thunderbird/guide.json`

- [ ] Import Gmail and Thunderbird plugins from `pkg/reader`.
- [ ] Import the PostgreSQL plugin from `pkg/writer`.
- [ ] Keep registration names, guide injection, and registry calls unchanged.
- [ ] Remove the empty `pkg/plugins` directory tree and unused guide copies.
- [ ] Confirm no `backend/pkg/plugins` imports remain.

### Task 4: Verify and publish

**Files:**
- Modify: PR #32 description using `.github/PULL_REQUEST_TEMPLATE.md`.

- [ ] Run `task fmt:be`.
- [ ] Run `task test:be`.
- [ ] Run `task lint:be:prod`.
- [ ] Run `task openapi:check`.
- [ ] Run `task test`.
- [ ] Run `git diff --check` and review rename detection.
- [ ] Commit with `git commit --no-gpg-sign -m "Consolidate plugin adapters"`.
- [ ] Push `pr/backend-package-organization-review`.
- [ ] Update PR #32 to describe the additional candidate.
