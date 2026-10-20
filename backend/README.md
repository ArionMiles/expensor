# Expensor Backend

Server that reads expense transactions from email sources and writes them to SQLite or PostgreSQL.

## Directory Structure

```
backend/
├── cmd/
│   ├── server/              # Process-only daemon entry point
│   │   └── main.go
│   └── auth/                # Standalone OAuth flow binary
│       └── main.go
├── internal/
│   ├── app/                 # Application composition and lifecycle
│   ├── catalog/             # Validated embedded rules, taxonomy, guides, and prompts
│   ├── community/           # Community content synchronization
│   ├── daemon/              # Reader → store ingestion pipeline and scan control
│   ├── httpapi/             # HTTP transport and consumer-owned control interfaces
│   ├── store/               # Backend-neutral store types and instrumentation
│   │   ├── postgres/        # PostgreSQL persistence, read models, and migrations
│   │   └── sqlite/          # SQLite persistence, read models, and migrations
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

Email providers are registered at startup through the plugin registry. Adding a provider requires the supported capabilities and registry entry. Database ingestion is owned by `internal/store`.

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
3. Register in `backend/internal/app/readers.go`:
   ```go
   registry.RegisterProvider(newreader.Provider(guideData))
   ```
4. Add any required config fields to `backend/pkg/config/config.go`

## Building

```bash
task build          # go build ./...
task build:binary   # Optimized host binary with the embedded UI
task package:release:docker # Four native archives and checksums
```

## Running

```bash
# Local backend using tests/config.dev.toml and the Postgres dev container
task run

# Full local app stack
task dev
```

Local tasks use PostgreSQL explicitly to keep development behavior stable. Production starts with SQLite when no backend is set. The local tasks load `tests/config.dev.toml` through `EXPENSOR_CONFIG_FILE`. Override values when required, for example `task run DB_BACKEND=postgres`.
See the root [README](../README.md) for the full configuration reference.
