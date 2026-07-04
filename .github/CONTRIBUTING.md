# Contributing to Expensor

Thank you for contributing to Expensor. This guide covers repository layout, local development, testing, pull requests, internationalization, and bundled rule contributions.

## Code of Conduct

Be respectful and constructive in issues, pull requests, and discussions.

## Repository Structure

```text
.
├── backend/                 # Go API, daemon, reader plugins, migrations, PostgreSQL store
├── content/                 # Bundled rule and asset source data
├── deploy/                  # Public deployment assets, including Docker Compose
├── frontend/                # React + Vite + Tailwind web UI
├── tests/                   # Component, contract, local DB, and integration helpers
├── docs/                    # Project notes, deployment docs, i18n docs, screenshots, and test docs
└── Taskfile.yml             # Build, lint, test, and dev automation
```

## Getting Started

Prerequisites:

- Go 1.26.4 or the version declared in `backend/go.mod`
- Node.js 20 for frontend tooling
- Task 3.x ([installation guide](https://taskfile.dev/installation/))
- Git
- Docker for local PostgreSQL, integration tests, component tests, contract tests, and image builds

Fork and clone the repository:

```bash
git clone https://github.com/YOUR_USERNAME/expensor.git
cd expensor
git remote add upstream https://github.com/ArionMiles/expensor.git
```

## Development

Use [Task](https://taskfile.dev) targets instead of direct `go`, `npm`, or `docker compose` commands where possible. Task targets handle working directories, environment loading, and project-specific setup.

Common commands:

```bash
task dev               # Start postgres + backend + frontend
task run               # Backend only
task run:frontend      # Frontend Vite dev server only

task fmt               # Format Go and frontend code
task lint              # Lint Go and type-check frontend
task lint:be:prod      # Strict Go lint used by CI
task test              # Default backend and frontend tests
task test:be           # Go tests
task test:fe           # Frontend unit/component tests
task test:fe:e2e       # Mocked Playwright E2E tests

task test:be:component # Black-box API component tests
task test:be:contract  # OpenAPI contract tests
task openapi:check     # Regenerate OpenAPI and fail on artifact drift

task build:binary      # Build optimized binary -> bin/expensor
task build:docker      # Build Docker image locally
task secrets:generate  # Generate a base64-encoded 32-byte encryption key
```

Run `task --list-all` to see the full command list.

PostgreSQL-backed tests use Docker. Run the relevant component, contract, or backend test target when changing storage, migrations, ingestion, API behavior, or OpenAPI annotations.

## Testing

Choose the narrowest test that proves the behavior, then run broader relevant suites before opening a pull request.

- Backend unit tests cover handlers, services, extractors, and store-facing behavior.
- Backend store tests and migration tests can start real PostgreSQL containers.
- Component tests live under `tests/component` and run through `task test:be:component`.
- Contract tests live under `tests/contract` and run through `task test:be:contract`.
- Frontend unit and component tests use Vitest and run through `task test:fe`.
- Browser tests use Playwright and run through `task test:fe:e2e` or `task test:fe:e2e:smoke`.

Before submitting backend changes, run:

```bash
task fmt:be
task lint:be:prod
task test:be
```

Before submitting frontend changes, run:

```bash
task fmt:fe
task lint:fe
task test:fe
```

For cross-stack changes, run:

```bash
task fmt
task lint
task test
```

## Coding Standards

### Go

- Follow [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments) and [Effective Go](https://go.dev/doc/effective_go).
- Wrap errors with useful context at package boundaries.
- Use `context.Context` for request, daemon, database, and external-service paths.
- Use structured logging through `log/slog`.
- Keep interfaces small and define them at consumer boundaries.
- Prefer `task lint:be:prod` before committing; it uses the strict CI lint configuration.

### Frontend

- Keep user-facing strings i18n-friendly.
- Follow the existing design system and shared components.
- Avoid browser-native controls where the app already provides custom components.
- Persist page tabs, filters, and comparable navigation state in the URL.

See `frontend/README.md` for frontend design and component conventions.

## Internationalization

Frontend i18n lives in `frontend/src/i18n/`. English is the source catalog in `messages.ts`.

To add a locale:

1. Copy the full English key set.
2. Translate values without renaming keys.
3. Run `task lint:fe` and `task test:fe`.

See [docs/i18n/adding-translations.md](../docs/i18n/adding-translations.md) and [docs/i18n/string-extraction.md](../docs/i18n/string-extraction.md).

## Pull Requests

1. Create a focused branch for your change.
2. Keep the pull request scoped to one problem.
3. Update docs, OpenAPI artifacts, screenshots, rules, or fixtures when the behavior they describe changes.
4. Run the relevant local verification commands and list them in the PR.
5. Use the repository PR template.
6. Address review comments and keep CI green.

Maintainer approval and passing CI are required before merge.

## Issues

Before opening an issue, search existing issues and try the latest version when possible.

Use the appropriate template:

- Bug report for unexpected behavior.
- Feature request for new workflows or enhancements.
- Bank support for bundled extraction-rule contributions.

Bug reports should include clear reproduction steps, expected behavior, actual behavior, version information, and relevant logs.

## Adding Bank Support

Use the Rule editor to create or fix extraction rules. It gives you a live workbench for sample emails and exports the files needed for a contribution.

1. Create an issue using the Bank Support template.
2. Provide a redacted sample email in the issue.
3. In Expensor, open **Rules → New Rule**. If the email already appears in Diagnostics, use its fix action to open the Rule editor with the sample preloaded.
4. Fill the rule details in the editor: exact sender email addresses, subject text, source type, bank, and extraction fields.
5. Add at least one redacted sample email in the workbench and fill the expected amount, merchant, and currency.
6. Click **Save Rule** and choose **Export & Continue** when the contribution prompt appears.
7. Fork the repository, create a branch, add the exported files as described below, and submit a pull request.

`content/rules.json` is the source of truth for contributed bundled rules. The repository currently also contains `backend/cmd/server/content/rules.json` because the Go binary embeds files from that package path; keep that mirror in sync until `content/` is the only definitive rule location.

The export downloads one contribution zip file containing:

- `<rule-name>.rule.json`: copy its rule entry into the `rules` array in `content/rules.json`. If it includes a new source type or bank, copy that value into the matching `presets.source_types` or `presets.banks` list too.
- One `<bank>_<source-type>_<case>.rule.fixture` file per populated workbench sample: copy these files into `tests/data/rule-emails`.

Rule email fixtures are self-contained `.rule.fixture` files with YAML front matter and the raw email body below the closing `---`. They must not duplicate regexes from `rules.json`. The test runner automatically discovers fixtures under `tests/data/rule-emails`, loads the named rule from the real rules document, and asserts sender/subject matching plus amount, merchant, and currency extraction.

Use one email per fixture file. Fixture filenames must follow `<bank>_<source-type>_<case>.rule.fixture`, with lowercase slug segments such as `hdfc_credit-card_classic-spend.rule.fixture`. Keep fixture emails redacted but realistic enough for the exported rule to match.

```yaml
---
rule: HDFC Credit Card
sender: alerts@hdfcbank.net
subject: "Alert : Update on your HDFC Bank Credit Card"
expected:
  amount: 999.00
  merchant: SWIGGY
  currency: INR
---
Dear Customer,
Rs.999.00 spent at SWIGGY on your HDFC Credit Card on 12-Apr-2026.
```

## Getting Help

- Questions: open a GitHub discussion.
- Issues: use the matching issue template.
- Security: email maintainers directly; do not open public issues for vulnerabilities.

## License

By contributing, you agree that your contributions are licensed under the project license.
