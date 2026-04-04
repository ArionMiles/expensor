# Backup & Restore — Design Spec

**Date:** 2026-04-04
**Status:** DEFERRED — implement **after** multi-tenant ships

---

## Goal

Allow users to export a portable snapshot of their Expensor data (transactions, rules, labels, categories, configuration) to a single file, and restore from that file on a fresh or existing installation. Primary use-cases: migrating to a new machine, and periodic safety backups of accumulated user data (custom rules, transaction descriptions, labels).

---

## What is (and is not) backed up

### Included

| Source | Contents |
|--------|---------|
| `transactions` table | All transactions including user-edited description, category, bucket |
| `transaction_labels` table | Label associations |
| `labels` table | Label taxonomy (name, color) |
| `categories` table | Category definitions |
| `buckets` table | Bucket definitions |
| `rules` table | All rules (system + user) |
| `app_config` table | Application settings (base_currency, scan_interval, lookback_days, rescan_pending) |
| `state.json` | Dedup state — tracks which emails have been processed |

### Excluded

| Item | Why |
|------|-----|
| `client_secret_*.json` | OAuth credentials are project-specific — re-upload on new machine |
| `token_*.json` | OAuth tokens expire and are re-generated via the auth flow |
| `active_reader` | Reader choice is re-configured through onboarding |
| PostgreSQL connection config | Infrastructure, not data |

### Why `state.json` is included

`state.json` stores SHA-256 hashes of `(source, messageID, date)`. The `messageID` is Gmail's (or Thunderbird's) globally unique identifier for an email — it is tied to the **email account**, not the machine. The same email has the same message ID regardless of which machine is running the daemon.

Restoring `state.json` to machine B means the daemon correctly skips emails it has already processed, preventing re-extraction. Excluding it would cause the daemon to re-process every email in the lookback window on first run, which — even with upsert handling — has two problems:

1. Transactions outside the lookback window (older than `lookback_days`) would be missed, creating a gap.
2. User-edited fields (see §ON CONFLICT fix below) would risk being overwritten before the fix is fully propagated.

---

## Backup file format

A single `.json.gz` file (gzipped JSON). Gzip keeps file size manageable for large transaction sets; the raw JSON is readable when unzipped for manual inspection or scripting.

### Schema

```json
{
  "backup_version": 1,
  "created_at": "2026-04-04T12:00:00Z",
  "expensor_version": "v1.2.3",
  "summary": {
    "transactions": 1247,
    "rules": 18,
    "labels": 12,
    "categories": 9,
    "buckets": 4,
    "state_entries": 3891
  },
  "data": {
    "transactions": [ ... ],
    "transaction_labels": [ ... ],
    "labels": [ ... ],
    "categories": [ ... ],
    "buckets": [ ... ],
    "rules": [ ... ],
    "app_config": [ {"key": "base_currency", "value": "INR"}, ... ]
  },
  "state": {
    "processed_messages": {
      "<sha256-hash>": "2026-01-15T10:32:00Z",
      ...
    }
  }
}
```

The `state` object mirrors the structure of `state.json` (map of SHA-256 key → timestamp). On restore, this is written back to `EXPENSOR_DATA_DIR/state.json`, replacing any existing state file.

### Version field

`backup_version` is a monotonically increasing integer. The restore logic checks this field and handles known versions. Unknown future versions are rejected with a clear error message. Old backups (lower version) are accepted — missing tables or columns are restored as empty/default.

**Current version: 1.** Every time a new feature adds a new DB table that should be backed up, `backup_version` is incremented and the restore logic for the previous version is documented.

---

## API

### `POST /api/backup`

Triggers a backup and streams the `.json.gz` file as a download.

- Response: `200` with `Content-Type: application/gzip`, `Content-Disposition: attachment; filename="expensor-backup-YYYYMMDD-HHMMSS.json.gz"`
- If the daemon is running: `409 {"error": "stop the daemon before creating a backup"}`
- On DB error: `500`

No request body. The full export is always complete (no partial/incremental exports).

### `POST /api/restore`

Accepts a backup file upload and restores data.

**Request:** `multipart/form-data` with field `file` containing the `.json.gz` backup.

**Behavior:**
1. Parse and validate the backup file (version check, JSON schema validation)
2. Return a preview response (row counts per table, version info) — does NOT apply yet
3. Client must make a second `POST /api/restore?confirm=true` to apply
4. On `confirm=true`: wrap entire restore in a DB transaction — truncate all target tables, insert backup data, commit
5. If the daemon is running: `409 {"error": "stop the daemon before restoring"}`

**Two-phase restore** (preview + confirm) prevents accidental data loss. The preview response:

```json
{
  "backup_version": 1,
  "created_at": "2026-04-04T12:00:00Z",
  "expensor_version": "v1.2.3",
  "summary": {
    "transactions": 1247,
    "rules": 18,
    "labels": 12,
    "categories": 9,
    "buckets": 4
  },
  "warnings": [
    "Backup was created on v1.1.0; restoring to v1.2.3 — new fields will use defaults"
  ]
}
```

The client shows this to the user before asking for confirmation.

---

## UI

New **"Backup & Restore"** tab in Settings (alongside appearance, labels, categories, etc.).

### Backup section

```
Backup your data
──────────────────────────────────────────────────────
Download a snapshot of all your transactions, rules,
labels, and settings. Use this to migrate to a new
machine or keep a periodic safety copy.

  [↓ Download backup]
```

If daemon is running, the button is disabled with tooltip: "Stop the daemon before creating a backup."

### Restore section

```
Restore from backup
──────────────────────────────────────────────────────
⚠ This will replace all current data.

  [Choose backup file...]

```

On file selection: show a preview card (transaction count, rules count, backup date, version). Show a `ConfirmModal` before applying:

> **Replace all data?**
> This will overwrite all 1,247 transactions, 18 rules, 12 labels, and settings with the contents of the backup from April 4 2026. This cannot be undone.
>
> [Cancel] [Replace with backup]

After successful restore: redirect to Dashboard, show a success toast.

---

## Forward-compatibility rule

> **Every new feature that adds a DB table containing user data MUST include that table in the backup.**

Enforcement checklist (add to PR template and code review):

1. Does this feature add a new DB table with user-generated content? → Add it to `BackupData` struct and the backup/restore SQL queries.
2. Does the new table have a foreign key to `transactions`? → Ensure the restore inserts transactions before the dependent table.
3. Increment `backup_version` and add migration notes in the backup version changelog (maintained in this spec or a companion doc).

---

## Backup version changelog

| Version | Added tables | Notes |
|---------|-------------|-------|
| 1 | transactions, transaction_labels, labels, categories, buckets, rules, app_config + state.json | Initial |

---

## Implementation notes

### Go backup logic

A single `Backup(ctx, pool) ([]byte, error)` function in a new `backend/internal/backup/` package. Queries all tables in dependency order (labels before transaction_labels, etc.), marshals to the JSON schema above, gzips, returns bytes.

A `Restore(ctx, pool, data BackupData) error` function runs inside a single `pgx` transaction: `TRUNCATE ... CASCADE` each target table (in reverse FK order), then batch-insert from the backup data.

### Restore FK ordering

Insert order to satisfy foreign keys:
1. `labels`, `categories`, `buckets`, `rules`, `app_config` (no FK dependencies)
2. `transactions`
3. `transaction_labels` (FK to transactions + labels)

### File size estimate

A typical user with 5,000 transactions: ~2–5 MB raw JSON, ~200–500 KB gzipped. The `state` object adds roughly 80 bytes per processed message; 5,000 entries ≈ 400 KB raw, negligible after gzip.

---

## ON CONFLICT fix — protect user-edited fields

### The problem

The postgres writer currently uses:

```sql
ON CONFLICT (message_id) DO UPDATE SET
    amount = EXCLUDED.amount,
    ...
    category = EXCLUDED.category,
    bucket   = EXCLUDED.bucket,
    description = EXCLUDED.description,
    updated_at = NOW()
```

This means any re-processing of an already-stored email (retroactive scan, daemon restart on a machine without `state.json`) silently overwrites user edits to `description`, `category`, and `bucket` with whatever the regex extracted.

### The fix

Split the fields into two groups:

**Always overwrite on conflict** (raw extraction results — the regex owns these):
- `amount`, `currency`, `original_amount`, `original_currency`, `exchange_rate`
- `timestamp`, `merchant_info`, `source`

**Never overwrite on conflict** (user-owned fields — preserve existing DB value):
- `description` — only ever entered by the user; the regex never produces a description
- `category` — may be set by rule extraction OR user edit; use `COALESCE` to keep existing non-null value
- `bucket` — same as category

Updated SQL:

```sql
ON CONFLICT (message_id) DO UPDATE SET
    amount            = EXCLUDED.amount,
    currency          = EXCLUDED.currency,
    original_amount   = EXCLUDED.original_amount,
    original_currency = EXCLUDED.original_currency,
    exchange_rate     = EXCLUDED.exchange_rate,
    timestamp         = EXCLUDED.timestamp,
    merchant_info     = EXCLUDED.merchant_info,
    source            = EXCLUDED.source,
    -- Preserve user edits: only update if the existing stored value is NULL/empty
    category    = COALESCE(NULLIF(transactions.category, ''), EXCLUDED.category),
    bucket      = COALESCE(NULLIF(transactions.bucket, ''), EXCLUDED.bucket),
    -- description is never set by extraction — never update it on conflict
    updated_at  = NOW()
```

### Retroactive scan behaviour after this fix

When a user saves a new rule and triggers a retroactive scan:
- `amount`, `merchant_info`, `currency` etc. are updated from the new regex extraction ✓
- `category`/`bucket` are updated only if the user has not already set them ✓
- `description` is never touched ✓

This is the correct behaviour: the user's intent when editing category/bucket is to override the auto-extraction permanently. If they want to reset to the auto-extracted value, they can clear the field in the UI.

### Files affected by this fix

`backend/pkg/writer/postgres/postgres.go` — update the `ON CONFLICT DO UPDATE SET` clause in `writeBatch`.

---

## Future considerations

### Multi-tenant

**Backup & Restore is intentionally implemented after multi-tenant ships.** This ensures backup v1 is the only format ever needed — it is designed multi-user-aware from day one. Shipping backup before multi-tenant would create a single-user v1 format that then requires a v1→v2 migration path to support per-account scoping, adding permanent backwards-compatibility complexity.

Once multi-tenant is live, backup is scoped to the authenticated user's account. The `POST /api/backup` endpoint returns only the requesting user's data. `POST /api/restore` only restores into the requesting user's account. A backup from user A cannot be restored into user B's account (enforced via the authenticated `user_id` in the restore handler).

### Family linking / multi-account

Out of scope for backup — each account backs up its own data independently. Cross-account data (shared labels, family transaction views) would need a separate export format defined when that feature is built.

### Scheduled backups

Not in scope for initial implementation. Could be added as a cron-style setting in `app_config` with the scheduler infrastructure introduced for monthly Signal reports (Group F).

---

## Files Created / Modified

| File | Change |
|------|--------|
| `backend/pkg/writer/postgres/postgres.go` | Fix `ON CONFLICT DO UPDATE SET` — protect description; COALESCE for category/bucket |
| `backend/pkg/writer/postgres/postgres_test.go` | Tests for ON CONFLICT field-preservation behaviour |
| `backend/internal/backup/backup.go` | New — `Backup()`, `Restore()`, `BackupData` struct, version constants; reads/writes state.json |
| `backend/internal/backup/backup_test.go` | Unit tests with an in-memory PG test container |
| `backend/internal/api/handlers.go` | Add `HandleBackup`, `HandleRestorePreview`, `HandleRestoreConfirm` |
| `backend/internal/api/handlers_test.go` | Tests for backup/restore handlers |
| `backend/internal/api/server.go` | Register `POST /api/backup`, `POST /api/restore` |
| `frontend/src/api/client.ts` | Add `api.backup.download()`, `api.backup.preview(file)`, `api.backup.restore(file)` |
| `frontend/src/api/queries.ts` | Add `useBackupPreview()` mutation, `useRestore()` mutation |
| `frontend/src/pages/settings/BackupSettings.tsx` | New — backup download button + restore file picker + preview card |
| `frontend/src/pages/Settings.tsx` | Add "Backup & Restore" tab |
