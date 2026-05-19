# Feedback Slice 1 Reliability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent missed data by fixing checkpoint behavior around Gmail failures and first-run rescans, then remove the two visible UI breakages from the feedback note.

**Architecture:** Gmail scans should return an iteration result so checkpoints are saved only after a complete successful pass. Rule creation should clear the active reader checkpoint once so a new rule gets an initial lookback scan. UI fixes stay narrow: stabilize `BreakdownTimeline` hook order and improve portal tooltip viewport alignment.

**Tech Stack:** Go, Gmail API client, slog, app_config-backed checkpoint storage, React, Vitest, React Testing Library, Task.

---

## File Structure

- Modify: `backend/pkg/reader/gmail/gmail.go`
  Purpose: return scan errors from rule evaluation, classify auth/network failures, retry auth failures once, and only checkpoint successful iterations.
- Modify: `backend/pkg/reader/gmail/gmail_test.go`
  Purpose: unit tests for `effectiveSince`, auth-error classification, log level, and checkpoint gating helpers.
- Modify: `backend/internal/api/handlers.go`
  Purpose: after successful rule creation, clear the active reader checkpoint and restart the daemon if needed.
- Modify: `backend/internal/api/handlers_test.go`
  Purpose: handler tests for checkpoint clearing on rule creation.
- Modify: `frontend/src/pages/Dashboard.tsx`
  Purpose: make `BreakdownTimeline` hook order stable and reorder spend breakdown toggles to `Categories`, `Buckets`, `Labels`.
- Create: `frontend/src/pages/Dashboard.test.tsx`
  Purpose: regression test for rerendering spend breakdown from populated to empty data.
- Modify: `frontend/src/hooks/useTooltip.tsx`
  Purpose: align below-tooltips to the viewport edge when the trigger is near the left or right edge.
- Create: `frontend/src/hooks/useTooltip.test.tsx`
  Purpose: regression test for right-edge tooltip alignment.
- Modify: `frontend/src/pages/Transactions.test.tsx`
  Purpose: regression test that the mute tooltip renders through the shared portal and does not use row-local absolute positioning.

## Task 1: Gmail Checkpoint Gating

**Files:**
- Modify: `backend/pkg/reader/gmail/gmail.go`
- Modify: `backend/pkg/reader/gmail/gmail_test.go`

- [ ] **Step 1: Write failing unit tests for full-scan checkpoint inputs**

Add these tests to `backend/pkg/reader/gmail/gmail_test.go`:

```go
func TestEffectiveSince_ForceFullScanIgnoresLastScanAt(t *testing.T) {
	now := time.Now()
	lastScan := now
	reader := &Reader{
		lastScanAt:    &lastScan,
		forceFullScan: true,
		lookbackDays:  14,
	}

	got := reader.effectiveSince()
	wantEarliest := time.Now().AddDate(0, 0, -14).Add(-2 * time.Second)
	wantLatest := time.Now().AddDate(0, 0, -14).Add(2 * time.Second)
	if got.Before(wantEarliest) || got.After(wantLatest) {
		t.Fatalf("effectiveSince() with forceFullScan = %v, want near 14-day lookback", got)
	}
}

func TestEffectiveSince_NormalScanUsesCheckpointBuffer(t *testing.T) {
	lastScan := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	reader := &Reader{
		lastScanAt:    &lastScan,
		forceFullScan: false,
		lookbackDays:  14,
	}

	got := reader.effectiveSince()
	want := lastScan.Add(-time.Hour)
	if !got.Equal(want) {
		t.Fatalf("effectiveSince() = %v, want %v", got, want)
	}
}
```

- [ ] **Step 2: Run tests to verify baseline behavior**

Run:

```bash
task test:be
```

Expected: these tests should pass or expose the current force-full-scan regression if present. If they pass, keep them as coverage for the feedback item.

- [ ] **Step 3: Write failing checkpoint-gating tests**

Add a helper-oriented test that proves a failed iteration does not call `OnCheckpoint`:

```go
func TestSaveCheckpointAfterSuccessfulIterationOnly(t *testing.T) {
	t.Run("successful iteration saves checkpoint", func(t *testing.T) {
		var saved bool
		reader := &Reader{
			onCheckpoint: func(time.Time) { saved = true },
		}

		reader.saveCheckpointAfterIteration(nil)

		if !saved {
			t.Fatal("expected checkpoint to be saved after successful iteration")
		}
	})

	t.Run("failed iteration leaves checkpoint untouched", func(t *testing.T) {
		var saved bool
		reader := &Reader{
			onCheckpoint: func(time.Time) { saved = true },
		}

		reader.saveCheckpointAfterIteration(errors.New("list messages: network unavailable"))

		if saved {
			t.Fatal("checkpoint was saved after a failed iteration")
		}
	})
}
```

Add imports for `errors` and `time`.

- [ ] **Step 4: Run test to verify it fails**

Run:

```bash
task test:be
```

Expected: FAIL with `reader.saveCheckpointAfterIteration undefined`.

- [ ] **Step 5: Implement checkpoint gating**

In `backend/pkg/reader/gmail/gmail.go`, change `Read` to use a helper:

```go
iterationErr := r.evaluateRules(ctx, out)
r.saveCheckpointAfterIteration(iterationErr)
```

Apply the same pattern inside the ticker case.

Add this helper near `saveCheckpoint`:

```go
func (r *Reader) saveCheckpointAfterIteration(iterationErr error) {
	if iterationErr != nil {
		r.logger.Warn("scan checkpoint not saved after incomplete scan", "error", iterationErr)
		return
	}
	r.saveCheckpoint()
}
```

Change `evaluateRules` to return `error`:

```go
func (r *Reader) evaluateRules(ctx context.Context, out chan<- *api.TransactionDetails) error {
	r.logger.Info("starting rule evaluation", "rule_count", len(r.rules))

	sem := make(chan struct{}, maxConcurrentRules)
	errs := make(chan error, len(r.rules))
	var wg sync.WaitGroup
	for _, rule := range r.rules {
		wg.Add(1)
		go func(rule api.Rule) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := r.processRule(ctx, rule, out); err != nil {
				errs <- err
			}
		}(rule)
	}
	wg.Wait()
	close(errs)

	var joined error
	for err := range errs {
		joined = errors.Join(joined, err)
	}
	if joined != nil {
		r.logger.Warn("rule evaluation incomplete", "error", joined)
		return joined
	}

	r.logger.Info("rule evaluation complete")
	return nil
}
```

Change `processRule` to return `error`. On list-message errors, call `logAPIError` and return `fmt.Errorf("listing messages for rule %q: %w", rule.Name, err)`. Accumulate `processMessage` errors with `errors.Join` while continuing through other messages. Return the accumulated error after logging rule completion.

- [ ] **Step 6: Run tests**

Run:

```bash
task test:be
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/pkg/reader/gmail/gmail.go backend/pkg/reader/gmail/gmail_test.go
git commit --no-gpg-sign -m "fix: gate gmail checkpoints on scan success"
```

## Task 2: Gmail Auth Error Retry And Logging

**Files:**
- Modify: `backend/pkg/reader/gmail/gmail.go`
- Modify: `backend/pkg/reader/gmail/gmail_test.go`

- [ ] **Step 1: Write failing tests for OAuth error classification**

Add these tests to `backend/pkg/reader/gmail/gmail_test.go`:

```go
func TestIsAuthErrorDetectsInvalidGrant(t *testing.T) {
	err := errors.New(`oauth2: "invalid_grant" "Token has been expired or revoked."`)
	if !isAuthError(err) {
		t.Fatal("expected invalid_grant token error to be classified as auth error")
	}
}

func TestLogAPIError_InvalidGrantUsesErrorLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	logAPIError(logger, "failed to list messages", errors.New(`oauth2: "invalid_grant" "Token has been expired or revoked."`))

	out := buf.String()
	if !strings.Contains(out, "level=ERROR") {
		t.Fatalf("expected ERROR log level, got %s", out)
	}
	if !strings.Contains(out, "OAuth token invalid") {
		t.Fatalf("expected OAuth invalid guidance, got %s", out)
	}
}

func TestDoWithAuthRetryRetriesAuthErrorsOnce(t *testing.T) {
	attempts := 0
	errExpired := errors.New(`oauth2: "invalid_grant" "Token has been expired or revoked."`)
	err := doWithAuthRetry(func() error {
		attempts++
		if attempts == 1 {
			return errExpired
		}
		return nil
	})
	if err != nil {
		t.Fatalf("doWithAuthRetry returned error: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}
```

Add imports for `bytes`, `log/slog`, `strings`, and `errors` if they are not already present.

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
task test:be
```

Expected: FAIL with missing `isAuthError` and `doWithAuthRetry`, and/or WARN-level invalid_grant logging.

- [ ] **Step 3: Implement auth classification and retry helper**

In `backend/pkg/reader/gmail/gmail.go`, add:

```go
func doWithAuthRetry(call func() error) error {
	err := call()
	if err == nil || !isAuthError(err) {
		return err
	}
	if retryErr := call(); retryErr != nil {
		return retryErr
	}
	return nil
}

func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) && apiErr.Code == http.StatusUnauthorized {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "invalid_grant") ||
		strings.Contains(s, "Token has been expired or revoked") ||
		strings.Contains(s, "token expired") ||
		strings.Contains(s, "refresh token")
}
```

Update `logAPIError` so it checks network errors first, then `isAuthError(err)` and logs at `Error` level with OAuth reauthorization guidance.

- [ ] **Step 4: Use auth retry around Gmail list and get calls**

In `processRule`, replace direct list execution with a factory so retries create a fresh request:

```go
var resp *gmail.ListMessagesResponse
err := doWithAuthRetry(func() error {
	req := r.client.Users.Messages.List("me").Q(query).Context(ctx)
	if pageToken != "" {
		req = req.PageToken(pageToken)
	}
	var callErr error
	resp, callErr = req.Do()
	return callErr
})
```

In `processMessage`, wrap the get call similarly:

```go
var msg *gmail.Message
err := doWithAuthRetry(func() error {
	var callErr error
	msg, callErr = r.client.Users.Messages.Get("me", msgID).Context(ctx).Do()
	return callErr
})
if err != nil {
	return fmt.Errorf("getting message: %w", err)
}
```

- [ ] **Step 5: Run tests**

Run:

```bash
task test:be
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/pkg/reader/gmail/gmail.go backend/pkg/reader/gmail/gmail_test.go
git commit --no-gpg-sign -m "fix: retry gmail auth failures once"
```

## Task 3: New Rule Clears Active Reader Checkpoint

**Files:**
- Modify: `backend/internal/api/handlers.go`
- Modify: `backend/internal/api/handlers_test.go`

- [ ] **Step 1: Write failing handler test**

Add this test near existing rule handler tests in `backend/internal/api/handlers_test.go`:

```go
func TestHandleCreateRule_ClearsActiveReaderCheckpoint(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "active_reader"), []byte("gmail"), 0o600); err != nil {
		t.Fatalf("write active reader: %v", err)
	}
	ms := &mockStore{
		ruleResult: &store.RuleRow{
			ID:            "rule-1",
			Name:          "New Rule",
			AmountRegex:   `Rs\.([\d.]+)`,
			MerchantRegex: `at (.*?) on`,
		},
		appConfig: map[string]string{"reader.gmail.last_scan_at": "2026-05-19T00:00:00Z"},
	}
	dm := &mockDaemon{}
	h := newTestHandlers(t, ms, dm)
	h.dataDir = dir
	restarted := ""
	h.restartFn = func(reader string) { restarted = reader }

	body := `{"name":"New Rule","amountRegex":"Rs\\.([\\d.]+)","merchantInfoRegex":"at (.*?) on"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/rules", strings.NewReader(body))
	rr := httptest.NewRecorder()

	h.HandleCreateRule(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if got := ms.appConfig["reader.gmail.last_scan_at"]; got != "" {
		t.Fatalf("reader checkpoint = %q, want empty", got)
	}
	if restarted != "" {
		t.Fatalf("restartFn called while daemon stopped: %q", restarted)
	}
}
```

Add `path/filepath` to the test imports if needed.

- [ ] **Step 2: Write failing running-daemon restart test**

Add:

```go
func TestHandleCreateRule_RestartsRunningDaemonAfterCheckpointClear(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "active_reader"), []byte("gmail"), 0o600); err != nil {
		t.Fatalf("write active reader: %v", err)
	}
	ms := &mockStore{
		ruleResult: &store.RuleRow{
			ID:            "rule-1",
			Name:          "New Rule",
			AmountRegex:   `Rs\.([\d.]+)`,
			MerchantRegex: `at (.*?) on`,
		},
		appConfig: map[string]string{"reader.gmail.last_scan_at": "2026-05-19T00:00:00Z"},
	}
	dm := &mockDaemon{running: true}
	h := newTestHandlers(t, ms, dm)
	h.dataDir = dir
	restarted := ""
	h.restartFn = func(reader string) { restarted = reader }

	body := `{"name":"New Rule","amountRegex":"Rs\\.([\\d.]+)","merchantInfoRegex":"at (.*?) on"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/rules", strings.NewReader(body))
	rr := httptest.NewRecorder()

	h.HandleCreateRule(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if restarted != "gmail" {
		t.Fatalf("restartFn reader = %q, want gmail", restarted)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run:

```bash
task test:be
```

Expected: FAIL because `HandleCreateRule` does not clear checkpoints yet.

- [ ] **Step 4: Implement helper and call it after successful rule creation**

In `backend/internal/api/handlers.go`, after `CreateRule` succeeds and before `writeJSON`, call:

```go
h.clearActiveReaderCheckpointForNewRule(r.Context())
```

Add this helper near the rule handlers:

```go
func (h *Handlers) clearActiveReaderCheckpointForNewRule(ctx context.Context) {
	reader, err := h.readActiveReader()
	if err != nil || strings.TrimSpace(reader) == "" || h.store == nil {
		return
	}
	reader = strings.TrimSpace(reader)
	key := "reader." + reader + ".last_scan_at"
	if err := h.store.SetAppConfig(ctx, key, ""); err != nil {
		h.logger.Warn("failed to clear checkpoint after rule creation", "reader", reader, "error", err)
		return
	}
	if h.daemon.Status().Running && h.restartFn != nil {
		h.restartFn(reader)
	}
}

func (h *Handlers) readActiveReader() (string, error) {
	b, err := os.ReadFile(filepath.Join(h.dataDir, "active_reader"))
	if err != nil {
		return "", err
	}
	return string(b), nil
}
```

Then update `HandleGetActiveReader` to use `h.readActiveReader()` to avoid duplicating file reads.

- [ ] **Step 5: Run tests**

Run:

```bash
task test:be
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/api/handlers.go backend/internal/api/handlers_test.go
git commit --no-gpg-sign -m "fix: rescan lookback after creating rule"
```

## Task 4: Dashboard Spend Breakdown Crash And Toggle Order

**Files:**
- Modify: `frontend/src/pages/Dashboard.tsx`
- Create: `frontend/src/pages/Dashboard.test.tsx`

- [ ] **Step 1: Export `BreakdownTimeline` for component testing**

In `frontend/src/pages/Dashboard.tsx`, change:

```tsx
function BreakdownTimeline({
```

to:

```tsx
export function BreakdownTimeline({
```

- [ ] **Step 2: Write failing rerender test**

Create `frontend/src/pages/Dashboard.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { BreakdownTimeline } from './Dashboard'

describe('BreakdownTimeline', () => {
  it('rerenders from populated data to empty data without changing hook order', () => {
    const populated = {
      labels: ['Groceries'],
      months: ['2026-03', '2026-04'],
      series: [{ label: 'Groceries', data: [10, 20] }],
    }
    const empty = {
      labels: [],
      months: ['2026-03', '2026-04'],
      series: [],
    }

    const { rerender } = render(
      <BreakdownTimeline
        data={populated}
        currency="INR"
        mode="labels"
        onModeChange={() => undefined}
      />,
    )

    rerender(
      <BreakdownTimeline
        data={empty}
        currency="INR"
        mode="labels"
        onModeChange={() => undefined}
      />,
    )

    expect(screen.getByText('No data')).toBeInTheDocument()
  })

  it('orders breakdown toggles as Categories, Buckets, Labels', () => {
    render(
      <BreakdownTimeline
        data={{ labels: [], months: [], series: [] }}
        currency="INR"
        mode="categories"
        onModeChange={() => undefined}
      />,
    )

    const buttons = screen.getAllByRole('button').map((button) => button.textContent)
    expect(buttons).toEqual(['Categories', 'Buckets', 'Labels'])
  })
})
```

- [ ] **Step 3: Run tests to verify they fail**

Run:

```bash
task test:fe
```

Expected: FAIL with the React hook-order error and/or old toggle order.

- [ ] **Step 4: Fix hook order and toggle order**

In `Dashboard.tsx`, set:

```tsx
const SPEND_BREAKDOWN_OPTIONS: { key: SpendBreakdownMode; label: string }[] = [
  { key: 'categories', label: 'Categories' },
  { key: 'buckets', label: 'Buckets' },
  { key: 'labels', label: 'Labels' },
]
```

Move this effect above the `if (visibleSeries.length === 0)` early return:

```tsx
useEffect(() => {
  if (normalizedSelectedLabels.length !== selectedLabels.length) {
    setSelectedLabels(normalizedSelectedLabels)
  }
}, [normalizedSelectedLabels, selectedLabels])
```

Leave the effect body unchanged so it still clears selections that are no longer visible.

- [ ] **Step 5: Run tests**

Run:

```bash
task test:fe
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/pages/Dashboard.tsx frontend/src/pages/Dashboard.test.tsx
git commit --no-gpg-sign -m "fix: stabilize spend breakdown rendering"
```

## Task 5: Mute Tooltip Viewport Alignment

**Files:**
- Modify: `frontend/src/hooks/useTooltip.tsx`
- Create: `frontend/src/hooks/useTooltip.test.tsx`
- Modify: `frontend/src/pages/Transactions.test.tsx`

- [ ] **Step 1: Write failing tooltip hook test**

Create `frontend/src/hooks/useTooltip.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { useTooltip } from './useTooltip'

function TooltipHarness() {
  const { handlers, tip } = useTooltip()
  return (
    <>
      <button type="button" {...handlers('Mute - exclude from totals and charts')}>
        trigger
      </button>
      {tip}
    </>
  )
}

describe('useTooltip', () => {
  it('aligns below tooltip to the right edge when trigger is near viewport edge', async () => {
    const user = userEvent.setup()
    render(<TooltipHarness />)
    const trigger = screen.getByRole('button', { name: 'trigger' })
    vi.spyOn(trigger, 'getBoundingClientRect').mockReturnValue({
      x: 1180,
      y: 120,
      left: 1180,
      right: 1200,
      top: 120,
      bottom: 140,
      width: 20,
      height: 20,
      toJSON: () => ({}),
    } as DOMRect)
    vi.stubGlobal('innerWidth', 1210)

    await user.hover(trigger)

    const tooltip = screen.getByText('Mute - exclude from totals and charts')
    expect(tooltip.className).toContain('-translate-x-full')
    expect(tooltip).toHaveStyle({ left: '1200px' })
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
task test:fe
```

Expected: FAIL because below-tooltips always use centered translation today.

- [ ] **Step 3: Implement viewport-aware tooltip placement**

In `frontend/src/hooks/useTooltip.tsx`, change state to include an x-alignment:

```tsx
type XAlign = 'start' | 'center' | 'end'

const [state, setState] = useState<{
  label: string
  x: number
  y: number
  xAlign: XAlign
} | null>(null)
```

Update `handlers`:

```tsx
const handlers = (label: string) => ({
  onMouseEnter: (e: React.MouseEvent<Element>) => {
    const r = e.currentTarget.getBoundingClientRect()
    if (placement === 'right') {
      setState({ label, x: r.right + 8, y: r.top + r.height / 2, xAlign: 'start' })
      return
    }

    const edgePadding = 160
    if (window.innerWidth - r.right < edgePadding) {
      setState({ label, x: r.right, y: r.bottom + 6, xAlign: 'end' })
      return
    }
    if (r.left < edgePadding) {
      setState({ label, x: r.left, y: r.bottom + 6, xAlign: 'start' })
      return
    }
    setState({ label, x: r.left + r.width / 2, y: r.bottom + 6, xAlign: 'center' })
  },
  onMouseLeave: () => setState(null),
})
```

Replace the transform class with:

```tsx
placement === 'right'
  ? '-translate-y-1/2'
  : state.xAlign === 'center'
    ? '-translate-x-1/2'
    : state.xAlign === 'end'
      ? '-translate-x-full'
      : ''
```

Also change the tooltip class from `whitespace-nowrap` to:

```tsx
'pointer-events-none fixed z-50 max-w-[calc(100vw-1rem)] whitespace-normal rounded bg-foreground px-2 py-1 text-xs text-background shadow'
```

- [ ] **Step 4: Add transaction-page regression test**

In `frontend/src/pages/Transactions.test.tsx`, add:

```tsx
it('renders mute tooltip through the document portal', async () => {
  const user = userEvent.setup()
  renderTransactions('/transactions')

  const muteButtons = await screen.findAllByRole('button', {
    name: /mute|unmute/i,
  })
  await user.hover(muteButtons[0])

  expect(screen.getByText(/exclude from totals/i)).toBeInTheDocument()
})
```

If the icon-only mute button has no accessible name, add `aria-label={label}` to the `MuteButton` button in `frontend/src/pages/Transactions.tsx`, then assert with `getAllByRole('button', { name: /mute/i })`.

- [ ] **Step 5: Run tests**

Run:

```bash
task test:fe
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/hooks/useTooltip.tsx frontend/src/hooks/useTooltip.test.tsx frontend/src/pages/Transactions.tsx frontend/src/pages/Transactions.test.tsx
git commit --no-gpg-sign -m "fix: keep mute tooltip in viewport"
```

## Task 6: Final Verification

**Files:**
- No production files unless verification exposes failures.

- [ ] **Step 1: Run backend tests**

Run:

```bash
task test:be
```

Expected: PASS.

- [ ] **Step 2: Run frontend tests**

Run:

```bash
task test:fe
```

Expected: PASS.

- [ ] **Step 3: Run lint for touched stacks**

Run:

```bash
task lint:be:new
task lint:fe
```

Expected: both pass with no issues.

- [ ] **Step 4: Run broader relevant tests**

Run:

```bash
task test:be
task test:fe
```

Expected: PASS.

- [ ] **Step 5: Final diff review**

Run:

```bash
git diff --stat HEAD
git diff -- backend/pkg/reader/gmail/gmail.go backend/internal/api/handlers.go frontend/src/pages/Dashboard.tsx frontend/src/hooks/useTooltip.tsx frontend/src/pages/Transactions.tsx
```

Expected: diff is limited to Slice 1 reliability and UI fixes.

## Self-Review

- Spec coverage: covers Slice 1 items for checkpoint failures, OAuth retry/log level, force full scan behavior, new rule first lookback scan, Spend Breakdown crash, tooltip clipping, and toggle order.
- Scope boundary: excludes extraction diagnostics, date picker redesign, search quality, DB-backed runtime state, a11y, i18n, and repository-pattern work.
- Placeholder scan: no `TBD`, `TODO`, or unspecified implementation steps remain.
- Type consistency: uses existing `Reader`, `Handlers`, `mockStore`, `mockDaemon`, `BreakdownTimeline`, and `useTooltip` names.
