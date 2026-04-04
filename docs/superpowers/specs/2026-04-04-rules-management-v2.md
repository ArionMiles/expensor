# Rules Management v2 — Design Spec

**Date:** 2026-04-04
**Scope:** Promote Rules to a first-class sidebar route; replace the slide-over form with a full-page create/edit experience; add multi-sample regex tester; add retroactive scan on rule save; fix table UX issues; enable client-side export of selected rules.

---

## Overview

Rules Management moves from a Settings tab with a slide-over panel to its own dedicated section in the sidebar. The create/edit experience becomes a full page with a live multi-sample email body tester. Saving an edited rule triggers a retroactive scan of the lookback window by default, with a one-step opt-out. Table UX is hardened: checkboxes on every row, always-visible Delete column, bulk enable/disable/delete, and a whitespace-nowrap fix on the Source badge.

---

## 1. Routing & Navigation

### New routes
| Path | Component | Purpose |
|------|-----------|---------|
| `/rules` | `RulesPage` | List page |
| `/rules/new` | `RuleFormPage` | Create form |
| `/rules/:id` | `RuleFormPage` | Edit form |

### Sidebar
Add `Rules` entry to `SECONDARY_NAV` (between Onboarding and Settings) using the `ScrollText` icon from lucide-react.

### Removals
- Remove the `'rules'` tab from `frontend/src/pages/Settings.tsx`
- Delete `frontend/src/pages/settings/RulesSettings.tsx`

---

## 2. Rules List Page (`/rules`)

### Table columns
`☐ | Enabled | Name | Sender | Subject | Source | Edit | Delete`

- **Checkbox column** — header checkbox toggles select-all/deselect-all for all visible rows
- **Enabled** — green/grey dot (read-only indicator; bulk toggle via action bar)
- **Source badge** — `inline-flex shrink-0 items-center gap-1 whitespace-nowrap`; system rules show `🔒 System`, user rules show `User`
- **Edit** — always visible; navigates to `/rules/:id`
- **Delete** — always visible; greyed out + `cursor-not-allowed` + `title="System rules cannot be deleted"` for system rules; clicking Delete on a user rule shows a confirm dialog then calls `DELETE /api/rules/:id`

### Bulk action toolbar
Appears above the table when ≥1 row is selected. Hidden when nothing is selected.

Buttons:
- `Enable (N)` — calls `ToggleRule(id, true)` for each selected rule
- `Disable (N)` — calls `ToggleRule(id, false)` for each selected rule
- `Delete (N)` — deletes only the user rules in the selection; if the selection contains system rules they are skipped silently with a note "N system rule(s) skipped"

### Export
- Button label: `Export (N selected)` when ≥1 row checked; `Export` (disabled) when nothing is selected
- **Client-side only** — no backend call; transforms selected `Rule` objects to the `RuleImport` wire format (`senderEmail`, `merchantInfoRegex`, etc.) and downloads via `Blob` + `URL.createObjectURL`. No changes to the backend export endpoint needed.

### Import
Unchanged: file picker → `POST /api/rules/import`.

### Action bar layout
```
[+ New rule]  [Export (N selected)]  [Import]     ← left side
                         [Enable (N)] [Disable (N)] [Delete (N)]  ← bulk, right side, shown when ≥1 selected
```

---

## 3. Rule Form Page (`/rules/new`, `/rules/:id`)

### Layout
Full page. Breadcrumb: `Rules › New Rule` or `Rules › {rule name}`. Single scrollable column, max-width `max-w-2xl`.

### Fields
| Field | Required | Notes |
|-------|----------|-------|
| Name | Yes | Text input |
| Sender email | No | Text input |
| Subject contains | No | Text input |
| Amount regex | Yes | Monospace input |
| Merchant regex | Yes | Monospace input |
| Currency regex | No | Monospace input |
| Enabled | — | Toggle; default on |

For **system rules in edit mode**: all fields are read-only except Enabled. The regex tester still works against the existing regex values.

### Multi-sample email body tester
Located below the regex fields in a clearly labelled "Test Regexes" section.

**Inputs:**
- Starts with one empty `<textarea>` (placeholder: `Paste an email body here…`)
- `+ Add sample` button appends another textarea
- Each textarea (beyond the first) has a `✕` remove button

**Live results table** (updates on every keystroke in any field or textarea):

| # | Body preview | Amount | Merchant | Currency |
|---|---|---|---|---|
| 1 | _first 60 chars…_ | `1234.50` ✓ | `Swiggy` ✓ | — |
| 2 | _first 60 chars…_ | `45.00` ✓ | `Amazon` ✓ | `USD` ✓ |

Cell states:
- **Match** — capture group 1 value in green
- **No match** — `—` in muted
- **Invalid regex** — `⚠ invalid` in orange

If a regex field is empty, its column shows `—` for all samples.

### Retroactive scan option (edit mode only, not shown on create)
```
☑  Retroactive scan — re-process emails from the past {lookback_days} days
```
Checked by default. Unchecking opts out.

### Save flow

**Create (`/rules/new`):**
1. `POST /api/rules`
2. Redirect to `/rules` on success

**Edit (`/rules/:id`):**
1. `PUT /api/rules/:id`
2. If retroactive scan is checked: `POST /api/daemon/rescan { "reader": activeReader }`
   - Response `status: "rescanning"` → show toast: _"Rule saved. Retroactive scan started."_
   - Response `status: "queued"` → show toast: _"Rule saved. Retroactive scan queued — will run on the next daemon start."_
3. Redirect to `/rules`

**Active reader** for the rescan is read from `GET /api/config/active-reader` (see §4), which reads the persisted `data/active_reader` file set during onboarding.

---

## 4. Backend — Retroactive Scan

### New store key
`app_config["rescan_pending"]` — `"true"` / `""` (empty = clear).

### New endpoint: `POST /api/daemon/rescan`
**Body:** `{ "reader": "gmail" }`

| Condition | Response |
|-----------|----------|
| Daemon not running | `202 {"status": "rescanning"}` — calls `rescanFn(reader)` |
| Daemon running | `202 {"status": "queued"}` — sets `app_config["rescan_pending"] = "true"` |
| Reader not found | `400 {"error": "reader not found"}` |
| No store | `503` |

### `rescanFn` in `main.go`
Mirrors `startDaemon` with two differences:
1. Skips `state.New` — leaves `RunConfig.StateManager = nil`
2. Sets `RunConfig.ForceRescan = true`

Readers already guard with `if r.state != nil && r.state.IsProcessed(...)` — nil StateManager naturally bypasses dedup.

### `startDaemon` change
On each daemon start, checks `app_config["rescan_pending"]`:
- If `"true"`: calls rescan variant (ForceRescan=true, nil StateManager), clears the flag via `SetAppConfig("rescan_pending", "")` immediately after the first forced scan cycle completes (before the daemon enters its normal interval loop)
- Otherwise: normal start

### `daemon.RunConfig` change
Add `ForceRescan bool`. In `runDaemon`, when `ForceRescan == true`, skip `state.New` and pass nil StateManager.

### `Handlers` struct change
Add `rescanFn func(reader string)` alongside existing `startFn`. Update `NewHandlers` signature (add as the 12th parameter; existing `//nolint:revive` comment covers it).

### Helper endpoint: `GET /api/config/active-reader`
Returns `{ "reader": "gmail" }` (or `""` if none set). Reads from `data/active_reader` file (same as `loadActiveReader`). Needed by the frontend to know which reader to pass to `/api/daemon/rescan`.

---

## 5. Files Created / Modified

| File | Change |
|------|--------|
| `backend/internal/daemon/runner.go` | Add `ForceRescan bool` to `RunConfig` |
| `backend/cmd/server/main.go` | Add `rescanDaemon` func; check `rescan_pending` in `startDaemon`; pass `rescanFn` to `NewHandlers` |
| `backend/internal/api/handlers.go` | Add `HandleRescan`; add `rescanFn` field to `Handlers`; add `HandleGetActiveReader` |
| `backend/internal/api/handlers_test.go` | Add mock + 2 tests for `HandleRescan` |
| `backend/internal/api/server.go` | Register `POST /api/daemon/rescan` and `GET /api/config/active-reader` |
| `frontend/src/components/Sidebar.tsx` | Add Rules nav entry |
| `frontend/src/App.tsx` | Add `/rules`, `/rules/new`, `/rules/:id` routes |
| `frontend/src/pages/Rules.tsx` | New — list page |
| `frontend/src/pages/rules/RuleForm.tsx` | New — create/edit page |
| `frontend/src/pages/Settings.tsx` | Remove Rules tab |
| `frontend/src/pages/settings/RulesSettings.tsx` | Delete |
| `frontend/src/api/client.ts` | Add `api.daemon.rescan(reader)` and `api.config.getActiveReader()` |
| `frontend/src/api/queries.ts` | Add `useRescan()` mutation |
