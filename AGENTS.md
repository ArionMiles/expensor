# Expensor Agent Instructions

Keep this file focused on durable rules that prevent common repository-wide mistakes. Discover changing details from the codebase and `task --list-all` instead of adding them here.

## Command Policy

- Prefer `task` targets over bare `go`, `npm`, or tool commands. Targets handle working directories, environment loading, and project tooling.
- Use `task --list-all` to find the correct target.
- Common targets:

```bash
task dev              # Start the local app stack
task fmt              # Format backend and frontend
task test             # Run default backend and frontend tests
task test:be          # Run backend tests; some use Docker
task test:fe          # Run frontend unit and component tests
task lint:be:prod     # Run strict backend lint; must report 0 issues
task openapi:check    # Check generated OpenAPI artifacts for drift
```

## Engineering Rules

### Backend

- Use `backend/pkg/errors` in production Go code. Do not use the standard `errors` package or `fmt.Errorf` outside tests and the error package itself.
- Wrap boundary failures with `errors.B`, a stable operation name, and a useful kind. Use the project package for `Is`, `As`, `Join`, and `Unwrap`.
- Pass caller contexts through request and daemon paths. Do not replace an available context with `context.Background()`.
- Define interfaces at consumer boundaries. Do not add an interface beside one implementation without a decorator or test boundary.
- Use a dependency struct when a constructor needs more than five parameters.
- Keep dependency-native types through configuration and internal APIs. Convert only at representation or unit boundaries.
- Prefer standard-library constants and helpers over manual equivalents.
- Use generics only when they improve type safety or remove meaningful repeated algorithms.
- Keep repositories focused on persistence. Put logging, metrics, and tracing in interface decorators.
- Use `slog` for logs and OpenTelemetry for traces and metrics. Supported telemetry exporters are `none` and `otlp`; stdout is for logs only.
- Never put sensitive or high-cardinality values in telemetry. This includes message content, addresses, IDs, merchant names, raw errors, and raw SQL.
- Route application environment configuration through `pkg/config/config.go`. Feature packages must not read application environment variables directly.

### HTTP API

- Model resources and collections before routes. Prefer standard HTTP methods, collection query parameters, and subresources over action suffixes.
- Model application settings as singleton resources with `GET` and `PATCH` or `PUT`.
- Add stable IDs only for resources with an independent identity or lifecycle.
- Keep distinct read models and protocol commands separate when resource semantics would be artificial.
- Update handlers, clients, mocks, OpenAPI, contract allowlists, and docs together when routes change. Add compatibility aliases only when required.
- Decode and validate request DTOs in the owning handler. Return `400` for malformed syntax and `422` for semantic validation failures.
- Return validation details as `field`, `location`, and `message`. Validate the complete request before persistence.
- Use parameterized SQL and keep domain or store invariants required by non-HTTP callers.

### Migrations

- Add paired Postgres migrations under `backend/internal/store/postgres/migrations/` as the next `NNN_description.up.sql` and `.down.sql` files.
- Make every migration idempotent with guards such as `IF NOT EXISTS` and `ON CONFLICT DO NOTHING`.

### Rules And Fixtures

- Treat `backend/internal/catalog/content/rules.json` as the source of truth for bundled extraction rules.
- Preserve `sender_emails`, `source.type`, `source.label`, and `source.bank`; update matching presets when a rule adds a type or bank.
- Add one email per `tests/data/rule-emails/<bank>_<source-type>_<case>.rule.fixture` file, with YAML front matter followed by the raw email.
- Keep fixture names lowercase, and do not put regexes or timestamps in fixtures.
- Assert sender and subject matching plus amount, merchant, and currency extraction.

## Frontend

Read `frontend/README.md` before frontend changes. It is the source of truth for design language, shared components, interaction patterns, internationalization, URL state, and document titles.

## Screenshots

Read `docs/screenshots/README.md` before changing screenshot assets, capture tooling, or screenshot seed data.

## Git Conventions

- Name branches `<type>/<short-description>` with a supported type: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, or `chore`.
- Use lowercase, hyphen-separated branch descriptions without Jira tickets.
- Format commits and PR titles as `<type>(<optional-scope>): <subject>` in the present tense.
- Do not commit directly to `main`; branch protection allows bypasses only for docs and chores.
- Use `.github/PULL_REQUEST_TEMPLATE.md` for every PR description.

## Testing Strategy

- For behavior changes, write a failing test first, confirm the failure, implement the change, then run the narrow relevant suite and `task test`.
- Do not add tests for mechanical refactors or documentation-only changes when existing checks already prove correctness.
- Use existing unit, component, contract, and Playwright infrastructure instead of ad hoc checks.
- Prefer component tests for isolated UI behavior and Playwright for routing, persistence, user flows, and backend integration.
- Run Postgres integration tests explicitly when changing store or ingestion behavior.
