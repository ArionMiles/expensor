# Backup And Restore DB Runtime Follow-Up Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Update the deferred backup/restore design and implementation plan for DB-backed runtime state after Slice 4 moves `state.json` and reader runtime config into PostgreSQL.

**Status:** Blocked on multi-tenant households as of 2026-05-17.

**Priority note:** Do not implement backup/restore before multi-tenant household scoping is designed and shipped. This document remains useful as a reminder that the older backup design must eventually replace `state.json` with DB-backed `processed_messages` and include only portable reader runtime config, but implementation is intentionally deferred.

**Architecture:** Treat DB runtime state as first-class backup data where it is user-portable, while keeping machine-specific OAuth secrets/tokens excluded by default. Replace the old `state.json` backup path with `processed_messages`.

**Tech Stack:** Go backup package, pgx/v5 transaction restore, gzip JSON, HTTP handlers, React Settings tab.

---

## Scope

This scope applies only after the multi-tenant household dependency is satisfied.

In scope:

- Update backup format from `backup_version: 1` to `backup_version: 2`.
- Include `processed_messages` from DB instead of `state.json`.
- Include active reader and reader config only where useful.
- Keep OAuth client secrets and tokens excluded by default.
- Add tests proving backup/restore works after DB runtime state migration.

Out of scope:

- Implementing multi-tenant backup scoping.
- Scheduled backups.
- Exporting OAuth secrets/tokens by default.

---

### Task 1: Update Backup/Restore Spec

**Files:**
- Modify: `docs/superpowers/specs/2026-05-19-backup-restore.md`

- [ ] **Step 1: Update included/excluded tables**

Change included runtime data:

```markdown
| `processed_messages` table | Dedup state — tracks processed email message keys |
| `app_config.active_reader` | Current reader selection |
| `reader_runtime.config` | Reader setup config, such as Thunderbird profile/mailboxes |
```

Change exclusions:

```markdown
| `reader_runtime.client_secret` | OAuth credentials are machine/project specific — re-upload on a new machine |
| `reader_runtime.oauth_token` | OAuth tokens expire and are regenerated through auth |
```

Remove language saying restore writes `state.json`.

- [ ] **Step 2: Add v2 schema**

Update example:

```json
{
  "backup_version": 2,
  "data": {
    "processed_messages": [
      { "message_key": "<sha256-hash>", "processed_at": "2026-01-15T10:32:00Z" }
    ],
    "reader_runtime": [
      { "reader": "thunderbird", "config": { "config": { "mailboxes": "Inbox" } } }
    ]
  }
}
```

- [ ] **Step 3: Add changelog row**

```markdown
| 2 | processed_messages, reader_runtime.config, active_reader in app_config | Replaces file-backed state.json backup after runtime state moved into DB |
```

- [ ] **Step 4: Commit spec update**

```bash
git add docs/superpowers/specs/2026-05-19-backup-restore.md
git commit --no-gpg-sign -m "docs: update backup spec for db runtime state"
```

---

### Task 2: Add Backup Data Structures

**Files:**
- Create: `backend/internal/backup/backup.go`
- Create: `backend/internal/backup/backup_test.go`

- [ ] **Step 1: Write failing serialization tests**

```go
func TestBackupDataVersion2SerializesRuntimeState(t *testing.T) {
	data := backup.BackupData{
		BackupVersion: 2,
		Data: backup.Data{
			ProcessedMessages: []backup.ProcessedMessageBackup{{MessageKey: "msg-1", ProcessedAt: time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)}},
			ReaderRuntime: []backup.ReaderRuntimeBackup{{Reader: "thunderbird", Config: json.RawMessage(`{"config":{"mailboxes":"Inbox"}}`)}},
		},
	}

	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !bytes.Contains(raw, []byte(`"backup_version":2`)) {
		t.Fatalf("missing version: %s", raw)
	}
	if bytes.Contains(raw, []byte("oauth_token")) || bytes.Contains(raw, []byte("client_secret")) {
		t.Fatalf("secret fields must not be serialized: %s", raw)
	}
}
```

- [ ] **Step 2: Run backend tests**

Run: `task test:be`

Expected: FAIL because backup package does not exist.

- [ ] **Step 3: Implement structs**

Create versioned structs:

```go
const CurrentVersion = 2

type BackupData struct {
	BackupVersion  int       `json:"backup_version"`
	CreatedAt      time.Time `json:"created_at"`
	ExpensorVersion string    `json:"expensor_version"`
	Summary         Summary   `json:"summary"`
	Data            Data      `json:"data"`
}

type ProcessedMessageBackup struct {
	MessageKey  string    `json:"message_key"`
	ProcessedAt time.Time `json:"processed_at"`
}

type ReaderRuntimeBackup struct {
	Reader string          `json:"reader"`
	Config json.RawMessage `json:"config,omitempty"`
}
```

Do not include client secret or OAuth token fields.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/backup/backup.go backend/internal/backup/backup_test.go
git commit --no-gpg-sign -m "feat: add backup v2 data model"
```

---

### Task 3: Implement DB Backup Export

**Files:**
- Modify: `backend/internal/backup/backup.go`
- Modify: `backend/internal/backup/backup_test.go`

- [ ] **Step 1: Write failing integration test**

```go
func TestBackupIncludesProcessedMessagesAndReaderConfig(t *testing.T) {
	ts := newTestStore(t)
	ctx := context.Background()
	must(t, ts.MarkMessageProcessed(ctx, "msg-1", time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)))
	must(t, ts.SetReaderConfig(ctx, "thunderbird", json.RawMessage(`{"config":{"mailboxes":"Inbox"}}`)))

	data, err := backup.Create(ctx, ts.PoolForTest(), "test-version")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(data.Data.ProcessedMessages) != 1 {
		t.Fatalf("processed messages = %+v", data.Data.ProcessedMessages)
	}
	if len(data.Data.ReaderRuntime) != 1 {
		t.Fatalf("reader runtime = %+v", data.Data.ReaderRuntime)
	}
}
```

- [ ] **Step 2: Run backend tests**

Run: `task test:be`

Expected: FAIL because export queries do not exist.

- [ ] **Step 3: Implement export queries**

Query in dependency order:

- labels
- categories
- buckets
- rules
- app_config
- transactions
- transaction_labels
- processed_messages
- reader_runtime config only

For reader runtime:

```sql
SELECT reader, config
FROM reader_runtime
WHERE config IS NOT NULL
ORDER BY reader
```

- [ ] **Step 4: Add gzip helpers**

Add:

```go
func EncodeGzip(data BackupData) ([]byte, error)
func DecodeGzip(raw []byte) (BackupData, error)
```

- [ ] **Step 5: Run tests and commit**

Run: `task test:be`

Expected: PASS.

```bash
git add backend/internal/backup/backup.go backend/internal/backup/backup_test.go
git commit --no-gpg-sign -m "feat: export db runtime backup data"
```

---

### Task 4: Implement Restore Preview And Apply

**Files:**
- Modify: `backend/internal/backup/backup.go`
- Modify: `backend/internal/backup/backup_test.go`

- [ ] **Step 1: Write failing restore test**

```go
func TestRestoreVersion2RestoresProcessedMessages(t *testing.T) {
	ts := newTestStore(t)
	ctx := context.Background()
	data := backup.BackupData{
		BackupVersion: 2,
		Data: backup.Data{
			ProcessedMessages: []backup.ProcessedMessageBackup{{MessageKey: "msg-1", ProcessedAt: time.Now()}},
		},
	}

	if err := backup.Restore(ctx, ts.PoolForTest(), data); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	ok, err := ts.IsMessageProcessed(ctx, "msg-1")
	if err != nil || !ok {
		t.Fatalf("processed message not restored ok=%v err=%v", ok, err)
	}
}
```

- [ ] **Step 2: Run tests**

Run: `task test:be`

Expected: FAIL because restore is not implemented.

- [ ] **Step 3: Implement preview**

Add:

```go
func Preview(data BackupData) (PreviewResult, error)
```

Validate:

- `backup_version` is 1 or 2.
- Future versions return an explicit unsupported-version error.
- Missing v2 runtime arrays default to empty.

- [ ] **Step 4: Implement transactional restore**

Restore inside one transaction. Insert `processed_messages` after independent config tables. Restore `reader_runtime.config` but never restore OAuth token or client secret.

- [ ] **Step 5: Run tests and commit**

Run: `task test:be`

Expected: PASS.

```bash
git add backend/internal/backup/backup.go backend/internal/backup/backup_test.go
git commit --no-gpg-sign -m "feat: restore db runtime backup data"
```

---

### Task 5: API And Settings UI

**Files:**
- Modify: `backend/internal/api/handlers.go`
- Modify: `backend/internal/api/server.go`
- Modify: `backend/internal/api/handlers_test.go`
- Modify: `frontend/src/api/client.ts`
- Modify: `frontend/src/api/queries.ts`
- Create: `frontend/src/pages/settings/BackupSettings.tsx`
- Modify: `frontend/src/pages/Settings.tsx`

- [ ] **Step 1: Write failing handler tests**

```go
func TestHandleBackupRejectsRunningDaemon(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{running: true})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/backup", nil)
	rr := httptest.NewRecorder()

	h.HandleBackup(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d", rr.Code)
	}
}
```

- [ ] **Step 2: Run backend tests**

Run: `task test:be`

Expected: FAIL because handlers do not exist.

- [ ] **Step 3: Implement handlers and routes**

Add:

- `POST /api/backup`
- `POST /api/restore`

Use the existing daemon status check. `POST /api/restore` without `confirm=true` returns preview only.

- [ ] **Step 4: Add frontend tests and UI**

Add tests for:

- Backup button disabled when daemon running.
- Restore file preview appears.
- ConfirmModal is used before apply.

Implement `BackupSettings` under Settings tab without native alert/confirm/prompt.

- [ ] **Step 5: Run tests and commit**

Run: `task test:be`

Expected: PASS.

Run: `task test:fe`

Expected: PASS.

```bash
git add backend/internal/api frontend/src
git commit --no-gpg-sign -m "feat: add backup restore workflow"
```

---

### Task 6: Final Verification

- [ ] **Step 1: Run backend verification**

Run: `task lint:be:prod`

Expected: `0 issues`.

Run: `task test:be`

Expected: PASS.

- [ ] **Step 2: Run frontend verification**

Run: `task lint:fe`

Expected: PASS.

Run: `task test:fe`

Expected: PASS.

- [ ] **Step 3: Run OpenAPI check**

Run: `task openapi:check`

Expected: PASS.

- [ ] **Step 4: Commit fixes if needed**

```bash
git add backend frontend docs api
git commit --no-gpg-sign -m "test: verify backup restore workflow"
```

Only create this commit if verification changed files.
