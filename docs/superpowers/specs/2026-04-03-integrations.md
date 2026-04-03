# Group F — Integrations: Design Spec

**Date:** 2026-04-03  
**Features:** Webhooks (#1), Monthly Signal reports (#2), Beancount export (#3)  
**Scope:** Backend-only for webhooks and reports; one new API endpoint + frontend button for export

---

## 1. Webhooks (#1)

### Goal
Fire an HTTP POST to registered URLs after each successful transaction batch is flushed to PostgreSQL. Consumers can use this to sync transactions to external systems (budgeting apps, spreadsheets, custom pipelines).

### Trigger point
The postgres writer (`backend/pkg/writer/postgres/postgres.go`) flushes a batch in `writeBatch`. After a successful flush, it fires any registered webhooks with the batch's transactions.

### Data Model

**Migration:** `backend/migrations/006_webhooks.sql`

```sql
CREATE TABLE IF NOT EXISTS webhooks (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    url        TEXT NOT NULL,
    secret     TEXT NOT NULL,       -- used for HMAC-SHA256 signature
    enabled    BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Webhook payload

```json
{
  "event": "transactions.created",
  "batch_id": "uuid-v4",
  "timestamp": "2025-01-15T14:32:00Z",
  "transactions": [
    {
      "id": "...",
      "amount": 1500.00,
      "currency": "INR",
      "merchant_info": "Swiggy",
      "category": "food & dining",
      "bucket": "wants",
      "timestamp": "2025-01-15T14:32:00Z",
      "labels": ["food"]
    }
  ]
}
```

### Signature
Each request includes `X-Expensor-Signature: sha256=<hex>` where the hex value is `HMAC-SHA256(secret, body)`. Consumers verify this to authenticate the delivery.

### Delivery

**File:** `backend/pkg/webhook/webhook.go` (new package)

```go
type Dispatcher struct {
    client *http.Client
    logger *slog.Logger
}

func (d *Dispatcher) Dispatch(ctx context.Context, hooks []Webhook, payload []byte) {
    for _, h := range hooks {
        go d.deliver(ctx, h, payload) // fire-and-forget per hook
    }
}

func (d *Dispatcher) deliver(ctx context.Context, hook Webhook, payload []byte) {
    sig := computeHMAC(hook.Secret, payload)
    var lastErr error
    for attempt := 1; attempt <= 3; attempt++ {
        req, _ := http.NewRequestWithContext(ctx, http.MethodPost, hook.URL, bytes.NewReader(payload))
        req.Header.Set("Content-Type", "application/json")
        req.Header.Set("X-Expensor-Signature", "sha256="+sig)
        req.Header.Set("X-Expensor-Delivery", hook.ID)
        resp, err := d.client.Do(req)
        if err == nil && resp.StatusCode < 500 {
            resp.Body.Close()
            return
        }
        if resp != nil {
            resp.Body.Close()
        }
        lastErr = err
        time.Sleep(time.Duration(attempt*attempt) * time.Second) // 1s, 4s, 9s
    }
    d.logger.Warn("webhook delivery failed after 3 attempts",
        "url", hook.URL, "error", lastErr)
}
```

- Delivery is fire-and-forget (goroutine per hook). Failed deliveries are logged but do not block or retry indefinitely.
- Webhook timeout: 10 seconds per attempt.
- A 4xx response (except 429) is treated as a permanent failure for that delivery — no retry.

### Integration with postgres writer

The postgres `Writer` is injected with a `*webhook.Dispatcher` (optional — nil means no-op). After `writeBatch` succeeds:

```go
if w.webhookDispatcher != nil {
    payload := buildWebhookPayload(transactions)
    go w.webhookDispatcher.Dispatch(context.Background(), w.webhookHooks, payload)
}
```

Webhooks are loaded from the DB at daemon startup and injected into the writer. They are not hot-reloaded during a run (restart to pick up new webhooks — same pattern as rules).

### API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/webhooks` | List all webhooks (secret masked as `****`) |
| POST | `/api/webhooks` | Register new webhook `{url, secret}` |
| DELETE | `/api/webhooks/{id}` | Remove webhook |
| PUT | `/api/webhooks/{id}/toggle` | Enable / disable |

### Frontend

**File:** `frontend/src/pages/settings/WebhooksSettings.tsx` (new — referenced in Group C spec)

Table with columns: URL, Status (enabled/disabled toggle), Created, Delete action. "+ Add webhook" button opens a form for URL and secret.

---

## 2. Monthly Signal Reports (#2)

### Goal
On the 1st of each month, generate a spending summary for the previous month and deliver it via Signal. The notification channel is abstracted behind an interface so Discord, WhatsApp, etc. can be added later with no changes to the report logic.

### Notification channel interface

**File:** `backend/pkg/notify/notify.go` (new)

```go
// Channel delivers a plain-text message to a notification endpoint.
type Channel interface {
    Name() string
    Send(ctx context.Context, subject, body string) error
}
```

### Signal implementation

**File:** `backend/pkg/notify/signal/signal.go`

Uses the [signal-cli REST API](https://github.com/bbernhard/signal-cli-rest-api) — a self-hosted sidecar that exposes a simple REST interface over signal-cli.

```go
type SignalChannel struct {
    apiURL    string // e.g. http://localhost:8080
    recipient string // phone number or group ID
    client    *http.Client
}

func (c *SignalChannel) Send(ctx context.Context, _, body string) error {
    payload := map[string]any{
        "message":    body,
        "recipients": []string{c.recipient},
    }
    // POST /v2/send
}
```

**Config env vars:**
- `SIGNAL_API_URL` — URL of the signal-cli REST API sidecar
- `SIGNAL_RECIPIENT` — recipient phone number (E.164 format, e.g. `+919876543210`)

If either env var is unset, the Signal channel is not registered and monthly reports are skipped (with a startup log warning).

### Report content

**File:** `backend/pkg/report/monthly.go` (new)

```
Monthly Expense Report — December 2024

Total spent: ₹45,230

Top categories:
  1. Food & Dining    ₹12,400  (27%)
  2. Transport        ₹8,200   (18%)
  3. Shopping         ₹7,100   (16%)

Top merchants:
  1. Swiggy           ₹4,200
  2. Ola              ₹3,100
  3. Amazon           ₹2,800

vs. November: ▲ 12% (₹40,360)
```

Plain text format — no markdown, no HTML. Readable in Signal's monospace block.

### Scheduler

**File:** `backend/internal/scheduler/scheduler.go` (new)

A lightweight ticker that fires once a day and checks if it's the 1st of the month:

```go
func Start(ctx context.Context, tasks []DailyTask, logger *slog.Logger) {
    ticker := time.NewTicker(1 * time.Hour)
    defer ticker.Stop()
    var lastRun time.Time
    for {
        select {
        case <-ctx.Done():
            return
        case t := <-ticker.C:
            if t.Day() == 1 && t.Month() != lastRun.Month() {
                for _, task := range tasks {
                    go task.Run(ctx)
                }
                lastRun = t
            }
        }
    }
}
```

The monthly report task is registered as a `DailyTask`. The scheduler is started in `main.go` alongside the HTTP server, sharing the root context.

### Store additions

```go
GetMonthlyReport(ctx context.Context, year int, month time.Month) (*MonthlyReport, error)
```

Runs three queries: total spend for the month, top 5 categories, top 5 merchants. Returns a `MonthlyReport` struct that `report.Generate` formats into the text message.

---

## 3. Beancount Export (#3)

### Goal
Allow the user to export their transactions as a `.beancount` file for use with the [Beancount](https://beancount.github.io/) plaintext accounting tool.

### Beancount format

Each transaction becomes a Beancount directive:

```
2024-01-15 * "Swiggy" "Food delivery"
  Expenses:FoodAndDining    1200.00 INR
  Assets:Checking          -1200.00 INR
```

**Category → account mapping:**
The transaction's `category` field is mapped to a Beancount account name:

```go
var categoryToAccount = map[string]string{
    "food & dining":  "Expenses:FoodAndDining",
    "transport":      "Expenses:Transport",
    "shopping":       "Expenses:Shopping",
    "utilities":      "Expenses:Utilities",
    "healthcare":     "Expenses:Healthcare",
    "entertainment":  "Expenses:Entertainment",
    "travel":         "Expenses:Travel",
    "finance":        "Expenses:Finance",
    "":               "Expenses:Uncategorized",
}

func categoryAccount(category string) string {
    if acc, ok := categoryToAccount[strings.ToLower(category)]; ok {
        return acc
    }
    // Unknown category: sanitise and use as-is
    sanitised := strings.ReplaceAll(strings.Title(category), " ", "")
    return "Expenses:" + sanitised
}
```

The `Assets:Checking` contra-account is hardcoded — beancount requires a balancing leg but Expensor does not track accounts. Users are expected to adjust this in their beancount config.

**File header:**
```
; Exported from Expensor
; Generated: 2025-01-15T14:32:00Z
; Period: 2024-01-01 – 2024-12-31

option "operating_currency" "INR"
```

### API endpoint

**File:** `backend/internal/api/handlers.go`

```go
// HandleExportBeancount handles GET /api/export/beancount
// Query params: from (RFC3339), to (RFC3339) — both optional
func (h *Handlers) HandleExportBeancount(w http.ResponseWriter, r *http.Request) {
    // Parse optional from/to
    // Fetch all transactions in range (no pagination — full export)
    // Render beancount text
    // Set Content-Disposition: attachment; filename="expensor-2024.beancount"
    // Write response
}
```

Route: `GET /api/export/beancount?from=2024-01-01T00:00:00Z&to=2024-12-31T23:59:59Z`

**File:** `backend/pkg/beancount/beancount.go` (new)

Contains `Render(transactions []store.Transaction, from, to *time.Time) string` — pure function, easily unit-testable.

### Store addition

```go
ListAllTransactions(ctx context.Context, from, to *time.Time) ([]Transaction, error)
```

Fetches all transactions in the date range without pagination (for export purposes). Uses the same WHERE clause builder as `ListTransactions` but without LIMIT/OFFSET.

### Frontend

**File:** `frontend/src/pages/Transactions.tsx`

Add an "Export" button in the Transactions page header (alongside the existing filters). Clicking opens a small popover with:
- Format: `Beancount` (only option for now — designed to add CSV, OFXM etc. later)
- Date range: uses the active filter range, or "All time" if no filter is set
- "Download" button → triggers `GET /api/export/beancount?from=...&to=...` as a file download

```ts
function downloadBeancount(from?: Date, to?: Date) {
  const params = new URLSearchParams()
  if (from) params.set('from', from.toISOString())
  if (to)   params.set('to',   to.toISOString())
  window.location.href = `/api/export/beancount?${params}`
}
```

---

## Files Created / Modified

| File | Change |
|------|--------|
| `backend/migrations/006_webhooks.sql` | New |
| `backend/pkg/webhook/webhook.go` | New — Dispatcher, HMAC signing, retry |
| `backend/pkg/notify/notify.go` | New — Channel interface |
| `backend/pkg/notify/signal/signal.go` | New — Signal implementation |
| `backend/pkg/report/monthly.go` | New — report generation |
| `backend/pkg/beancount/beancount.go` | New — beancount renderer |
| `backend/internal/scheduler/scheduler.go` | New — daily task scheduler |
| `backend/internal/store/store.go` | Add heatmap, webhook, monthly report, export queries |
| `backend/internal/api/store.go` | Extend Storer interface |
| `backend/internal/api/handlers.go` | Add webhook CRUD + beancount export handlers |
| `backend/internal/api/server.go` | Register new routes |
| `backend/cmd/server/main.go` | Wire up scheduler + Signal channel + webhook dispatcher |
| `frontend/src/pages/settings/WebhooksSettings.tsx` | New |
| `frontend/src/pages/Transactions.tsx` | Add Export button |
| `frontend/src/api/queries.ts` | Add webhook query/mutation hooks |
| `frontend/src/api/types.ts` | Add Webhook type |
