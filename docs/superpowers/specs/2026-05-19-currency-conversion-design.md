# Currency Conversion Engine — Design Spec

**Date:** 2026-05-19
**Status:** Approved
**Scope:** Multi-currency transaction support — rate fetching, decimal precision migration, frontend indicator

---

## Problem

Transactions paid in a foreign currency (e.g., USD) are currently stored with whatever amount the extractor pulls from the email body — there is no conversion to the base currency, no original-amount preservation, and no rate record. The DB columns (`original_amount`, `original_currency`, `exchange_rate`) and the rule field (`currency_regex`) are already scaffolded but nothing populates them.

---

## Solution Overview

1. Add a `exchange_rates` DB table for persistent rate caching (Frankfurter API, keyed by `(from, to, date)`).
2. Add `pkg/currency` — a Frankfurter client that checks the DB cache before hitting the network.
3. Migrate all currency amount fields from `float64` to `shopspring/decimal` for exact decimal arithmetic.
4. Wire the currency client into the postgres writer: detect foreign-currency transactions, fetch the historical rate, store original values, write the converted amount in base currency.
5. Frontend: dashed-underline toggle on converted rows in the transactions table.

Old transactions with `NULL` original fields are left as-is (no backfill). This is an alpha-stage product.

---

## Backend

### Migration — `exchange_rates` table

New file `backend/migrations/010_exchange_rates.sql`:

```sql
CREATE TABLE IF NOT EXISTS exchange_rates (
    from_currency TEXT        NOT NULL,
    to_currency   TEXT        NOT NULL,
    date          DATE        NOT NULL,
    rate          NUMERIC(10,6) NOT NULL,
    fetched_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (from_currency, to_currency, date)
);
```

`fetched_at` is for observability only. Rates for historical dates are immutable once fetched and are never evicted.

Two new methods added to `internal/store/store.go` and the `Storer` interface in `internal/api/store.go`:

```go
GetExchangeRate(ctx context.Context, from, to string, date time.Time) (decimal.Decimal, bool, error)
UpsertExchangeRate(ctx context.Context, from, to string, date time.Time, rate decimal.Decimal) error
```

The `bool` return on `GetExchangeRate` indicates cache hit. Both methods operate on the `exchange_rates` table directly.

---

### `shopspring/decimal` migration

**New dependency:** `github.com/shopspring/decimal`

All currency amount fields migrate from `float64` to `decimal.Decimal`. This is a mechanical, well-scoped change.

#### `pkg/api/api.go` — `TransactionDetails`

| Field | Before | After |
|-------|--------|-------|
| `Amount` | `float64` | `decimal.Decimal` |
| `OriginalAmount` | `*float64` | `*decimal.Decimal` |
| `ExchangeRate` | `*float64` | `*decimal.Decimal` |

#### `internal/store/store.go` — `Transaction`

Same three fields, same change.

**pgx scanning:** NUMERIC columns are scanned into `string` then parsed:

```go
var amountStr string
// ... scan &amountStr ...
amount, err := decimal.NewFromString(amountStr)
```

pgx can scan PostgreSQL `NUMERIC` into Go `string` natively. No codec registration needed.

**pgx inserting:** pass `amount.String()` for NUMERIC parameters. pgx accepts string-encoded numerics for NUMERIC columns.

#### `pkg/extractor/extractor.go`

`extractAmount` changes from `strconv.ParseFloat` to `decimal.NewFromString`:

```go
func extractAmount(body string, re *regexp.Regexp) decimal.Decimal {
    // ... find match ...
    d, err := decimal.NewFromString(strings.ReplaceAll(m[1], ",", ""))
    if err != nil {
        return decimal.Zero
    }
    return d
}
```

#### Other files

- `pkg/writer/postgres/postgres.go` — pass `txn.Amount.String()` to pgx
- `internal/api/handlers.go` — any dashboard aggregation using amount fields
- `internal/api/handlers_test.go` — update mock store + test assertions
- `pkg/writer/postgres/postgres_test.go` — update test values
- `pkg/extractor/extractor_test.go` — update assertions

**JSON serialization:** `shopspring/decimal` implements `json.Marshaler`. It marshals as a JSON string (`"83.4200"`, not `83.42`). The frontend must use `parseFloat(txn.amount)` before any arithmetic — audit all amount usages during implementation.

---

### `pkg/currency` package

New package `backend/pkg/currency`:

```go
// Storer is the narrow DB interface required by the currency client.
type Storer interface {
    GetExchangeRate(ctx context.Context, from, to string, date time.Time) (decimal.Decimal, bool, error)
    UpsertExchangeRate(ctx context.Context, from, to string, date time.Time, rate decimal.Decimal) error
}

type Client struct {
    store Storer
    http  *http.Client
}

func NewClient(store Storer, httpClient *http.Client) *Client

// GetRate returns the exchange rate from→to on the given date.
// DB cache is checked first; on miss, Frankfurter is called and the result is stored.
// Returns (1, nil) immediately when from == to.
// On Frankfurter failure, returns (0, err) — the caller decides how to handle.
func (c *Client) GetRate(ctx context.Context, from, to string, date time.Time) (decimal.Decimal, error)
```

**Frankfurter API call:**

```
GET https://api.frankfurter.app/{YYYY-MM-DD}?from={FROM}&to={TO}
```

Response: `{"rates": {"INR": 83.42}}` — single-key parse. The rate value is parsed via `decimal.NewFromFloat` or directly from the JSON string representation.

**Error handling:**
- `from == to`: return `decimal.NewFromInt(1), nil` — no network call, no DB write.
- Non-200 / network error: return `decimal.Zero, err`.
- DB upsert failure: log warning only; the fetched rate is still returned to the caller.
- Today's date: Frankfurter returns the latest available rate (ECB publishes ~4pm CET on business days). This is acceptable.

---

### Writer changes

#### `pgwriter.Config`

```go
BaseCurrency string // e.g. "INR" — from config.Config.BaseCurrency
```

#### `pgwriter.Writer`

```go
type Writer struct {
    pool           *pgxpool.Pool
    logger         *slog.Logger
    batchSize      int
    flushInterval  time.Duration
    baseCurrency   string
    currencyClient *currency.Client
}
```

`currency.Client` is constructed inside `pgwriter.New()` using the same pool (wrapped in a thin adapter that satisfies `currency.Storer`).

#### Conversion logic (in batch write loop)

After timestamp parse, before `batch.Queue`:

```go
currency := txn.Currency
if currency == "" {
    currency = w.baseCurrency
}

if currency != w.baseCurrency {
    rate, err := w.currencyClient.GetRate(ctx, currency, w.baseCurrency, timestamp)
    if err != nil {
        w.logger.Warn("exchange rate fetch failed, storing original amount as-is",
            "from", currency, "to", w.baseCurrency, "error", err)
        // transaction is still written; original_amount/exchange_rate remain NULL
    } else {
        orig := txn.Amount
        origCcy := currency
        txn.OriginalAmount = &orig
        txn.OriginalCurrency = &origCcy
        txn.ExchangeRate = &rate
        txn.Amount = orig.Mul(rate).Round(4)
        currency = w.baseCurrency
    }
}
```

On rate fetch failure: transaction is written with the original amount stored as `amount`, and `original_amount`/`exchange_rate` are left NULL. The transaction is never dropped.

#### Plugin wiring

`pkg/plugins/writers/postgres/plugin.go` passes `cfg.BaseCurrency` into `pgwriter.Config`. The HTTP client passed to `NewWriter` is forwarded to `currency.NewClient`.

---

## Frontend

### Amount display in transaction table

`Transactions.tsx` — amount cell render logic:

A transaction is "converted" when `original_currency` is non-null and differs from `base_currency` (from `useStatus()`).

**Default state (showing base currency):**
```tsx
<span
  className="amount border-b border-dashed border-blue-500 cursor-pointer"
  onClick={() => toggleRow(txn.id)}
  title={`Paid in ${txn.original_currency}`}
>
  {formatCurrency(parseFloat(txn.amount), baseCurrency)}
</span>
<span className="text-[10px] text-[#555] ml-1">{txn.original_currency}</span>
```

**Toggled state (showing original currency):**
```tsx
<span
  className="amount text-amber-400 cursor-pointer"
  onClick={() => toggleRow(txn.id)}
>
  {formatCurrency(parseFloat(txn.original_amount), txn.original_currency)}
</span>
<span className="text-[10px] text-[#555] ml-1">
  ≈ {formatCurrency(parseFloat(txn.amount), baseCurrency, { maximumFractionDigits: 0 })}
</span>
```

**Toggle state:**

```ts
const [toggled, setToggled] = useState<Set<string>>(new Set())
const toggleRow = (id: string) =>
  setToggled(prev => {
    const next = new Set(prev)
    next.has(id) ? next.delete(id) : next.add(id)
    return next
  })
```

Local to page mount. No URL persistence — this is a transient display preference.

### JSON amount parsing

`shopspring/decimal` marshals amounts as JSON strings. All places in the frontend that use `txn.amount`, `txn.original_amount`, or `txn.exchange_rate` must call `parseFloat()` before arithmetic or formatting. Audit during implementation.

---

## Rule layer

No changes. `currency_regex` on rules is already fully wired: DB schema, Go structs, API handlers, extractor, and the `api.Rule.Currency` regexp field all exist. The extractor populates `TransactionDetails.Currency` with the ISO code when a match is found.

The hardcoded `"INR"` fallback in the writer is replaced by `w.baseCurrency` (part of the writer changes above).

---

## Error Handling Summary

| Scenario | Behavior |
|----------|----------|
| Frankfurter down during scan | Log warning; transaction written with original amount; `original_amount`/`exchange_rate` NULL |
| `from == to` | Short-circuit; no network, no DB write |
| DB upsert of rate fails | Log warning; rate still used for this write; next scan will re-fetch |
| `currency_regex` produces no match | `txn.Currency` is empty; treated as base currency |
| Old transactions (NULL `original_currency`) | Rendered as plain amount; no toggle affordance |

---

## What This Does Not Cover

- Backfill of existing transactions (out of scope for alpha).
- Currency conversion on the dashboard aggregations (heatmap, label timeline, daily spend) — these continue to sum `amount` which is always stored in base currency.
- Per-currency filtering or reporting.
- Exchange rate display in transaction detail view.
