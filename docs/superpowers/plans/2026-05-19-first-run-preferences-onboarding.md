# First-Run Preferences Onboarding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Require base currency, timezone, and time format before reader setup on fresh installs.

**Architecture:** Add a small backend setup-status endpoint backed by existing `app_config`, stop seeding base currency for fresh databases, and gate the existing `/setup` wizard with a first preferences step when status is incomplete. Reuse current config update endpoints and existing custom settings controls rather than adding a second persistence path.

**Tech Stack:** Go `net/http` handlers with `httptest`, pgx store repository tests, React/Vite/TypeScript, TanStack Query, MSW, Vitest/Testing Library.

---

## File Map

- Modify `backend/internal/store/runtime_repository.go`: remove fresh `base_currency` seeding.
- Modify `backend/internal/store/store_repositories_test.go`: assert app config init leaves base currency unset.
- Modify `backend/internal/api/handlers.go`: add setup status response type, helper, and handler.
- Modify `backend/internal/api/server.go`: register `GET /api/config/setup-status`.
- Modify `backend/internal/api/handlers_test.go`: cover setup status required/complete cases.
- Modify `frontend/src/api/client.ts`: add `config.getSetupStatus`.
- Modify `frontend/src/api/queries.ts`: add `useSetupStatus`.
- Modify `frontend/src/api/types.ts`: add setup status type if no local type is preferable.
- Modify `frontend/src/mocks/handlers.ts`: mock setup status complete by default.
- Modify `frontend/src/pages/settings/GeneralSettings.tsx`: export reusable preference control constants/components where practical.
- Create `frontend/src/pages/setup/steps/PreferencesStep.tsx`: first-run preferences form.
- Modify `frontend/src/pages/setup/Wizard.tsx`: gate reader setup behind preferences.
- Modify `frontend/src/pages/setup/Wizard.layout.test.tsx`: add setup gating tests and update query mocks.

## Task 1: Backend Setup Status and Fresh Defaults

**Files:**
- Modify: `backend/internal/store/runtime_repository.go`
- Modify: `backend/internal/store/store_repositories_test.go`
- Modify: `backend/internal/api/handlers.go`
- Modify: `backend/internal/api/server.go`
- Modify: `backend/internal/api/handlers_test.go`

- [ ] **Step 1: Write failing handler tests**

Add tests near the existing config handler tests in `backend/internal/api/handlers_test.go`:

```go
func TestHandleGetSetupStatusRequiresMissingPreferences(t *testing.T) {
	h := newTestHandlers(t, &mockStore{appConfig: map[string]string{
		"scan_interval": "60",
	}})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/config/setup-status", nil)
	rr := httptest.NewRecorder()

	h.HandleGetSetupStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Required bool     `json:"required"`
		Missing  []string `json:"missing"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Required {
		t.Fatalf("required = false, want true")
	}
	want := []string{"base_currency", "timezone", "time_format"}
	if !reflect.DeepEqual(resp.Missing, want) {
		t.Fatalf("missing = %#v, want %#v", resp.Missing, want)
	}
}

func TestHandleGetSetupStatusCompleteWhenPreferencesExist(t *testing.T) {
	h := newTestHandlers(t, &mockStore{appConfig: map[string]string{
		"base_currency":   "USD",
		"app.timezone":    "America/New_York",
		"app.time_format": "h:mm a",
	}})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/config/setup-status", nil)
	rr := httptest.NewRecorder()

	h.HandleGetSetupStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Required bool     `json:"required"`
		Missing  []string `json:"missing"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Required {
		t.Fatalf("required = true, want false")
	}
	if len(resp.Missing) != 0 {
		t.Fatalf("missing = %#v, want empty", resp.Missing)
	}
}
```

Add `reflect` to the test imports if it is not already present.

- [ ] **Step 2: Run handler tests to verify RED**

Run: `task test:be -- ./internal/api -run 'TestHandleGetSetupStatus'`

Expected: FAIL because `HandleGetSetupStatus` is undefined.

- [ ] **Step 3: Implement setup status handler**

In `backend/internal/api/handlers.go`, add response docs near the other config doc types:

```go
// DocSetupStatusResponse is the first-run setup status payload.
type DocSetupStatusResponse struct {
	Required bool     `json:"required" example:"true"`
	Missing  []string `json:"missing" example:"base_currency,timezone,time_format"`
}
```

Add helper and handler near the other config handlers:

```go
func (h *Handlers) missingSetupPreferences(ctx context.Context) []string {
	required := []struct {
		key   string
		field string
	}{
		{key: "base_currency", field: "base_currency"},
		{key: "app.timezone", field: "timezone"},
		{key: "app.time_format", field: "time_format"},
	}
	missing := make([]string, 0, len(required))
	for _, pref := range required {
		value, err := h.store.GetAppConfig(ctx, pref.key)
		if err != nil || strings.TrimSpace(value) == "" {
			missing = append(missing, pref.field)
		}
	}
	return missing
}

// HandleGetSetupStatus handles GET /api/config/setup-status.
// @Summary Get first-run setup status
// @Tags Config
// @Produce json
// @Success 200 {object} DocSetupStatusResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/setup-status [get]
func (h *Handlers) HandleGetSetupStatus(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	missing := h.missingSetupPreferences(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"required": len(missing) > 0,
		"missing":  missing,
	})
}
```

In `backend/internal/api/server.go`, register before the other `/api/config/...` routes:

```go
mux.HandleFunc("GET /api/config/setup-status", h.HandleGetSetupStatus)
```

- [ ] **Step 4: Run handler tests to verify GREEN**

Run: `task test:be -- ./internal/api -run 'TestHandleGetSetupStatus'`

Expected: PASS.

- [ ] **Step 5: Write failing store default test**

In `backend/internal/store/store_repositories_test.go`, add a test near runtime repository tests:

```go
func TestInitAppConfigLeavesBaseCurrencyUnset(t *testing.T) {
	ts := newTestStore(t)
	ctx := context.Background()

	if err := ts.InitAppConfig(ctx); err != nil {
		t.Fatalf("InitAppConfig: %v", err)
	}

	if got, err := ts.GetAppConfig(ctx, "base_currency"); err == nil {
		t.Fatalf("base_currency = %q, want missing", got)
	}
}
```

- [ ] **Step 6: Run store test to verify RED**

Run: `task test:be -- ./internal/store -run TestInitAppConfigLeavesBaseCurrencyUnset`

Expected: FAIL because `base_currency` is currently seeded as `INR`.

- [ ] **Step 7: Remove base currency seed**

In `backend/internal/store/runtime_repository.go`, change the `INSERT INTO app_config` values in `InitAppConfig` from:

```sql
('base_currency', 'INR'),
('scan_interval', '60'),
('lookback_days', '180')
```

to:

```sql
('scan_interval', '60'),
('lookback_days', '180')
```

- [ ] **Step 8: Run backend tests for this slice**

Run: `task test:be -- ./internal/store -run TestInitAppConfigLeavesBaseCurrencyUnset`

Expected: PASS.

Run: `task test:be -- ./internal/api -run 'TestHandleGetSetupStatus|TestHandleGetBaseCurrency|TestHandleSetBaseCurrency|TestHandleGetTimezone|TestHandleGetTimeFormat'`

Expected: PASS.

- [ ] **Step 9: Commit backend slice**

```bash
git add backend/internal/store/runtime_repository.go backend/internal/store/store_repositories_test.go backend/internal/api/handlers.go backend/internal/api/server.go backend/internal/api/handlers_test.go
git commit --no-gpg-sign -m "feat: expose first-run setup status"
```

## Task 2: Frontend Setup Status API

**Files:**
- Modify: `frontend/src/api/types.ts`
- Modify: `frontend/src/api/client.ts`
- Modify: `frontend/src/api/queries.ts`
- Modify: `frontend/src/mocks/handlers.ts`

- [ ] **Step 1: Add API type and client method**

In `frontend/src/api/types.ts`, add:

```ts
export interface SetupStatus {
  required: boolean
  missing: Array<'base_currency' | 'timezone' | 'time_format'>
}
```

In `frontend/src/api/client.ts`, import `SetupStatus` if needed and add under `config`:

```ts
getSetupStatus: () => apiClient.get<SetupStatus>('/config/setup-status'),
```

- [ ] **Step 2: Add query hook**

In `frontend/src/api/queries.ts`, add:

```ts
export function useSetupStatus() {
  return useQuery({
    queryKey: ['config', 'setup-status'] as const,
    queryFn: () => api.config.getSetupStatus().then((r) => r.data),
    staleTime: 30_000,
  })
}
```

- [ ] **Step 3: Update MSW default**

In `frontend/src/mocks/handlers.ts`, add:

```ts
http.get('/api/config/setup-status', () => HttpResponse.json({ required: false, missing: [] })),
```

near other config mocks.

- [ ] **Step 4: Run frontend typecheck**

Run: `task lint:fe`

Expected: PASS.

- [ ] **Step 5: Commit API slice**

```bash
git add frontend/src/api/types.ts frontend/src/api/client.ts frontend/src/api/queries.ts frontend/src/mocks/handlers.ts
git commit --no-gpg-sign -m "feat: add setup status query"
```

## Task 3: Setup Preferences Step and Wizard Gate

**Files:**
- Modify: `frontend/src/pages/settings/GeneralSettings.tsx`
- Create: `frontend/src/pages/setup/steps/PreferencesStep.tsx`
- Modify: `frontend/src/pages/setup/Wizard.tsx`
- Modify: `frontend/src/pages/setup/Wizard.layout.test.tsx`

- [ ] **Step 1: Write failing wizard tests**

In `frontend/src/pages/setup/Wizard.layout.test.tsx`, add mocks for the new hook and setup-required tests:

```ts
let setupStatus = { required: false, missing: [] as string[] }
const invalidateQueries = vi.fn()
```

Update the `@/api/queries` mock:

```ts
useQueryClient: () => ({ invalidateQueries }),
useSetupStatus: () => ({ data: setupStatus, isLoading: false }),
useSetTimezone: () => ({ mutateAsync: vi.fn(), isPending: false }),
useSetTimeFormat: () => ({ mutateAsync: vi.fn(), isPending: false }),
```

Add tests:

```ts
it('shows preferences before reader setup when setup is incomplete', async () => {
  setupStatus = { required: true, missing: ['base_currency', 'timezone', 'time_format'] }

  renderWithProviders(<Wizard />, { route: '/setup' })

  expect(await screen.findByText('Preferences')).toBeInTheDocument()
  expect(screen.getByText('Set these preferences before connecting a reader.')).toBeInTheDocument()
  expect(screen.queryByText('Reader configuration')).not.toBeInTheDocument()
})

it('gates reader-focused setup urls behind preferences when setup is incomplete', async () => {
  setupStatus = { required: true, missing: ['base_currency'] }

  renderWithProviders(<Wizard />, { route: '/setup?step=guide&reader=gmail' })

  expect(await screen.findByText('Preferences')).toBeInTheDocument()
  expect(screen.queryByTestId('reader-setup-guide')).not.toBeInTheDocument()
})
```

- [ ] **Step 2: Run wizard tests to verify RED**

Run: `task test:fe -- Wizard.layout.test.tsx`

Expected: FAIL because `PreferencesStep` and setup gating do not exist.

- [ ] **Step 3: Export reusable settings controls**

In `frontend/src/pages/settings/GeneralSettings.tsx`, export:

```ts
export const COMMON_CURRENCIES = [...]
export function CurrencyCombobox(...)
export function TimezoneCombobox(...)
export function TimeFormatSelect(...)
```

Keep behavior unchanged.

- [ ] **Step 4: Create preferences step**

Create `frontend/src/pages/setup/steps/PreferencesStep.tsx`:

```tsx
import { api } from '@/api/client'
import { queryKeys, useSetTimeFormat, useSetTimezone } from '@/api/queries'
import { TIME_FORMATS, type TimeFormatValue } from '@/contexts/DisplayContext'
import { getBrowserTimezone, normalizeTimezone } from '@/lib/timezone'
import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import {
  CurrencyCombobox,
  TimeFormatSelect,
  TimezoneCombobox,
} from '@/pages/settings/GeneralSettings'

export function PreferencesStep({ onNext }: { onNext: () => void }) {
  const qc = useQueryClient()
  const setTimezone = useSetTimezone()
  const setTimeFormat = useSetTimeFormat()
  const [currency, setCurrency] = useState('USD')
  const [timezone, setTimezoneDraft] = useState(normalizeTimezone(getBrowserTimezone()))
  const [timeFormat, setTimeFormatDraft] = useState<TimeFormatValue>(TIME_FORMATS[0].value)
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  const handleSave = async () => {
    setSaving(true)
    setError(null)
    try {
      await api.config.setBaseCurrency(currency)
      await setTimezone.mutateAsync(timezone)
      await setTimeFormat.mutateAsync(timeFormat)
      await qc.invalidateQueries({ queryKey: ['config', 'setup-status'] })
      onNext()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save preferences')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-5">
      <div>
        <p className="mb-2 text-xs uppercase tracking-widest text-muted-foreground">Setup</p>
        <h1 className="text-lg font-semibold text-foreground">Preferences</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Set these preferences before connecting a reader.
        </p>
      </div>

      <div className="space-y-4">
        <div>
          <p className="mb-1.5 text-xs uppercase tracking-wider text-muted-foreground">
            Base currency
          </p>
          <CurrencyCombobox value={currency} onChange={setCurrency} />
        </div>
        <div>
          <p className="mb-1.5 text-xs uppercase tracking-wider text-muted-foreground">Timezone</p>
          <TimezoneCombobox value={timezone} onChange={setTimezoneDraft} />
        </div>
        <div>
          <p className="mb-1.5 text-xs uppercase tracking-wider text-muted-foreground">
            Time format
          </p>
          <TimeFormatSelect value={timeFormat} onChange={setTimeFormatDraft} />
        </div>
      </div>

      <button
        onClick={handleSave}
        disabled={saving || !currency || !timezone || !timeFormat}
        className="rounded-md bg-primary px-4 py-2 text-sm text-primary-foreground transition-colors hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-40"
      >
        {saving ? 'Saving...' : 'Continue'}
      </button>
      {error && <p className="text-xs text-destructive">{error}</p>}
    </div>
  )
}
```

If `queryKeys` has no setup-status key, remove that import and use the literal query key shown above.

- [ ] **Step 5: Gate the wizard**

In `frontend/src/pages/setup/Wizard.tsx`, import `useSetupStatus` and `PreferencesStep`. At the top of `Wizard`, before overview/wizard selection, load setup status:

```tsx
const { data: setupStatus, isLoading: setupLoading } = useSetupStatus()
const [preferencesComplete, setPreferencesComplete] = useState(false)
const needsPreferences = Boolean(setupStatus?.required && !preferencesComplete)
```

When `setupLoading`, render the existing loading style. When `needsPreferences`, render the same setup shell with:

```tsx
<PreferencesStep onNext={() => setPreferencesComplete(true)} />
```

before any reader overview or reader step rendering.

- [ ] **Step 6: Run wizard tests to verify GREEN**

Run: `task test:fe -- Wizard.layout.test.tsx`

Expected: PASS.

- [ ] **Step 7: Run broader frontend checks**

Run: `task lint:fe`

Expected: PASS.

Run: `task test:fe -- Settings.test.tsx Wizard.layout.test.tsx`

Expected: PASS.

- [ ] **Step 8: Commit frontend slice**

```bash
git add frontend/src/pages/settings/GeneralSettings.tsx frontend/src/pages/setup/steps/PreferencesStep.tsx frontend/src/pages/setup/Wizard.tsx frontend/src/pages/setup/Wizard.layout.test.tsx
git commit --no-gpg-sign -m "feat: require preferences before reader setup"
```

## Task 4: Verification With Fresh DB

**Files:**
- No source changes expected.

- [ ] **Step 1: Run backend unit tests**

Run: `task test:be -- ./internal/api ./internal/store`

Expected: PASS.

- [ ] **Step 2: Run frontend tests**

Run: `task test:fe`

Expected: PASS.

- [ ] **Step 3: Run production lint gates**

Run: `task lint:be:prod`

Expected: PASS with `0 issues`.

Run: `task lint:fe`

Expected: PASS.

- [ ] **Step 4: Optional manual fresh DB run**

Bring up backend and frontend on separate ports with a fresh database name or fresh Postgres volume. Verify:

- `/setup` first shows Preferences.
- Saving base currency, timezone, and time format advances to reader configuration.
- Refreshing `/setup` after save does not show Preferences again.
- `/setup?step=guide&reader=gmail` shows Preferences first when using a fresh DB.

## Self-Review

Spec coverage:

- Fresh default removal is covered in Task 1.
- Backend setup status is covered in Task 1.
- Frontend setup gate and reader URL gating are covered in Task 3.
- Validation reuses existing endpoints; no new validation task is needed.
- Fresh DB verification is covered in Task 4.

Placeholder scan: no TBD/TODO/fill-later placeholders remain.

Type consistency: backend response fields are `required` and `missing`; frontend `SetupStatus` matches those names.
