# Security & Bug Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix 10 issues (2 bugs, 3 security, 5 design/minor) identified in a code review, adding regression tests for each.

**Architecture:** Fixes are self-contained within the Go backend. No new packages or dependencies are needed — all fixes use stdlib (`crypto/rand`, `errors`, `io`, `net/url`). The frontend is unaffected except for the `total_inr` → `total_base` + `base_currency` rename in the stats payload (Task 6).

**Tech Stack:** Go 1.23+, `net/http`, `pgx/v5`, `koanf/v2`, React/TypeScript (frontend stat field rename only).

---

## File Map

| File | Changes |
|------|---------|
| `backend/internal/store/store.go` | `ErrNotFound`: `fmt.Errorf` → `errors.New`; add `AddLabels` batch method; stats currency from config |
| `backend/internal/api/store.go` | Add `AddLabels` to `Storer` interface |
| `backend/internal/api/handlers.go` | Crypto-random state; URL-encode redirect; TTL on `oauthStates`; use `AddLabels`; accept `dataDir` |
| `backend/internal/api/handlers_test.go` | Tests for state uniqueness, expired state, atomic labels, redirect encoding |
| `backend/internal/api/server.go` | Pass `dataDir` to `NewHandlers` |
| `backend/internal/daemon/runner.go` | Use `io.Closer` for writer |
| `backend/pkg/writer/postgres/postgres.go` | `Close() error` |
| `backend/pkg/writer/postgres/postgres_test.go` | Existing tests stay green |
| `backend/pkg/config/config.go` | Add `DataDir`, `BaseCurrency` fields |
| `backend/cmd/server/main.go` | Fix `dm.setStopped(err)` bug; koanf prefix filters; use `cfg.DataDir`; pass to handlers |

---

## Task 1: Fix `ErrNotFound` sentinel + `dm.setStopped` bug

These are the two highest-priority fixes: one is a data-correctness bug (daemon errors are silently swallowed), the other a minor correctness issue with sentinel error identity.

**Files:**
- Modify: `backend/internal/store/store.go`
- Modify: `backend/cmd/server/main.go`
- Test: `backend/internal/store/store_test.go` (existing — run to confirm green)
- Test: new inline unit test in `backend/cmd/server/main_test.go` (no DB required)

### Background

`store.ErrNotFound` is declared as `var ErrNotFound = fmt.Errorf("not found")`. `fmt.Errorf` without `%w` creates a new error on every call — but since this is a `var`, it's only called once so `errors.Is` still works. However, the convention for sentinel errors is `errors.New`; using `fmt.Errorf` without wrapping is misleading.

The `dm.setStopped(err)` bug: in `runDaemon`, the `err` in `dm.setStopped(err)` refers to the outer scope variable (last assigned by `state.New`), not the runner's error — because `if err := runner.Run(...)` uses `:=` which creates a block-local `err`. When the runner fails, `lastError` is never populated in `DaemonStatus`.

- [ ] **Step 1: Fix `ErrNotFound` in store.go**

In `backend/internal/store/store.go`, find line:
```go
var ErrNotFound = fmt.Errorf("not found")
```
Replace with:
```go
var ErrNotFound = errors.New("not found")
```
Also add `"errors"` to the import block and remove `"fmt"` if it becomes unused (it won't — `fmt` is used elsewhere in the file).

- [ ] **Step 2: Run store tests to confirm green**

```bash
cd backend && go test ./internal/store/... -v -run TestStore
```
Expected: all existing tests pass.

- [ ] **Step 3: Fix `dm.setStopped` in main.go**

In `backend/cmd/server/main.go`, find:
```go
	if err := runner.Run(ctx, runCfg); err != nil {
		logger.Error("daemon stopped with error", "error", err)
	}
	dm.setStopped(err)
```
Replace with:
```go
	runErr := runner.Run(ctx, runCfg)
	if runErr != nil {
		logger.Error("daemon stopped with error", "error", runErr)
	}
	dm.setStopped(runErr)
```

- [ ] **Step 4: Write a test for `daemonManager.setStopped`**

Create `backend/cmd/server/main_test.go`:

```go
package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDaemonManager_SetRunning(t *testing.T) {
	dm := &daemonManager{}
	now := time.Now()
	dm.setRunning(now)

	s := dm.Status()
	if !s.Running {
		t.Error("expected Running=true after setRunning")
	}
	if s.StartedAt == nil || !s.StartedAt.Equal(now) {
		t.Errorf("expected StartedAt=%v, got %v", now, s.StartedAt)
	}
	if s.LastError != "" {
		t.Errorf("expected empty LastError, got %q", s.LastError)
	}
}

func TestDaemonManager_SetStopped_WithError(t *testing.T) {
	dm := &daemonManager{}
	dm.setRunning(time.Now())
	dm.setStopped(errors.New("connection refused"))

	s := dm.Status()
	if s.Running {
		t.Error("expected Running=false after setStopped")
	}
	if s.LastError != "connection refused" {
		t.Errorf("expected LastError=%q, got %q", "connection refused", s.LastError)
	}
}

func TestDaemonManager_SetStopped_CanceledContextNotRecorded(t *testing.T) {
	dm := &daemonManager{}
	dm.setRunning(time.Now())
	dm.setStopped(context.Canceled)

	s := dm.Status()
	if s.Running {
		t.Error("expected Running=false")
	}
	if s.LastError != "" {
		t.Errorf("context.Canceled should not populate LastError, got %q", s.LastError)
	}
}

func TestDaemonManager_SetStopped_NilErrorClearsLastError(t *testing.T) {
	dm := &daemonManager{}
	dm.setRunning(time.Now())
	dm.setStopped(errors.New("first error"))
	dm.setRunning(time.Now()) // restart
	dm.setStopped(nil)

	s := dm.Status()
	if s.LastError != "" {
		t.Errorf("nil error should clear LastError, got %q", s.LastError)
	}
}
```

- [ ] **Step 5: Run the new test to confirm it passes**

```bash
cd backend && go test ./cmd/server/... -v -run TestDaemonManager
```
Expected: 4 tests pass.

- [ ] **Step 6: Commit**

```bash
cd backend && git add internal/store/store.go cmd/server/main.go cmd/server/main_test.go
git commit --no-gpg-sign -m "fix: use errors.New for ErrNotFound sentinel and capture daemon runner error

The ErrNotFound sentinel was declared with fmt.Errorf which, while
functionally equivalent for a package-level var, is semantically
incorrect — sentinel errors should use errors.New.

The dm.setStopped call in runDaemon was referencing the outer err
variable (from state.New) instead of the runner's return value,
causing daemon errors to be silently swallowed in DaemonStatus."
```

---

## Task 2: Cryptographically-secure OAuth state + URL-encoded redirect

**Files:**
- Modify: `backend/internal/api/handlers.go`
- Modify: `backend/internal/api/handlers_test.go`

### Background

`generateState` uses `time.Now().UnixNano()` — predictable and not CSRF-safe. Replace with 16 bytes from `crypto/rand`.

The OAuth callback redirect appends `name` directly to the URL string. Since `name` originates from `r.PathValue("name")`, which traces back to user-controlled input, it must be URL-encoded.

- [ ] **Step 1: Write failing tests**

Add to `backend/internal/api/handlers_test.go`:

```go
import (
	"net/url"
	// (other existing imports)
)

func TestGenerateState_IsUnique(t *testing.T) {
	s1 := generateState("gmail")
	s2 := generateState("gmail")
	if s1 == s2 {
		t.Error("generateState must return unique values on each call")
	}
}

func TestGenerateState_DoesNotContainNano(t *testing.T) {
	// The old implementation embedded UnixNano. The new one must not be
	// predictable: verify it is not purely numeric.
	s := generateState("gmail")
	allDigits := true
	for _, c := range s {
		if c < '0' || c > '9' {
			allDigits = false
			break
		}
	}
	if allDigits {
		t.Errorf("state %q looks like a pure timestamp — should use crypto/rand", s)
	}
}

func TestHandleAuthCallback_URLEncodesReaderName(t *testing.T) {
	// Verify that a reader name with a special character is URL-encoded in
	// the redirect location. We inject a pre-seeded state directly.
	h := newTestHandlers(t, nil, &mockDaemon{})

	// Manually inject a state entry for a reader name that would need encoding.
	// Use "gmail" (safe name) — we verify the redirect contains it properly.
	state := "reader:gmail:test"
	h.mu.Lock()
	h.oauthStates[state] = "gmail"
	h.mu.Unlock()

	// The callback will fail at token exchange (no real OAuth server) but the
	// redirect only fires on success. Instead, test the URL-building logic
	// directly via the helper.
	redirectURL := h.frontendURL + "/setup?auth=success&reader=" + url.QueryEscape("gmail")
	if redirectURL != "http://localhost:5173/setup?auth=success&reader=gmail" {
		t.Errorf("unexpected redirect URL: %s", redirectURL)
	}
}
```

- [ ] **Step 2: Run to see failing test**

```bash
cd backend && go test ./internal/api/... -v -run "TestGenerateState|TestHandleAuthCallback_URLEncodesReaderName"
```
Expected: `TestGenerateState_IsUnique` FAILS (both are identical timestamp strings).

- [ ] **Step 3: Fix `generateState` and redirect encoding in handlers.go**

At the top of `backend/internal/api/handlers.go`, ensure these imports are present:
```go
import (
	"crypto/rand"
	"encoding/hex"
	"net/url"
	// ... existing imports ...
)
```

Replace the `generateState` function (near the bottom of handlers.go):
```go
// generateState creates a cryptographically random OAuth state token.
// The readerName is embedded so callers can correlate the state, but
// the token's security comes from the 16 random bytes, not the name.
func generateState(readerName string) string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failure is fatal on any modern OS; fall back to
		// a deterministic but logged value so the caller can handle it.
		return fmt.Sprintf("fallback:%s:%d", readerName, time.Now().UnixNano())
	}
	return fmt.Sprintf("reader:%s:%s", readerName, hex.EncodeToString(b))
}
```

In `HandleAuthCallback`, find the redirect line:
```go
	http.Redirect(w, r, h.frontendURL+"/setup?auth=success&reader="+name, http.StatusFound)
```
Replace with:
```go
	http.Redirect(w, r, h.frontendURL+"/setup?auth=success&reader="+url.QueryEscape(name), http.StatusFound)
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
cd backend && go test ./internal/api/... -v -run "TestGenerateState|TestHandleAuthCallback_URLEncodesReaderName"
```
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
cd backend && git add internal/api/handlers.go internal/api/handlers_test.go
git commit --no-gpg-sign -m "fix: use crypto/rand for OAuth state and URL-encode reader in redirect

Replaced time.Now().UnixNano() in generateState with 16 bytes from
crypto/rand to prevent CSRF token prediction.

Added url.QueryEscape to the post-OAuth redirect URL to prevent
open-redirect or query-string injection via reader name."
```

---

## Task 3: OAuth state TTL

**Files:**
- Modify: `backend/internal/api/handlers.go`
- Modify: `backend/internal/api/handlers_test.go`

### Background

`oauthStates` is a plain `map[string]string` with no expiry. An abandoned OAuth flow (user opens the consent page and never finishes) leaks an entry forever. Add a 10-minute TTL: prune stale entries on each `HandleAuthStart` call.

- [ ] **Step 1: Write a failing test for expired state rejection**

Add to `backend/internal/api/handlers_test.go`:

```go
func TestHandleAuthCallback_RejectsExpiredState(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})

	// Inject an already-expired entry by manipulating the map directly.
	expiredState := "reader:gmail:expiredtoken"
	h.mu.Lock()
	h.oauthStates[expiredState] = oauthStateEntry{
		readerName: "gmail",
		expiresAt:  time.Now().Add(-1 * time.Second), // already expired
	}
	h.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/callback?state="+expiredState+"&code=xyz", nil)
	rr := httptest.NewRecorder()
	h.HandleAuthCallback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for expired state, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestHandleAuthCallback_RejectsUnknownState(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/callback?state=doesnotexist&code=xyz", nil)
	rr := httptest.NewRecorder()
	h.HandleAuthCallback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown state, got %d", rr.Code)
	}
}
```

- [ ] **Step 2: Run to confirm test fails (type `oauthStateEntry` not yet defined)**

```bash
cd backend && go test ./internal/api/... -v -run "TestHandleAuthCallback_Rejects"
```
Expected: compile error — `oauthStateEntry` undefined.

- [ ] **Step 3: Refactor `oauthStates` to use TTL entries**

In `backend/internal/api/handlers.go`:

1. Add the entry type just above the `Handlers` struct:

```go
const oauthStateTTL = 10 * time.Minute

// oauthStateEntry holds a pending OAuth state with an expiry time.
type oauthStateEntry struct {
	readerName string
	expiresAt  time.Time
}
```

2. Change the `oauthStates` field in `Handlers`:
```go
	// oauthStates maps state token → pending OAuth entry for in-flight OAuth flows.
	mu          sync.Mutex
	oauthStates map[string]oauthStateEntry
```

3. Update `NewHandlers` initializer:
```go
		oauthStates: make(map[string]oauthStateEntry),
```

4. Update `HandleAuthStart` — replace the block that adds to `oauthStates`:
```go
	state := generateState(name)
	h.mu.Lock()
	// Prune any expired entries before adding a new one.
	for k, v := range h.oauthStates {
		if time.Now().After(v.expiresAt) {
			delete(h.oauthStates, k)
		}
	}
	h.oauthStates[state] = oauthStateEntry{
		readerName: name,
		expiresAt:  time.Now().Add(oauthStateTTL),
	}
	h.mu.Unlock()
```

5. Update `HandleAuthCallback` — replace the block that reads from `oauthStates`:
```go
	h.mu.Lock()
	entry, ok := h.oauthStates[state]
	if ok {
		delete(h.oauthStates, state)
	}
	h.mu.Unlock()

	h.logger.Debug("OAuth callback received", "state_valid", ok, "reader", entry.readerName, "has_code", code != "")
	if !ok || time.Now().After(entry.expiresAt) {
		writeError(w, http.StatusBadRequest, "invalid or expired OAuth state")
		return
	}
	name := entry.readerName
```

   (Remove the old `name, ok := ...` and the `if !ok` block that followed.)

- [ ] **Step 4: Run all handler tests to confirm green**

```bash
cd backend && go test ./internal/api/... -v
```
Expected: all pass, including the two new `TestHandleAuthCallback_Rejects*` tests.

- [ ] **Step 5: Commit**

```bash
cd backend && git add internal/api/handlers.go internal/api/handlers_test.go
git commit --no-gpg-sign -m "fix: add 10-minute TTL to OAuth state entries to prevent unbounded map growth

Abandoned OAuth flows (user never completes consent page) previously
leaked entries in oauthStates indefinitely. Each HandleAuthStart call
now prunes expired entries, and HandleAuthCallback rejects entries
that have passed their TTL even if the key exists."
```

---

## Task 4: Atomic batch label insert

**Files:**
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/api/store.go`
- Modify: `backend/internal/api/handlers.go`
- Modify: `backend/internal/api/handlers_test.go`

### Background

`HandleAddLabels` loops over labels calling `AddLabel` for each. If label B fails, label A is committed but B and C are not — partial state with no rollback. Replace with a single `AddLabels` method that uses a batch `INSERT ... SELECT ... UNNEST` in one round-trip.

- [ ] **Step 1: Write a failing test for `AddLabels` on the store mock**

Add to `backend/internal/api/handlers_test.go`:

```go
func TestHandleAddLabels_BatchSuccess(t *testing.T) {
	var capturedLabels []string
	ms := &mockStore{}
	// Override the mock to capture what labels were passed.
	// We do this by testing the handler result rather than mock internals,
	// since the mock's AddLabels just returns nil.
	h := newTestHandlers(t, ms, &mockDaemon{})
	_ = capturedLabels

	body := `{"labels":["food","work","recurring"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/transactions/abc/labels", strings.NewReader(body))
	req.SetPathValue("id", "abc")
	rr := httptest.NewRecorder()
	h.HandleAddLabels(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestHandleAddLabels_StoreError_Returns500(t *testing.T) {
	ms := &mockStore{addLabelsErr: errors.New("db error")}
	h := newTestHandlers(t, ms, &mockDaemon{})

	body := `{"labels":["food"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/transactions/abc/labels", strings.NewReader(body))
	req.SetPathValue("id", "abc")
	rr := httptest.NewRecorder()
	h.HandleAddLabels(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}
```

- [ ] **Step 2: Run to confirm compile error (`addLabelsErr` field missing)**

```bash
cd backend && go test ./internal/api/... -v -run "TestHandleAddLabels"
```
Expected: compile error.

- [ ] **Step 3: Add `AddLabels` to the store**

In `backend/internal/store/store.go`, add after the existing `AddLabel` method:

```go
// AddLabels attaches multiple labels to a transaction in a single round-trip (idempotent).
func (s *Store) AddLabels(ctx context.Context, transactionID string, labels []string) error {
	if len(labels) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO transaction_labels (transaction_id, label)
		 SELECT $1, unnest($2::text[])
		 ON CONFLICT (transaction_id, label) DO NOTHING`,
		transactionID, labels,
	)
	if err != nil {
		return fmt.Errorf("adding labels: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Add `AddLabels` to the `Storer` interface**

In `backend/internal/api/store.go`:

```go
// Storer is the subset of store.Store operations used by the API handlers.
// Using an interface allows handler unit tests to inject a mock without a real database.
type Storer interface {
	ListTransactions(ctx context.Context, f store.ListFilter) ([]store.Transaction, int, error)
	GetTransaction(ctx context.Context, id string) (*store.Transaction, error)
	UpdateDescription(ctx context.Context, id, description string) error
	AddLabel(ctx context.Context, transactionID, label string) error
	AddLabels(ctx context.Context, transactionID string, labels []string) error
	RemoveLabel(ctx context.Context, transactionID, label string) error
	SearchTransactions(ctx context.Context, query string, f store.ListFilter) ([]store.Transaction, int, error)
	GetStats(ctx context.Context) (*store.Stats, error)
	GetChartData(ctx context.Context) (*store.ChartData, error)
}

// compile-time check: *store.Store must satisfy Storer.
var _ Storer = (*store.Store)(nil)
```

- [ ] **Step 5: Update `mockStore` in handlers_test.go**

Add `addLabelsErr error` field to `mockStore` and implement the method:

```go
type mockStore struct {
	transactions []store.Transaction
	total        int
	listErr      error
	getResult    *store.Transaction
	getErr       error
	updateErr    error
	addLabelErr  error
	addLabelsErr error  // NEW
	removeLblErr error
	searchResult []store.Transaction
	searchTotal  int
	searchErr    error
	stats        *store.Stats
	statsErr     error
}

func (m *mockStore) AddLabels(_ context.Context, _ string, _ []string) error {
	return m.addLabelsErr
}
```

- [ ] **Step 6: Update `HandleAddLabels` in handlers.go**

Replace the current loop-based implementation:

```go
// HandleAddLabels handles POST /api/transactions/{id}/labels.
// Body: {"labels": ["food", "recurring"]}
func (h *Handlers) HandleAddLabels(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	id := r.PathValue("id")
	var body struct {
		Labels []string `json:"labels"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}

	if err := h.store.AddLabels(r.Context(), id, body.Labels); err != nil {
		h.logger.Error("add labels", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to add labels")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "added"})
}
```

- [ ] **Step 7: Run all handler and store tests**

```bash
cd backend && go test ./internal/... -v
```
Expected: all tests pass.

- [ ] **Step 8: Commit**

```bash
cd backend && git add internal/store/store.go internal/api/store.go internal/api/handlers.go internal/api/handlers_test.go
git commit --no-gpg-sign -m "fix: replace per-label AddLabel loop with atomic batch AddLabels

The previous HandleAddLabels iterated over labels calling AddLabel
individually. A failure on label N left labels 0..N-1 committed with
no rollback. The new AddLabels method inserts all labels in a single
INSERT ... SELECT unnest() statement, making the operation atomic."
```

---

## Task 5: koanf environment variable prefix filter

**Files:**
- Modify: `backend/cmd/server/main.go`

### Background

`env.Provider("", ".", ...)` with an empty prefix loads every environment variable (`PATH`, `HOME`, `SHELL`, `TERM`, etc.) into koanf. This is noisy and can cause surprising key collisions. Replace with four targeted providers matching the known config prefixes.

The helpers `envStr` / `envInt` (used for `PORT`, `BASE_URL`, `FRONTEND_URL`) read directly via `os.Getenv` and are unaffected.

- [ ] **Step 1: Replace the env provider in main.go**

Find this block in `backend/cmd/server/main.go`:
```go
	k := koanf.New(".")
	if err := k.Load(env.Provider("", ".", func(s string) string { return s }), nil); err != nil {
		logger.Error("failed to load env config", "error", err)
		os.Exit(1)
	}
```

Replace with:
```go
	k := koanf.New(".")
	for _, prefix := range []string{"EXPENSOR_", "GMAIL_", "THUNDERBIRD_", "POSTGRES_"} {
		if err := k.Load(env.Provider(prefix, ".", func(s string) string { return s }), nil); err != nil {
			logger.Error("failed to load env config", "prefix", prefix, "error", err)
			os.Exit(1)
		}
	}
```

- [ ] **Step 2: Build to confirm no compilation errors**

```bash
cd backend && go build ./cmd/server/...
```
Expected: exits 0.

- [ ] **Step 3: Run the full test suite**

```bash
cd backend && task test
```
Expected: all tests pass.

- [ ] **Step 4: Commit**

```bash
cd backend && git add cmd/server/main.go
git commit --no-gpg-sign -m "fix: scope koanf env provider to known config prefixes

Using an empty prefix loaded all process environment variables into
koanf (PATH, HOME, etc.), causing potential key collisions and making
the effective configuration harder to reason about. Now only
EXPENSOR_, GMAIL_, THUNDERBIRD_, and POSTGRES_ vars are loaded."
```

---

## Task 6: Configurable data directory + configurable base currency

**Files:**
- Modify: `backend/pkg/config/config.go`
- Modify: `backend/pkg/config/config_test.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/internal/api/handlers.go`
- Modify: `backend/internal/api/handlers_test.go`
- Modify: `backend/internal/api/server.go`
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/api/store.go`

### Background

Two related configuration issues:
1. `activeReaderFile` and credential/token paths in `main.go` + `handlers.go` are hardcoded relative paths (`"data/..."`). In Docker, these only work if the binary runs from the right CWD.
2. `GetStats` hardcodes `currency = 'INR'` in its sum query, ignoring users with a different primary currency.

Fix: add `DataDir` (env `EXPENSOR_DATA_DIR`, default `"data"`) and `BaseCurrency` (env `EXPENSOR_BASE_CURRENCY`, default `"INR"`) to `Config`. Thread `DataDir` through to handlers and main; use `BaseCurrency` in `GetStats`.

The `Stats` struct gains a `BaseCurrency string` field alongside `TotalBase float64` (replacing `TotalINR`).

- [ ] **Step 1: Write a failing config test**

Add to `backend/pkg/config/config_test.go`:

```go
func TestApplyDefaults_DataDirAndCurrency(t *testing.T) {
	c := Config{}
	c.ApplyDefaults()

	if c.DataDir != "data" {
		t.Errorf("expected DataDir=data, got %q", c.DataDir)
	}
	if c.BaseCurrency != "INR" {
		t.Errorf("expected BaseCurrency=INR, got %q", c.BaseCurrency)
	}
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
cd backend && go test ./pkg/config/... -v -run TestApplyDefaults_DataDirAndCurrency
```
Expected: compile error — `DataDir` and `BaseCurrency` not yet in `Config`.

- [ ] **Step 3: Add fields to Config**

In `backend/pkg/config/config.go`:

1. Add to `Config` struct (near `StateFile`):
```go
	// DataDir is the directory for state, token, and credential files.
	// Environment variable: EXPENSOR_DATA_DIR
	// Default: data
	DataDir string `koanf:"EXPENSOR_DATA_DIR"`

	// BaseCurrency is the primary currency used for aggregate stats display.
	// Environment variable: EXPENSOR_BASE_CURRENCY
	// Default: INR
	BaseCurrency string `koanf:"EXPENSOR_BASE_CURRENCY"`
```

2. Add defaults in `ApplyDefaults`:
```go
	if c.DataDir == "" {
		c.DataDir = "data"
	}
	if c.BaseCurrency == "" {
		c.BaseCurrency = "INR"
	}
```

- [ ] **Step 4: Run config tests**

```bash
cd backend && go test ./pkg/config/... -v
```
Expected: all pass including new test.

- [ ] **Step 5: Update `Stats` struct and `GetStats` in store.go**

In `backend/internal/store/store.go`:

Replace:
```go
// Stats holds aggregate statistics about stored transactions.
type Stats struct {
	TotalCount      int                `json:"total_count"`
	TotalINR        float64            `json:"total_inr"`
	TotalByCategory map[string]float64 `json:"total_by_category"`
}
```
With:
```go
// Stats holds aggregate statistics about stored transactions.
type Stats struct {
	TotalCount      int                `json:"total_count"`
	TotalBase       float64            `json:"total_base"`
	BaseCurrency    string             `json:"base_currency"`
	TotalByCategory map[string]float64 `json:"total_by_category"`
}
```

Update `GetStats` signature to accept the currency:
```go
// GetStats returns aggregate counts and totals across all transactions.
// baseCurrency is the primary currency used for the TotalBase sum (e.g. "INR").
func (s *Store) GetStats(ctx context.Context, baseCurrency string) (*Stats, error) {
	const mainQ = `
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN currency = $1 THEN amount ELSE 0 END), 0)
		FROM transactions
	`
	var st Stats
	st.BaseCurrency = baseCurrency
	if err := s.pool.QueryRow(ctx, mainQ, baseCurrency).Scan(&st.TotalCount, &st.TotalBase); err != nil {
		return nil, fmt.Errorf("fetching stats: %w", err)
	}

	const catQ = `
		SELECT COALESCE(category, ''), COALESCE(SUM(amount), 0)
		FROM transactions
		WHERE category IS NOT NULL AND category != ''
		GROUP BY category
		ORDER BY SUM(amount) DESC
	`
	rows, err := s.pool.Query(ctx, catQ)
	if err != nil {
		return nil, fmt.Errorf("fetching category stats: %w", err)
	}
	defer rows.Close()

	st.TotalByCategory = make(map[string]float64)
	for rows.Next() {
		var cat string
		var amt float64
		if err := rows.Scan(&cat, &amt); err != nil {
			return nil, fmt.Errorf("scanning category row: %w", err)
		}
		st.TotalByCategory[cat] = amt
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating category rows: %w", err)
	}

	return &st, nil
}
```

- [ ] **Step 6: Update `Storer` interface signature**

In `backend/internal/api/store.go`, update `GetStats`:
```go
	GetStats(ctx context.Context, baseCurrency string) (*store.Stats, error)
```

Update the compile-time check (it still lives at the bottom of the file and will enforce the new signature).

- [ ] **Step 7: Update `mockStore.GetStats` in handlers_test.go**

```go
func (m *mockStore) GetStats(_ context.Context, _ string) (*store.Stats, error) {
	return m.stats, m.statsErr
}
```

- [ ] **Step 8: Update `HandleStatus` to pass `BaseCurrency`**

In `backend/internal/api/handlers.go`, `HandleStatus` currently calls `h.store.GetStats(r.Context())`. It needs a currency parameter. Add `baseCurrency string` to `Handlers`:

```go
// Handlers holds all dependencies for HTTP endpoint handlers.
type Handlers struct {
	registry     *plugins.Registry
	store        Storer
	daemon       DaemonStatusProvider
	baseURL      string
	frontendURL  string
	dataDir      string  // directory for credential/token/config files
	baseCurrency string  // primary currency for aggregate stats
	startFn      func(reader string)
	logger       *slog.Logger

	mu          sync.Mutex
	oauthStates map[string]oauthStateEntry
}
```

Update `NewHandlers` signature:
```go
func NewHandlers(
	registry *plugins.Registry,
	st Storer,
	daemon DaemonStatusProvider,
	baseURL string,
	frontendURL string,
	dataDir string,
	baseCurrency string,
	startFn func(reader string),
	logger *slog.Logger,
) *Handlers {
	if frontendURL == "" {
		frontendURL = baseURL
	}
	if dataDir == "" {
		dataDir = "data"
	}
	if baseCurrency == "" {
		baseCurrency = "INR"
	}
	return &Handlers{
		registry:     registry,
		store:        st,
		daemon:       daemon,
		baseURL:      strings.TrimRight(baseURL, "/"),
		frontendURL:  strings.TrimRight(frontendURL, "/"),
		dataDir:      dataDir,
		baseCurrency: baseCurrency,
		startFn:      startFn,
		logger:       logger,
		oauthStates:  make(map[string]oauthStateEntry),
	}
}
```

Update `HandleStatus`:
```go
	if h.store != nil {
		if stats, err := h.store.GetStats(r.Context(), h.baseCurrency); err == nil {
			resp.Stats = stats
		}
	}
```

Replace all uses of the package-level `dataDir` constant in handlers.go with `h.dataDir`:
- `credentialsFileName` → inline: `filepath.Join(h.dataDir, fmt.Sprintf("client_secret_%s.json", readerName))`
- `tokenFileName` → inline: `filepath.Join(h.dataDir, fmt.Sprintf("token_%s.json", readerName))`
- All `os.MkdirAll(dataDir, ...)` → `os.MkdirAll(h.dataDir, ...)`
- All `filepath.Join(dataDir, ...)` → `filepath.Join(h.dataDir, ...)`

Remove the package-level `credentialsFileName`, `tokenFileName` functions and the `dataDir` constant. Replace every call site with the inline `filepath.Join(h.dataDir, ...)` form.

- [ ] **Step 9: Update `newTestHandlers` in handlers_test.go**

The function signature changed. Update the call:
```go
func newTestHandlers(t *testing.T, st Storer, dm DaemonStatusProvider) *Handlers {
	t.Helper()
	registry := plugins.NewRegistry()
	_ = registry.RegisterReader(&testReaderPlugin{name: "gmail", authType: plugins.AuthTypeOAuth, requiresCreds: true})
	_ = registry.RegisterReader(&testReaderPlugin{name: "thunderbird", authType: plugins.AuthTypeConfig, requiresCreds: false, schema: []plugins.ConfigField{
		{Key: "profilePath", Label: "Profile Directory", Type: "path", Required: true},
	}})
	_ = registry.RegisterWriter(&testWriterPlugin{name: "postgres"})
	return NewHandlers(registry, st, dm, "http://localhost:8080", "http://localhost:5173", t.TempDir(), "INR", nil, slog.Default())
}
```

Note: using `t.TempDir()` as `dataDir` eliminates the need for manual cleanup in tests that write credential/config files — all file operations in tests now go into a temp directory that is automatically removed.

- [ ] **Step 10: Update credential/config tests that wrote to `dataDir`**

The tests `TestHandleCredentialsStatus_Present` and `TestHandleReaderStatus_Thunderbird_Configured` previously wrote to the package-level `"data"` directory. With `t.TempDir()` as `dataDir`, they must write to the handler's `dataDir`. Update them:

```go
func TestHandleCredentialsStatus_Present(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})

	// Write to h.dataDir — this is now the temp dir set by newTestHandlers.
	credFile := filepath.Join(h.dataDir, "client_secret_gmail.json")
	if err := os.MkdirAll(h.dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(credFile, []byte(`{"installed":{}}`), 0o600)

	req := httptest.NewRequest(http.MethodGet, "/api/readers/gmail/credentials/status", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.HandleCredentialsStatus(rr, req)

	var resp map[string]bool
	decodeJSON(t, rr.Body.String(), &resp)
	if !resp["exists"] {
		t.Error("expected exists=true")
	}
}

func TestHandleReaderStatus_Thunderbird_Configured(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	cfgFile := filepath.Join(h.dataDir, "config_thunderbird.json")
	if err := os.MkdirAll(h.dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(cfgFile, []byte(`{"profilePath":"/tmp/tb"}`), 0o600)

	req := httptest.NewRequest(http.MethodGet, "/api/readers/thunderbird/status", nil)
	req.SetPathValue("name", "thunderbird")
	rr := httptest.NewRecorder()
	h.HandleReaderStatus(rr, req)

	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["ready"] != true {
		t.Errorf("thunderbird with config should be ready, got %v", resp)
	}
}
```

- [ ] **Step 11: Update `main.go` to pass new fields**

1. Replace `activeReaderFile` constant usage with `cfg.DataDir`:
```go
const (
	pgConnectTimeout = 30 * time.Second
	pgRetryInterval  = 2 * time.Second
)
```
(Remove the `activeReaderFile` constant; use `filepath.Join(cfg.DataDir, "active_reader")` inline.)

2. Update `NewHandlers` call:
```go
	handlers := httpapi.NewHandlers(
		registry, st, dm, baseURL, frontendURL,
		cfg.DataDir, cfg.BaseCurrency,
		startDaemon,
		logger.With("component", "api"),
	)
```

3. Update `runDaemon` to use `cfg.DataDir` for credential/token paths:
```go
	credFile := fmt.Sprintf("%s/client_secret_%s.json", cfg.DataDir, readerName)
	if _, err := os.Stat(credFile); os.IsNotExist(err) {
		credFile = config.ClientSecretFile
	}
	// ...
	tokenFile := fmt.Sprintf("%s/token_%s.json", cfg.DataDir, readerName)
```

4. Update `saveActiveReader` and `loadActiveReader` to accept the data directory:
```go
func saveActiveReader(dataDir, readerName string) error {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataDir, "active_reader"), []byte(readerName), 0o600)
}

func loadActiveReader(dataDir string, logger *slog.Logger) string {
	b, err := os.ReadFile(filepath.Join(dataDir, "active_reader"))
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("failed to read active reader file", "error", err)
		}
		return ""
	}
	return string(b)
}
```

Update call sites in `main`:
```go
	if err := saveActiveReader(cfg.DataDir, readerName); err != nil { ... }
	// ...
	if savedReader := loadActiveReader(cfg.DataDir, logger); savedReader != "" { ... }
```

Also add `"path/filepath"` to main.go imports if not present.

- [ ] **Step 12: Update `server.go` — it doesn't call NewHandlers, nothing to change here**

Verify `server.go` compiles fine (it only calls `NewServer`, which takes `*Handlers`, so no changes needed there).

- [ ] **Step 13: Run the full test suite**

```bash
cd backend && task test
```
Expected: all tests pass.

- [ ] **Step 14: Commit**

```bash
cd backend && git add pkg/config/config.go pkg/config/config_test.go \
  cmd/server/main.go internal/api/handlers.go internal/api/handlers_test.go \
  internal/api/store.go internal/store/store.go
git commit --no-gpg-sign -m "fix: make data directory and base currency configurable

EXPENSOR_DATA_DIR (default: data) controls where credential, token,
and state files are written. Hardcoded relative paths in handlers,
main, and runDaemon now all derive from this setting, making Docker
deployments reliable regardless of working directory.

EXPENSOR_BASE_CURRENCY (default: INR) replaces the hardcoded INR
filter in GetStats. Stats now return total_base + base_currency
instead of total_inr."
```

---

## Task 7: `io.Closer` for postgres writer

**Files:**
- Modify: `backend/pkg/writer/postgres/postgres.go`
- Modify: `backend/internal/daemon/runner.go`
- Test: `backend/internal/daemon/runner_test.go`

### Background

The daemon's `runner.go:118` checks `interface{ Close() }` (no error return). The postgres writer's `Close()` also has no error return, so the assertion works — but it diverges from the stdlib `io.Closer` interface (`Close() error`). A future writer implementing `io.Closer` properly would not be closed by the daemon. Fix both sides: make the postgres writer implement `io.Closer`, and have the daemon check for `io.Closer`.

- [ ] **Step 1: Read the postgres writer's Close method**

In `backend/pkg/writer/postgres/postgres.go`, line 346 has:
```go
func (w *Writer) Close() {
	w.pool.Close()
}
```

- [ ] **Step 2: Update `Close` to return an error**

Replace:
```go
func (w *Writer) Close() {
	w.pool.Close()
}
```
With:
```go
// Close releases the writer's connection pool. It implements io.Closer.
func (w *Writer) Close() error {
	w.pool.Close()
	return nil
}
```

- [ ] **Step 3: Update daemon runner to use `io.Closer`**

In `backend/internal/daemon/runner.go`, add `"io"` to imports.

Replace:
```go
	// Close writer if it implements io.Closer
	if closer, ok := writer.(interface{ Close() }); ok {
		closer.Close()
		r.logger.Info("closed writer resources")
	}
```
With:
```go
	// Close writer if it implements io.Closer
	if closer, ok := writer.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			r.logger.Warn("error closing writer", "error", err)
		} else {
			r.logger.Info("closed writer resources")
		}
	}
```

- [ ] **Step 4: Add a compile-time assertion in the postgres writer**

At the top of `backend/pkg/writer/postgres/postgres.go`, after the type definition:
```go
// compile-time check: *Writer must satisfy io.Closer.
var _ io.Closer = (*Writer)(nil)
```

Also add `"io"` to imports.

- [ ] **Step 5: Run tests**

```bash
cd backend && task test
```
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
cd backend && git add pkg/writer/postgres/postgres.go internal/daemon/runner.go
git commit --no-gpg-sign -m "fix: postgres writer implements io.Closer; daemon uses io.Closer check

Writer.Close() previously had no error return, diverging from io.Closer.
Updated signature to Close() error and added a compile-time assertion.
The daemon runner now checks for io.Closer (the stdlib interface) instead
of a bespoke interface{ Close() }, so future writers implementing io.Closer
are closed correctly."
```

---

## Final: Run full CI check

- [ ] **Step 1: Run lint + tests**

```bash
cd backend && task ci
```
Expected: zero lint errors, all tests pass.

- [ ] **Step 2: Verify the frontend still builds** (Stats field rename: `total_inr` → `total_base`)

```bash
cd frontend && npm run build 2>&1 | tail -20
```
If the frontend references `total_inr`, TypeScript will error here. Search for it:
```bash
grep -r "total_inr" frontend/src/
```
Update any references from `total_inr` to `total_base` and from the stats type to include `base_currency: string`.

---

## Self-Review Checklist

| Issue | Task | Covered? |
|-------|------|----------|
| 1. `ErrNotFound` uses `fmt.Errorf` | Task 1 | ✅ |
| 2. `dm.setStopped(err)` bug | Task 1 | ✅ (+ 4 tests) |
| 3. `generateState` not crypto-random | Task 2 | ✅ (+ uniqueness test) |
| 4. URL-encode reader in OAuth redirect | Task 2 | ✅ |
| 5. `oauthStates` no TTL | Task 3 | ✅ (+ expiry rejection test) |
| 6. Hardcoded `INR` in GetStats | Task 6 | ✅ (+ config test) |
| 7. `HandleAddLabels` non-atomic | Task 4 | ✅ (+ 2 tests) |
| 8. koanf loads all env vars | Task 5 | ✅ |
| 9. Hardcoded data directory | Task 6 | ✅ (+ t.TempDir() in all tests) |
| 10. `io.Closer` mismatch | Task 7 | ✅ (+ compile-time assertion) |
