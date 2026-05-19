# Extraction Diagnostics Follow-Up Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve the diagnostics workflow after Slice 2 by adding guided repair, explicit reprocessing, and better Thunderbird deduplication.

**Status:** Parked on 2026-05-17.

**Priority note:** The core diagnostics workflow is already implemented and has proven useful in production: failed emails can appear on the diagnostics page and can be opened in the rule editor. This follow-up is not currently high priority. Pick it up only if duplicate Thunderbird diagnostics, unclear repair guidance, or manual post-fix verification becomes recurring friction. Until then, prefer higher-impact product work.

**Architecture:** Keep diagnostics as the source of repair context. Add deterministic body hashing for readers without stable message IDs, add manual diagnostic reprocessing through existing extraction/rule logic, and add UI helpers that explain which regex failed.

**Tech Stack:** Go diagnostics store/API, Gmail/Thunderbird readers, React diagnostics page, RuleForm regex tester.

---

## Prerequisite

Complete Slice 2 extraction diagnostics first. This plan assumes:

- `extraction_diagnostics` table exists.
- `/diagnostics` page exists.
- `RuleForm` can load `?diagnostic=<id>`.

---

### Task 1: Thunderbird Diagnostic Deduplication

**Files:**
- Modify: `backend/pkg/api/api.go`
- Modify: `backend/pkg/reader/thunderbird/thunderbird.go`
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/store/store_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestDiagnosticBodyHashIsStable(t *testing.T) {
	a := api.DiagnosticBodyHash("sender@example.com", "Subject", "Body")
	b := api.DiagnosticBodyHash("sender@example.com", "Subject", "Body")
	if a == "" || a != b {
		t.Fatalf("hashes differ: %q %q", a, b)
	}
}

func TestExtractionDiagnostics_DedupesOpenRowsByBodyHash(t *testing.T) {
	ts := newTestStore(t)
	ctx := context.Background()
	input := api.ExtractionDiagnostic{
		Reader: "thunderbird",
		BodyHash: "hash-1",
		RuleName: "Card",
		FailureReasons: []string{api.FailureMerchantEmpty},
	}
	first, err := ts.RecordExtractionDiagnostic(ctx, input)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := ts.RecordExtractionDiagnostic(ctx, input)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected dedupe, got %s and %s", first.ID, second.ID)
	}
}
```

- [ ] **Step 2: Run backend tests**

Run: `task test:be`

Expected: FAIL because body hash does not exist.

- [ ] **Step 3: Add schema and store support**

Add migration:

```sql
ALTER TABLE extraction_diagnostics
    ADD COLUMN IF NOT EXISTS body_hash TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS extraction_diagnostics_open_body_hash_unique
    ON extraction_diagnostics (reader, body_hash, rule_name)
    WHERE status = 'open' AND body_hash <> '';
```

Update insert/upsert to use body hash when message ID is unavailable.

- [ ] **Step 4: Populate hash in Thunderbird**

Add helper:

```go
func DiagnosticBodyHash(sender, subject, body string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(sender) + "\n" + strings.TrimSpace(subject) + "\n" + strings.TrimSpace(body)))
	return hex.EncodeToString(sum[:])
}
```

Thunderbird diagnostics set `BodyHash`.

- [ ] **Step 5: Run tests and commit**

Run: `task test:be`

Expected: PASS.

```bash
git add backend/pkg/api/api.go backend/pkg/reader/thunderbird/thunderbird.go backend/internal/store/store.go backend/internal/store/store_test.go backend/migrations
git commit --no-gpg-sign -m "feat: dedupe thunderbird diagnostics"
```

---

### Task 2: Diagnostic Reprocess Endpoint

**Files:**
- Modify: `backend/internal/api/handlers.go`
- Modify: `backend/internal/api/server.go`
- Modify: `backend/internal/api/handlers_test.go`
- Modify: `backend/internal/store/store.go`

- [ ] **Step 1: Write failing handler test**

```go
func TestHandleReprocessDiagnostic_ResolvedOnSuccessfulExtraction(t *testing.T) {
	ms := &mockStore{
		diagnostic: &store.ExtractionDiagnosticRow{
			ID: "diag-1",
			EmailBody: "Amount: 12.34 at Cafe",
			AmountRegex: `Amount: ([\d.]+)`,
			MerchantRegex: `at (.+)`,
		},
	}
	h := newTestHandlersWithStore(ms)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/extraction-diagnostics/diag-1/reprocess", nil)
	req.SetPathValue("id", "diag-1")
	rr := httptest.NewRecorder()

	h.HandleReprocessExtractionDiagnostic(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if ms.updatedDiagnosticStatus != store.DiagnosticStatusResolved {
		t.Fatalf("status not resolved")
	}
}
```

- [ ] **Step 2: Run backend tests**

Run: `task test:be`

Expected: FAIL because endpoint does not exist.

- [ ] **Step 3: Implement reprocess**

Add `POST /api/extraction-diagnostics/{id}/reprocess`. It:

1. Loads diagnostic.
2. Compiles regex snapshots or the current rule if `rule_id` exists.
3. Runs `extractor.ExtractTransactionDetails` against `email_body`.
4. If amount non-zero and merchant non-empty, marks diagnostic `resolved`.
5. Returns extracted preview JSON.

Do not write a transaction in this task; this endpoint proves the rule now extracts.

- [ ] **Step 4: Run tests and commit**

Run: `task test:be`

Expected: PASS.

```bash
git add backend/internal/api/handlers.go backend/internal/api/server.go backend/internal/api/handlers_test.go backend/internal/store/store.go
git commit --no-gpg-sign -m "feat: reprocess extraction diagnostics"
```

---

### Task 3: Rule Repair Guidance UI

**Files:**
- Modify: `frontend/src/pages/Diagnostics.tsx`
- Modify: `frontend/src/pages/rules/RuleForm.tsx`
- Modify: related tests

- [ ] **Step 1: Write failing UI tests**

```tsx
it('shows which extraction fields failed', async () => {
  renderWithProviders(<Diagnostics />, { route: '/diagnostics' })

  expect(await screen.findByText('Amount did not extract')).toBeInTheDocument()
  expect(screen.getByText('Merchant did not extract')).toBeInTheDocument()
})

it('shows diagnostic regex guidance in rule form', async () => {
  renderWithProviders(<RuleForm />, { route: '/rules/rule-1?diagnostic=diag-1', path: '/rules/:id' })

  expect(await screen.findByText(/This diagnostic failed because/i)).toBeInTheDocument()
})
```

- [ ] **Step 2: Run frontend tests**

Run: `task test:fe`

Expected: FAIL because guidance text does not exist.

- [ ] **Step 3: Implement guidance**

Map reasons:

```ts
const reasonLabels: Record<string, string> = {
  amount_zero: 'Amount did not extract',
  merchant_empty: 'Merchant did not extract',
}
```

In `RuleForm`, show a compact warning band near regex inputs listing failed fields. Keep it concise and avoid explaining regex syntax in-app.

- [ ] **Step 4: Run tests and commit**

Run: `task test:fe`

Expected: PASS.

```bash
git add frontend/src/pages/Diagnostics.tsx frontend/src/pages/rules/RuleForm.tsx frontend/src/pages/*.test.tsx frontend/src/pages/rules/*.test.tsx
git commit --no-gpg-sign -m "feat: add diagnostic repair guidance"
```

---

### Task 4: Reprocess From Diagnostics Page

**Files:**
- Modify: `frontend/src/api/client.ts`
- Modify: `frontend/src/api/queries.ts`
- Modify: `frontend/src/pages/Diagnostics.tsx`
- Modify: `frontend/src/mocks/handlers.ts`
- Test: `frontend/src/pages/Diagnostics.test.tsx`

- [ ] **Step 1: Write failing tests**

```tsx
it('reprocesses a diagnostic and refreshes the list', async () => {
  const user = userEvent.setup()
  renderWithProviders(<Diagnostics />, { route: '/diagnostics' })

  await user.click(await screen.findByRole('button', { name: /reprocess/i }))

  expect(await screen.findByText(/extracted/i)).toBeInTheDocument()
})
```

- [ ] **Step 2: Run frontend tests**

Run: `task test:fe`

Expected: FAIL because hook/UI does not exist.

- [ ] **Step 3: Add client hook and UI button**

Add:

```ts
reprocess: (id: string) => apiClient.post<ExtractionDiagnosticReprocessResult>(`/extraction-diagnostics/${id}/reprocess`)
```

Show a `Reprocess` button on open diagnostics. On success, invalidate diagnostics and show a slide notification or inline status region.

- [ ] **Step 4: Run tests and commit**

Run: `task test:fe`

Expected: PASS.

```bash
git add frontend/src/api frontend/src/pages/Diagnostics.tsx frontend/src/pages/Diagnostics.test.tsx frontend/src/mocks/handlers.ts
git commit --no-gpg-sign -m "feat: reprocess diagnostics from ui"
```

---

### Task 5: Final Verification

- [ ] **Step 1: Run backend and frontend suites**

Run: `task test:be`

Expected: PASS.

Run: `task test:fe`

Expected: PASS.

- [ ] **Step 2: Lint**

Run: `task lint`

Expected: PASS.

- [ ] **Step 3: Commit fixes if needed**

```bash
git add backend frontend
git commit --no-gpg-sign -m "test: verify diagnostics follow-up"
```

Only create this commit if verification changed files.
