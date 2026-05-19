# Beancount Export — Design Spec

**Date:** 2026-05-19
**Status:** Proposed — no implementation plan yet
**Split from:** `2026-05-19-integrations.md` (Feature #3)

---

## Goal

Allow the user to export their transactions as a `.beancount` file for use with the [Beancount](https://beancount.github.io/) plaintext accounting tool.

## Beancount format

Each transaction becomes a Beancount directive:

```
2024-01-15 * "Swiggy" "Food delivery"
  Expenses:FoodAndDining    1200.00 INR
  Assets:Checking          -1200.00 INR
```

**Category → account mapping:**

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

The `Assets:Checking` contra-account is hardcoded — Beancount requires a balancing leg but Expensor does not track accounts. Users are expected to adjust this in their beancount config.

**File header:**

```
; Exported from Expensor
; Generated: 2025-01-15T14:32:00Z
; Period: 2024-01-01 – 2024-12-31

option "operating_currency" "INR"
```

## API endpoint

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

## Backend

**File:** `backend/pkg/beancount/beancount.go` (new)

Contains `Render(transactions []store.Transaction, from, to *time.Time) string` — pure function, easily unit-testable.

**Store addition:**

```go
ListAllTransactions(ctx context.Context, from, to *time.Time) ([]Transaction, error)
```

Fetches all transactions in the date range without pagination. Uses the same WHERE clause builder as `ListTransactions` but without LIMIT/OFFSET.

## Frontend

**File:** `frontend/src/pages/Transactions.tsx`

Add an "Export" button in the Transactions page header alongside the existing filters. Clicking opens a small popover with:
- Format: `Beancount` (only option for now — designed to extend with CSV, OFX etc. later)
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

## Files Changed

| File | Change |
|------|--------|
| `backend/pkg/beancount/beancount.go` | New — renderer + category mapping |
| `backend/internal/store/store.go` | Add `ListAllTransactions` |
| `backend/internal/api/store.go` | Extend Storer interface |
| `backend/internal/api/handlers.go` | Add `HandleExportBeancount` |
| `backend/internal/api/server.go` | Register export route |
| `frontend/src/pages/Transactions.tsx` | Add Export button + download helper |
