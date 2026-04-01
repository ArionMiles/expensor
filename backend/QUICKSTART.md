# Expensor Backend Quick Start

Get up and running with the Expensor backend in 5 minutes.

## Prerequisites

- Go 1.25.5 or higher
- Google OAuth credentials (`data/client_secret.json`)
- OAuth token (`data/token.json`) - obtain via web interface

## Quick Build

```bash
# From project root
go build -C backend/cmd/server -o backend/bin/expensor-server
```

## Quick Run (Legacy Config)

```bash
# Set required environment variables
export GSHEETS_NAME="January"
export GSHEETS_TITLE="Expenses 2025"

# Run the server
./backend/bin/expensor-server
```

## Quick Run (Plugin Config)

### Gmail → Google Sheets

```bash
export EXPENSOR_READER="gmail"
export EXPENSOR_WRITER="sheets"
export EXPENSOR_READER_CONFIG='{
  "rules": [
    {
      "name": "Example Rule",
      "query": "from:bank@example.com subject:transaction",
      "amountRegex": "\\$([\\d,]+\\.\\d{2})",
      "merchantInfoRegex": "at ([^.]+)\\.",
      "enabled": true,
      "source": "Bank"
    }
  ],
  "labels": {
    "AMAZON": {"category": "Shopping", "bucket": "Want"}
  },
  "interval": 10
}'
export EXPENSOR_WRITER_CONFIG='{
  "sheetName": "January",
  "sheetTitle": "Expenses 2025"
}'
./backend/bin/expensor-server
```

### Gmail → CSV File

```bash
export EXPENSOR_READER="gmail"
export EXPENSOR_WRITER="csv"
export EXPENSOR_READER_CONFIG='...'  # Same as above
export EXPENSOR_WRITER_CONFIG='{"filePath": "data/expenses.csv"}'
./backend/bin/expensor-server
```

### Gmail → JSON File

```bash
export EXPENSOR_READER="gmail"
export EXPENSOR_WRITER="json"
export EXPENSOR_READER_CONFIG='...'  # Same as above
export EXPENSOR_WRITER_CONFIG='{"filePath": "data/expenses.json"}'
./backend/bin/expensor-server
```

## Development

### Project Structure

```
backend/
├── cmd/server/          # Application entry point
├── internal/            # Private packages
│   ├── daemon/         # Core daemon logic
│   └── plugins/        # Plugin registry
└── pkg/                # Public packages
    ├── api/            # Interfaces
    ├── reader/         # Reader implementations
    ├── writer/         # Writer implementations
    └── plugins/        # Plugin wrappers
```

### Adding a New Reader Plugin

1. Create implementation in `backend/pkg/reader/myreader/`
2. Create plugin wrapper in `backend/pkg/plugins/readers/myreader/plugin.go`:

```go
package myreader

import (
    "encoding/json"
    "log/slog"
    "net/http"

    "github.com/ArionMiles/expensor/backend/pkg/api"
    myreaderimpl "github.com/ArionMiles/expensor/backend/pkg/reader/myreader"
)

type Plugin struct{}

func (p *Plugin) Name() string { return "myreader" }

func (p *Plugin) Description() string {
    return "My custom reader"
}

func (p *Plugin) RequiredScopes() []string {
    return []string{"https://www.googleapis.com/auth/scope"}
}

func (p *Plugin) ConfigSchema() map[string]any {
    return map[string]any{
        "type": "object",
        "properties": map[string]any{
            "setting": map[string]any{
                "type": "string",
                "description": "A config setting",
            },
        },
    }
}

type Config struct {
    Setting string `json:"setting"`
}

func (p *Plugin) NewReader(httpClient *http.Client, configData json.RawMessage, logger *slog.Logger) (api.Reader, error) {
    var cfg Config
    if err := json.Unmarshal(configData, &cfg); err != nil {
        return nil, err
    }

    return myreaderimpl.New(httpClient, cfg, logger)
}
```

3. Register in `backend/cmd/server/main.go`:

```go
import myreaderplugin "github.com/ArionMiles/expensor/backend/pkg/plugins/readers/myreader"

// In main():
registry.RegisterReader(&myreaderplugin.Plugin{})
```

### Adding a New Writer Plugin

Follow same pattern as reader, but implement `WriterPlugin` interface.

### Running Tests

```bash
go test -C backend ./...
```

### Linting

```bash
golangci-lint run backend/...
```

## Environment Variables Reference

### Plugin Configuration

| Variable | Description | Example |
|----------|-------------|---------|
| `EXPENSOR_READER` | Reader plugin name | `gmail` |
| `EXPENSOR_WRITER` | Writer plugin name | `sheets`, `csv`, `json` |
| `EXPENSOR_READER_CONFIG` | JSON config for reader | `{"rules": [...]}` |
| `EXPENSOR_WRITER_CONFIG` | JSON config for writer | `{"filePath": "..."}` |

### Legacy Configuration (Still Supported)

| Variable | Description | Example |
|----------|-------------|---------|
| `GSHEETS_TITLE` | Sheet title (for new sheets) | `Expenses 2025` |
| `GSHEETS_ID` | Existing sheet ID | `1abc123...` |
| `GSHEETS_NAME` | Sheet tab name | `January` |

## OAuth Setup

1. Get Google OAuth credentials from [Google Cloud Console](https://console.cloud.google.com)
2. Save to `data/client_secret.json`
3. Obtain OAuth token via web interface (Phase 4)
4. Token saved to `data/token.json`

For now, use the old CLI tool to get a token:

```bash
# From project root (original CLI still works)
go run ./cmd/expensor setup
```

## Stopping the Server

Press `Ctrl+C` or send `SIGTERM`:

```bash
kill -TERM $(pidof expensor-server)
```

The server will:
- Stop reading new emails
- Flush buffered transactions
- Close connections gracefully
- Exit with code 0

## Logs

The server uses structured logging (slog):

```
2026-01-11T01:49:00.000Z INFO plugins registered readers=1 writers=3
2026-01-11T01:49:00.001Z INFO configuration loaded reader=gmail writer=sheets
2026-01-11T01:49:00.002Z INFO OAuth scopes required scopes=[gmail.readonly gmail.modify spreadsheets]
2026-01-11T01:49:00.100Z INFO daemon started
2026-01-11T01:49:00.200Z INFO gmail reader starting rule_count=5
```

## Troubleshooting

### "loading token: open data/token.json: no such file or directory"

**Solution:** Run OAuth setup first (use old CLI or wait for web interface):
```bash
go run ./cmd/expensor setup
```

### "reader plugin "xyz" not found"

**Solution:** Check plugin name spelling and registration in `main.go`.

### "unmarshaling gmail config: invalid character"

**Solution:** Verify JSON syntax in `EXPENSOR_READER_CONFIG`:
```bash
echo $EXPENSOR_READER_CONFIG | jq .
```

### "GSHEETS_NAME is required"

**Solution:** Either set legacy vars OR use plugin config:
```bash
# Option 1: Legacy
export GSHEETS_NAME="January"

# Option 2: Plugin config
export EXPENSOR_WRITER_CONFIG='{"sheetName": "January", ...}'
```

## Performance

- Transaction buffer: 100 items
- Batch writes: 10 transactions/batch (configurable)
- Flush interval: 30 seconds (configurable)
- Gmail poll interval: 10 seconds (configurable)

## Next Steps

- [ ] Phase 3: Build web UI
- [ ] Phase 4: Implement REST API
- [ ] Phase 5: Deploy with Docker

## Support

See full documentation:
- `backend/README.md` - Architecture guide
- `PHASE2_MIGRATION.md` - Migration details
- `PHASE2_SUMMARY.md` - Complete feature list
