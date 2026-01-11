# Expensor Backend

This is the restructured backend for Expensor, featuring a plugin-based architecture for readers and writers.

## Directory Structure

```
backend/
├── cmd/server/              # Server entry point
│   ├── main.go             # Main application with plugin registration
│   └── config/             # Embedded configuration files
│       ├── rules.json      # Transaction extraction rules
│       └── labels.json     # Merchant categorization labels
├── internal/               # Private application code
│   ├── daemon/            # Daemon runner
│   │   └── runner.go      # Core daemon logic with plugin support
│   └── plugins/           # Plugin registry
│       └── registry.go    # Plugin registration and factory
├── pkg/                   # Public packages (can be imported)
│   ├── api/              # Core interfaces and data structures
│   ├── client/           # OAuth2 client (web-only)
│   ├── config/           # Configuration structures
│   ├── logging/          # Logging setup
│   ├── reader/           # Reader implementations
│   │   └── gmail/        # Gmail reader
│   ├── writer/           # Writer implementations
│   │   ├── buffered/     # Buffered writer base
│   │   ├── sheets/       # Google Sheets writer
│   │   ├── csv/          # CSV file writer
│   │   └── json/         # JSON file writer
│   └── plugins/          # Plugin wrappers
│       ├── readers/
│       │   └── gmail/    # Gmail reader plugin
│       └── writers/
│           ├── sheets/   # Sheets writer plugin
│           ├── csv/      # CSV writer plugin
│           └── json/     # JSON writer plugin
└── bin/                  # Compiled binaries
    └── expensor-server   # Server binary
```

## Plugin System

The plugin system allows for extensible readers and writers without hardcoding implementations.

### Reader Plugins

Reader plugins implement the `ReaderPlugin` interface:

```go
type ReaderPlugin interface {
    Name() string
    Description() string
    RequiredScopes() []string
    ConfigSchema() map[string]any
    NewReader(httpClient *http.Client, config json.RawMessage, logger *slog.Logger) (api.Reader, error)
}
```

**Registered Readers:**
- `gmail` - Read transactions from Gmail messages

### Writer Plugins

Writer plugins implement the `WriterPlugin` interface:

```go
type WriterPlugin interface {
    Name() string
    Description() string
    RequiredScopes() []string
    ConfigSchema() map[string]any
    NewWriter(httpClient *http.Client, config json.RawMessage, logger *slog.Logger) (api.Writer, error)
}
```

**Registered Writers:**
- `sheets` - Write to Google Sheets (requires OAuth)
- `csv` - Write to CSV file (local file)
- `json` - Write to JSON file (local file)

## Configuration

Configuration is now plugin-based via environment variables:

### Required Environment Variables

- `EXPENSOR_READER` - Reader plugin name (e.g., "gmail")
- `EXPENSOR_WRITER` - Writer plugin name (e.g., "sheets", "csv", "json")
- `EXPENSOR_READER_CONFIG` - JSON configuration for reader plugin
- `EXPENSOR_WRITER_CONFIG` - JSON configuration for writer plugin

### Legacy Support (Temporary)

For backward compatibility, the following environment variables are still supported:

- `GSHEETS_TITLE` - Google Sheet title (for new sheets)
- `GSHEETS_ID` - Existing Google Sheet ID
- `GSHEETS_NAME` - Sheet tab name (required)

If plugin config vars are not set, the app will build default configs from legacy vars and embedded files.

## Gmail Reader Configuration

```json
{
  "rules": [
    {
      "name": "ICICI Credit Card",
      "query": "from:credit-cards@icicibank.com subject:Alert",
      "amountRegex": "Rs\\. ?([\\d,]+\\.\\d{2})",
      "merchantInfoRegex": "at ([^.]+)\\.",
      "enabled": true,
      "source": "ICICI CC"
    }
  ],
  "labels": {
    "SWIGGY": {
      "category": "Food",
      "bucket": "Need"
    }
  },
  "interval": 10
}
```

## Sheets Writer Configuration

```json
{
  "sheetTitle": "Expenses 2025",
  "sheetId": "1abc123...",
  "sheetName": "January",
  "batchSize": 10,
  "flushInterval": 30
}
```

## CSV Writer Configuration

```json
{
  "filePath": "data/expenses.csv",
  "batchSize": 10,
  "flushInterval": 30
}
```

## JSON Writer Configuration

```json
{
  "filePath": "data/expenses.json",
  "batchSize": 10,
  "flushInterval": 30
}
```

## Building

```bash
go build -C backend/cmd/server -o backend/bin/expensor-server
```

## Running

```bash
# Using Sheets writer (legacy env vars)
export GSHEETS_NAME="January"
export GSHEETS_TITLE="Expenses 2025"
./backend/bin/expensor-server

# Using CSV writer
export EXPENSOR_READER="gmail"
export EXPENSOR_WRITER="csv"
export EXPENSOR_READER_CONFIG='{"rules": [...], "labels": {...}}'
export EXPENSOR_WRITER_CONFIG='{"filePath": "data/expenses.csv"}'
./backend/bin/expensor-server

# Using JSON writer
export EXPENSOR_READER="gmail"
export EXPENSOR_WRITER="json"
export EXPENSOR_READER_CONFIG='{"rules": [...], "labels": {...}}'
export EXPENSOR_WRITER_CONFIG='{"filePath": "data/expenses.json"}'
./backend/bin/expensor-server
```

## OAuth Authentication

OAuth tokens must be managed by the web application. The CLI-based OAuth callback has been removed.

Token file location: `data/token.json`

## Key Changes from Original

1. **Plugin-based architecture**: Readers and writers are now plugins registered at startup
2. **Removed CLI commands**: `setup` and `status` commands deleted (web-only interface)
3. **Removed CLI OAuth**: Local callback server removed (web app will handle OAuth)
4. **Simplified config**: Plugin-based configuration via environment variables
5. **Daemon runner**: Core run logic moved to `internal/daemon/runner.go` with context support
6. **Import paths**: All imports updated to `github.com/ArionMiles/expensor/backend/pkg/*`

## Adding New Plugins

### Adding a New Reader

1. Create implementation in `backend/pkg/reader/{name}/`
2. Create plugin wrapper in `backend/pkg/plugins/readers/{name}/plugin.go`
3. Implement `ReaderPlugin` interface
4. Register in `cmd/server/main.go`:
   ```go
   registry.RegisterReader(&newreaderplugin.Plugin{})
   ```

### Adding a New Writer

1. Create implementation in `backend/pkg/writer/{name}/`
2. Create plugin wrapper in `backend/pkg/plugins/writers/{name}/plugin.go`
3. Implement `WriterPlugin` interface
4. Register in `cmd/server/main.go`:
   ```go
   registry.RegisterWriter(&newwriterplugin.Plugin{})
   ```

## Next Steps (Phase 3 & 4)

- Phase 3: Web UI (React/Svelte)
- Phase 4: REST API for OAuth, config management, and daemon control
- Phase 5: Deployment with Docker
