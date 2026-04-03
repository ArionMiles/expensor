# Group D — Rules Management: Design Spec

**Date:** 2026-04-03  
**Features:** Create rules from UI (#4), Export/import rules (#5)  
**Scope:** DB migration, backend API, rule evaluation changes, frontend Settings tab

---

## Overview

Rules are currently compiled into the binary as embedded JSON (`backend/cmd/server/content/rules.json`). Each rule matches incoming emails by sender, subject, and amount regex, and extracts transaction details. To make rules editable from the UI:

1. Rules move to PostgreSQL with the embedded JSON as the seed/default set.
2. The embedded file is retained as the immutable system baseline.
3. User-created rules are stored in the DB with `source = 'user'`.
4. At runtime, system rules and user rules are merged; user rules with the same `name` as a system rule take precedence.

---

## Data Model

**Migration:** `backend/migrations/005_rules.sql`

```sql
CREATE TYPE rule_source AS ENUM ('system', 'user');

CREATE TABLE IF NOT EXISTS rules (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name             TEXT NOT NULL,
    sender_email     TEXT NOT NULL DEFAULT '',
    subject_contains TEXT NOT NULL DEFAULT '',
    amount_regex     TEXT NOT NULL,
    merchant_regex   TEXT NOT NULL,
    currency_regex   TEXT NOT NULL DEFAULT '',
    enabled          BOOLEAN NOT NULL DEFAULT true,
    source           rule_source NOT NULL DEFAULT 'user',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (name, source)
);
```

**Seeding:** On startup, for each rule in the embedded `rules.json`, upsert into the `rules` table with `source = 'system'` using `ON CONFLICT (name, source) DO UPDATE SET ...`. This ensures system rules stay in sync with binary updates while preserving user customisations.

---

## Rule Merge Logic

**File:** `backend/cmd/server/main.go` and a new `backend/pkg/rules/rules.go`

```go
// MergeRules combines system rules and user rules.
// A user rule with the same name as a system rule completely replaces it.
// All other rules from both sets are included.
func MergeRules(system, user []api.Rule) []api.Rule {
    userByName := make(map[string]api.Rule, len(user))
    for _, r := range user {
        userByName[r.Name] = r
    }
    merged := make([]api.Rule, 0, len(system)+len(user))
    for _, r := range system {
        if override, ok := userByName[r.Name]; ok {
            merged = append(merged, override)
        } else {
            merged = append(merged, r)
        }
    }
    for _, r := range user {
        if _, exists := userByName[r.Name]; !exists {
            merged = append(merged, r)
        }
    }
    // Re-add user-only rules (not overrides)
    systemNames := make(map[string]struct{}, len(system))
    for _, r := range system {
        systemNames[r.Name] = struct{}{}
    }
    result := make([]api.Rule, 0)
    for _, r := range merged {
        result = append(result, r)
    }
    // Include user rules not in system set
    for _, r := range user {
        if _, inSystem := systemNames[r.Name]; !inSystem {
            result = append(result, r)
        }
    }
    return result
}
```

Simpler: at startup, load system rules from embedded JSON + user rules from DB, apply merge, pass to daemon as before.

**Correct merge algorithm:**
```go
func MergeRules(system, user []api.Rule) []api.Rule {
    userByName := make(map[string]api.Rule, len(user))
    for _, r := range user {
        userByName[r.Name] = r
    }
    out := make([]api.Rule, 0, len(system)+len(user))
    seen := make(map[string]struct{})
    for _, r := range system {
        if override, ok := userByName[r.Name]; ok {
            out = append(out, override)
        } else {
            out = append(out, r)
        }
        seen[r.Name] = struct{}{}
    }
    for _, r := range user {
        if _, already := seen[r.Name]; !already {
            out = append(out, r)
        }
    }
    return out
}
```

Only enabled rules are passed to the extractor (`filter(merged, func(r Rule) bool { return r.Enabled })`).

---

## Store Layer

**File:** `backend/internal/store/store.go`

```go
type RuleRow struct {
    ID              string     `json:"id"`
    Name            string     `json:"name"`
    SenderEmail     string     `json:"sender_email"`
    SubjectContains string     `json:"subject_contains"`
    AmountRegex     string     `json:"amount_regex"`
    MerchantRegex   string     `json:"merchant_regex"`
    CurrencyRegex   string     `json:"currency_regex"`
    Enabled         bool       `json:"enabled"`
    Source          string     `json:"source"` // "system" | "user"
    CreatedAt       time.Time  `json:"created_at"`
    UpdatedAt       time.Time  `json:"updated_at"`
}

ListRules(ctx context.Context) ([]RuleRow, error)
GetRule(ctx context.Context, id string) (*RuleRow, error)
CreateRule(ctx context.Context, r RuleRow) (*RuleRow, error)
UpdateRule(ctx context.Context, id string, r RuleRow) (*RuleRow, error)
DeleteRule(ctx context.Context, id string) error            // only user rules
ToggleRule(ctx context.Context, id string, enabled bool) error
SeedSystemRules(ctx context.Context, rules []RuleRow) error // upsert on (name, source=system)
```

---

## API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/rules` | List all rules (system + user), sorted: user first |
| POST | `/api/rules` | Create user rule |
| PUT | `/api/rules/{id}` | Update user rule (system rules: only `enabled` field writable) |
| DELETE | `/api/rules/{id}` | Delete user rule (returns 403 for system rules) |
| GET | `/api/rules/export` | Download all user rules as JSON |
| POST | `/api/rules/import` | Upload JSON; validates regexes; upserts as user rules |

**`POST /api/rules` body:**
```json
{
  "name": "HDFC Credit Card",
  "sender_email": "alerts@hdfcbank.net",
  "subject_contains": "transaction",
  "amount_regex": "Rs\\.([0-9,]+\\.?[0-9]*)",
  "merchant_regex": "at ([A-Za-z0-9 ]+)",
  "currency_regex": "",
  "enabled": true
}
```

**Validation (server-side):**
- `amount_regex` and `merchant_regex` are required; both must compile as valid Go regexes.
- `name` must be unique among user rules.
- Returns `422` with per-field error messages on failure.

**Export format (`GET /api/rules/export`):**
```json
[
  {
    "name": "...",
    "senderEmail": "...",
    "subjectContains": "...",
    "amountRegex": "...",
    "merchantInfoRegex": "...",
    "currencyRegex": "...",
    "enabled": true
  }
]
```
Uses the same field names as the existing `rules.json` for round-trip compatibility.

**Import (`POST /api/rules/import`):**
- Accepts `Content-Type: application/json` body or `multipart/form-data` file upload.
- Each rule is validated (regex compilation, name uniqueness).
- Import is transactional: if any rule fails validation the entire import is rejected.
- Rules with the same `name` as an existing user rule are updated (upsert).

---

## Runtime Reload

When the user saves or deletes a rule via the API, the running daemon does not automatically pick up the change — it reads rules once at startup. Two options:

**Chosen approach:** Lazy reload on next daemon start. Restarting the daemon (via `POST /api/daemon/start` after stopping) picks up the new rules. The UI shows a banner: _"Rule changes will apply on the next daemon restart."_

**Deferred:** Live-reload via channel injection (more complex, deferred to a future iteration).

---

## Frontend

### Settings page — Rules tab

**File:** `frontend/src/pages/settings/RulesSettings.tsx` (new)

**Layout:**
- Top action bar: "Import" button (file input) + "Export" button + "New rule" button.
- Table columns: Name, Sender, Subject, Enabled toggle, Source badge (System / User), Actions (Edit / Delete — disabled for system rows).
- System rules shown with a lock icon and "System" badge; editing only toggles `enabled`.
- "New rule" and "Edit" open a slide-over panel with a form.

**Rule form fields:**
- Name (text)
- Sender email (text, optional)
- Subject contains (text, optional)
- Amount regex (text, required) + live test input showing match/no-match
- Merchant regex (text, required) + live test input
- Currency regex (text, optional)
- Enabled (toggle)

**Regex live test:** A small input below each regex field. User types a sample string; the frontend runs `new RegExp(pattern).test(sample)` and shows a green ✓ or red ✗ inline. This does not require a server round-trip.

**Import flow:**
- Click "Import" → file picker (`.json` only).
- File is POSTed to `/api/rules/import`.
- On success: table refreshes, toast "N rules imported".
- On failure: error message listing which rules failed validation.

---

## Files Created / Modified

| File | Change |
|------|--------|
| `backend/migrations/005_rules.sql` | New migration |
| `backend/internal/store/store.go` | Add rule CRUD methods |
| `backend/internal/api/store.go` | Extend Storer interface |
| `backend/internal/api/handlers.go` | Add rules API endpoints |
| `backend/internal/api/server.go` | Register new routes |
| `backend/cmd/server/main.go` | Seed system rules on startup; load+merge at daemon start |
| `backend/pkg/rules/rules.go` | New package: `MergeRules`, `FilterEnabled` |
| `frontend/src/pages/settings/RulesSettings.tsx` | New |
| `frontend/src/api/queries.ts` | Add rules query/mutation hooks |
| `frontend/src/api/types.ts` | Add `Rule`, `RuleRow` types |
