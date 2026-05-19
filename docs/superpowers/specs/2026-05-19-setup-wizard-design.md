# First-Time Setup Wizard — Design Spec

**Date:** 2026-05-19
**Status:** Approved
**Scope:** Non-dismissible modal wizard for first-time app configuration

---

## Problem

A freshly deployed Expensor instance requires the user to manually navigate to Settings and configure base currency, timezone, time format, and a reader before the daemon can run. There is no guided path, no enforced ordering, and no signal that setup is incomplete. New users have no clear starting point.

---

## Solution Overview

A non-dismissible modal wizard appears on every page load when `setup_complete = false` in `app_config`. It guides the user through four steps: base currency, timezone, time format, and reader selection. Each step eagerly saves to the DB on "Next". After step 4, the wizard redirects to the reader's setup page in Settings and hides itself for the current session. `setup_complete` is only set to `true` when the selected reader finishes its configuration (OAuth token saved for Gmail; profile path saved for Thunderbird).

---

## Backend

### Migration `011_setup_complete.sql`

```sql
INSERT INTO app_config (key, value) VALUES ('setup_complete', 'false')
    ON CONFLICT (key) DO NOTHING;
INSERT INTO app_config (key, value) VALUES ('active_reader', '')
    ON CONFLICT (key) DO NOTHING;
```

### `Storer` interface additions

```go
GetSetupComplete(ctx context.Context) (bool, error)
SetSetupComplete(ctx context.Context, complete bool) error
GetActiveReader(ctx context.Context) (string, error)
SetActiveReader(ctx context.Context, reader string) error
```

Implemented on `*store.Store` via `GetAppConfig`/`SetAppConfig`. The compile-time assertion in `internal/api/store.go` catches mismatches.

### `/api/status` response change

`setup_complete bool` is added to the status response object. The `HandleStatus` handler reads it from the store alongside `base_currency`:

```go
setupComplete, _ := h.store.GetSetupComplete(r.Context())
// included in JSON response
```

Missing key (fresh install before migration runs) defaults to `false`.

### New endpoint: `POST /api/config/active-reader`

Saves the selected reader name to `app_config`. Follows the same pattern as `POST /api/config/base-currency`:

- Body: `{"reader": "gmail"}` or `{"reader": "thunderbird"}`
- Validates that the named reader is registered in the plugin registry
- Returns `200 OK` on success

### `setup_complete = true` trigger points

| Reader | Where set |
|--------|-----------|
| Gmail | End of OAuth callback handler (`HandleOAuthCallback`), after the token is saved to disk |
| Thunderbird | End of the Thunderbird profile-path save handler, after the path is written to `app_config` |

Both call `h.store.SetSetupComplete(ctx, true)`. No other code path sets this flag.

---

## Frontend

### Wizard trigger

In the root layout (the component that wraps all routes), `useStatus()` is already consumed. Add:

```tsx
const { data: status } = useStatus()
if (!status?.setup_complete) return <SetupWizard />
```

The wizard renders over the full application as a fixed overlay. Normal app content is mounted but visually blocked. This avoids unmounting/remounting the router on wizard completion.

### `<SetupWizard>` component

**Location:** `frontend/src/components/SetupWizard.tsx`

**Overlay:** `position: fixed; inset: 0; z-index: 50; background: rgba(0,0,0,0.8)` — covers the full viewport. No close button. No `Escape` handler. No click-outside dismissal.

**Step state:** `useState<1 | 2 | 3 | 4>(1)` — local, resets to 1 on page refresh (fields are pre-populated from status data, so the restart is invisible to the user).

**Session dismissal:** `useState<boolean>(false)` called `dismissed`. Set to `true` after step 4 Finish. When `dismissed = true`, the component renders `null`. This allows the redirect to Settings to complete without the modal re-appearing in the same session. On the next page load, `setup_complete` is still `false`, so the wizard re-appears from step 1.

**Step definitions:**

| Step | Label | Component | Save endpoint |
|------|-------|-----------|---------------|
| 1 | Base currency | `CurrencyCombobox` (reused from `GeneralSettings.tsx`) | `POST /api/config/base-currency` |
| 2 | Timezone | Timezone combobox (reused from `GeneralSettings.tsx`) | `POST /api/config/timezone` |
| 3 | Time format | Time format selector (reused from `GeneralSettings.tsx`) | `POST /api/config/time-format` |
| 4 | Reader selection | Radio: Gmail / Thunderbird | `POST /api/config/active-reader` |

Each "Next" button is disabled while the save is in flight. On save error, display an inline error message — do not advance the step.

**Step 4 Finish action:**
1. Save active reader via `POST /api/config/active-reader`
2. Set `dismissed = true`
3. `navigate('/settings?tab=sync&reader={selected}')` — lands directly on the reader setup section

**Field pre-population:** Each field initialises from `useStatus()` data:
- Currency: `status.stats?.base_currency ?? 'INR'`
- Timezone: from `useTimezone()` query
- Time format: from `useTimeFormat()` query
- Reader: from `useActiveReader()` query (new query, calls `GET /api/config/active-reader`)

**Progress indicator:** Four dots at the top of the modal. Active step is blue, completed steps are green.

**Modal dimensions:** `width: min(480px, 90vw)` centered in the overlay. No scroll within the modal — each step fits in a single screen.

### New API query: `useActiveReader`

```ts
export function useActiveReader() {
  return useQuery({ queryKey: ['active-reader'], queryFn: () => api.config.getActiveReader() })
}
```

New `GET /api/config/active-reader` endpoint returns `{"reader": "gmail"}` or `{"reader": ""}`.

---

## Sequence Diagram

```
Fresh load → useStatus() → setup_complete=false → <SetupWizard> renders
  Step 1 (currency) → Next → POST /api/config/base-currency → step=2
  Step 2 (timezone) → Next → POST /api/config/timezone → step=3
  Step 3 (time format) → Next → POST /api/config/time-format → step=4
  Step 4 (reader) → Finish → POST /api/config/active-reader
                            → dismissed=true
                            → navigate('/settings?tab=sync&reader=gmail')
                            → wizard unmounts (dismissed)

User completes Gmail OAuth in Settings:
  HandleOAuthCallback → token saved → SetSetupComplete(true)
  useStatus() next poll → setup_complete=true → wizard never mounts again
```

---

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Save fails on Next | Inline error below field; step does not advance |
| Status fetch fails (server down) | `useStatus()` returns undefined; wizard not shown (app shows existing error state) |
| User refreshes mid-wizard | Modal re-appears at step 1; fields pre-populated from saved values |
| Reader setup abandoned (never completes OAuth) | `setup_complete` stays false; wizard re-appears on next load from step 1 with reader pre-selected |
| `setup_complete` key missing from DB (old install) | Defaults to `false` → wizard shown; safe for existing installs |

---

## What This Does Not Cover

- Skipping the wizard (intentionally not supported)
- Multi-reader setup within the wizard (only one active reader selected)
- Progress persistence across steps without a page refresh (steps are stateful in memory only; saves are per-step-completion)
- Editing wizard choices after completion (use Settings)
