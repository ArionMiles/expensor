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
│   │       └── labels.json  # Merchant categorization labels
│   └── auth/                # Standalone OAuth flow binary
│       └── main.go
├── internal/
│   ├── daemon/              # Reader → store ingestion pipeline
│   ├── store/               # PostgreSQL persistence and read models
│   │   └── runner.go
│   └── plugins/             # Reader plugin catalog/registry
│       └── registry.go
├── migrations/              # SQL migrations (run on startup)
└── pkg/
    ├── api/                 # Core interfaces & types (Reader, Rule, Labels)
    ├── client/              # OAuth2 HTTP client helper
    ├── config/              # Environment-based configuration
    ├── extractor/           # Regex amount & merchant extraction
    ├── observability/       # slog setup plus OpenTelemetry traces/metrics
    ├── state/               # SHA-256 keyed dedup state (prevents reprocessing)
    ├── reader/
    │   ├── gmail/           # Gmail API reader
    │   └── thunderbird/     # MBOX file reader
```

## Plugin System

Readers are registered at startup via the plugin registry. Adding a new source requires implementing the reader interface and registering the plugin. PostgreSQL ingestion is owned by `internal/store`.

### Reader Plugins

```go
type ReaderPlugin interface {
    Metadata() ReaderMetadata
    NewReader(input ReaderInput) (api.Reader, error)
}
```

**Registered readers:** `gmail`, `thunderbird`

## Adding a New Plugin

### New Reader

1. Implement the reader in `backend/pkg/reader/{name}/`
2. Add the plugin metadata and constructor adapter in `backend/pkg/reader/{name}/plugin.go`
3. Register in `backend/cmd/server/main.go`:
   ```go
   registry.RegisterReader(&newreader.Plugin{})
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
export POSTGRES_HOST=localhost
export POSTGRES_DB=expensor
export POSTGRES_USER=expensor
export POSTGRES_PASSWORD=secret
task run

# Thunderbird + Postgres
export THUNDERBIRD_PROFILE=/home/user/.thunderbird/abc123.default
export THUNDERBIRD_MAILBOXES=INBOX,Archives
export POSTGRES_HOST=localhost
export POSTGRES_DB=expensor
export POSTGRES_USER=expensor
export POSTGRES_PASSWORD=secret
task run
```

See the root [README](../README.md) for the full environment variable reference.
