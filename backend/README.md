# Expensor Backend

Plugin-based daemon that reads expense transactions from email sources and writes them to PostgreSQL.

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
│   ├── daemon/              # Reader → writer pipeline
│   │   └── runner.go
│   └── plugins/             # Plugin registry & factory
│       └── registry.go
├── migrations/              # SQL migrations (run on startup)
└── pkg/
    ├── api/                 # Core interfaces & types (Reader, Writer, Rule, Labels)
    ├── client/              # OAuth2 HTTP client helper
    ├── config/              # Environment-based configuration
    ├── extractor/           # Regex amount & merchant extraction
    ├── logging/             # Structured logging setup
    ├── state/               # SHA-256 keyed dedup state (prevents reprocessing)
    ├── reader/
    │   ├── gmail/           # Gmail API reader
    │   └── thunderbird/     # MBOX file reader
    ├── writer/
    │   └── postgres/        # PostgreSQL writer (batched inserts)
    └── plugins/             # Thin plugin wrappers (config wiring)
        ├── readers/
        │   ├── gmail/
        │   └── thunderbird/
        └── writers/
            └── postgres/
```

## Plugin System

Readers and writers are registered at startup via the plugin registry. Adding a new source only requires implementing the relevant interface and registering the plugin.

### Reader Plugins

```go
type ReaderPlugin interface {
    Name() string
    Description() string
    RequiredScopes() []string
    NewReader(httpClient *http.Client, cfg *config.Config, rules []api.Rule,
              labels api.Labels, stateManager *state.Manager, logger *slog.Logger) (api.Reader, error)
}
```

**Registered readers:** `gmail`, `thunderbird`

### Writer Plugins

```go
type WriterPlugin interface {
    Name() string
    Description() string
    RequiredScopes() []string
    NewWriter(httpClient *http.Client, cfg *config.Config, logger *slog.Logger) (api.Writer, error)
}
```

**Registered writers:** `postgres`

## Adding a New Plugin

### New Reader

1. Implement the reader in `backend/pkg/reader/{name}/`
2. Create the plugin wrapper in `backend/pkg/plugins/readers/{name}/plugin.go`
3. Register in `backend/cmd/server/main.go`:
   ```go
   registry.RegisterReader(&newreaderplugin.Plugin{})
   ```
4. Add any required config fields to `backend/pkg/config/config.go`

### New Writer

1. Implement the writer in `backend/pkg/writer/{name}/`
2. Create the plugin wrapper in `backend/pkg/plugins/writers/{name}/plugin.go`
3. Register in `backend/cmd/server/main.go`:
   ```go
   registry.RegisterWriter(&newwriterplugin.Plugin{})
   ```

## Building

```bash
task build          # go build ./...
task build:binary   # optimised binary at ../bin/expensor
```

## Running

```bash
# Gmail + Postgres
export EXPENSOR_READER=gmail
export EXPENSOR_WRITER=postgres
export POSTGRES_HOST=localhost
export POSTGRES_DB=expensor
export POSTGRES_USER=expensor
export POSTGRES_PASSWORD=secret
task run

# Thunderbird + Postgres
export EXPENSOR_READER=thunderbird
export EXPENSOR_WRITER=postgres
export THUNDERBIRD_PROFILE=/home/user/.thunderbird/abc123.default
export THUNDERBIRD_MAILBOXES=INBOX,Archives
export POSTGRES_HOST=localhost
export POSTGRES_DB=expensor
export POSTGRES_USER=expensor
export POSTGRES_PASSWORD=secret
task run
```

See the root [README](../README.md) for the full environment variable reference.
