# Backend Package Organization Candidate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Clarify three backend package boundaries through behavior-neutral moves and names.

**Architecture:** Rename the HTTP transport package, make OAuth ownership explicit, and retain shared extraction as a focused internal package. Update all imports without adding compatibility packages or changing runtime behavior.

**Tech Stack:** Go 1.26, standard Go package layout, Task.

---

### Task 1: Rename the HTTP transport package

**Files:**
- Move: `backend/internal/api/*.go` to `backend/internal/httpapi/*.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/cmd/server/main_test.go`

- [ ] Move the package directory from `internal/api` to `internal/httpapi`.
- [ ] Change package declarations from `package api` to `package httpapi`.
- [ ] Update imports of `github.com/ArionMiles/expensor/backend/internal/api` to `github.com/ArionMiles/expensor/backend/internal/httpapi`.
- [ ] Update the OpenTelemetry instrumentation scope to end in `/internal/httpapi`.
- [ ] Run `task test:be` and confirm the backend suite passes.

### Task 2: Move the OAuth package

**Files:**
- Move: `backend/pkg/client/oauth.go` to `backend/internal/oauth/oauth.go`
- Move: `backend/pkg/client/oauth_test.go` to `backend/internal/oauth/oauth_test.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/internal/httpapi/handlers_readers.go`

- [ ] Move the OAuth implementation and tests to `internal/oauth`.
- [ ] Change package declarations and aliases from `client` or `oauthclient` to `oauth`.
- [ ] Update imports to `github.com/ArionMiles/expensor/backend/internal/oauth`.
- [ ] Preserve the existing exported function and interface names in this candidate.
- [ ] Run `task test:be` and confirm the backend suite passes.

### Task 3: Move shared extraction logic

**Files:**
- Move: `backend/pkg/extractor/extractor.go` to `backend/internal/extractor/extractor.go`
- Move: `backend/pkg/extractor/extractor_test.go` to `backend/internal/extractor/extractor_test.go`
- Modify: `backend/pkg/reader/gmail/gmail.go`
- Modify: `backend/pkg/reader/thunderbird/thunderbird.go`
- Modify: `backend/pkg/rules/fixtures_test.go`

- [ ] Move extraction implementation and tests to `internal/extractor`.
- [ ] Update imports to `github.com/ArionMiles/expensor/backend/internal/extractor`.
- [ ] Keep `ExtractTransactionDetails` and its behavior unchanged.
- [ ] Run `task test:be` and confirm the backend suite passes.

### Task 4: Verify and publish

**Files:**
- Modify: `.github/PULL_REQUEST_TEMPLATE.md` only through the PR body, not in the repository.

- [ ] Run `task fmt:be`.
- [ ] Run `task test:be`.
- [ ] Run `task lint:be:prod` and confirm it reports zero issues.
- [ ] Run `task openapi:generate`, review the expected `api.*` to `httpapi.*`
      component identifier changes, then run `task openapi:check`.
- [ ] Run `task test`.
- [ ] Review `git diff --check` and the final diff for accidental behavior changes.
- [ ] Commit using imperative mood and `--no-gpg-sign`.
- [ ] Push `pr/backend-package-organization-review`.
- [ ] Create a pull request using every section of `.github/PULL_REQUEST_TEMPLATE.md`.
