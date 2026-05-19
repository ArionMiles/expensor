# Monthly Signal Reports — Design Spec

**Date:** 2026-05-19
**Status:** Proposed — no implementation plan yet
**Split from:** `2026-05-19-integrations.md` (Feature #2)

---

## Goal

On the 1st of each month, generate a spending summary for the previous month and deliver it via Signal. The notification channel is abstracted behind an interface so Discord, WhatsApp, etc. can be added later with no changes to the report logic.

## Notification channel interface

**File:** `backend/pkg/notify/notify.go` (new)

```go
// Channel delivers a plain-text message to a notification endpoint.
type Channel interface {
    Name() string
    Send(ctx context.Context, subject, body string) error
}
```

## Signal implementation

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

If either env var is unset, the Signal channel is not registered and monthly reports are skipped (startup log warning emitted).

## Report content

Plain text, readable in Signal's monospace block:

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

**File:** `backend/pkg/report/monthly.go` (new)

## Scheduler

**File:** `backend/internal/scheduler/scheduler.go` (new)

A lightweight ticker that fires once an hour and checks if it's the 1st of the month. Idempotent — uses a `lastRun` month guard so it fires at most once per calendar month regardless of restarts.

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

## Store additions

```go
GetMonthlyReport(ctx context.Context, year int, month time.Month) (*MonthlyReport, error)
```

Runs three queries: total spend for the month, top 5 categories, top 5 merchants. Returns a `MonthlyReport` struct that `report.Generate` formats into the text message.

## Files Changed

| File | Change |
|------|--------|
| `backend/pkg/notify/notify.go` | New — Channel interface |
| `backend/pkg/notify/signal/signal.go` | New — Signal implementation |
| `backend/pkg/report/monthly.go` | New — report generation + formatting |
| `backend/internal/scheduler/scheduler.go` | New — hourly tick, monthly fire |
| `backend/internal/store/store.go` | Add `GetMonthlyReport` query |
| `backend/cmd/server/main.go` | Wire Signal channel + scheduler at startup |
