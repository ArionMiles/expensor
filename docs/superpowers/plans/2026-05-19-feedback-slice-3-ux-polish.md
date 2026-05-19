# Feedback Slice 3 UX Polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve daily-use frontend workflows without mixing UI polish into deeper platform rewrites.

**Architecture:** Keep this as a frontend-first slice with one bounded backend search improvement. Use existing project components and URL-state patterns; avoid native controls and clipped floating UI.

**Tech Stack:** React, Vite, Tailwind, React Router `useSearchParams`, TanStack Query, Vitest/MSW, Go store tests for transaction search.

---

## Scope Notes

The Spend Breakdown toggle order item from the feedback triage was already completed in Slice 1. This plan keeps a verification step for it and implements the remaining Slice 3 items:

- Reader setup guide responsive width.
- Command palette descriptions and searchable metadata.
- Date picker custom time controls.
- Selected transaction sum in the transactions toolbar.
- Improved transaction search.

---

### Task 1: Setup Guide Responsive Width

**Files:**
- Modify: `frontend/src/pages/setup/Wizard.tsx`
- Test: `frontend/src/pages/setup/Wizard.test.tsx` if present, otherwise create `frontend/src/pages/setup/Wizard.layout.test.tsx`

- [ ] **Step 1: Write the failing test**

Add a test that renders the guide step and asserts the guide panel can use a wider max width than the form content:

```tsx
it('renders the setup guide in a wider responsive panel', async () => {
  renderWithProviders(<Wizard />, { route: '/setup?step=guide&reader=gmail' })

  const guide = await screen.findByTestId('reader-setup-guide')
  expect(guide).toHaveClass('max-w-5xl')
  expect(screen.getByTestId('setup-form-shell')).toHaveClass('max-w-2xl')
})
```

- [ ] **Step 2: Run the failing test**

Run: `task test:fe`

Expected: FAIL because the setup guide/form shells do not expose the expected layout test IDs/classes.

- [ ] **Step 3: Implement the layout split**

In `Wizard.tsx`, keep the onboarding form shell at the existing narrow width and wrap only the reader guide content in a wider shell:

```tsx
const shellClass = currentStep === 'guide' ? 'mx-auto w-full max-w-5xl px-4 py-6' : 'mx-auto w-full max-w-2xl px-4 py-6'
```

Add `data-testid="reader-setup-guide"` to the guide container and `data-testid="setup-form-shell"` to the standard form shell. Preserve mobile behavior with `w-full`, `min-w-0`, and no fixed pixel widths.

- [ ] **Step 4: Run tests**

Run: `task test:fe`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/setup/Wizard.tsx frontend/src/pages/setup/Wizard.layout.test.tsx
git commit --no-gpg-sign -m "fix: widen reader setup guide"
```

---

### Task 2: Command Palette Descriptions

**Files:**
- Modify: `frontend/src/lib/navigation.ts`
- Modify: `frontend/src/components/CommandPalette.tsx`
- Test: create or modify `frontend/src/components/CommandPalette.test.tsx`

- [ ] **Step 1: Write the failing tests**

Add tests for searchable descriptions and no path display:

```tsx
it('searches command descriptions', async () => {
  const user = userEvent.setup()
  render(
    <CommandPalette
      open
      targets={[{ id: 'rules', title: 'Rules', description: 'Tune email extraction patterns', path: '/rules' }]}
      onClose={vi.fn()}
      onNavigate={vi.fn()}
    />,
  )

  await user.type(screen.getByRole('textbox'), 'extraction')

  expect(screen.getByText('Rules')).toBeInTheDocument()
  expect(screen.getByText('Tune email extraction patterns')).toBeInTheDocument()
  expect(screen.queryByText('/rules')).not.toBeInTheDocument()
})

it('renders subtitle as breadcrumb text when present', () => {
  render(
    <CommandPalette
      open
      targets={[{ id: 'settings-sync', title: 'Settings', subtitle: 'Community Sync', description: 'Refresh shared categories', path: '/settings?tab=sync' }]}
      onClose={vi.fn()}
      onNavigate={vi.fn()}
    />,
  )

  expect(screen.getByText('Settings / Community Sync')).toBeInTheDocument()
})
```

- [ ] **Step 2: Run the failing tests**

Run: `task test:fe`

Expected: FAIL because `description` is not part of `NavigationTarget`, search ignores it, and paths are rendered.

- [ ] **Step 3: Add navigation descriptions**

Update `NavigationTarget`:

```ts
export interface NavigationTarget {
  id: string
  title: string
  subtitle?: string
  description: string
  path: string
  keywords?: string[]
}
```

Add concise descriptions to every `NAVIGATION_TARGETS` entry.

- [ ] **Step 4: Update palette matching and rendering**

Include `description` in the search haystack:

```ts
const haystack = [target.title, target.subtitle ?? '', target.description, ...(target.keywords ?? [])]
  .join(' ')
  .toLowerCase()
```

Render breadcrumb/title text instead of URL paths:

```tsx
<div className="truncate text-sm">
  {target.subtitle ? `${target.title} / ${target.subtitle}` : target.title}
</div>
<div className="truncate text-xs text-muted-foreground">{target.description}</div>
```

Remove the right-side `target.path` span.

- [ ] **Step 5: Run tests**

Run: `task test:fe`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/navigation.ts frontend/src/components/CommandPalette.tsx frontend/src/components/CommandPalette.test.tsx
git commit --no-gpg-sign -m "feat: improve command palette search"
```

---

### Task 3: Custom Date Picker Time Controls

**Files:**
- Modify: `frontend/src/components/DateRangePicker.tsx`
- Modify: `frontend/src/components/DateRangePicker.test.tsx`

- [ ] **Step 1: Write the failing tests**

Add tests proving native selects are gone and custom time buttons update the value:

```tsx
it('uses custom time controls instead of native selects', async () => {
  const user = userEvent.setup()
  render(<DateRangePicker value={{ from: new Date('2026-05-19T00:00:00') }} onChange={vi.fn()} />)

  await user.click(screen.getByRole('button', { name: /select date range/i }))

  expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Increase from hour' })).toBeInTheDocument()
})
```

- [ ] **Step 2: Run the failing tests**

Run: `task test:fe`

Expected: FAIL because `TimeInput` uses native `<select>`.

- [ ] **Step 3: Replace `TimeInput`**

Replace the two selects with custom steppers:

```tsx
function TimeStepper({ value, onChange, label }: { value: Date; onChange: (d: Date) => void; label: string }) {
  const updateHour = (delta: number) => onChange(setHours(value, (value.getHours() + delta + 24) % 24))
  const updateMinute = (delta: number) => onChange(setMinutes(value, (value.getMinutes() + delta + 60) % 60))

  return (
    <div className="grid grid-cols-[3rem_1fr] items-center gap-2">
      <span className="text-[10px] text-muted-foreground">{label}</span>
      <div className="flex items-center gap-1">
        <button type="button" aria-label={`Decrease ${label.toLowerCase()} hour`} onClick={() => updateHour(-1)} className="rounded border border-border px-2 py-1 text-xs">-</button>
        <span className="w-8 text-center font-mono text-xs tabular-nums">{padTwo(value.getHours())}</span>
        <button type="button" aria-label={`Increase ${label.toLowerCase()} hour`} onClick={() => updateHour(1)} className="rounded border border-border px-2 py-1 text-xs">+</button>
        <span className="px-1 text-xs text-muted-foreground">:</span>
        <button type="button" aria-label={`Decrease ${label.toLowerCase()} minute`} onClick={() => updateMinute(-15)} className="rounded border border-border px-2 py-1 text-xs">-</button>
        <span className="w-8 text-center font-mono text-xs tabular-nums">{padTwo(value.getMinutes())}</span>
        <button type="button" aria-label={`Increase ${label.toLowerCase()} minute`} onClick={() => updateMinute(15)} className="rounded border border-border px-2 py-1 text-xs">+</button>
      </div>
    </div>
  )
}
```

Use icons from `lucide-react` if the implementation already imports nearby icon buttons; otherwise the explicit `+`/`-` labels are acceptable because the controls are numeric steppers.

- [ ] **Step 4: Run tests and grep for native selects**

Run: `task test:fe`

Expected: PASS.

Run: `rg -n "<select|<datalist|alert\\(|confirm\\(|prompt\\(" frontend/src`

Expected: no matches in files touched by this task. Existing unrelated matches must be evaluated before committing.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/DateRangePicker.tsx frontend/src/components/DateRangePicker.test.tsx
git commit --no-gpg-sign -m "fix: replace native date picker time controls"
```

---

### Task 4: Selected Transaction Sum

**Files:**
- Modify: `frontend/src/pages/Transactions.tsx`
- Modify: `frontend/src/pages/Transactions.test.tsx`

- [ ] **Step 1: Write the failing test**

Add a test that selects one transaction and expects the toolbar sum to use selected rows:

```tsx
it('shows selected transaction sum when rows are selected', async () => {
  const user = userEvent.setup()
  renderTransactions('/transactions')

  await user.click(await screen.findByLabelText(/select transaction amazon/i))

  expect(screen.getByText('1 selected')).toBeInTheDocument()
  expect(screen.getByText(/selected total/i)).toBeInTheDocument()
  expect(screen.queryByText(/total spend/i)).not.toBeInTheDocument()
})
```

- [ ] **Step 2: Run the failing test**

Run: `task test:fe`

Expected: FAIL because the toolbar always renders the API `total_amount`.

- [ ] **Step 3: Compute selected sum**

Near `selectedTransactions`, add:

```ts
const selectedAmount = selectedTransactions.reduce((sum, tx) => sum + tx.amount, 0)
const displayedAmount = selectedCount > 0 ? selectedAmount : totalAmount
const displayedAmountLabel = selectedCount > 0 ? 'Selected total' : 'Total spend'
```

Render:

```tsx
<span className="text-xs text-muted-foreground">{displayedAmountLabel}</span>
<span className="font-mono text-xs tabular-nums text-primary">
  {formatCurrency(displayedAmount, baseCurrency)}
</span>
```

- [ ] **Step 4: Run tests**

Run: `task test:fe`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/Transactions.tsx frontend/src/pages/Transactions.test.tsx
git commit --no-gpg-sign -m "feat: show selected transaction sum"
```

---

### Task 5: Improved Transaction Search

**Files:**
- Create: `backend/migrations/013_search_trigram.sql` after Slice 2 diagnostics creates `012_extraction_diagnostics.sql`.
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/store/store_test.go`

- [ ] **Step 1: Write failing store tests**

Add tests showing substring and multi-word search both work:

```go
func TestSearchTransactions_SubstringMerchantMatch(t *testing.T) {
	ts := newTestStore(t)
	ctx := context.Background()
	insertTestTransaction(t, ts, store.Transaction{MerchantInfo: "Swiggy Instamart", Description: "groceries", Amount: 250})

	rows, _, err := ts.SearchTransactions(ctx, "insta", store.ListFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("SearchTransactions: %v", err)
	}
	if len(rows) != 1 || rows[0].MerchantInfo != "Swiggy Instamart" {
		t.Fatalf("unexpected search rows: %+v", rows)
	}
}

func TestSearchTransactions_WebStyleQuery(t *testing.T) {
	ts := newTestStore(t)
	ctx := context.Background()
	insertTestTransaction(t, ts, store.Transaction{MerchantInfo: "Amazon Pay", Description: "prime membership", Amount: 1499})

	rows, _, err := ts.SearchTransactions(ctx, "amazon prime", store.ListFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("SearchTransactions: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one row, got %+v", rows)
	}
}
```

- [ ] **Step 2: Run backend tests**

Run: `task test:be`

Expected: FAIL because substring search does not match `plainto_tsquery` behavior.

- [ ] **Step 3: Add trigram support**

Create `backend/migrations/013_search_trigram.sql`:

```sql
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS transactions_merchant_trgm_idx
    ON transactions USING GIN (merchant_info gin_trgm_ops);

CREATE INDEX IF NOT EXISTS transactions_description_trgm_idx
    ON transactions USING GIN (description gin_trgm_ops);
```

- [ ] **Step 4: Update search condition**

Replace the current `plainto_tsquery`-only condition with a hybrid condition:

```go
func buildSearchCondition(query string, args *[]any) string {
	term := strings.TrimSpace(query)
	tsArg := nextArg(args, term)
	likeArg := nextArg(args, "%"+term+"%")
	return fmt.Sprintf(`(
		(to_tsvector('english', t.merchant_info) || to_tsvector('english', COALESCE(t.description,''))) @@ websearch_to_tsquery('english', $%d)
		OR t.merchant_info ILIKE $%d
		OR COALESCE(t.description, '') ILIKE $%d
	)`, tsArg, likeArg, likeArg)
}
```

Use the repo’s existing argument helper pattern; do not concatenate user input into SQL.

- [ ] **Step 5: Run backend tests**

Run: `task test:be`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/migrations backend/internal/store/store.go backend/internal/store/store_test.go
git commit --no-gpg-sign -m "feat: improve transaction search"
```

---

### Task 6: Slice Verification

**Files:**
- Modify only if verification exposes drift.

- [ ] **Step 1: Verify Spend Breakdown order remains fixed**

Run: `task test:fe`

Expected: PASS, including the existing Dashboard test that asserts `Categories`, `Buckets`, `Labels`.

- [ ] **Step 2: Format and lint**

Run: `task fmt`

Expected: PASS.

Run: `task lint`

Expected: PASS.

- [ ] **Step 3: Run focused suites**

Run: `task test:fe`

Expected: PASS.

Run: `task test:be`

Expected: PASS.

- [ ] **Step 4: Commit final verification fixes if needed**

```bash
git add frontend backend
git commit --no-gpg-sign -m "test: verify ux polish slice"
```

Only create this commit if verification changed files.
