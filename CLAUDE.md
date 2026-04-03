# Expensor — Claude Instructions

## Project Overview

Expensor is a personal finance tracker that reads expense emails (Gmail, Thunderbird), extracts transaction details via regex rules, and stores them in PostgreSQL. It has a Go backend (HTTP API + daemon runner) and a React/Vite/Tailwind frontend.

## Repository Layout

```
backend/          Go module (github.com/ArionMiles/expensor/backend)
  cmd/server/     Main binary — HTTP server + daemon orchestration
  internal/
    api/          HTTP handlers, routing, middleware, Storer interface
    daemon/       Reader → writer pipeline
    plugins/      Plugin registry
    store/        PostgreSQL query layer (pgx/v5)
  migrations/     Numbered SQL files embedded into the binary (001_, 002_, …)
  pkg/
    api/          Core interfaces: Reader, Writer, Rule, Labels
    config/       koanf-based env config
    extractor/    Regex extraction helpers
    plugins/      Plugin wrappers (readers: gmail, thunderbird; writer: postgres)
    reader/       Concrete reader implementations
    writer/       Concrete writer implementations
frontend/         React + Vite + Tailwind (src/, public/)
tests/            Integration test helpers, local docker-compose
```

## Essential Commands

Always prefer `task` over bare `go`/`npm` commands — task targets handle tooling, env loading, and working directory.

```bash
task dev           # Start postgres + backend + frontend (full stack, loads tests/.env)
task run           # Backend only
task run:frontend  # Frontend Vite dev server only
task fmt           # Format (gci import ordering + gofumpt)
task lint          # Lint with local config (.golangci.toml)
task lint:prod     # Strict CI lint (.golangci-prod.toml) — must be clean before commit
task test          # All tests (integration tests require Docker; -short skips them)
task build:binary  # Optimized binary → bin/expensor
task ci            # lint:prod + test
task db:start      # Start local dev postgres container
task db:stop       # Stop local dev postgres container
```

## Architecture Patterns

### Plugin system
Reader and writer plugins implement interfaces in `pkg/api`. Plugins are registered in `cmd/server/main.go` and selected at runtime via the web UI (not env vars). Adding a new reader means implementing `plugins.ReaderPlugin` and registering it in the registry.

### Storer interface
`internal/api/store.go` defines `Storer` — a narrow interface over `*store.Store` used by HTTP handlers. It is **not** `*store.Store` itself. When adding a new store method that a handler needs, add it to `Storer` first, then implement it on `*store.Store`. The compile-time assertion `var _ Storer = (*store.Store)(nil)` at the bottom of `store.go` will catch mismatches.

### Handler tests
Unit tests in `internal/api/handlers_test.go` use `mockStore` (not a real DB). When adding a new `Storer` method, add the corresponding mock method too. Use `httptest.NewRequestWithContext(context.Background(), ...)` — not `httptest.NewRequest`.

### Integration tests
`internal/store/store_test.go` and `pkg/writer/postgres/postgres_test.go` spin up a real Postgres container via testcontainers-go. Skip with `-short`. These tests live in `package store_test` / `package postgres_test` (external test packages).

### Migrations
SQL files in `backend/migrations/` are embedded into the binary and run automatically on startup. Name new files `NNN_description.sql` (next sequential number). Migrations use `IF NOT EXISTS` and `ON CONFLICT DO NOTHING` — they must be idempotent.

### Configuration
All env config flows through `pkg/config/config.go` using koanf. Only four env prefixes are loaded: `EXPENSOR_`, `GMAIL_`, `THUNDERBIRD_`, `POSTGRES_`. Do not add config fields under other prefixes.

## Lint Rules to Know

The prod linter is strict. Common traps:

- **noctx**: use `httptest.NewRequestWithContext` in tests, never `httptest.NewRequest`
- **lll**: max line length is 160 characters
- **misspell**: use American spellings (`canceled`, `labeled`)
- **revive argument-limit**: max 5 params per function — use `//nolint:revive` with a justification for DI constructors and test helpers
- **gosec G703/G705**: file path taint analysis — add `//nolint:gosec // path built from validated reader name` when paths are safe but taint-analysis can't prove it
- **gocognit**: max cognitive complexity 20 — extract helper functions if a function grows complex
- **import-shadowing**: don't name local variables the same as imported packages (e.g. don't shadow `url`, `path`, `time`)

Run `task lint:prod` before every commit. It must report `0 issues`.

## Git Conventions

- Branch format: `pr/<short-description>` (no Jira ticket for this repo)
- Commits: imperative mood, Tim Pope style, `--no-gpg-sign`
- Never commit to `main` directly — branch protection requires PRs (bypass only for docs/chore)
- Check `.pre-commit-config.yaml` before committing if it exists

## Testing Strategy

Follow TDD:
1. Write failing tests first
2. Confirm they fail
3. Implement to make them pass
4. Run full suite (`task test`) to catch regressions

Unit tests mock the store via `mockStore`. Integration tests hit a real container — run them explicitly when changing `store/` or `writer/postgres/`.
