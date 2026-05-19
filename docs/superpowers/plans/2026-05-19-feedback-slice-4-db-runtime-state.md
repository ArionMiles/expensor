# Feedback Slice 4 DB Runtime State Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Move disk-backed runtime state and reader configuration into PostgreSQL while keeping a temporary, isolated file-to-DB importer for existing installs.

**Status:** Complete. Implemented and merged to `main`; the compatibility importer remains intentionally isolated until the explicit removal follow-up is safe to run.

**Architecture:** Add DB-backed runtime tables and adapters, then replace direct file reads/writes in handlers, daemon startup, OAuth token persistence, and processed-message state. Legacy file migration lives in a small compatibility package with one startup call and an explicit deletion follow-up.

**Tech Stack:** Go, PostgreSQL migrations, pgx/v5, OAuth2 token JSON serialization, existing `app_config`, backend unit/integration tests, component tests where reader setup status changes.

---

## Scope

Move these disk-backed runtime values into DB-backed storage:

- `state.json` processed-message state.
- `active_reader`.
- `client_secret_<reader>.json`.
- `token_<reader>.json`.
- `config_<reader>.json` for Thunderbird and any future config-only reader.

Out of scope for this plan:

- Accessibility audit.
- i18n foundation.
- Repository-pattern rewrite.
- Removing the compatibility importer in the same release.

Compatibility constraint: the importer must be easy to delete later. It lives in one package, is called once at startup, does not participate in normal reads/writes after startup, and has a dedicated deletion task.

---

### Task 1: Runtime State Schema

**Files:**
- Create: `backend/migrations/014_runtime_state.sql` after Slice 2 diagnostics and Slice 3 search migrations.
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/store/store_test.go`

- [x] **Step 1: Write failing store tests**

Add tests for runtime state/config CRUD:

```go
func TestRuntimeState_ActiveReader(t *testing.T) {
	ts := newTestStore(t)
	ctx := context.Background()

	if err := ts.SetActiveReader(ctx, "gmail"); err != nil {
		t.Fatalf("SetActiveReader: %v", err)
	}
	got, err := ts.GetActiveReader(ctx)
	if err != nil {
		t.Fatalf("GetActiveReader: %v", err)
	}
	if got != "gmail" {
		t.Fatalf("active reader = %q", got)
	}
}

func TestRuntimeState_ReaderBlobAndConfig(t *testing.T) {
	ts := newTestStore(t)
	ctx := context.Background()

	if err := ts.SetReaderSecret(ctx, "gmail", []byte(`{"installed":{}}`)); err != nil {
		t.Fatalf("SetReaderSecret: %v", err)
	}
	secret, ok, err := ts.GetReaderSecret(ctx, "gmail")
	if err != nil || !ok {
		t.Fatalf("GetReaderSecret: ok=%v err=%v", ok, err)
	}
	if string(secret) != `{"installed":{}}` {
		t.Fatalf("secret = %s", secret)
	}

	if err := ts.SetReaderConfig(ctx, "thunderbird", json.RawMessage(`{"config":{"mailboxes":"Inbox"}}`)); err != nil {
		t.Fatalf("SetReaderConfig: %v", err)
	}
	cfg, ok, err := ts.GetReaderConfig(ctx, "thunderbird")
	if err != nil || !ok {
		t.Fatalf("GetReaderConfig: ok=%v err=%v", ok, err)
	}
	if !bytes.Contains(cfg, []byte(`"Inbox"`)) {
		t.Fatalf("config = %s", cfg)
	}
}
```

- [x] **Step 2: Run tests to verify they fail**

Run: `task test:be`

Expected: FAIL because runtime state tables and store methods do not exist.

- [x] **Step 3: Add migration**

Create `backend/migrations/014_runtime_state.sql`:

```sql
CREATE TABLE IF NOT EXISTS reader_runtime (
    reader TEXT PRIMARY KEY,
    client_secret JSONB,
    oauth_token JSONB,
    config JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS processed_messages (
    message_key TEXT PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO app_config (key, value)
VALUES ('active_reader', '')
ON CONFLICT (key) DO NOTHING;
```

- [x] **Step 4: Implement store methods**

Add methods to `Store`:

```go
func (s *Store) SetActiveReader(ctx context.Context, reader string) error
func (s *Store) GetActiveReader(ctx context.Context) (string, error)
func (s *Store) SetReaderSecret(ctx context.Context, reader string, secret []byte) error
func (s *Store) GetReaderSecret(ctx context.Context, reader string) ([]byte, bool, error)
func (s *Store) SetReaderToken(ctx context.Context, reader string, token []byte) error
func (s *Store) GetReaderToken(ctx context.Context, reader string) ([]byte, bool, error)
func (s *Store) DeleteReaderToken(ctx context.Context, reader string) error
func (s *Store) SetReaderConfig(ctx context.Context, reader string, config json.RawMessage) error
func (s *Store) GetReaderConfig(ctx context.Context, reader string) (json.RawMessage, bool, error)
func (s *Store) DeleteReaderRuntime(ctx context.Context, reader string) error
func (s *Store) IsMessageProcessed(ctx context.Context, key string) (bool, error)
func (s *Store) MarkMessageProcessed(ctx context.Context, key string, at time.Time) error
```

Use upserts with `updated_at = NOW()`. Keep reader names validated by existing plugin lookup in handlers and daemon setup; store methods still reject blank reader names.

- [x] **Step 5: Run tests**

Run: `task test:be`

Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add backend/migrations backend/internal/store/store.go backend/internal/store/store_test.go
git commit --no-gpg-sign -m "feat: add db runtime state storage"
```

---

### Task 2: DB-Backed Processed Message State

**Files:**
- Modify: `backend/pkg/state/state.go`
- Modify: `backend/pkg/state/state_test.go`
- Modify: `backend/cmd/server/main.go`

- [x] **Step 1: Write failing state tests**

Add tests for a DB-backed manager using a fake store:

```go
type fakeProcessedMessageStore struct {
	processed map[string]time.Time
}

func (f *fakeProcessedMessageStore) IsMessageProcessed(_ context.Context, key string) (bool, error) {
	_, ok := f.processed[key]
	return ok, nil
}

func (f *fakeProcessedMessageStore) MarkMessageProcessed(_ context.Context, key string, at time.Time) error {
	f.processed[key] = at
	return nil
}

func TestDBManagerMarksProcessedMessages(t *testing.T) {
	store := &fakeProcessedMessageStore{processed: map[string]time.Time{}}
	m := state.NewDBManager(store, slog.Default())

	if m.IsProcessed("msg-1") {
		t.Fatal("message should not start processed")
	}
	if err := m.MarkProcessed("msg-1"); err != nil {
		t.Fatalf("MarkProcessed: %v", err)
	}
	if !m.IsProcessed("msg-1") {
		t.Fatal("message should be processed")
	}
}
```

- [x] **Step 2: Run tests to verify they fail**

Run: `task test:be`

Expected: FAIL because DB state manager does not exist.

- [x] **Step 3: Add DB manager adapter**

Add a store interface in `state.go`:

```go
type ProcessedMessageStore interface {
	IsMessageProcessed(ctx context.Context, key string) (bool, error)
	MarkMessageProcessed(ctx context.Context, key string, at time.Time) error
}
```

Either adapt `Manager` to support a store-backed mode or add `DBManager` with the same public methods used by readers:

```go
func NewDBManager(store ProcessedMessageStore, logger *slog.Logger) *Manager
func (m *Manager) IsProcessed(msgKey string) bool
func (m *Manager) MarkProcessed(msgKey string) error
```

If keeping the existing `Manager` type, add optional `store ProcessedMessageStore` and branch in `IsProcessed`/`MarkProcessed`. On DB errors, log and return conservative values:

- `IsProcessed` returns `false` so a DB outage does not silently skip emails.
- `MarkProcessed` returns the error so callers can log failed checkpointing.

- [x] **Step 4: Wire daemon startup to DB state**

In `runDaemon`, construct `state.NewDBManager(pgStore, logger)` or the equivalent DB-backed manager instead of `state.New(cfg.StateFile, logger)`. Preserve the existing forced-rescan behavior: when `forceRescan` is true, keep `stateManager` nil.

- [x] **Step 5: Run tests**

Run: `task test:be`

Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add backend/pkg/state/state.go backend/pkg/state/state_test.go backend/cmd/server/main.go
git commit --no-gpg-sign -m "feat: use db-backed processed message state"
```

---

### Task 3: Reader Runtime Store Interface

**Files:**
- Modify: `backend/internal/api/store.go`
- Modify: `backend/internal/api/handlers.go`
- Modify: `backend/internal/api/handlers_test.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/internal/daemon/runner.go`

- [x] **Step 1: Write failing handler tests**

Update existing tests for active reader, credentials, token revoke, and reader config so they assert DB methods are called rather than files being written:

```go
func TestHandleUploadCredentials_SavesToStore(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := multipartCredentialsRequest(t, "/api/readers/gmail/credentials", `{"installed":{}}`)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()

	h.HandleUploadCredentials(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if string(ms.readerSecrets["gmail"]) != `{"installed":{}}` {
		t.Fatalf("secret was not persisted to store")
	}
}

func TestHandleSaveReaderConfig_SavesToStore(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/readers/thunderbird/config", strings.NewReader(`{"config":{"mailboxes":"Inbox"}}`))
	req.SetPathValue("name", "thunderbird")
	rr := httptest.NewRecorder()

	h.HandleSaveReaderConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(ms.readerConfigs["thunderbird"], []byte("Inbox")) {
		t.Fatalf("config not persisted: %s", ms.readerConfigs["thunderbird"])
	}
}
```

- [x] **Step 2: Run tests to verify they fail**

Run: `task test:be`

Expected: FAIL because handlers still use files.

- [x] **Step 3: Extend `Storer` and mock store**

Add runtime methods from Task 1 that handlers need:

```go
SetActiveReader(ctx context.Context, reader string) error
GetActiveReader(ctx context.Context) (string, error)
SetReaderSecret(ctx context.Context, reader string, secret []byte) error
GetReaderSecret(ctx context.Context, reader string) ([]byte, bool, error)
SetReaderToken(ctx context.Context, reader string, token []byte) error
GetReaderToken(ctx context.Context, reader string) ([]byte, bool, error)
DeleteReaderToken(ctx context.Context, reader string) error
SetReaderConfig(ctx context.Context, reader string, config json.RawMessage) error
GetReaderConfig(ctx context.Context, reader string) (json.RawMessage, bool, error)
DeleteReaderRuntime(ctx context.Context, reader string) error
```

- [x] **Step 4: Replace file-backed handlers**

Replace direct file operations:

- `HandleUploadCredentials`: validate JSON, call `SetReaderSecret`.
- `HandleCredentialsStatus`: call `GetReaderSecret`.
- `HandleAuthStart`: load secret JSON through `GetReaderSecret`.
- `exchangeAndSaveToken`: call `SetReaderToken`.
- `HandleAuthStatus`: load token JSON through `GetReaderToken`.
- `HandleRevokeToken`: call `DeleteReaderToken`.
- `HandleGetReaderConfig`: call `GetReaderConfig`.
- `HandleSaveReaderConfig`: validate JSON, call `SetReaderConfig`.
- `HandleDisconnectReader`: call `DeleteReaderRuntime`.
- `readActiveReader` / `saveActiveReader`: use `GetActiveReader` / `SetActiveReader`.

Keep JSON response shapes stable.

- [x] **Step 5: Replace daemon file loading**

In `applyPersistedReaderConfig`, load `config_<reader>.json` data from `GetReaderConfig` through a small interface passed into daemon config. In `runDaemon`, load OAuth credentials/token from DB instead of disk. If `pkg/client.New` only accepts paths, add a DB-token-aware constructor in Task 4 before switching daemon OAuth.

- [x] **Step 6: Run tests**

Run: `task test:be`

Expected: PASS.

- [x] **Step 7: Commit**

```bash
git add backend/internal/api/store.go backend/internal/api/handlers.go backend/internal/api/handlers_test.go backend/cmd/server/main.go backend/internal/daemon/runner.go
git commit --no-gpg-sign -m "feat: store reader runtime config in db"
```

---

### Task 4: DB OAuth Token Source

**Files:**
- Modify: `backend/pkg/client/oauth.go`
- Modify: `backend/pkg/client/oauth_test.go`
- Modify: `backend/cmd/server/main.go`

- [x] **Step 1: Write failing OAuth tests**

Add tests that load and save tokens through a store interface:

```go
type fakeTokenStore struct {
	token []byte
}

func (f *fakeTokenStore) GetReaderToken(_ context.Context, _ string) ([]byte, bool, error) {
	return f.token, len(f.token) > 0, nil
}

func (f *fakeTokenStore) SetReaderToken(_ context.Context, _ string, token []byte) error {
	f.token = token
	return nil
}

func TestNewFromJSONAndStoreLoadsToken(t *testing.T) {
	store := &fakeTokenStore{token: []byte(`{"access_token":"a","token_type":"Bearer","expiry":"2999-01-01T00:00:00Z"}`)}
	client, err := oauthclient.NewFromJSONAndStore(context.Background(), []byte(testSecretJSON), store, "gmail", gmail.GmailReadonlyScope)
	if err != nil {
		t.Fatalf("NewFromJSONAndStore: %v", err)
	}
	if client == nil {
		t.Fatal("client is nil")
	}
}
```

- [x] **Step 2: Run tests to verify they fail**

Run: `task test:be`

Expected: FAIL because store-backed OAuth does not exist.

- [x] **Step 3: Add token store interface and constructor**

In `oauth.go`, add:

```go
type ReaderTokenStore interface {
	GetReaderToken(ctx context.Context, reader string) ([]byte, bool, error)
	SetReaderToken(ctx context.Context, reader string, token []byte) error
}

func NewFromJSONAndStore(ctx context.Context, secretJSON []byte, store ReaderTokenStore, reader string, scope ...string) (*http.Client, error)
```

Implement a persisting token source that writes refreshed tokens with `SetReaderToken`. Keep existing file-based functions for compatibility tests and the temporary importer only.

- [x] **Step 4: Wire daemon OAuth**

In `runDaemon`, load client secret JSON from DB and call `client.NewFromJSONAndStore(ctx, secretJSON, pgStore, readerName, scopes...)`.

- [x] **Step 5: Run tests**

Run: `task test:be`

Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add backend/pkg/client/oauth.go backend/pkg/client/oauth_test.go backend/cmd/server/main.go
git commit --no-gpg-sign -m "feat: persist oauth tokens in db"
```

---

### Task 5: Temporary File-To-DB Importer

**Files:**
- Create: `backend/internal/compat/runtime_importer.go`
- Create: `backend/internal/compat/runtime_importer_test.go`
- Modify: `backend/cmd/server/main.go`

- [x] **Step 1: Write failing importer tests**

Create tests for importing legacy files:

```go
func TestRuntimeImporterImportsLegacyFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "active_reader"), "gmail")
	writeFile(t, filepath.Join(dir, "client_secret_gmail.json"), `{"installed":{}}`)
	writeFile(t, filepath.Join(dir, "token_gmail.json"), `{"access_token":"a","token_type":"Bearer"}`)
	writeFile(t, filepath.Join(dir, "config_thunderbird.json"), `{"config":{"mailboxes":"Inbox"}}`)
	writeFile(t, filepath.Join(dir, "state.json"), `{"processed_messages":{"msg-1":"2026-05-19T00:00:00Z"}}`)

	store := newFakeRuntimeStore()
	importer := compat.NewRuntimeImporter(dir, store, slog.Default())
	result, err := importer.Import(context.Background())
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.ImportedFiles != 5 {
		t.Fatalf("imported files = %d", result.ImportedFiles)
	}
	if store.activeReader != "gmail" || len(store.processed) != 1 {
		t.Fatalf("store not populated: %+v", store)
	}
}
```

- [x] **Step 2: Run tests to verify they fail**

Run: `task test:be`

Expected: FAIL because the compatibility importer does not exist.

- [x] **Step 3: Implement isolated importer**

Create `backend/internal/compat/runtime_importer.go` with:

```go
type RuntimeStore interface {
	SetActiveReader(ctx context.Context, reader string) error
	SetReaderSecret(ctx context.Context, reader string, secret []byte) error
	SetReaderToken(ctx context.Context, reader string, token []byte) error
	SetReaderConfig(ctx context.Context, reader string, config json.RawMessage) error
	MarkMessageProcessed(ctx context.Context, key string, at time.Time) error
}

type RuntimeImporter struct {
	dataDir string
	store   RuntimeStore
	logger *slog.Logger
}
```

The importer reads known legacy files once and writes their content to DB. It never writes back to disk and never becomes part of normal runtime reads/writes.

- [x] **Step 4: Call importer once at startup**

After migrations and store construction in `cmd/server/main.go`, call:

```go
if result, err := compat.NewRuntimeImporter(cfg.DataDir, pgStore, logger).Import(ctx); err != nil {
	logger.Warn("legacy runtime import failed", "error", err)
} else if result.ImportedFiles > 0 {
	logger.Info("legacy runtime files imported", "files", result.ImportedFiles)
}
```

Do not call the importer from handlers, daemon loops, readers, or token refresh paths.

- [x] **Step 5: Run tests**

Run: `task test:be`

Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add backend/internal/compat/runtime_importer.go backend/internal/compat/runtime_importer_test.go backend/cmd/server/main.go
git commit --no-gpg-sign -m "feat: import legacy runtime files to db"
```

---

### Task 6: Removal Plan For Temporary Importer

**Files:**
- Create: `docs/superpowers/plans/2026-05-19-remove-runtime-compat-importer.md`
- Modify: no production code in this task.

- [x] **Step 1: Write the deletion plan**

Create a follow-up plan with this exact scope:

```markdown
# Remove Runtime Compatibility Importer Implementation Plan

**Goal:** Delete the temporary file-to-DB runtime importer after existing users have updated Docker images.

## Delete

- `backend/internal/compat/runtime_importer.go`
- `backend/internal/compat/runtime_importer_test.go`
- The startup call to `compat.NewRuntimeImporter(...).Import(...)` in `backend/cmd/server/main.go`
- Any comments that mention temporary legacy runtime file import.

## Verify

- `rg -n "RuntimeImporter|legacy runtime|runtime_importer|client_secret_.*json|token_.*json|config_.*json|active_reader|state.json" backend`
- `task test:be`
- `task lint:be:prod`
```

- [x] **Step 2: Commit**

```bash
git add docs/superpowers/plans/2026-05-19-remove-runtime-compat-importer.md
git commit --no-gpg-sign -m "docs: plan runtime importer removal"
```

---

### Task 7: Final Verification

**Files:**
- Modify only if verification exposes drift.

- [x] **Step 1: Search for normal runtime file usage**

Run:

```bash
rg -n "os\\.ReadFile|os\\.WriteFile|os\\.Remove|client_secret_.*json|token_.*json|config_.*json|active_reader|state.json" backend
```

Expected: matches are limited to the temporary importer, tests, backup/restore docs or code if still intentionally file-oriented, and legacy file helper functions retained only for importer/tests.

- [x] **Step 2: Backend format/lint/tests**

Run: `task fmt:be`

Expected: PASS.

Run: `task lint:be:prod`

Expected: `0 issues`.

Run: `task test:be`

Expected: PASS.

- [x] **Step 3: Component and contract checks if API response shapes changed**

Run: `task test:be:component`

Expected: PASS.

Run: `task openapi:check`

Expected: PASS.

- [x] **Step 4: Commit final fixes if needed**

```bash
git add backend docs api
git commit --no-gpg-sign -m "test: verify db runtime state migration"
```

Only create this commit if verification changed files.

---

## Implementation Notes

- Do not remove the legacy importer during this slice.
- Do not keep syncing DB changes back to disk.
- Do not make the normal runtime path read legacy files after startup import.
- Keep the importer package name and startup log text easy to find with `rg`.
