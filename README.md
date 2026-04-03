# expensor

> [!IMPORTANT]
> This project is built with AI-assisted tooling.

Expensor reads expense-related emails from your inbox, extracts transaction details, and writes them to PostgreSQL. It ships with a web UI for setup, monitoring, and transaction management.

I've documented why exactly expensor works for me [on my blog](https://kanishk.io/posts/expensor/). The rules are dead-simple regex extractions which are fast and can be updated easily.

## How does it work?

1. Open the web UI and complete the onboarding wizard (select reader, upload credentials, authenticate)
2. Start the daemon — it periodically polls the configured inbox (Gmail or Thunderbird)
3. Match emails against [rules](backend/cmd/server/content/rules.json) by sender and/or subject
4. Extract transaction details — amount, merchant name, date — via regex
5. Write them to PostgreSQL
6. Browse, filter, search, and label transactions in the Transactions view

## Architecture

```
readers (gmail | thunderbird)
        │
        ▼
  daemon runner
        │
        ▼
  postgres writer
        │
        ▼
  PostgreSQL DB  ──▶  REST API  ──▶  Web UI (React)
```

## Repository Structure

```
.
├── backend/
│   ├── cmd/server/          # Main server binary
│   │   └── content/         # Embedded rules.json & labels.json
│   ├── internal/
│   │   ├── api/             # HTTP handlers, routing, middleware
│   │   ├── daemon/          # Reader → writer pipeline
│   │   ├── plugins/         # Plugin registry
│   │   └── store/           # PostgreSQL query layer
│   ├── migrations/          # SQL migrations (run automatically on startup)
│   └── pkg/
│       ├── api/             # Core interfaces & types (Reader, Writer, Rule)
│       ├── config/          # Environment-based configuration
│       ├── extractor/       # Amount & merchant regex extraction
│       ├── state/           # SHA-256 keyed dedup state
│       ├── reader/
│       │   ├── gmail/       # Gmail API reader
│       │   └── thunderbird/ # MBOX file reader
│       ├── writer/
│       │   └── postgres/    # PostgreSQL writer (batched)
│       └── plugins/         # Plugin wrappers for readers & writers
├── frontend/                # React + Vite + Tailwind web UI
├── tests/                   # Integration test helpers & local docker-compose
├── deployment/              # Docker Compose files per reader+writer combo
├── docker-compose.yml       # Default compose (gmail + postgres)
└── Taskfile.yml             # Build & dev automation
```

## Quick Start

### With Docker Compose

Pre-built compose files live in [`deployment/`](deployment/):

```bash
# Gmail + Postgres
docker compose -f deployment/docker-compose.gmail-postgres.yml up -d

# Thunderbird + Postgres
docker compose -f deployment/docker-compose.thunderbird-postgres.yml up -d
```

Once up, open **http://localhost:8080** and follow the onboarding wizard.

### From Source

Requires Go 1.25+ and Node 20+:

```bash
# Start postgres
task db:start

# Start everything (backend + frontend dev server)
task dev
```

Then open **http://localhost:5173**.

## Configuration

All configuration is via environment variables. Reader and writer selection is handled through the web UI, not env vars.

### Core

| Variable | Default | Description |
|----------|---------|-------------|
| `EXPENSOR_DATA_DIR` | `data` | Directory for credentials, tokens, and state files |
| `EXPENSOR_STATE_FILE` | `data/state.json` | Path to dedup state file |
| `EXPENSOR_BASE_CURRENCY` | `INR` | Currency used for aggregate stats |

### Server

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `BASE_URL` | `http://localhost:8080` | Public base URL (used for OAuth redirect) |
| `FRONTEND_URL` | `http://localhost:5173` | Frontend URL (used for post-auth redirects) |

### Gmail reader

| Variable | Default | Description |
|----------|---------|-------------|
| `GMAIL_INTERVAL` | `60` | Polling interval (seconds) |
| `GMAIL_LOOKBACK_DAYS` | `180` | How far back to search for emails |

OAuth credentials are uploaded through the web UI (`/setup`), not via env vars.

### Thunderbird reader

| Variable | Default | Description |
|----------|---------|-------------|
| `THUNDERBIRD_PROFILE` | — | Path to Thunderbird profile directory |
| `THUNDERBIRD_MAILBOXES` | — | Comma-separated mailbox names, e.g. `INBOX,Archives` |
| `THUNDERBIRD_INTERVAL` | `60` | Polling interval (seconds) |

### PostgreSQL

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTGRES_HOST` | — | **Required.** Database host |
| `POSTGRES_DB` | — | **Required.** Database name |
| `POSTGRES_USER` | — | **Required.** Database user |
| `POSTGRES_PASSWORD` | — | Database password |
| `POSTGRES_PORT` | `5432` | Database port |
| `POSTGRES_SSLMODE` | `disable` | SSL mode |
| `POSTGRES_BATCH_SIZE` | `10` | Rows to buffer before flushing |
| `POSTGRES_FLUSH_INTERVAL` | `30` | Max seconds between flushes |
| `POSTGRES_MAX_POOL_SIZE` | `10` | Connection pool size |

### Logging

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_LEVEL` | `INFO` | `DEBUG`, `INFO`, `WARN`, or `ERROR` |
| `LOG_JSON` | `false` | Emit structured JSON logs |

## Development

This project uses [Task](https://taskfile.dev) for automation.

```bash
task dev            # Start postgres + backend + frontend (full stack)
task run            # Backend only (loads tests/.env)
task run:frontend   # Frontend Vite dev server only

task fmt            # Format code (gci + gofumpt)
task lint           # Lint with local config
task lint:prod      # Lint with strict CI config
task test           # Run tests (unit; integration tests require Docker)
task test:cover     # Run tests with coverage report
task build:binary   # Build optimized binary → bin/expensor
task build:docker   # Build Docker image locally
task vulncheck      # Run govulncheck vulnerability scanner
task ci             # Run lint:prod + test (matches CI pipeline)

task db:start       # Start local dev postgres container
task db:stop        # Stop local dev postgres container
```

Integration tests (in `backend/internal/store/` and `backend/pkg/writer/postgres/`) spin up a real Postgres container via testcontainers. Run them with:

```bash
go test ./backend/internal/store/... ./backend/pkg/writer/postgres/...
```

Skip them in short mode: `go test -short ./...`

## Adding Rules

Rules live in [`backend/cmd/server/content/rules.json`](backend/cmd/server/content/rules.json). Each rule specifies a sender email, subject fragment, and regex patterns to extract amount and merchant:

```json
{
  "name": "ICICI Credit Card",
  "senderEmail": "credit-cards@icicibank.com",
  "subjectContains": "Alert",
  "amountRegex": "Rs\\.\\s?([\\d,]+\\.\\d{2})",
  "merchantInfoRegex": "at ([^.]+)\\.",
  "enabled": true
}
```

Both `senderEmail` and `subjectContains` can be specified — a rule matches only when **both** conditions are met. Either field can be omitted to match any sender or subject.

### Amount regex patterns

```
Rs\.\s*([\d,]+\.?\d*)       → Rs. 1,234.56
INR\s*([\d,]+\.?\d*)        → INR 500.00
₹\s*([\d,]+\.?\d*)          → ₹ 2,500
\$\s*([\d,]+\.?\d*)         → $ 99.99
```

### Merchant regex patterns

```
at\s+([A-Z0-9\s]+)          → at AMAZON INDIA
on\s+([A-Z\s]+)             → on SWIGGY
for\s+([A-Z\s]+)            → for UBER TRIP
to\s+([A-Z\s]+)             → to NETFLIX COM
```

Each pattern must have exactly one capture group. Test patterns at [regex101.com](https://regex101.com) with Go flavour selected.

## Expensor doesn't recognise transactions from my bank

Open an issue with the email body content and I'll take a look.
