# Testing Enhancement Proposal

**Date:** 2026-05-19  
**Status:** Complete
**Last updated:** 2026-05-19
**Supersedes:** Previous frontend-only testing draft at this path

---

## Program Status

| Workstream | Status | Plan |
|---|---|---|
| Phase 0 — Test inventory and risk evaluation | Complete | `docs/superpowers/plans/2026-05-19-testing-phase-0-inventory.md` |
| Phase 1 — Coverage plumbing and frontend unit harness | Complete | `docs/superpowers/plans/2026-05-19-testing-phase-1-coverage-frontend-unit.md` |
| Phase 2 — OpenAPI generation and drift checks | Complete | `docs/superpowers/plans/2026-05-19-testing-phase-2-openapi.md` |
| Phase 3 — Backend component harness | Complete | `docs/superpowers/plans/2026-05-19-testing-phase-3-backend-component.md` |
| Phase 4 — Backend contract harness | Complete | `docs/superpowers/plans/2026-05-19-testing-phase-4-backend-contract.md` |
| Phase 5 — Playwright readiness and first flows | Complete | `docs/superpowers/plans/2026-05-19-testing-phase-5-playwright.md` |
| Phase 6 — Coverage enforcement rollout | Complete | `docs/superpowers/plans/2026-05-19-testing-phase-6-coverage-thresholds.md` |

**Status rule:** whenever Codex works on this testing program, it must update:

- this spec’s `Program Status` table
- the touched implementation plan’s status section
- any completed or re-scoped deliverables for that workstream

This keeps the spec as the portfolio view and the plan files as the execution view.

**Current implementation note:** Phases 0, 1, 2, 3, 4, 5, and 6 are complete. Frontend harness/tests, backend+frontend coverage artifacts, OpenAPI generation/drift checks, backend component harness/tests, backend contract validation, the initial Playwright browser layer, and the first conservative coverage gates are in place. Coverage enforcement is GitHub Actions-only for now and uses floor thresholds on backend unit coverage (`45.0%`) and cleaned frontend app coverage (`22.5%`). Backend component coverage and combined repo coverage remain visibility-only. Rules and stats OpenAPI coverage remains intentionally deferred.

---

## Objective

Establish a practical, staged testing strategy for Expensor that:

- introduces an OpenAPI artifact for the backend
- adds backend component and contract test scaffolding
- adds reliable frontend unit test infrastructure
- creates a clear path to Playwright browser tests
- introduces uniform coverage collection across backend and frontend
- stages coverage enforcement only after the suites become stable

This proposal is intentionally broader than frontend testing. The goal is to treat testing as a repo capability, not a set of isolated tools.

---

## Current State

### Backend

- Go backend has unit tests across handlers, packages, and some integration-style tests
- Coverage is measured and reported in CI artifacts via `task test:be:cover`
- A generated API contract artifact exists at `api/openapi/expensor.openapi.yaml`
- Backend component tests exist under `tests/component` and run through `task test:be:component`
- There is no dedicated `tests/contract` structure yet
- Docker Compose support now covers both local dev Postgres and the backend component harness

### Frontend

- Frontend unit/component testing exists via Vitest + React Testing Library + MSW
- CI runs frontend tests and uploads frontend coverage artifacts
- Playwright browser coverage now exists via preview-mode mocked flows and a small real-stack smoke layer backed by the shared seeded dataset

### CI / Reporting

- CI has separate backend/frontend linting, backend/frontend test execution, backend component tests, backend contract tests, Playwright browser tests, and OpenAPI drift checks
- Coverage artifacts are produced for backend, frontend, and backend component stacks
- Backend component CI prints container logs on failure for debugging startup and dependency failures
- Playwright CI is currently non-blocking and uploads browser traces, screenshots, videos, and HTML/JUnit reports
- There is no combined coverage view for the repo, and Phase 6 will continue to treat combined coverage as visibility-only rather than a gate

---

## Goals

1. Generate and version an OpenAPI spec for the backend under `api/openapi`.
2. Add backend component tests under `tests/component` that run against a real backend and seeded data.
3. Add backend contract tests under `tests/contract` that validate the running backend against the OpenAPI spec.
4. Add frontend unit and component tests that reliably catch regressions in UI behavior and URL-state handling.
5. Define a clean path to Playwright E2E without forcing all browser tests into the first implementation phase.
6. Add Task targets and Docker Compose entrypoints so local and GitHub Actions execution are as similar as possible.
7. Collect backend coverage, frontend coverage, and a combined repo-level coverage report.
8. Stage coverage enforcement: report first, then gate later once the numbers are stable and trustworthy.
9. Require a deliberate test-case evaluation pass before committing to implementation details.

---

## Non-Goals

- Achieve exhaustive test coverage in the first pass
- Introduce browser E2E as a hard CI gate immediately
- Rewrite all existing backend tests into the new structure
- Hand-maintain a large OpenAPI file with no generation path
- Add snapshot-heavy frontend testing patterns that are brittle in open source maintenance

---

## Constraints And Repo Preferences

- Backend component and contract test scaffolding must live under `tests/component` and `tests/contract`
- OpenAPI spec must live under `api/openapi` at the repo root
- Backend component and contract tests must be runnable as Docker Compose applications
- Taskfile targets must exist for all supported test workflows
- Frontend unit and Playwright tests should follow maintainable OSS conventions: small helpers, explicit fixtures, minimal magic, no opaque snapshot dependence
- We should thoroughly evaluate candidate test cases before finalizing the implementation plan

---

## Recommended Tooling

### OpenAPI Generation

**Recommended:** `swaggo/swag` to annotate handlers and generate a baseline OpenAPI document, committed under `api/openapi/expensor.openapi.yaml`.

Why:

- works naturally in Go projects that use `net/http`
- low barrier to initial adoption
- generation can be driven from comments close to handlers and DTOs
- practical for incremental rollout across a large existing route surface

**Alternative considered:** `ogen` or a fully spec-first workflow. Rejected for this phase because the current backend already exists and is not organized around generated server stubs.

**Important caveat:** generated spec quality will only be as good as handler annotations and shared schema modeling. The first milestone should target accuracy for the most-used API groups first, then close gaps.

### Backend Contract Testing

**Recommended:** `schemathesis` running against the generated OpenAPI spec and a live backend container.

Why:

- directly validates request/response behavior against OpenAPI
- supports property-based API probing and catches schema drift well
- runs cleanly as a containerized test client

**Alternative considered:** Dredd. Rejected because `schemathesis` is generally better suited for ongoing contract validation and richer failure discovery.

### Backend Component Testing

**Recommended:** Go stdlib `testing` packages under `tests/component`, using black-box functional tests that talk to a real backend over HTTP.

Why:

- stays in the project’s primary backend language
- uses standard Go tooling with no second test language to maintain
- keeps functional tests explicit and portable across local and CI runs
- allows reuse of Go helpers for fixtures, polling, request building, and assertions
- supports build tags so these suites are opt-in and do not affect normal `go build` or default unit-test workflows

**Execution shape:** build-tagged Go functional tests run inside Docker Compose against a live backend and Postgres, with deterministic seeded state.

### Backend Seed Data

**Recommended:** SQL seed files plus small test-only setup scripts mounted into the Postgres/backend test environment.

Why:

- explicit and inspectable
- easy to reset per suite
- stable across local and CI runs

### Frontend Unit / Component Testing

**Recommended:** `Vitest` + `React Testing Library` + `@testing-library/user-event` + `jsdom`.

Why:

- fits Vite naturally
- encourages user-behavior tests rather than implementation-detail tests
- low maintenance overhead for an open source React repo

### Frontend Network Mocking

**Recommended:** `MSW` for frontend test-time API mocking, shared between Vitest and future Playwright scenarios where appropriate.

Why:

- keeps test handlers close to API semantics
- avoids ad hoc axios mocking across the suite
- creates a path to consistent mocked browser scenarios later

### Browser E2E

**Recommended:** `Playwright` with Chromium in CI initially.

Why:

- strong ergonomics, traces, retries, fixtures, and good CI behavior
- good balance between developer experience and debuggability
- supports staged adoption without requiring all browsers on day one

### Coverage

**Recommended producers:**

- backend: native Go coverage via `go test -coverprofile`
- backend component/integration coverage: Go 1.20+ `go build -cover` / `GOCOVERDIR` / `go tool covdata`
- frontend: Vitest V8 coverage provider

**Recommended interchange/report formats:**

- keep native Go coverage data as the source of truth
- emit frontend `lcov`
- optionally convert Go coverage to a format preferred by downstream tooling
- generate a combined repo summary in CI with a small aggregation script or upload both reports to a coverage service

**Recommended reporting service:** GitHub Actions job summary plus uploaded artifacts first. Add Codecov as the hosted aggregation and PR-feedback layer once the raw reports are stable.

Why staged:

- native stack tools are the most stable source of truth
- Go now has first-class integration coverage support via `GOCOVERDIR` and `go tool covdata`, which is the right base for component-test coverage collection
- repo-level combined coverage is useful, but it should sit on top of native reports rather than replace them

---

## Proposed Repository Layout

```text
api/
  openapi/
    expensor.openapi.yaml
    README.md

tests/
  component/
    docker-compose.yml
    README.md
    go.mod
    fixtures/
      seed.sql
      seed_notes.md
    helpers/
      client.go
      assertions.go
      waits.go
    health_test.go
    transactions_test.go
    rules_test.go
    settings_test.go

  contract/
    docker-compose.yml
    README.md
    schemathesis.yaml
    requirements.txt
    hooks.py
    allowlist.py

frontend/
  src/
    test/
      setup.ts
      render.tsx
      server.ts
      handlers/
      fixtures/
    components/**/*.test.tsx
    pages/**/*.test.tsx
    hooks/**/*.test.tsx

  playwright/
    fixtures/
    mocks/
    utils/
  playwright.config.ts
```

Notes:

- `tests/component` and `tests/contract` are black-box suites owned at the repo level
- `api/openapi` is a versioned artifact directory, not a scratch output folder
- frontend test helpers stay near the frontend codebase, because they are UI-specific and should not be mixed with backend black-box harnesses

---

## Test Layers

### Layer 1: Existing Backend Unit Tests

Keep and extend the existing Go unit tests. They remain the fastest safety net for handler logic, store behavior, and package-level correctness.

Enhancements in this proposal:

- standard coverage generation for existing Go test runs
- clearer separation between unit coverage and component/contract coverage in reporting

### Layer 2: Backend Component Tests

Scope:

- run the real backend in Docker
- run against a real Postgres container
- apply deterministic seed data
- verify business correctness from the outside via HTTP

Examples:

- seeded transactions appear in list and search endpoints as expected
- filter parameters return the right subsets
- label/category/bucket operations mutate data correctly
- dashboard and heatmap endpoints reflect the seeded dataset correctly
- invalid inputs return expected status codes and error bodies

This layer is about correctness of the implemented system, not just schema shape.

### Layer 3: Backend Contract Tests

Scope:

- validate the running backend against the generated OpenAPI document
- catch schema drift between implementation and contract
- verify content type, status code, required fields, and schema compatibility

Examples:

- endpoints return declared response shapes
- documented query parameters are accepted and validated
- documented error responses are actually emitted
- unexpected undocumented fields or malformed shapes are surfaced

This layer is not a replacement for component tests. It is a boundary check.

### Layer 4: Frontend Unit / Component Tests

Scope:

- verify critical UI logic, user interactions, and URL-state invariants
- focus on behavior visible to users, not internal state

Examples:

- URL search param persistence on pages like `Transactions` and `Settings`
- keyboard and outside-click behavior for custom comboboxes and dropdowns
- portal-based floating UI behavior where the code actually uses portals
- confirmation and notification flows
- conditional rendering and query-state handling for key pages

### Layer 5: Browser E2E

Scope:

- a small number of high-value user journeys
- initially non-blocking or separately gated while the suite matures

Examples:

- onboarding/setup happy path with mocked backend responses
- transactions filtering and reload persistence
- rules create/edit/delete flow
- settings tab persistence and save feedback

---

## Test Case Evaluation Before Implementation Planning

Before writing the implementation plan, we should run a dedicated test-case evaluation pass and document it in the spec or a companion matrix.

### Evaluation Method

For each backend API area and frontend page/component:

1. List critical user-visible behaviors.
2. Classify each behavior into the right layer:
   - backend unit
   - backend component
   - backend contract
   - frontend unit/component
   - Playwright E2E
3. Mark each case by value and brittleness:
   - high value / low brittleness
   - high value / high brittleness
   - low value / low brittleness
   - low value / high brittleness
4. Prefer cases that are high value and low brittleness in early phases.
5. Explicitly reject low-value snapshot-style or duplicate cross-layer cases unless they serve a unique purpose.

### Required Output

The implementation plan should not begin until we have:

- a backend API test inventory by endpoint group
- a frontend UI test inventory by page/component group
- a first-pass mapping of cases to test layers
- a list of intentionally deferred cases

This is the guardrail that prevents us from buying tooling first and discovering later that the chosen suite does not target the right risks.

---

## Backend OpenAPI Strategy

### Generation Model

1. Annotate handlers and DTOs in the Go backend.
2. Generate an OpenAPI artifact into `api/openapi`.
3. Commit the generated artifact.
4. Add a verification target that detects drift between source annotations and the committed spec.

### Why Commit The Spec

- frontend and external tooling can consume a stable artifact
- contract tests can pin against a versioned document
- PR review can see API changes directly

### Initial Accuracy Strategy

The first implementation should focus on documenting the most actively used route groups first:

- health/status/version
- transactions
- config subsets used by the frontend
- labels/categories/buckets taxonomy

Lower-priority or operational routes can be added incrementally once the toolchain is working. `rules` and `dashboard/stats` are explicitly acceptable follow-on groups once the initial generation and drift-check path is stable.

---

## Backend Component Test Design

### Execution Model

Each component test run starts a dedicated Compose stack with:

- Postgres
- backend service built from the local repo
- optional seed/init service or mounted SQL fixture
- Go test runner container

### Design Principles

- tests are black-box from the runner’s point of view
- seed data is deterministic and documented
- test helpers hide waiting/retry boilerplate, not assertions
- each test file maps to a user-facing domain, not a technical transport detail
- component suites use Go build tags such as `component` so they are opt-in

### Coverage Collection For Component Tests

Backend component tests should collect coverage from the running application binary, not just the test package.

Recommended approach:

1. Build the backend service with `go build -cover`.
2. Run the service in Docker with `GOCOVERDIR` set to a mounted directory.
3. Execute the Go functional test suite against that live service.
4. Post-process coverage with `go tool covdata`.
5. Merge component-test coverage with unit-test coverage for backend reporting where useful.

This uses modern Go integration coverage support rather than forcing component tests through a non-native harness.

### Candidate Initial Component Suites

- `health_test.go`
- `transactions_test.go`
- `rules_test.go`
- `taxonomy_test.go`
- `dashboard_test.go`
- `settings_test.go`

### Seed Data Guidance

Seed data should be small but intentional:

- a few banks
- multiple currencies
- transactions with and without labels
- muted and unmuted merchants
- categories and buckets covering common and edge paths
- rules that exercise predefined and user-created behavior where practical

---

## Backend Contract Test Design

### Execution Model

Each contract test run starts a dedicated Compose stack with:

- Postgres
- backend service
- contract runner container using `schemathesis`
- mounted OpenAPI spec from `api/openapi`

### Contract Test Scope Rules

- verify documented behavior only
- use explicit allowlists/skip rules for endpoints that are inherently nondeterministic or operationally awkward
- keep auth/setup flows explicit rather than letting property-based exploration create noisy failures

### Expected Value

This layer should catch:

- undocumented status code changes
- mismatched response payloads
- missing required fields
- wrong content types
- query and path parameter mismatches

---

## Frontend Unit Test Strategy

### Test Style

Prefer:

- user-centric interaction tests
- page-level behavior tests for URL state and query-driven rendering
- focused component tests for reusable primitives with real interaction complexity
- shared render helpers for router, query client, and test providers

Avoid:

- broad snapshots of entire pages
- asserting implementation details like hook internals unless the hook itself is the unit
- duplicating the same scenario across component and Playwright layers without a clear reason

### Candidate Initial Targets

#### Shared Components

- `InlineSelect`
- `LabelCombobox`
- `DateRangePicker`
- `ConfirmModal`
- `SlideNotification`
- `useTooltip`

#### Pages

- `Transactions`
- `Settings`
- setup wizard steps with the highest branching logic

### Frontend Best Practices For This Repo

- keep shared test utilities in `frontend/src/test`
- use `MSW` handlers, not ad hoc axios stubs, for networked page tests
- reset query client and mock handlers per test
- prefer explicit fixtures over large JSON dumps
- test URL persistence as a first-class behavior because the repo requires it
- test portal/floating UI behavior only where components actually use portals today
- avoid coupling tests to styling details except where styling encodes state semantics that matter to users

---

## Playwright Path Forward

### Principle

Do not block the initial testing rollout on Playwright. Instead, make the codebase Playwright-ready while landing unit/component coverage first.

### Phase-Ready Requirements

- stable frontend test helpers and MSW handlers
- documented strategy for browser mocking versus real backend usage
- deterministic app bootstrapping in test mode
- a small initial suite of high-value flows only

### Recommended Initial Mode

Start with mocked backend responses for Playwright to keep the suite fast and deterministic. Revisit real-backend browser tests later if the mocked suite proves insufficient for critical workflows.

### First Playwright Flows

1. Setup wizard happy path
2. Transactions filters reflected in URL and preserved on reload
3. Rules create/edit/delete flow
4. Settings tabs persisted in URL

### CI Position

- first phase: Playwright runs in a separate, non-blocking job or manual workflow
- later phase: promote to required once flakes are understood and controlled

---

## Coverage Strategy

### Principles

- use native per-stack coverage producers first
- publish per-stack reports and a combined summary
- do not make coverage a hard gate immediately

### Backend Coverage

Run Go tests with `coverprofile` generation and publish:

- total backend line coverage
- package-level breakdown
- artifact for later aggregation

For backend component/functional coverage, use Go integration coverage support:

- instrument the backend binary with `go build -cover`
- capture runtime coverage via `GOCOVERDIR`
- post-process with `go tool covdata percent` and `go tool covdata textfmt`

### Frontend Coverage

Run Vitest with coverage enabled and publish:

- total frontend statement/branch/function/line coverage
- per-file breakdown
- `lcov` artifact for later aggregation

### Combined Repo Coverage

Generate a CI summary that includes:

- backend coverage %
- frontend coverage %
- combined repo coverage %

Combined coverage should be treated as a visibility metric, not a replacement for per-stack numbers.

### Codecov Position

Codecov is a good fit here, but it is not the producer of coverage. It should be treated as the aggregation, visualization, and PR-feedback layer on top of native backend and frontend coverage outputs.

Recommended model:

- backend unit tests produce Go coverage artifacts
- backend component tests produce Go integration coverage artifacts
- frontend tests produce `lcov`
- CI uploads all reports to Codecov
- Codecov provides combined repo reporting, PR annotations, and historical tracking

This is a better long-term setup than trying to invent a repo-local combined coverage system as the primary source of truth.

### Enforcement Rollout

**Stage 1: Measure only**

- generate and upload reports
- no minimum thresholds

**Stage 2: Soft expectations**

- document target thresholds in the repo
- fail only on broken report generation, not on percentage dips

**Stage 3: Hard thresholds**

- add minimum per-stack thresholds
- optionally add combined coverage floor

Hard thresholds should only be introduced after enough baseline history exists to choose numbers that are realistic and defensible.

---

## Taskfile Additions

The repo should gain explicit targets for:

```text
task openapi:generate
task openapi:check

task test:be:cover
task test:be:component
task test:be:contract

task test:fe
task test:fe:watch
task test:fe:cover
task test:fe:e2e

task coverage
task coverage:report

task test
```

Expected role of each:

- `openapi:generate`: generate the spec into `api/openapi`
- `openapi:check`: detect drift in CI
- `test:be:component`: run Compose-based backend component tests
- `test:be:contract`: run Compose-based contract tests
- `test:fe:cover`: run frontend tests with coverage output
- `coverage`: run all coverage-producing targets
- `coverage:report`: aggregate and summarize artifacts

---

## CI Workflow Changes

Add or evolve jobs along these lines:

1. `openapi-check`
   - generate spec
   - fail on drift

2. `test-backend-unit`
   - run Go unit tests
   - emit backend coverage

3. `test-backend-component`
   - run `tests/component` Compose suite

4. `test-backend-contract`
   - run `tests/contract` Compose suite

5. `test-frontend-unit`
   - run Vitest suite
   - emit frontend coverage

6. `coverage-summary`
   - download artifacts
   - publish job summary with backend, frontend, and combined metrics

7. `test-frontend-e2e`
   - separate job, initially optional or non-blocking

The key rule is parity: local Task targets and CI jobs should call the same underlying commands wherever possible.

---

## Phased Implementation Recommendation

### Phase 0: Test Inventory And Design Validation

- evaluate backend and frontend test cases
- map them to the right layers
- identify initial high-value scope

### Phase 1: Coverage Plumbing And Frontend Unit Harness

- add Vitest/RTL/MSW setup
- add frontend test helpers
- add frontend coverage reporting
- add backend coverage reporting for existing Go tests
- add Codecov upload plumbing only after native report generation is stable

### Phase 2: OpenAPI Generation

- add handler/schema annotations
- generate and commit initial OpenAPI document
- add drift check target

### Phase 3: Backend Component Test Harness

- add `tests/component`
- add Go build-tagged functional test packages
- add Compose stack and seed data
- implement first correctness suites
- collect runtime coverage from the instrumented backend binary

### Phase 4: Backend Contract Test Harness

- add `tests/contract`
- run `schemathesis` against the generated spec and live backend

### Phase 5: Playwright Readiness And First Flows

- add Playwright config and fixtures
- implement a small initial browser suite

### Phase 6: Coverage Enforcement

- review baseline data
- set realistic thresholds
- enable hard gates gradually

---

## Plan Decomposition

This proposal should not be implemented from a single monolithic plan. It is split into deliverable-specific plans:

1. `docs/superpowers/plans/2026-05-19-testing-phase-0-inventory.md`
2. `docs/superpowers/plans/2026-05-19-testing-phase-1-coverage-frontend-unit.md`
3. `docs/superpowers/plans/2026-05-19-testing-phase-2-openapi.md`
4. `docs/superpowers/plans/2026-05-19-testing-phase-3-backend-component.md`
5. `docs/superpowers/plans/2026-05-19-testing-phase-4-backend-contract.md`
6. `docs/superpowers/plans/2026-05-19-testing-phase-5-playwright.md`
7. `docs/superpowers/plans/2026-05-19-testing-phase-6-coverage-thresholds.md`

Phase 0 output docs:

- `docs/testing/test-inventory.md`
- `docs/testing/backend-test-matrix.md`
- `docs/testing/frontend-test-matrix.md`

Each plan should be independently executable and reviewable. Future work on this program should pick a single plan, update its status, complete the work, then reflect the result back into this spec.

---

## Risks And Mitigations

### Risk: OpenAPI generation becomes stale or incomplete

Mitigation:

- commit generated spec
- add drift check in CI
- phase route coverage rather than pretending all endpoints are accurate from day one

### Risk: Component and contract suites become slow

Mitigation:

- keep seed data small
- split correctness and contract concerns
- avoid overloading Playwright with API coverage work

### Risk: Frontend tests become brittle

Mitigation:

- prefer behavior-oriented assertions
- minimize snapshots
- use shared render helpers and explicit fixtures

### Risk: Coverage metrics drive bad incentives early

Mitigation:

- stage enforcement
- keep per-stack metrics visible
- use coverage as one signal, not the only quality signal

---

## Open Questions

- Should the initial OpenAPI artifact aim for full route coverage or only the frontend-consumed API surface?
- Should combined repo coverage be a weighted line-based aggregate only, or should branch metrics be surfaced separately too?
- Do we want to adopt Codecov later, or keep coverage reporting entirely inside GitHub Actions artifacts and summaries?
- For Playwright, should the first browser suite run entirely against MSW, or should one smoke flow hit a real backend stack as a second-stage validation?

---

## Recommendation

Proceed with a staged repo-wide testing initiative, anchored on:

- generated OpenAPI under `api/openapi`
- Dockerized backend component and contract suites under `tests/component` and `tests/contract`
- Vitest/RTL/MSW for frontend unit and component testing
- Playwright as a later phase, not a day-one gate
- native coverage generation per stack plus combined reporting
- explicit test-case evaluation before implementation planning

This gives Expensor a testing architecture that is realistic for the current codebase, aligned with local and CI execution, and maintainable in an open source repository.
