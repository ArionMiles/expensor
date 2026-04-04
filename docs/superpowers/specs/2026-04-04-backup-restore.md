# Backup & Restore — Design Spec

**Date:** 2026-04-04
**Status:** DEFERRED — implement before any multi-tenant work begins

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

### Excluded

| Item | Why |
|------|-----|
| `client_secret_*.json` | OAuth credentials are account/project-specific — re-upload on new machine |
| `token_*.json` | OAuth tokens are machine-specific and expire |
| `state.json` | Dedup state — reconstructed naturally on first daemon run |
| `active_reader` | Reader choice is re-configured through onboarding |
| PostgreSQL connection config | Infrastructure, not data |

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
    "buckets": 4
  },
  "data": {
    "transactions": [ ... ],
    "transaction_labels": [ ... ],
    "labels": [ ... ],
    "categories": [ ... ],
    "buckets": [ ... ],
    "rules": [ ... ],
    "app_config": [ {"key": "base_currency", "value": "INR"}, ... ]
  }
}
```

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
| 1 | transactions, transaction_labels, labels, categories, buckets, rules, app_config | Initial |

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

A typical user with 5,000 transactions: ~2–5 MB raw JSON, ~200–500 KB gzipped.

---

## Future considerations

### Multi-tenant

When multi-tenant is added (users with individual accounts on a shared instance), backups must be scoped per account. The backup format is designed for this: add an `"account_id"` field to the root object. The restore endpoint would only restore data belonging to the authenticated account.

The `backup_version` field will be incremented when multi-tenant scope is added, and the restore logic will enforce that a backup from account A cannot be restored into account B.

### Family linking / multi-account

Out of scope for backup — each account backs up its own data independently. Cross-account data (shared labels, family transaction views) would need a separate export format defined when that feature is built.

### Scheduled backups

Not in scope for initial implementation. Could be added as a cron-style setting in `app_config` with the scheduler infrastructure introduced for monthly Signal reports (Group F).

---

## Files Created / Modified

| File | Change |
|------|--------|
| `backend/internal/backup/backup.go` | New — `Backup()`, `Restore()`, `BackupData` struct, version constants |
| `backend/internal/backup/backup_test.go` | Unit tests with an in-memory PG test container |
| `backend/internal/api/handlers.go` | Add `HandleBackup`, `HandleRestorePreview`, `HandleRestoreConfirm` |
| `backend/internal/api/handlers_test.go` | Tests for backup/restore handlers |
| `backend/internal/api/server.go` | Register `POST /api/backup`, `POST /api/restore` |
| `frontend/src/api/client.ts` | Add `api.backup.download()`, `api.backup.preview(file)`, `api.backup.restore(file)` |
| `frontend/src/api/queries.ts` | Add `useBackupPreview()` mutation, `useRestore()` mutation |
| `frontend/src/pages/settings/BackupSettings.tsx` | New — backup download button + restore file picker + preview card |
| `frontend/src/pages/Settings.tsx` | Add "Backup & Restore" tab |
