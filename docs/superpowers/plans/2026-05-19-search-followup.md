# Search Follow-Up Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build on Slice 3 search improvements with ranking and safe match highlighting.

**Status:** Partially complete; safe highlighting implemented on 2026-05-17, ranking remains open.

**Architecture:** Keep basic search backwards-compatible. Add ranked search metadata from the backend and render highlights safely on the frontend. Do not add saved searches or advanced query syntax in this plan.

**Tech Stack:** PostgreSQL full-text/trigram search, Go store/API, React Transactions page, TanStack Query, URL search params.

---

## Prerequisite

Complete Slice 3 improved transaction search first. This plan assumes hybrid full-text plus substring search exists.

---

### Task 1: Ranked Search Results

**Files:**
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/store/store_test.go`
- Modify: `backend/internal/api/handlers.go`
- Modify: `frontend/src/api/types.ts`

- [ ] **Step 1: Write failing backend tests**

```go
func TestSearchTransactions_RanksMerchantMatchAboveDescription(t *testing.T) {
	ts := newTestStore(t)
	ctx := context.Background()
	insertTestTransaction(t, ts, store.Transaction{MerchantInfo: "Amazon", Description: "shopping", Amount: 10})
	insertTestTransaction(t, ts, store.Transaction{MerchantInfo: "Other", Description: "Amazon subscription", Amount: 20})

	rows, _, err := ts.SearchTransactions(ctx, "amazon", store.ListFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("SearchTransactions: %v", err)
	}
	if rows[0].MerchantInfo != "Amazon" {
		t.Fatalf("merchant match should rank first: %+v", rows)
	}
}
```

- [ ] **Step 2: Run backend tests**

Run: `task test:be`

Expected: FAIL because order is still timestamp/default sort.

- [ ] **Step 3: Add rank field**

Add optional field:

```go
SearchRank float64 `json:"search_rank,omitempty"`
```

Compute rank with:

- merchant exact/ILIKE match boost
- full-text rank
- trigram similarity

Keep explicit user sort parameters dominant when provided.

- [ ] **Step 4: Run tests and commit**

Run: `task test:be`

Expected: PASS.

```bash
git add backend/internal/store/store.go backend/internal/store/store_test.go backend/internal/api/handlers.go frontend/src/api/types.ts
git commit --no-gpg-sign -m "feat: rank transaction search results"
```

---

### Task 2: Safe Search Highlighting

**Files:**
- Modify: `backend/internal/store/store.go`
- Modify: `frontend/src/api/types.ts`
- Modify: `frontend/src/pages/Transactions.tsx`
- Modify: `frontend/src/pages/Transactions.test.tsx`

- [x] **Step 1: Write failing frontend test**

```tsx
it('highlights matching merchant text safely', async () => {
  renderTransactions('/transactions?q=ama')

  expect(await screen.findByText('Ama')).toHaveClass('bg-primary/20')
})
```

- [x] **Step 2: Run tests**

Run: `task test:fe`

Expected: FAIL because highlights do not render.

- [x] **Step 3: Add highlight spans without raw HTML**

Add frontend helper:

```tsx
function HighlightedText({ text, query }: { text: string; query: string }) {
  const normalized = query.trim()
  if (!normalized) return <>{text}</>
  const index = text.toLowerCase().indexOf(normalized.toLowerCase())
  if (index === -1) return <>{text}</>
  return (
    <>
      {text.slice(0, index)}
      <mark className="rounded bg-primary/20 px-0.5 text-primary">{text.slice(index, index + normalized.length)}</mark>
      {text.slice(index + normalized.length)}
    </>
  )
}
```

Do not use `dangerouslySetInnerHTML`.

- [x] **Step 4: Run tests**

Run: `task test:fe`

Expected: PASS. Completed with `task test:fe` and `task lint:fe`.

```bash
git add frontend/src/pages/Transactions.tsx frontend/src/pages/Transactions.test.tsx frontend/src/api/types.ts backend/internal/store/store.go
git commit --no-gpg-sign -m "feat: highlight transaction search matches"
```

---

### Out Of Scope

- Saved searches.
- Advanced query syntax such as `merchant:amazon source:"Credit Card"`.
- New saved-search migrations, endpoints, or UI.

These can be reconsidered later if user workflows show repeated need, but they should not be picked up as part of this follow-up.

---

### Task 3: Final Verification

- [ ] **Step 1: Run suites**

Run: `task test:be`

Expected: PASS.

Run: `task test:fe`

Expected: PASS.

Run: `task lint`

Expected: PASS.

- [ ] **Step 2: Commit fixes if needed**

```bash
git add backend frontend
git commit --no-gpg-sign -m "test: verify search follow-up"
```

Only create this commit if verification changed files.
