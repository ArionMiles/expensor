# Extraction Diagnostics Workflow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist failed extraction diagnostics from Gmail and Thunderbird, expose them through API/UI, and open the rule editor with the failed email body loaded as a test sample.

**Architecture:** Readers emit best-effort diagnostics through a shared `pkg/api.DiagnosticSink`; `internal/store.Store` implements persistence and API listing/status updates. The frontend adds a `/diagnostics` page and extends `RuleForm` to consume a diagnostic ID from URL search params.

**Tech Stack:** Go, pgx/v5, PostgreSQL migrations, net/http handlers, React, Vite, TanStack Query, React Router URL state, Vitest/MSW.

---

## File Structure

- Create `backend/migrations/012_extraction_diagnostics.sql`: diagnostics table, constraints, indexes.
- Modify `backend/pkg/api/api.go`: add `ExtractionDiagnostic`, `DiagnosticSink`, failure reason constants, and rule ID/regex snapshot helpers on `Rule`.
- Modify `backend/internal/store/store.go`: add `ExtractionDiagnosticRow`, insert/list/status methods.
- Modify `backend/internal/store/store_test.go`: integration coverage for diagnostics persistence and status transitions.
- Modify `backend/internal/api/store.go`: add diagnostics methods to `Storer`.
- Modify `backend/internal/api/handlers.go`: add list and status handlers.
- Modify `backend/internal/api/handlers_test.go`: handler and mock store coverage.
- Modify `backend/internal/api/server.go`: register diagnostics routes.
- Modify `backend/pkg/reader/gmail/gmail.go` and tests: record diagnostics after invalid extraction.
- Modify `backend/pkg/reader/thunderbird/thunderbird.go` and tests: record diagnostics after invalid extraction.
- Modify `backend/pkg/plugins/readers/gmail/plugin.go` and `backend/pkg/plugins/readers/thunderbird/plugin.go`: pass sink through reader config.
- Modify `backend/internal/plugins/registry.go`, `backend/internal/daemon/runner.go`, and `backend/cmd/server/main.go`: thread the store sink into reader construction.
- Modify `frontend/src/api/types.ts`, `frontend/src/api/client.ts`, `frontend/src/api/queries.ts`: diagnostics types and hooks.
- Create `frontend/src/pages/Diagnostics.tsx` and `frontend/src/pages/Diagnostics.test.tsx`.
- Modify `frontend/src/App.tsx`, `frontend/src/components/Sidebar.tsx`, and `frontend/src/lib/navigation.ts`: route/navigation.
- Modify `frontend/src/pages/rules/RuleForm.tsx` and add/extend tests for diagnostic prefill.
- Modify `frontend/src/mocks/handlers.ts`: MSW diagnostics handlers.

---

### Task 1: Shared Diagnostic Contract

**Files:**
- Modify: `backend/pkg/api/api.go`
- Test: `backend/pkg/api/api_test.go`

- [ ] **Step 1: Write the failing tests**

Add tests proving rule snapshots preserve regex strings and failure reasons are derived from empty merchant/zero amount:

```go
func TestRuleDiagnosticSnapshot(t *testing.T) {
	rule := api.Rule{
		ID:              "rule-1",
		Name:            "Card",
		Amount:          regexp.MustCompile(`Amount: ([\d.]+)`),
		MerchantInfo:    regexp.MustCompile(`at (.+)`),
		Currency:        regexp.MustCompile(`Currency: ([A-Z]{3})`),
		Source:          "Credit Card",
		SenderEmail:     "alerts@example.com",
		SubjectContains: "spent",
	}

	snapshot := rule.DiagnosticSnapshot()

	if snapshot.RuleID != "rule-1" || snapshot.RuleName != "Card" {
		t.Fatalf("unexpected rule identity: %+v", snapshot)
	}
	if snapshot.AmountRegex != `Amount: ([\d.]+)` {
		t.Fatalf("amount regex = %q", snapshot.AmountRegex)
	}
	if snapshot.MerchantRegex != `at (.+)` {
		t.Fatalf("merchant regex = %q", snapshot.MerchantRegex)
	}
	if snapshot.CurrencyRegex != `Currency: ([A-Z]{3})` {
		t.Fatalf("currency regex = %q", snapshot.CurrencyRegex)
	}
}

func TestExtractionFailureReasons(t *testing.T) {
	reasons := api.ExtractionFailureReasons(&api.TransactionDetails{Amount: 0, MerchantInfo: ""})
	if diff := cmp.Diff([]string{api.FailureAmountZero, api.FailureMerchantEmpty}, reasons); diff != "" {
		t.Fatalf("reasons mismatch (-want +got):\n%s", diff)
	}

	reasons = api.ExtractionFailureReasons(&api.TransactionDetails{Amount: 42, MerchantInfo: "Cafe"})
	if len(reasons) != 0 {
		t.Fatalf("expected no reasons, got %v", reasons)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `task test:be`

Expected: FAIL because `Rule.ID`, `DiagnosticSnapshot`, constants, and `ExtractionFailureReasons` do not exist.

- [ ] **Step 3: Implement the shared contract**

In `backend/pkg/api/api.go`, add:

```go
const (
	FailureAmountZero    = "amount_zero"
	FailureMerchantEmpty = "merchant_empty"
)

type RuleDiagnosticSnapshot struct {
	RuleID        string
	RuleName      string
	AmountRegex   string
	MerchantRegex string
	CurrencyRegex string
}

type ExtractionDiagnostic struct {
	Reader         string
	MessageID      string
	SenderEmail    string
	Subject        string
	EmailBody      string
	RuleID         string
	RuleName       string
	AmountRegex    string
	MerchantRegex  string
	CurrencyRegex  string
	FailureReasons []string
}

type DiagnosticSink interface {
	RecordExtractionDiagnostic(ctx context.Context, diagnostic ExtractionDiagnostic) error
}
```

Add `ID string` to `Rule`, then add:

```go
func (r Rule) DiagnosticSnapshot() RuleDiagnosticSnapshot {
	snapshot := RuleDiagnosticSnapshot{
		RuleID:   r.ID,
		RuleName: r.Name,
	}
	if r.Amount != nil {
		snapshot.AmountRegex = r.Amount.String()
	}
	if r.MerchantInfo != nil {
		snapshot.MerchantRegex = r.MerchantInfo.String()
	}
	if r.Currency != nil {
		snapshot.CurrencyRegex = r.Currency.String()
	}
	return snapshot
}

func ExtractionFailureReasons(transaction *TransactionDetails) []string {
	if transaction == nil {
		return nil
	}
	reasons := make([]string, 0, 2)
	if transaction.Amount == 0 {
		reasons = append(reasons, FailureAmountZero)
	}
	if strings.TrimSpace(transaction.MerchantInfo) == "" {
		reasons = append(reasons, FailureMerchantEmpty)
	}
	return reasons
}
```

Update `backend/cmd/server/main.go` `compileRule` to set `ID: row.ID`.

- [ ] **Step 4: Run tests**

Run: `task test:be`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/pkg/api/api.go backend/pkg/api/api_test.go backend/cmd/server/main.go
git commit --no-gpg-sign -m "feat: add extraction diagnostic contract"
```

---

### Task 2: Store Diagnostics

**Files:**
- Create: `backend/migrations/012_extraction_diagnostics.sql`
- Modify: `backend/internal/store/store.go`
- Test: `backend/internal/store/store_test.go`

- [ ] **Step 1: Add failing store tests**

Add integration tests:

```go
func TestExtractionDiagnostics_InsertListAndStatus(t *testing.T) {
	ts := newTestStore(t)
	ctx := context.Background()

	row, err := ts.RecordExtractionDiagnostic(ctx, api.ExtractionDiagnostic{
		Reader:         "gmail",
		MessageID:      "msg-1",
		SenderEmail:    "alerts@example.com",
		Subject:        "Spent",
		EmailBody:      "Amount: 0",
		RuleID:         "",
		RuleName:       "Card",
		AmountRegex:    `Amount: ([\d.]+)`,
		MerchantRegex:  `at (.+)`,
		CurrencyRegex:  `Currency: ([A-Z]{3})`,
		FailureReasons: []string{api.FailureAmountZero, api.FailureMerchantEmpty},
	})
	if err != nil {
		t.Fatalf("RecordExtractionDiagnostic: %v", err)
	}
	if row.Status != store.DiagnosticStatusOpen {
		t.Fatalf("status = %q", row.Status)
	}

	rows, err := ts.ListExtractionDiagnostics(ctx, store.DiagnosticFilter{Status: store.DiagnosticStatusOpen})
	if err != nil {
		t.Fatalf("ListExtractionDiagnostics: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != row.ID {
		t.Fatalf("unexpected rows: %+v", rows)
	}

	updated, err := ts.UpdateExtractionDiagnosticStatus(ctx, row.ID, store.DiagnosticStatusResolved)
	if err != nil {
		t.Fatalf("UpdateExtractionDiagnosticStatus: %v", err)
	}
	if updated.Status != store.DiagnosticStatusResolved || updated.ResolvedAt == nil {
		t.Fatalf("unexpected updated row: %+v", updated)
	}
}

func TestExtractionDiagnostics_DedupesOpenGmailRows(t *testing.T) {
	ts := newTestStore(t)
	ctx := context.Background()
	input := api.ExtractionDiagnostic{
		Reader:         "gmail",
		MessageID:      "msg-1",
		RuleName:       "Card",
		EmailBody:      "body",
		FailureReasons: []string{api.FailureAmountZero},
	}

	first, err := ts.RecordExtractionDiagnostic(ctx, input)
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}
	second, err := ts.RecordExtractionDiagnostic(ctx, input)
	if err != nil {
		t.Fatalf("second insert: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected duplicate open diagnostic to update existing row, got %s and %s", first.ID, second.ID)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `task test:be`

Expected: FAIL because diagnostics store methods and migration do not exist.

- [ ] **Step 3: Add migration**

Create `backend/migrations/012_extraction_diagnostics.sql`:

```sql
CREATE TABLE IF NOT EXISTS extraction_diagnostics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status TEXT NOT NULL DEFAULT 'open'
        CHECK (status IN ('open', 'resolved', 'ignored')),
    reader TEXT NOT NULL,
    message_id TEXT,
    sender_email TEXT NOT NULL DEFAULT '',
    subject TEXT NOT NULL DEFAULT '',
    email_body TEXT NOT NULL DEFAULT '',
    rule_id UUID,
    rule_name TEXT NOT NULL DEFAULT '',
    amount_regex TEXT NOT NULL DEFAULT '',
    merchant_regex TEXT NOT NULL DEFAULT '',
    currency_regex TEXT NOT NULL DEFAULT '',
    failure_reasons TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS extraction_diagnostics_status_created_idx
    ON extraction_diagnostics (status, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS extraction_diagnostics_open_unique
    ON extraction_diagnostics (reader, message_id, rule_name)
    WHERE status = 'open' AND message_id IS NOT NULL;
```

- [ ] **Step 4: Add store types and methods**

Add statuses:

```go
const (
	DiagnosticStatusOpen     = "open"
	DiagnosticStatusResolved = "resolved"
	DiagnosticStatusIgnored  = "ignored"
	DiagnosticStatusAll      = "all"
)
```

Add `ExtractionDiagnosticRow`, `DiagnosticFilter`, `ValidateDiagnosticStatus`, `RecordExtractionDiagnostic`, `ListExtractionDiagnostics`, `GetExtractionDiagnostic`, and `UpdateExtractionDiagnosticStatus`. Use `ON CONFLICT ON CONSTRAINT` only if the index is a named constraint; for a partial unique index, use:

```sql
ON CONFLICT (reader, message_id, rule_name) WHERE status = 'open' AND message_id IS NOT NULL
DO UPDATE SET
    sender_email = EXCLUDED.sender_email,
    subject = EXCLUDED.subject,
    email_body = EXCLUDED.email_body,
    rule_id = EXCLUDED.rule_id,
    amount_regex = EXCLUDED.amount_regex,
    merchant_regex = EXCLUDED.merchant_regex,
    currency_regex = EXCLUDED.currency_regex,
    failure_reasons = EXCLUDED.failure_reasons,
    updated_at = NOW()
```

Convert empty `RuleID` to SQL `NULL`; parse non-empty IDs as UUID text. Return `ErrNotFound` when status update affects no rows.

- [ ] **Step 5: Run tests**

Run: `task test:be`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/migrations/012_extraction_diagnostics.sql backend/internal/store/store.go backend/internal/store/store_test.go
git commit --no-gpg-sign -m "feat: persist extraction diagnostics"
```

---

### Task 3: Diagnostics API

**Files:**
- Modify: `backend/internal/api/store.go`
- Modify: `backend/internal/api/handlers.go`
- Modify: `backend/internal/api/handlers_test.go`
- Modify: `backend/internal/api/server.go`

- [ ] **Step 1: Write failing handler tests**

Add tests:

```go
func TestHandleListExtractionDiagnostics_DefaultsOpen(t *testing.T) {
	ms := &mockStore{diagnostics: []store.ExtractionDiagnosticRow{{ID: "diag-1", Status: store.DiagnosticStatusOpen, RuleName: "Card"}}}
	h := newTestHandlersWithStore(ms)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/extraction-diagnostics", nil)
	rr := httptest.NewRecorder()

	h.HandleListExtractionDiagnostics(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if ms.diagnosticFilter.Status != store.DiagnosticStatusOpen {
		t.Fatalf("status filter = %q", ms.diagnosticFilter.Status)
	}
}

func TestHandleUpdateExtractionDiagnosticStatus_InvalidStatus(t *testing.T) {
	h := newTestHandlersWithStore(&mockStore{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/extraction-diagnostics/diag-1/status", strings.NewReader(`{"status":"done"}`))
	req.SetPathValue("id", "diag-1")
	rr := httptest.NewRecorder()

	h.HandleUpdateExtractionDiagnosticStatus(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `task test:be`

Expected: FAIL because handlers and mock methods do not exist.

- [ ] **Step 3: Add Storer methods and mock methods**

Add to `Storer`:

```go
ListExtractionDiagnostics(ctx context.Context, filter store.DiagnosticFilter) ([]store.ExtractionDiagnosticRow, error)
GetExtractionDiagnostic(ctx context.Context, id string) (*store.ExtractionDiagnosticRow, error)
UpdateExtractionDiagnosticStatus(ctx context.Context, id, status string) (*store.ExtractionDiagnosticRow, error)
```

Add matching fields and methods on `mockStore` in `handlers_test.go`.

- [ ] **Step 4: Add handlers**

Add:

```go
func (h *Handlers) HandleListExtractionDiagnostics(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	status := r.URL.Query().Get("status")
	if status == "" {
		status = store.DiagnosticStatusOpen
	}
	if !store.ValidDiagnosticListStatus(status) {
		writeError(w, http.StatusUnprocessableEntity, "invalid status")
		return
	}
	rows, err := h.store.ListExtractionDiagnostics(r.Context(), store.DiagnosticFilter{Status: status})
	if err != nil {
		h.logger.Error("list extraction diagnostics", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list diagnostics")
		return
	}
	writeJSON(w, http.StatusOK, rows)
}
```

Add a status request struct and `HandleUpdateExtractionDiagnosticStatus` with `404` for `store.ErrNotFound` and `422` for invalid status.

Add `HandleGetExtractionDiagnostic`:

```go
func (h *Handlers) HandleGetExtractionDiagnostic(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	row, err := h.store.GetExtractionDiagnostic(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "diagnostic not found")
			return
		}
		h.logger.Error("get extraction diagnostic", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get diagnostic")
		return
	}
	writeJSON(w, http.StatusOK, row)
}
```

- [ ] **Step 5: Register routes**

In `registerRoutes` add:

```go
mux.HandleFunc("GET /api/extraction-diagnostics", h.HandleListExtractionDiagnostics)
mux.HandleFunc("GET /api/extraction-diagnostics/{id}", h.HandleGetExtractionDiagnostic)
mux.HandleFunc("PUT /api/extraction-diagnostics/{id}/status", h.HandleUpdateExtractionDiagnosticStatus)
```

- [ ] **Step 6: Run tests**

Run: `task test:be`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/api/store.go backend/internal/api/handlers.go backend/internal/api/handlers_test.go backend/internal/api/server.go
git commit --no-gpg-sign -m "feat: expose extraction diagnostics api"
```

---

### Task 4: Wire Diagnostic Sink Into Readers

**Files:**
- Modify: `backend/internal/plugins/registry.go`
- Modify: `backend/internal/daemon/runner.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/pkg/plugins/readers/gmail/plugin.go`
- Modify: `backend/pkg/plugins/readers/thunderbird/plugin.go`
- Modify: `backend/pkg/reader/gmail/gmail.go`
- Modify: `backend/pkg/reader/gmail/gmail_test.go`
- Modify: `backend/pkg/reader/thunderbird/thunderbird.go`
- Modify: `backend/pkg/reader/thunderbird/thunderbird_test.go`

- [ ] **Step 1: Write failing reader tests**

For Gmail, add a fake sink and a test around `processMessage` where extraction returns amount zero or empty merchant:

```go
type recordingDiagnosticSink struct {
	diagnostics []api.ExtractionDiagnostic
	err         error
}

func (s *recordingDiagnosticSink) RecordExtractionDiagnostic(_ context.Context, d api.ExtractionDiagnostic) error {
	s.diagnostics = append(s.diagnostics, d)
	return s.err
}
```

Assert `len(sink.diagnostics) == 1`, `Reader == "gmail"`, `MessageID == "msg-1"`, and reasons include `amount_zero` or `merchant_empty`. Add a second test where the sink returns an error and `processMessage` still returns `nil`.

For Thunderbird, test `extractTransaction` with a message/rule producing the same failed extraction and assert a diagnostic is recorded.

- [ ] **Step 2: Run tests to verify they fail**

Run: `task test:be`

Expected: FAIL because reader configs do not accept a diagnostic sink and readers do not record diagnostics.

- [ ] **Step 3: Extend plugin and daemon interfaces**

Change the reader factory signatures to accept `diagnosticSink api.DiagnosticSink`. The affected path is:

- `internal/plugins.ReaderFactory`
- `internal/plugins.Registry.CreateReader`
- `internal/daemon.Config`
- `internal/daemon.Runner`
- `cmd/server/main.go` runner construction
- Gmail and Thunderbird plugin `NewReader`

Pass the store as the sink from `main.go` because `*store.Store` implements `RecordExtractionDiagnostic`.

- [ ] **Step 4: Add reader config fields**

Add `DiagnosticSink api.DiagnosticSink` to Gmail and Thunderbird reader configs and structs. In plugins, pass it through:

```go
DiagnosticSink: diagnosticSink,
```

- [ ] **Step 5: Record diagnostics in readers**

After extraction and category resolution, call a helper when `api.ExtractionFailureReasons(transaction)` is non-empty:

```go
func (r *Reader) recordExtractionDiagnostic(ctx context.Context, d api.ExtractionDiagnostic) {
	if r.diagnosticSink == nil {
		return
	}
	if err := r.diagnosticSink.RecordExtractionDiagnostic(ctx, d); err != nil {
		r.logger.Warn("failed to record extraction diagnostic",
			"reader", d.Reader,
			"rule", d.RuleName,
			"subject", d.Subject,
			"error", err,
		)
	}
}
```

Build diagnostic data from headers/body/rule snapshot. Gmail uses `message_id`; Thunderbird leaves `message_id` empty unless a reliable message ID is already available from headers.

- [ ] **Step 6: Run tests**

Run: `task test:be`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/plugins/registry.go backend/internal/daemon/runner.go backend/cmd/server/main.go backend/pkg/plugins/readers/gmail/plugin.go backend/pkg/plugins/readers/thunderbird/plugin.go backend/pkg/reader/gmail/gmail.go backend/pkg/reader/gmail/gmail_test.go backend/pkg/reader/thunderbird/thunderbird.go backend/pkg/reader/thunderbird/thunderbird_test.go
git commit --no-gpg-sign -m "feat: record reader extraction diagnostics"
```

---

### Task 5: Frontend API Hooks

**Files:**
- Modify: `frontend/src/api/types.ts`
- Modify: `frontend/src/api/client.ts`
- Modify: `frontend/src/api/queries.ts`
- Modify: `frontend/src/mocks/handlers.ts`

- [ ] **Step 1: Add frontend types**

Add:

```ts
export type ExtractionDiagnosticStatus = 'open' | 'resolved' | 'ignored'
export type ExtractionDiagnosticListStatus = ExtractionDiagnosticStatus | 'all'

export interface ExtractionDiagnostic {
  id: string
  status: ExtractionDiagnosticStatus
  reader: string
  message_id: string
  sender_email: string
  subject: string
  email_body: string
  rule_id?: string
  rule_name: string
  amount_regex: string
  merchant_regex: string
  currency_regex: string
  failure_reasons: string[]
  created_at: string
  updated_at: string
  resolved_at?: string | null
}
```

- [ ] **Step 2: Add client methods**

Add `api.extractionDiagnostics.list(status)`, `api.extractionDiagnostics.get(id)`, and `api.extractionDiagnostics.updateStatus(id, status)`.

```ts
extractionDiagnostics: {
  list: (status: ExtractionDiagnosticListStatus = 'open') =>
    apiClient.get<ExtractionDiagnostic[]>(`/extraction-diagnostics?status=${encodeURIComponent(status)}`),
  get: (id: string) => apiClient.get<ExtractionDiagnostic>(`/extraction-diagnostics/${id}`),
  updateStatus: (id: string, status: ExtractionDiagnosticStatus) =>
    apiClient.put<ExtractionDiagnostic>(`/extraction-diagnostics/${id}/status`, { status }),
},
```

- [ ] **Step 3: Add query hooks**

Add:

```ts
export function useExtractionDiagnostics(status: ExtractionDiagnosticListStatus) {
  return useQuery({
    queryKey: ['extraction-diagnostics', status] as const,
    queryFn: () => api.extractionDiagnostics.list(status).then((r) => r.data),
  })
}

export function useExtractionDiagnostic(id: string | null) {
  return useQuery({
    queryKey: ['extraction-diagnostics', id] as const,
    queryFn: () => api.extractionDiagnostics.get(id!).then((r) => r.data),
    enabled: Boolean(id),
  })
}

export function useUpdateExtractionDiagnosticStatus() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, status }: { id: string; status: ExtractionDiagnosticStatus }) =>
      api.extractionDiagnostics.updateStatus(id, status).then((r) => r.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['extraction-diagnostics'] }),
  })
}
```

Invalidate `['extraction-diagnostics']` after status updates.

- [ ] **Step 4: Add MSW handlers**

Add in-memory handlers for list, get, and status update to `frontend/src/mocks/handlers.ts`.

- [ ] **Step 5: Run frontend tests**

Run: `task test:fe`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/api/types.ts frontend/src/api/client.ts frontend/src/api/queries.ts frontend/src/mocks/handlers.ts
git commit --no-gpg-sign -m "feat: add extraction diagnostics frontend api"
```

---

### Task 6: Diagnostics Page

**Files:**
- Create: `frontend/src/pages/Diagnostics.tsx`
- Create: `frontend/src/pages/Diagnostics.test.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/components/Sidebar.tsx`
- Modify: `frontend/src/lib/navigation.ts`

- [ ] **Step 1: Write failing component tests**

Test:

```tsx
it('renders diagnostics and persists status filter in the URL', async () => {
  renderWithProviders(<Diagnostics />, { route: '/diagnostics?status=ignored' })

  expect(await screen.findByText('Extraction diagnostics')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Ignored' })).toHaveAttribute('aria-pressed', 'true')
})

it('links fix rule to the rule editor with the diagnostic id', async () => {
  renderWithProviders(<Diagnostics />, { route: '/diagnostics' })

  const link = await screen.findByRole('link', { name: /fix rule/i })
  expect(link).toHaveAttribute('href', '/rules/rule-1?diagnostic=diag-1')
})
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `task test:fe`

Expected: FAIL because the page does not exist.

- [ ] **Step 3: Implement page**

Use `useSearchParams` for `status`; render segmented buttons for `Open`, `Resolved`, `Ignored`, `All`. Render a table with fixed, dense rows and actions:

- `Fix rule`: `<Link to={diagnostic.rule_id ? `/rules/${diagnostic.rule_id}?diagnostic=${diagnostic.id}` : `/rules/new?diagnostic=${diagnostic.id}`}>`
- `Mark resolved`: mutation to `resolved`.
- `Ignore`: mutation to `ignored`.
- `Reopen`: mutation to `open` for non-open rows.

Use plain buttons styled like existing pages; avoid native select.

- [ ] **Step 4: Add route/navigation**

Add lazy route `/diagnostics`, sidebar item with a lucide icon such as `CircleAlert`, and command palette target with keywords `diagnostics`, `failed extraction`, `regex`.

- [ ] **Step 5: Run frontend tests**

Run: `task test:fe`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/pages/Diagnostics.tsx frontend/src/pages/Diagnostics.test.tsx frontend/src/App.tsx frontend/src/components/Sidebar.tsx frontend/src/lib/navigation.ts
git commit --no-gpg-sign -m "feat: add extraction diagnostics page"
```

---

### Task 7: Rule Editor Diagnostic Prefill

**Files:**
- Modify: `frontend/src/pages/rules/RuleForm.tsx`
- Test: create or modify `frontend/src/pages/rules/RuleForm.test.tsx`

- [ ] **Step 1: Write failing tests**

Test create-mode prefill:

```tsx
it('loads diagnostic email body into the first test sample', async () => {
  renderWithProviders(<RuleForm />, { route: '/rules/new?diagnostic=diag-1' })

  expect(await screen.findByDisplayValue(/Amount: 0/)).toBeInTheDocument()
  expect(screen.getByDisplayValue('Card')).toBeInTheDocument()
})
```

Test edit-mode keeps existing rule fields but loads sample:

```tsx
it('keeps existing edit rule fields and loads diagnostic sample', async () => {
  renderWithProviders(<RuleForm />, { route: '/rules/rule-1?diagnostic=diag-1', path: '/rules/:id' })

  expect(await screen.findByDisplayValue('Existing rule name')).toBeInTheDocument()
  expect(screen.getByDisplayValue(/Amount: 0/)).toBeInTheDocument()
})
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `task test:fe`

Expected: FAIL because `RuleForm` ignores `diagnostic`.

- [ ] **Step 3: Implement diagnostic loading**

Import `useSearchParams` and `useExtractionDiagnostic`. Read `diagnostic` from search params. When loaded:

- Set `samples[0]` to `diagnostic.email_body`.
- In create mode, populate rule fields from diagnostic snapshots:
  - `name = diagnostic.rule_name`
  - `senderEmail = diagnostic.sender_email`
  - `subjectContains = diagnostic.subject`
  - regex/source fields from diagnostic snapshots.
- In edit mode, keep the loaded rule values and only fill blank regex/source fields if present.

Add a small non-card notice above the test email area: `Loaded diagnostic sample from {diagnostic.sender_email || 'email'}.`

- [ ] **Step 4: Run frontend tests**

Run: `task test:fe`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/rules/RuleForm.tsx frontend/src/pages/rules/RuleForm.test.tsx
git commit --no-gpg-sign -m "feat: preload rule editor from diagnostics"
```

---

### Task 8: Final Verification

**Files:**
- Potentially modify generated OpenAPI artifacts if this repo’s current API docs generation reports drift.

- [ ] **Step 1: Format**

Run: `task fmt`

Expected: PASS.

- [ ] **Step 2: Backend lint**

Run: `task lint:be:prod`

Expected: `0 issues`.

- [ ] **Step 3: Frontend lint**

Run: `task lint:fe`

Expected: PASS.

- [ ] **Step 4: Tests**

Run: `task test:be`

Expected: PASS.

Run: `task test:fe`

Expected: PASS.

- [ ] **Step 5: Run broad verification**

Run:
```bash
task lint:be:prod
task test:be
task lint:fe
task fmt:fe:check
task audit:fe
```

Expected: PASS. If audit fails because an external registry is unavailable, record the exact failure and rerun the narrow commands that do not need the network.

- [ ] **Step 6: Commit final fixes if any**

```bash
git add backend frontend api docs
git commit --no-gpg-sign -m "test: verify extraction diagnostics workflow"
```

Only create this commit if formatting, generated files, or test fixes changed files after Task 7.

---

## Self-Review Notes

- Spec coverage: persistence, Gmail, Thunderbird, API list/status, frontend diagnostics page, and rule-editor prefill are each covered by a task.
- Backend observability requirement: Task 4 keeps diagnostic writes best-effort and logs sink failures.
- Reader scope: both Gmail and Thunderbird are explicitly included.
- URL state: Task 6 requires diagnostics status filter through `useSearchParams`.
- No native UI controls are required by the planned frontend work.
