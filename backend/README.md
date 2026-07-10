# Expensor Backend

Daemon that reads expense transactions from email sources and writes them to PostgreSQL.

## Directory Structure

```
backend/
├── cmd/
│   ├── server/              # Main daemon entry point
│   │   ├── main.go
│   │   └── content/         # Embedded config files
│   │       ├── rules.json   # Transaction extraction rules
│   │       ├── mcc.json     # MCC category seed data
│   │       └── llm/         # Backend-owned LLM prompt catalog
│   └── auth/                # Standalone OAuth flow binary
│       └── main.go
├── internal/
│   ├── daemon/              # Reader → store ingestion pipeline
│   ├── store/               # Backend-neutral store types and instrumentation
│   │   └── postgres/        # PostgreSQL persistence, read models, and migrations
│   └── plugins/             # Reader plugin catalog/registry
│       └── registry.go
└── pkg/
    ├── api/                 # Core interfaces & types (Reader, Rule, Labels)
    ├── client/              # OAuth2 HTTP client helper
    ├── config/              # TOML and environment-based configuration
    ├── extractor/           # Regex amount & merchant extraction
    ├── observability/       # slog setup plus OpenTelemetry traces/metrics
    ├── state/               # SHA-256 keyed dedup state (prevents reprocessing)
    ├── reader/
    │   ├── gmail/           # Gmail API reader
    │   └── thunderbird/     # MBOX file reader
```

## Plugin System

Email providers are registered at startup via the plugin registry. Adding a new provider requires implementing the required capabilities and registering the provider. PostgreSQL ingestion is owned by `internal/store`.

### Providers

```go
type Provider struct {
    Metadata ProviderMetadata

    NewReader func(ProviderInput) (api.Reader, error)
    NewEmailSearcher func(ProviderInput) (api.EmailSearcher, error)
}
```

**Registered providers:** `gmail`, `thunderbird`

## Adding a New Plugin

### New Provider

1. Implement the reader in `backend/pkg/reader/{name}/`
2. Add the provider metadata and constructor adapters in `backend/pkg/reader/{name}/plugin.go`
3. Register in `backend/cmd/server/bootstrap.go`:
   ```go
   registry.RegisterProvider(newreader.Provider(guideData))
   ```
4. Add any required config fields to `backend/pkg/config/config.go`

## Building

```bash
task build          # go build ./...
task build:binary   # optimised binary at ../bin/expensor
```

## Running

```bash
# Gmail + Postgres
export EXPENSOR_DB_BACKEND=postgres
export POSTGRES_HOST=localhost
export POSTGRES_DB=expensor
export POSTGRES_USER=expensor
export POSTGRES_PASSWORD=secret
task run

# Thunderbird + Postgres
export EXPENSOR_DB_BACKEND=postgres
export THUNDERBIRD_PROFILE=/home/user/.thunderbird/abc123.default
export THUNDERBIRD_MAILBOXES=INBOX,Archives
export POSTGRES_HOST=localhost
export POSTGRES_DB=expensor
export POSTGRES_USER=expensor
export POSTGRES_PASSWORD=secret
task run
```

See the root [README](../README.md) for the full environment variable reference.
