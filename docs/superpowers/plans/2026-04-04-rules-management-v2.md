# Rules Management v2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Promote Rules to a first-class sidebar route with a full-page create/edit form, multi-sample regex tester, retroactive scan on save, and a hardened list table with checkboxes, bulk actions, and client-side export.

**Architecture:** Backend adds `ForceRescan bool` to `daemon.RunConfig`, a `POST /api/daemon/rescan` handler, and a `GET /api/config/active-reader` helper. Frontend replaces the Settings tab + slide-over with `/rules` (list), `/rules/new` (create), and `/rules/:id` (edit) routed pages. Export is entirely client-side via Blob URL. The retroactive scan queues via `app_config["rescan_pending"]` when the daemon is running, and auto-applies on the next daemon start.

**Tech Stack:** Go 1.23+, pgx/v5, React 18, TypeScript, React Router v6, TanStack Query, Tailwind CSS.

---

## File Map

| File | Change |
|------|--------|
| `backend/internal/daemon/runner.go` | Add `ForceRescan bool` to `RunConfig` |
| `backend/internal/api/handlers.go` | Add `rescanFn` field; add `HandleRescan`, `HandleGetActiveReader` |
| `backend/internal/api/handlers_test.go` | Update `newTestHandlers`; add 2 rescan tests |
| `backend/internal/api/server.go` | Register `POST /api/daemon/rescan`, `GET /api/config/active-reader` |
| `backend/cmd/server/main.go` | Add `forceRescan bool` param to `runDaemon`; define `rescanDaemon`; check `rescan_pending` in `startDaemon`; pass `rescanFn` to `NewHandlers` |
| `frontend/src/api/client.ts` | Add `api.daemon.rescan(reader)`, `api.config.getActiveReader()` |
| `frontend/src/api/queries.ts` | Add `useRescan()`, `useActiveReader()` |
| `frontend/src/components/Sidebar.tsx` | Add Rules nav entry |
| `frontend/src/App.tsx` | Add `/rules`, `/rules/new`, `/rules/:id` routes; lazy-import `Rules` + `RuleForm` |
| `frontend/src/pages/Settings.tsx` | Remove Rules tab |
| `frontend/src/pages/settings/RulesSettings.tsx` | Delete |
| `frontend/src/pages/Rules.tsx` | New — list page with checkboxes, bulk actions, client-side export |
| `frontend/src/pages/rules/RuleForm.tsx` | New — full-page create/edit form with multi-sample tester + retroactive scan |

---

## Task 1: Backend — ForceRescan, rescan handlers, routes, tests

**Files:**
- Modify: `backend/internal/daemon/runner.go`
- Modify: `backend/internal/api/handlers.go`
- Modify: `backend/internal/api/handlers_test.go`
- Modify: `backend/internal/api/server.go`

### Background

`daemon.RunConfig` gains `ForceRescan bool`. When true, `runDaemon` (Task 2) skips `state.New` and passes nil as `StateManager` — readers already guard with `if r.state != nil && r.state.IsProcessed(...)` so nil naturally bypasses dedup. Two new handlers: `HandleRescan` calls `rescanFn` when daemon is idle or queues via `app_config["rescan_pending"]` when running; `HandleGetActiveReader` reads the `data/active_reader` file so the frontend knows which reader to pass to the rescan endpoint.

TDD order: add mock + tests first (compile error), then implement.

- [ ] **Step 1: Add ForceRescan to RunConfig**

In `backend/internal/daemon/runner.go`, change the `RunConfig` struct:

```go
// RunConfig holds the configuration for running the daemon.
type RunConfig struct {
	// ReaderName is the plugin name of the reader to use (e.g. "gmail").
	// Set by the web UI via POST /api/daemon/start.
	ReaderName string
	// WriterName is the plugin name of the writer to use (e.g. "postgres").
	WriterName   string
	Config       *config.Config
	Rules        []api.Rule
	Labels       api.Labels
	StateManager *state.Manager
	// ForceRescan bypasses state deduplication for the current run.
	// When true, StateManager should be nil — readers handle nil gracefully.
	ForceRescan bool
}
```

- [ ] **Step 2: Write the two failing handler tests**

Add to `backend/internal/api/handlers_test.go` at the end:

```go
// --- rescan ---

func TestHandleRescan_DaemonRunning_Returns202Queued(t *testing.T) {
	ms := &mockStore{}
	dm := &mockDaemon{status: DaemonStatus{Running: true}}
	h := newTestHandlers(t, ms, dm)

	body := `{"reader":"gmail"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/daemon/rescan", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.HandleRescan(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["status"] != "queued" {
		t.Errorf("expected status=queued, got %q", resp["status"])
	}
}

func TestHandleRescan_DaemonNotRunning_Returns202Rescanning(t *testing.T) {
	called := false
	ms := &mockStore{}
	dm := &mockDaemon{status: DaemonStatus{Running: false}}
	h := newTestHandlers(t, ms, dm)
	h.rescanFn = func(_ string) { called = true }

	body := `{"reader":"gmail"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/daemon/rescan", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.HandleRescan(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["status"] != "rescanning" {
		t.Errorf("expected status=rescanning, got %q", resp["status"])
	}
	if !called {
		t.Error("expected rescanFn to be called")
	}
}
```

- [ ] **Step 3: Confirm compile error**

```bash
cd /Users/ksingh/code/expensor/backend && go test ./internal/api/... -short -run "TestHandleRescan" 2>&1 | head -10
```
Expected: compile error — `h.rescanFn` undefined, `HandleRescan` undefined.

- [ ] **Step 4: Add rescanFn to Handlers struct and NewHandlers**

In `backend/internal/api/handlers.go`:

Add `rescanFn` to the `Handlers` struct (after `startFn`):
```go
	startFn      func(reader string) // called by POST /api/daemon/start; may be nil
	rescanFn     func(reader string) // called by POST /api/daemon/rescan; may be nil
```

Update `NewHandlers` signature — add `rescanFn func(reader string)` between `startFn` and `logger`:
```go
func NewHandlers( //nolint:revive // dependency injection requires all these parameters; callers use named fields
	registry *plugins.Registry,
	st Storer,
	daemon DaemonStatusProvider,
	baseURL string,
	frontendURL string,
	dataDir string,
	baseCurrency string,
	scanInterval int,
	lookbackDays int,
	startFn func(reader string),
	rescanFn func(reader string),
	logger *slog.Logger,
) *Handlers {
```

Add `rescanFn: rescanFn,` to the return statement inside `NewHandlers`.

- [ ] **Step 5: Add HandleRescan and HandleGetActiveReader to handlers.go**

Add after `HandleStartDaemon`:

```go
// HandleRescan handles POST /api/daemon/rescan.
// Body: {"reader": "<name>"}
// If the daemon is idle, starts a forced rescan immediately.
// If the daemon is running, queues the rescan via app_config["rescan_pending"].
func (h *Handlers) HandleRescan(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	var body struct {
		Reader string `json:"reader"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Reader == "" {
		writeError(w, http.StatusBadRequest, `body must be {"reader": "<name>"}`)
		return
	}
	if _, err := h.registry.GetReader(body.Reader); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("reader %q not found", body.Reader))
		return
	}

	if h.daemon.Status().Running {
		if err := h.store.SetAppConfig(r.Context(), "rescan_pending", "true"); err != nil {
			h.logger.Error("failed to queue rescan", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to queue rescan")
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
		return
	}

	if h.rescanFn == nil {
		writeError(w, http.StatusNotImplemented, "rescan not configured")
		return
	}
	h.rescanFn(body.Reader)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "rescanning"})
}

// HandleGetActiveReader handles GET /api/config/active-reader.
// Returns the reader name persisted from the last daemon start, or "" if none.
func (h *Handlers) HandleGetActiveReader(w http.ResponseWriter, r *http.Request) {
	b, err := os.ReadFile(filepath.Join(h.dataDir, "active_reader")) //nolint:gosec // path built from validated dataDir
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, map[string]string{"reader": ""})
			return
		}
		h.logger.Error("failed to read active reader", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to read active reader")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"reader": string(b)})
}
```

- [ ] **Step 6: Update newTestHandlers to pass nil for rescanFn**

In `backend/internal/api/handlers_test.go`, update the `NewHandlers` call in `newTestHandlers`:

```go
return NewHandlers(registry, st, dm, "http://localhost:8080", "http://localhost:5173", t.TempDir(), "INR", 60, 180, nil, nil, slog.Default())
```
(Added a second `nil` for `rescanFn` between `startFn=nil` and `logger`.)

- [ ] **Step 7: Register routes in server.go**

In `backend/internal/api/server.go`, add after the `POST /api/daemon/start` line:

```go
	mux.HandleFunc("POST /api/daemon/rescan", h.HandleRescan)
	mux.HandleFunc("GET /api/config/active-reader", h.HandleGetActiveReader)
```

- [ ] **Step 8: Run the two rescan tests**

```bash
cd /Users/ksingh/code/expensor/backend && go test ./internal/api/... -v -run "TestHandleRescan" 2>&1
```
Expected: both pass.

- [ ] **Step 9: Run full backend tests + prod linter**

```bash
cd /Users/ksingh/code/expensor/backend && go test -short ./... 2>&1 | tail -5
```
```bash
cd /Users/ksingh/code/expensor && task lint:be:prod 2>&1 | tail -5
```
Expected: all pass, 0 issues.

- [ ] **Step 10: Commit**

```bash
cd /Users/ksingh/code/expensor && git add \
  backend/internal/daemon/runner.go \
  backend/internal/api/handlers.go \
  backend/internal/api/handlers_test.go \
  backend/internal/api/server.go
git commit --no-gpg-sign -m "feat: add ForceRescan to RunConfig, HandleRescan, and HandleGetActiveReader

POST /api/daemon/rescan starts an immediate forced rescan when the daemon
is idle, or queues it via app_config[rescan_pending] when running.
GET /api/config/active-reader exposes the persisted reader name.
NewHandlers gains a rescanFn parameter alongside the existing startFn."
```

---

## Task 2: Wire main.go — forceRescan param, rescanDaemon, pending check

**Files:**
- Modify: `backend/cmd/server/main.go`

### Background

`runDaemon` gains a `forceRescan bool` last parameter. When true it skips `state.New`, leaving `StateManager` nil in `RunConfig`. A new `rescanDaemon` closure mirrors `startDaemon` but always passes `forceRescan=true`. `startDaemon` checks `app_config["rescan_pending"]` on each call; if set it flips `forceRescan=true` and clears the flag. Both closures are passed to `NewHandlers`.

- [ ] **Step 1: Add forceRescan parameter to runDaemon**

In `backend/cmd/server/main.go`, change the `runDaemon` signature — add `forceRescan bool` as the last parameter:

```go
func runDaemon( //nolint:revive // all parameters are required; splitting further would obscure the call site
	ctx context.Context,
	registry *plugins.Registry,
	readerName string,
	cfg config.Config,
	rules []api.Rule,
	labels api.Labels,
	dm *daemonManager,
	logger *slog.Logger,
	forceRescan bool,
) {
```

Inside `runDaemon`, replace the existing `state.New` block:

```go
	// Create state manager. Skip for forced rescans — readers handle nil gracefully
	// (they guard with `if r.state != nil && r.state.IsProcessed(...)`), allowing
	// already-processed emails to be re-extracted and upserted into the DB.
	var stateManager *state.Manager
	if !forceRescan {
		stateManager, err = state.New(cfg.StateFile, logger)
		if err != nil {
			logger.Error("failed to create state manager", "error", err)
			dm.setStopped(err)
			return
		}
	}
```

Then update the `RunConfig` literal to remove the inline `stateManager, err := state.New(...)` call (it's now above) and set:
```go
	runCfg := daemon.RunConfig{
		ReaderName:   readerName,
		WriterName:   writerName,
		Config:       &cfg,
		Rules:        rules,
		Labels:       labels,
		StateManager: stateManager,
		ForceRescan:  forceRescan,
	}
```

- [ ] **Step 2: Update the startDaemon closure**

Replace the existing `startDaemon` closure with this version that checks `rescan_pending` and passes `forceRescan`:

```go
	var startMu sync.Mutex
	startDaemon := func(readerName string) {
		startMu.Lock()
		defer startMu.Unlock()
		if dm.Status().Running {
			return
		}

		// Check if a retroactive scan was queued while the daemon was running.
		forceRescan := false
		if st != nil {
			if val, configErr := st.GetAppConfig(ctx, "rescan_pending"); configErr == nil && val == "true" {
				forceRescan = true
				_ = st.SetAppConfig(ctx, "rescan_pending", "") // clear flag; best-effort
				logger.Info("rescan_pending detected — running with force rescan")
			}
		}

		if err := saveActiveReader(cfg.DataDir, readerName); err != nil {
			logger.Warn("failed to persist active reader", "error", err)
		}
		runtimeCfg := applyScanOverrides(cfg, st)
		merged := pkgrules.FilterEnabled(pkgrules.MergeRules(systemRules, loadUserRules(ctx, st, logger)))
		go runDaemon(ctx, registry, readerName, runtimeCfg, merged, labels, dm, logger, forceRescan)
	}
```

- [ ] **Step 3: Define rescanDaemon closure**

Add after `startDaemon`, before the auto-start block:

```go
	// rescanDaemon starts a one-shot daemon run that ignores state deduplication.
	// Called by POST /api/daemon/rescan when the daemon is not running.
	rescanDaemon := func(readerName string) {
		startMu.Lock()
		defer startMu.Unlock()
		if dm.Status().Running {
			return
		}
		runtimeCfg := applyScanOverrides(cfg, st)
		merged := pkgrules.FilterEnabled(pkgrules.MergeRules(systemRules, loadUserRules(ctx, st, logger)))
		go runDaemon(ctx, registry, readerName, runtimeCfg, merged, labels, dm, logger, true)
	}
```

- [ ] **Step 4: Pass rescanDaemon to NewHandlers**

Update the `httpapi.NewHandlers` call to include `rescanDaemon` between `startDaemon` and `logger`:

```go
	handlers := httpapi.NewHandlers(
		registry, st, dm, baseURL, frontendURL,
		cfg.DataDir, cfg.BaseCurrency,
		cfg.ScanInterval, cfg.LookbackDays,
		startDaemon, rescanDaemon,
		logger.With("component", "api"),
	)
```

- [ ] **Step 5: Build and test**

```bash
cd /Users/ksingh/code/expensor/backend && go build ./cmd/server/... 2>&1
```
Expected: exits 0.

```bash
cd /Users/ksingh/code/expensor/backend && go test -short ./... 2>&1 | tail -5
```
Expected: all pass.

```bash
cd /Users/ksingh/code/expensor && task lint:be:prod 2>&1 | tail -5
```
Expected: 0 issues.

- [ ] **Step 6: Commit**

```bash
cd /Users/ksingh/code/expensor && git add backend/cmd/server/main.go
git commit --no-gpg-sign -m "feat: wire forceRescan into runDaemon and startDaemon

runDaemon gains a forceRescan bool parameter; when true it skips
state.New so the StateManager is nil and readers re-process all emails
in the lookback window. startDaemon checks app_config[rescan_pending]
on each call and auto-promotes the run to a forced rescan if the flag
is set, clearing it before the run begins. rescanDaemon always uses
forceRescan=true and is wired to the new HandleRescan endpoint."
```

---

## Task 3: Frontend API additions

**Files:**
- Modify: `frontend/src/api/client.ts`
- Modify: `frontend/src/api/queries.ts`

### Background

Two new client methods: `api.daemon.rescan(reader)` and `api.config.getActiveReader()`. Two new query hooks: `useRescan()` (mutation) and `useActiveReader()` (query). These are used by the Rule Form page on save.

- [ ] **Step 1: Add to client.ts**

In `frontend/src/api/client.ts`, add to the `api.daemon` object (create it if missing, or add alongside the existing `start`):

```ts
  daemon: {
    start: (reader: string) => apiClient.post<{ status: string }>('/daemon/start', { reader }),
    rescan: (reader: string) => apiClient.post<{ status: 'rescanning' | 'queued' }>('/daemon/rescan', { reader }),
  },
```

Add `getActiveReader` to `api.config`:
```ts
    getActiveReader: () => apiClient.get<{ reader: string }>('/config/active-reader'),
```

- [ ] **Step 2: Add to queries.ts**

Add at the end of `frontend/src/api/queries.ts`:

```ts
export function useRescan() {
  return useMutation({
    mutationFn: (reader: string) => api.daemon.rescan(reader).then((r) => r.data),
  })
}

export function useActiveReader() {
  return useQuery({
    queryKey: ['config', 'active-reader'] as const,
    queryFn: () => api.config.getActiveReader().then((r) => r.data.reader),
    staleTime: 60_000,
  })
}
```

- [ ] **Step 3: TypeScript check**

```bash
cd /Users/ksingh/code/expensor && task lint:fe 2>&1 | tail -5
```
Expected: 0 errors.

- [ ] **Step 4: Commit**

```bash
cd /Users/ksingh/code/expensor && git add frontend/src/api/client.ts frontend/src/api/queries.ts
git commit --no-gpg-sign -m "feat(frontend): add api.daemon.rescan, api.config.getActiveReader, and hooks"
```

---

## Task 4: Routing, sidebar, Settings cleanup

**Files:**
- Modify: `frontend/src/components/Sidebar.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/pages/Settings.tsx`
- Delete: `frontend/src/pages/settings/RulesSettings.tsx`

### Background

Rules gets a dedicated sidebar entry using the `ScrollText` icon and three routes. The Settings tab and slide-over component are removed. The `RulesSettings.tsx` file is deleted entirely.

- [ ] **Step 1: Add Rules to sidebar**

In `frontend/src/components/Sidebar.tsx`:

1. Add `ScrollText` to the lucide-react import:
```ts
import {
  ArrowLeftRight,
  ChevronLeft,
  ChevronRight,
  Download,
  FileBarChart,
  LayoutDashboard,
  type LucideIcon,
  Plug,
  ScrollText,
  Settings2,
} from 'lucide-react'
```

2. Add to `SECONDARY_NAV` between Onboarding and Settings:
```ts
const SECONDARY_NAV: NavItemDef[] = [
  { label: 'Onboarding', icon: Plug, href: '/setup' },
  { label: 'Rules', icon: ScrollText, href: '/rules' },
  { label: 'Settings', icon: Settings2, href: '/settings' },
]
```

- [ ] **Step 2: Add routes to App.tsx**

In `frontend/src/App.tsx`, add two new lazy imports after the existing `Settings` import:

```ts
const Rules = lazy(() => import('@/pages/Rules'))
const RuleForm = lazy(() => import('@/pages/rules/RuleForm').then((m) => ({ default: m.RuleForm })))
```

Add three new routes inside the `<Route element={<AppLayout />}>` block:

```tsx
<Route
  path="/rules"
  element={
    <PageSuspense>
      <Rules />
    </PageSuspense>
  }
/>
<Route
  path="/rules/new"
  element={
    <PageSuspense>
      <RuleForm />
    </PageSuspense>
  }
/>
<Route
  path="/rules/:id"
  element={
    <PageSuspense>
      <RuleForm />
    </PageSuspense>
  }
/>
```

- [ ] **Step 3: Remove Rules tab from Settings.tsx**

In `frontend/src/pages/Settings.tsx`:

1. Remove the `RulesSettings` import line.
2. Remove `'rules'` from the `SettingsTab` type.
3. Remove `{ id: 'rules', label: 'Rules' }` from the `TABS` array.
4. Remove `{tab === 'rules' && <RulesSettings />}` from the render.

- [ ] **Step 4: Delete RulesSettings.tsx**

```bash
rm /Users/ksingh/code/expensor/frontend/src/pages/settings/RulesSettings.tsx
```

- [ ] **Step 5: TypeScript check + Prettier**

```bash
cd /Users/ksingh/code/expensor && task lint:fe 2>&1 | tail -5
task fmt:fe:check 2>&1 | tail -5
```
If Prettier violations: `task fmt:fe` then re-check.

- [ ] **Step 6: Commit**

```bash
cd /Users/ksingh/code/expensor && git add \
  frontend/src/components/Sidebar.tsx \
  frontend/src/App.tsx \
  frontend/src/pages/Settings.tsx
git rm frontend/src/pages/settings/RulesSettings.tsx
git commit --no-gpg-sign -m "feat(frontend): promote Rules to sidebar route, remove Settings tab

Adds /rules, /rules/new, /rules/:id routes with lazy-loaded components.
Adds Rules nav entry to sidebar (ScrollText icon, between Onboarding
and Settings). Removes the Rules tab from Settings and deletes the
now-superseded RulesSettings.tsx slide-over component."
```

---

## Task 5: Rules list page

**Files:**
- Create: `frontend/src/pages/Rules.tsx`

### Background

The list page replicates the table from the old `RulesSettings` component but adds: row checkboxes with select-all, bulk Enable/Disable/Delete toolbar (appears only when ≥1 row selected), always-visible Delete column (disabled for system rules), and client-side export of selected rules as a `.json` Blob download. Source badge gets `whitespace-nowrap`. The Edit column navigates to `/rules/:id`.

- [ ] **Step 1: Create frontend/src/pages/Rules.tsx**

```tsx
import { useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useDeleteRule, useImportRules, useRules, useUpdateRule } from '@/api/queries'
import type { Rule, RuleImport } from '@/api/types'

// ─── Client-side export ───────────────────────────────────────────────────────

function downloadRules(rules: Rule[], selectedIds: Set<string>) {
  const toExport: RuleImport[] = rules
    .filter((r) => selectedIds.has(r.id))
    .map((r) => ({
      name: r.name,
      senderEmail: r.sender_email,
      subjectContains: r.subject_contains,
      amountRegex: r.amount_regex,
      merchantInfoRegex: r.merchant_regex,
      currencyRegex: r.currency_regex || undefined,
      enabled: r.enabled,
    }))
  const blob = new Blob([JSON.stringify(toExport, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'expensor-rules.json'
  a.click()
  URL.revokeObjectURL(url)
}

// ─── Rules list page ──────────────────────────────────────────────────────────

export default function Rules() {
  const navigate = useNavigate()
  const { data: rules = [], isLoading } = useRules()
  const { mutate: updateRule } = useUpdateRule()
  const { mutate: deleteRule } = useDeleteRule()
  const { mutate: importRules, isPending: importing } = useImportRules()

  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [importMsg, setImportMsg] = useState('')
  const fileRef = useRef<HTMLInputElement>(null)

  const allSelected = rules.length > 0 && selected.size === rules.length
  const noneSelected = selected.size === 0

  const toggleAll = () =>
    setSelected(allSelected ? new Set() : new Set(rules.map((r) => r.id)))

  const toggleRow = (id: string) =>
    setSelected((prev) => {
      const next = new Set(prev)
      next.has(id) ? next.delete(id) : next.add(id)
      return next
    })

  const bulkEnable = () =>
    rules
      .filter((r) => selected.has(r.id))
      .forEach((r) => updateRule({ id: r.id, body: { enabled: true } }))

  const bulkDisable = () =>
    rules
      .filter((r) => selected.has(r.id))
      .forEach((r) => updateRule({ id: r.id, body: { enabled: false } }))

  const bulkDelete = () => {
    const userRules = rules.filter((r) => selected.has(r.id) && r.source === 'user')
    if (userRules.length === 0) return
    if (!confirm(`Delete ${userRules.length} user rule${userRules.length !== 1 ? 's' : ''}?`)) return
    userRules.forEach((r) => deleteRule(r.id, { onSuccess: () => setSelected((s) => { const n = new Set(s); n.delete(r.id); return n }) }))
  }

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    void file.text().then((text) => {
      let parsed: RuleImport[]
      try { parsed = JSON.parse(text) as RuleImport[] } catch { setImportMsg('Invalid JSON file'); return }
      importRules(parsed, {
        onSuccess: (data) => setImportMsg(`${data.imported} rule${data.imported !== 1 ? 's' : ''} imported`),
        onError: (err) => setImportMsg(err.message),
      })
    })
    e.target.value = ''
  }

  const handleDelete = (r: Rule) => {
    if (!confirm(`Delete rule "${r.name}"?`)) return
    deleteRule(r.id)
  }

  if (isLoading) {
    return (
      <div className="mx-auto w-full max-w-5xl px-6 py-6">
        <p className="text-xs text-muted-foreground">Loading…</p>
      </div>
    )
  }

  const selectedUserRuleCount = rules.filter((r) => selected.has(r.id) && r.source === 'user').length

  return (
    <div className="mx-auto w-full max-w-5xl space-y-4 px-6 py-6">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold text-foreground">Rules</h1>
      </div>

      {/* Action bar */}
      <div className="flex flex-wrap items-center gap-2">
        <Link
          to="/rules/new"
          className="rounded bg-primary px-3 py-1.5 text-xs text-primary-foreground hover:bg-primary/90"
        >
          + New rule
        </Link>
        <button
          onClick={() => downloadRules(rules, selected)}
          disabled={noneSelected}
          className="rounded border border-border px-3 py-1.5 text-xs text-muted-foreground hover:text-foreground disabled:cursor-not-allowed disabled:opacity-50"
        >
          {noneSelected ? 'Export' : `Export (${selected.size} selected)`}
        </button>
        <button
          onClick={() => fileRef.current?.click()}
          disabled={importing}
          className="rounded border border-border px-3 py-1.5 text-xs text-muted-foreground hover:text-foreground disabled:opacity-50"
        >
          {importing ? 'Importing…' : 'Import'}
        </button>
        <input ref={fileRef} type="file" accept=".json" className="hidden" onChange={handleFileChange} />
        {importMsg && <span className="text-xs text-muted-foreground">{importMsg}</span>}

        {/* Bulk actions — right side */}
        {!noneSelected && (
          <div className="ml-auto flex items-center gap-2">
            <span className="text-xs text-muted-foreground">{selected.size} selected</span>
            <button
              onClick={bulkEnable}
              className="rounded border border-border px-2 py-1 text-xs text-muted-foreground hover:text-foreground"
            >
              Enable ({selected.size})
            </button>
            <button
              onClick={bulkDisable}
              className="rounded border border-border px-2 py-1 text-xs text-muted-foreground hover:text-foreground"
            >
              Disable ({selected.size})
            </button>
            {selectedUserRuleCount > 0 && (
              <button
                onClick={bulkDelete}
                className="rounded border border-destructive/40 px-2 py-1 text-xs text-destructive hover:bg-destructive/10"
              >
                Delete ({selectedUserRuleCount})
              </button>
            )}
          </div>
        )}
      </div>

      {/* Table */}
      <div className="overflow-x-auto rounded-lg border border-border">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border bg-secondary">
              <th className="w-10 px-3 py-2">
                <input
                  type="checkbox"
                  checked={allSelected}
                  onChange={toggleAll}
                  aria-label="Select all"
                />
              </th>
              {['Enabled', 'Name', 'Sender', 'Subject', 'Source', '', ''].map((h) => (
                <th
                  key={h}
                  className="px-3 py-2 text-left text-xs uppercase tracking-wider text-muted-foreground"
                >
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {rules.map((rule) => (
              <tr key={rule.id} className={`hover:bg-secondary/50 ${selected.has(rule.id) ? 'bg-secondary/30' : ''}`}>
                <td className="px-3 py-2">
                  <input
                    type="checkbox"
                    checked={selected.has(rule.id)}
                    onChange={() => toggleRow(rule.id)}
                    aria-label={`Select ${rule.name}`}
                  />
                </td>
                <td className="px-3 py-2">
                  <span
                    className={`inline-block h-2 w-2 rounded-full ${rule.enabled ? 'bg-green-500' : 'bg-muted-foreground'}`}
                  />
                </td>
                <td className="px-3 py-2 font-medium text-foreground">{rule.name}</td>
                <td className="px-3 py-2 font-mono text-xs text-muted-foreground">
                  {rule.sender_email || '—'}
                </td>
                <td className="max-w-[200px] truncate px-3 py-2 text-xs text-muted-foreground">
                  {rule.subject_contains || '—'}
                </td>
                <td className="px-3 py-2">
                  {rule.source === 'system' ? (
                    <span className="inline-flex shrink-0 items-center gap-1 whitespace-nowrap rounded border border-border px-1.5 py-0.5 text-xs text-muted-foreground">
                      🔒 System
                    </span>
                  ) : (
                    <span className="inline-flex shrink-0 items-center whitespace-nowrap rounded border border-primary/40 px-1.5 py-0.5 text-xs text-primary">
                      User
                    </span>
                  )}
                </td>
                <td className="px-3 py-2">
                  <button
                    onClick={() => navigate(`/rules/${rule.id}`)}
                    className="text-xs text-muted-foreground hover:text-foreground"
                  >
                    Edit
                  </button>
                </td>
                <td className="px-3 py-2">
                  <button
                    onClick={() => handleDelete(rule)}
                    disabled={rule.source === 'system'}
                    title={rule.source === 'system' ? 'System rules cannot be deleted' : undefined}
                    className="text-xs text-destructive hover:underline disabled:cursor-not-allowed disabled:opacity-30"
                  >
                    Delete
                  </button>
                </td>
              </tr>
            ))}
            {rules.length === 0 && (
              <tr>
                <td colSpan={8} className="px-3 py-6 text-center text-xs text-muted-foreground">
                  No rules yet.{' '}
                  <Link to="/rules/new" className="text-primary hover:underline">
                    Create one
                  </Link>
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: TypeScript check + Prettier**

```bash
cd /Users/ksingh/code/expensor && task lint:fe 2>&1 | tail -5
task fmt:fe:check 2>&1 | tail -5
```
Fix any issues before committing.

- [ ] **Step 3: Commit**

```bash
cd /Users/ksingh/code/expensor && git add frontend/src/pages/Rules.tsx
git commit --no-gpg-sign -m "feat(frontend): add Rules list page with checkboxes, bulk actions, and export

Row checkboxes with select-all header. Bulk Enable/Disable/Delete toolbar
appears when rows are selected (Delete only applies to user rules in the
selection). Export is client-side only — downloads selected rules as a
rules.json-compatible Blob. Source badge uses whitespace-nowrap. Delete
column is always visible but disabled for system rules."
```

---

## Task 6: Rule form page

**Files:**
- Create: `frontend/src/pages/rules/RuleForm.tsx`

### Background

The form page serves both create (`/rules/new`) and edit (`/rules/:id`) via the `id` URL param. For edit, it loads the rule from the cached `useRules()` result. The multi-sample tester section runs entirely client-side: for each (sample, regex) pair it calls `new RegExp(pattern).exec(body)` and shows capture group 1 or a no-match/invalid indicator. On save in edit mode, if the retroactive scan checkbox is checked, it calls `POST /api/daemon/rescan`; the response `status` determines the toast message. System rules show all fields as read-only except Enabled.

- [ ] **Step 1: Create the directory**

```bash
mkdir -p /Users/ksingh/code/expensor/frontend/src/pages/rules
```

- [ ] **Step 2: Create frontend/src/pages/rules/RuleForm.tsx**

```tsx
import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useActiveReader, useCreateRule, useRescan, useRules, useUpdateRule } from '@/api/queries'

// ─── Regex helpers ────────────────────────────────────────────────────────────

interface RegexResult {
  match: string | null
  invalid: boolean
}

function testRegex(pattern: string, body: string): RegexResult {
  if (!pattern) return { match: null, invalid: false }
  try {
    const m = new RegExp(pattern).exec(body)
    return { match: m?.[1] ?? null, invalid: false }
  } catch {
    return { match: null, invalid: true }
  }
}

function ResultCell({ result }: { result: RegexResult }) {
  if (result.invalid) {
    return <span className="text-warning-foreground text-xs">⚠ invalid</span>
  }
  if (result.match !== null) {
    return <span className="text-green-500 text-xs font-mono">{result.match} ✓</span>
  }
  return <span className="text-muted-foreground text-xs">—</span>
}

// ─── Form state ───────────────────────────────────────────────────────────────

interface FormState {
  name: string
  senderEmail: string
  subjectContains: string
  amountRegex: string
  merchantRegex: string
  currencyRegex: string
  enabled: boolean
}

const emptyForm: FormState = {
  name: '',
  senderEmail: '',
  subjectContains: '',
  amountRegex: '',
  merchantRegex: '',
  currencyRegex: '',
  enabled: true,
}

// ─── Rule form page ───────────────────────────────────────────────────────────

export function RuleForm() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const isCreate = !id

  const { data: rules = [], isLoading: rulesLoading } = useRules()
  const rule = id ? rules.find((r) => r.id === id) : null

  const { data: activeReader = '' } = useActiveReader()

  const [form, setForm] = useState<FormState>(emptyForm)
  const [samples, setSamples] = useState<string[]>([''])
  const [rescan, setRescan] = useState(true)
  const [toast, setToast] = useState('')
  const [formError, setFormError] = useState('')

  const { mutate: createRule, isPending: creating } = useCreateRule()
  const { mutate: updateRule, isPending: updating } = useUpdateRule()
  const { mutate: triggerRescan } = useRescan()

  // Populate form when rule data arrives (edit mode)
  useEffect(() => {
    if (rule) {
      setForm({
        name: rule.name,
        senderEmail: rule.sender_email,
        subjectContains: rule.subject_contains,
        amountRegex: rule.amount_regex,
        merchantRegex: rule.merchant_regex,
        currencyRegex: rule.currency_regex,
        enabled: rule.enabled,
      })
    }
  }, [rule?.id]) // eslint-disable-line react-hooks/exhaustive-deps

  const isSystem = rule?.source === 'system'

  const set =
    (k: keyof FormState) =>
    (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) =>
      setForm((f) => ({
        ...f,
        [k]: e.target.type === 'checkbox'
          ? (e.target as HTMLInputElement).checked
          : e.target.value,
      }))

  // Live regex results for each sample
  const results = useMemo(
    () =>
      samples.map((body) => ({
        body,
        amount: testRegex(form.amountRegex, body),
        merchant: testRegex(form.merchantRegex, body),
        currency: testRegex(form.currencyRegex, body),
      })),
    [samples, form.amountRegex, form.merchantRegex, form.currencyRegex],
  )

  const addSample = () => setSamples((s) => [...s, ''])
  const removeSample = (i: number) => setSamples((s) => s.filter((_, idx) => idx !== i))
  const updateSample = (i: number, val: string) =>
    setSamples((s) => s.map((v, idx) => (idx === i ? val : v)))

  const handleSubmit = () => {
    setFormError('')
    if (!form.name) { setFormError('Name is required'); return }
    if (!isSystem && !form.amountRegex) { setFormError('Amount regex is required'); return }
    if (!isSystem && !form.merchantRegex) { setFormError('Merchant regex is required'); return }

    const body = isSystem
      ? { enabled: form.enabled }
      : {
          name: form.name,
          sender_email: form.senderEmail,
          subject_contains: form.subjectContains,
          amount_regex: form.amountRegex,
          merchant_regex: form.merchantRegex,
          currency_regex: form.currencyRegex,
          enabled: form.enabled,
        }

    if (isCreate) {
      createRule(body as Parameters<typeof createRule>[0], {
        onSuccess: () => navigate('/rules'),
        onError: (e) => setFormError(e.message),
      })
      return
    }

    updateRule(
      { id: id!, body },
      {
        onSuccess: () => {
          if (!rescan || !activeReader) {
            navigate('/rules')
            return
          }
          triggerRescan(activeReader, {
            onSuccess: (data) => {
              const msg = data.status === 'rescanning'
                ? 'Rule saved. Retroactive scan started.'
                : 'Rule saved. Retroactive scan queued — will run on the next daemon start.'
              setToast(msg)
              setTimeout(() => navigate('/rules'), 2500)
            },
            onError: () => navigate('/rules'),
          })
        },
        onError: (e) => setFormError(e.message),
      },
    )
  }

  if (!isCreate && rulesLoading) {
    return (
      <div className="mx-auto w-full max-w-2xl px-6 py-6">
        <p className="text-xs text-muted-foreground">Loading…</p>
      </div>
    )
  }

  if (!isCreate && !rule) {
    return (
      <div className="mx-auto w-full max-w-2xl px-6 py-6">
        <p className="text-sm text-destructive">Rule not found.</p>
        <Link to="/rules" className="text-xs text-primary hover:underline">← Back to rules</Link>
      </div>
    )
  }

  const isPending = creating || updating

  return (
    <div className="mx-auto w-full max-w-2xl space-y-6 px-6 py-6">
      {/* Breadcrumb */}
      <nav className="flex items-center gap-1.5 text-xs text-muted-foreground">
        <Link to="/rules" className="hover:text-foreground">Rules</Link>
        <span>›</span>
        <span className="text-foreground">{isCreate ? 'New Rule' : (rule?.name ?? id)}</span>
      </nav>

      {toast && (
        <div className="rounded border border-border bg-secondary px-4 py-3 text-sm text-foreground">
          {toast}
        </div>
      )}

      {isSystem && (
        <p className="rounded border border-border bg-secondary px-3 py-2 text-xs text-muted-foreground">
          System rule — only the enabled toggle can be changed.
        </p>
      )}

      {/* Core fields */}
      <div className="space-y-4">
        {([
          { key: 'name' as const, label: 'Name', required: true, mono: false },
          { key: 'senderEmail' as const, label: 'Sender email', required: false, mono: false },
          { key: 'subjectContains' as const, label: 'Subject contains', required: false, mono: false },
        ] as const).map(({ key, label, required, mono }) => (
          <div key={key}>
            <label className="mb-1 block text-xs uppercase tracking-wider text-muted-foreground">
              {label}{required && ' *'}
            </label>
            <input
              value={form[key]}
              onChange={set(key)}
              disabled={isSystem}
              className={`w-full rounded border border-border bg-input px-2 py-1.5 text-sm disabled:opacity-50 ${mono ? 'font-mono text-xs' : ''}`}
            />
          </div>
        ))}

        {([
          { key: 'amountRegex' as const, label: 'Amount regex *' },
          { key: 'merchantRegex' as const, label: 'Merchant regex *' },
          { key: 'currencyRegex' as const, label: 'Currency regex' },
        ] as const).map(({ key, label }) => (
          <div key={key}>
            <label className="mb-1 block text-xs uppercase tracking-wider text-muted-foreground">
              {label}
            </label>
            <input
              value={form[key]}
              onChange={set(key)}
              disabled={isSystem}
              className="w-full rounded border border-border bg-input px-2 py-1.5 font-mono text-xs disabled:opacity-50"
            />
          </div>
        ))}

        <div className="flex items-center gap-2">
          <input
            type="checkbox"
            id="rule-enabled"
            checked={form.enabled}
            onChange={set('enabled')}
          />
          <label htmlFor="rule-enabled" className="text-sm text-foreground">Enabled</label>
        </div>
      </div>

      {/* Multi-sample regex tester */}
      <div className="space-y-3 rounded-lg border border-border p-4">
        <div className="flex items-center justify-between">
          <h3 className="text-xs uppercase tracking-wider text-muted-foreground">Test Regexes</h3>
          <button
            type="button"
            onClick={addSample}
            className="text-xs text-primary hover:underline"
          >
            + Add sample
          </button>
        </div>

        {samples.map((body, i) => (
          <div key={i} className="space-y-1">
            <div className="flex items-start gap-2">
              <textarea
                value={body}
                onChange={(e) => updateSample(i, e.target.value)}
                placeholder="Paste an email body here…"
                rows={3}
                className="flex-1 rounded border border-border bg-input px-2 py-1.5 font-mono text-xs"
              />
              {samples.length > 1 && (
                <button
                  type="button"
                  onClick={() => removeSample(i)}
                  className="mt-1 text-xs text-muted-foreground hover:text-foreground"
                  aria-label="Remove sample"
                >
                  ✕
                </button>
              )}
            </div>
          </div>
        ))}

        {/* Results table */}
        {samples.some((s) => s.trim()) && (
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b border-border">
                  <th className="py-1 pr-3 text-left text-muted-foreground">#</th>
                  <th className="py-1 pr-3 text-left text-muted-foreground">Preview</th>
                  <th className="py-1 pr-3 text-left text-muted-foreground">Amount</th>
                  <th className="py-1 pr-3 text-left text-muted-foreground">Merchant</th>
                  <th className="py-1 text-left text-muted-foreground">Currency</th>
                </tr>
              </thead>
              <tbody>
                {results.map((r, i) => (
                  <tr key={i} className="border-b border-border/50 last:border-0">
                    <td className="py-1.5 pr-3 text-muted-foreground">{i + 1}</td>
                    <td className="max-w-[160px] truncate py-1.5 pr-3 font-mono text-muted-foreground">
                      {r.body.slice(0, 60) || '—'}
                    </td>
                    <td className="py-1.5 pr-3"><ResultCell result={r.amount} /></td>
                    <td className="py-1.5 pr-3"><ResultCell result={r.merchant} /></td>
                    <td className="py-1.5"><ResultCell result={r.currency} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Retroactive scan (edit mode only) */}
      {!isCreate && (
        <div className="flex items-start gap-2 rounded-lg border border-border bg-secondary/40 px-4 py-3">
          <input
            type="checkbox"
            id="rescan"
            checked={rescan}
            onChange={(e) => setRescan(e.target.checked)}
            className="mt-0.5"
          />
          <label htmlFor="rescan" className="text-sm text-foreground">
            Retroactive scan — re-process emails from the lookback window
            <p className="mt-0.5 text-xs text-muted-foreground">
              Previously processed emails will be re-extracted using the updated regexes and
              their transaction records updated. If the daemon is running, the scan will be
              queued for the next daemon start.
            </p>
          </label>
        </div>
      )}

      {formError && <p className="text-xs text-destructive">{formError}</p>}

      {/* Save / Cancel */}
      <div className="flex gap-2">
        <button
          onClick={handleSubmit}
          disabled={isPending}
          className="rounded bg-primary px-4 py-2 text-sm text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
        >
          {isPending ? 'Saving…' : 'Save rule'}
        </button>
        <Link
          to="/rules"
          className="rounded border border-border px-4 py-2 text-sm text-muted-foreground hover:text-foreground"
        >
          Cancel
        </Link>
      </div>
    </div>
  )
}
```

- [ ] **Step 3: TypeScript check + Prettier**

```bash
cd /Users/ksingh/code/expensor && task lint:fe 2>&1 | tail -5
task fmt:fe:check 2>&1 | tail -5
```
Fix any issues.

- [ ] **Step 4: Run full CI gate**

```bash
cd /Users/ksingh/code/expensor && task ci 2>&1 | tail -20
```
Expected: Go lint (0 issues), Go tests (all pass), TypeScript (0 errors), npm audit (clean).

- [ ] **Step 5: Commit**

```bash
cd /Users/ksingh/code/expensor && git add frontend/src/pages/rules/RuleForm.tsx
git commit --no-gpg-sign -m "feat(frontend): add full-page rule create/edit form with regex tester

RuleForm serves /rules/new (create) and /rules/:id (edit). Multi-sample
textarea section lets users paste email bodies and see live capture-group
results for amount, merchant, and currency regexes in a results table.
Edit mode shows a retroactive scan checkbox (checked by default) that
calls POST /api/daemon/rescan on save; the response determines whether
the toast shows 'scan started' or 'scan queued'. System rules are
read-only except for the enabled toggle."
```

---

## Self-Review Checklist

| Spec requirement | Task |
|---|---|
| `/rules`, `/rules/new`, `/rules/:id` routes | Task 4 |
| Rules sidebar entry (ScrollText, between Onboarding and Settings) | Task 4 |
| Remove Rules Settings tab + delete RulesSettings.tsx | Task 4 |
| `ForceRescan bool` in `daemon.RunConfig` | Task 1 |
| `runDaemon` skips `state.New` when `forceRescan=true` | Task 2 |
| `POST /api/daemon/rescan` — queues or rescans | Task 1 |
| `GET /api/config/active-reader` | Task 1 |
| `rescanDaemon` closure in main.go | Task 2 |
| `startDaemon` checks `rescan_pending` on each start | Task 2 |
| `rescanFn` wired to `NewHandlers` | Tasks 1 + 2 |
| `api.daemon.rescan()`, `api.config.getActiveReader()` | Task 3 |
| `useRescan()`, `useActiveReader()` hooks | Task 3 |
| Table columns: ☐ \| Enabled \| Name \| Sender \| Subject \| Source \| Edit \| Delete | Task 5 |
| Select-all header checkbox | Task 5 |
| Bulk Enable/Disable/Delete toolbar (≥1 selected) | Task 5 |
| Delete always visible, disabled for system rules | Task 5 |
| Source badge whitespace-nowrap | Task 5 |
| Client-side export via Blob URL (selected rules only) | Task 5 |
| Import unchanged (file → POST /api/rules/import) | Task 5 |
| Full-page create/edit form | Task 6 |
| Multi-sample textarea + live results table | Task 6 |
| System rules read-only except Enabled | Task 6 |
| Retroactive scan checkbox (edit only, checked by default) | Task 6 |
| Toast: "scan started" vs "scan queued" | Task 6 |
| TDD for backend (tests before implementation) | Tasks 1 + 2 |
| `task ci` passes at end | Task 6 step 4 |
