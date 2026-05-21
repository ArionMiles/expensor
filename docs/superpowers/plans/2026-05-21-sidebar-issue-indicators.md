# Sidebar Issue Indicators Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Surface Gmail authorization and open diagnostics issues directly in the sidebar, including collapsed sidebar state.

**Architecture:** Keep the current sidebar navigation maps and icons. Add small alert metadata derived from existing React Query hooks, and render lightweight badges inside each `NavLink` with collapsed-mode overflow allowed on nav links that have badges.

**Tech Stack:** React, TypeScript, TanStack Query, Vitest, Testing Library, Tailwind CSS, lucide-react.

---

## File Structure

- Modify `frontend/src/components/Sidebar.tsx`: consume `useReaderStatus('gmail')` and `useExtractionDiagnostics('open')`, derive indicator state, render subtle setup dot and diagnostics count badge, and adjust collapsed tooltip labels.
- Modify `frontend/src/components/Sidebar.test.tsx`: mock API query hooks and assert expanded/collapsed indicators.

### Task 1: Sidebar Indicator Tests

**Files:**
- Modify: `frontend/src/components/Sidebar.test.tsx`

- [ ] **Step 1: Write failing tests**

Add mocks for `useReaderStatus` and `useExtractionDiagnostics`, then add tests:

```tsx
vi.mock('@/api/queries', () => ({
  useReaderStatus: vi.fn(),
  useExtractionDiagnostics: vi.fn(),
}))

const mockUseReaderStatus = vi.mocked(useReaderStatus)
const mockUseExtractionDiagnostics = vi.mocked(useExtractionDiagnostics)

beforeEach(() => {
  mockUseReaderStatus.mockReturnValue({
    data: { credentials_uploaded: true, authenticated: false, config_present: true, auth_type: 'oauth', ready: false },
    isSuccess: true,
  } as ReturnType<typeof useReaderStatus>)
  mockUseExtractionDiagnostics.mockReturnValue({
    data: [{ id: 'diag-1' }, { id: 'diag-2' }, { id: 'diag-3' }],
  } as ReturnType<typeof useExtractionDiagnostics>)
})
```

Assert:

```tsx
expect(screen.getByTestId('setup-attention-dot')).toBeInTheDocument()
expect(screen.getByText('3')).toHaveAttribute('aria-label', '3 open diagnostics')
expect(screen.getByTestId('diagnostics-count-badge')).toHaveClass('items-center')
expect(screen.getByTestId('diagnostics-count-badge')).toHaveClass('justify-center')
expect(screen.getByTestId('nav-link-diagnostics')).toHaveClass('overflow-visible')
```

- [ ] **Step 2: Run test to verify it fails**

Run: `task test:fe -- Sidebar.test.tsx`

Expected: FAIL because the sidebar does not call the query hooks and does not render the new test IDs.

### Task 2: Sidebar Indicator Rendering

**Files:**
- Modify: `frontend/src/components/Sidebar.tsx`

- [ ] **Step 1: Implement minimal indicator state**

Import the hooks:

```tsx
import { useExtractionDiagnostics, useReaderStatus } from '@/api/queries'
```

Inside `Sidebar`, derive:

```tsx
const { data: gmailStatus, isSuccess: gmailStatusLoaded } = useReaderStatus('gmail')
const { data: openDiagnostics } = useExtractionDiagnostics('open')
const setupNeedsAttention =
  gmailStatusLoaded && gmailStatus?.auth_type === 'oauth' && !gmailStatus.ready
const openDiagnosticsCount = openDiagnostics?.length ?? 0
```

- [ ] **Step 2: Render badges without changing icons**

Use per-item metadata while mapping `SECONDARY_NAV`:

```tsx
const isSetup = item.href === '/setup'
const isDiagnostics = item.href === '/diagnostics'
const showSetupDot = isSetup && setupNeedsAttention
const diagnosticsCount = isDiagnostics ? openDiagnosticsCount : 0
```

Add `relative` and `overflow-visible` to nav links with collapsed badges. Keep `<item.icon />` unchanged. Render the setup dot with `data-testid="setup-attention-dot"`. Render the diagnostics badge with `data-testid="diagnostics-count-badge"`, `aria-label={`${diagnosticsCount} open diagnostics`}`, and centering classes `inline-flex items-center justify-center`.

- [ ] **Step 3: Run test to verify it passes**

Run: `task test:fe -- Sidebar.test.tsx`

Expected: PASS.

### Task 3: Broader Verification

**Files:**
- No code files

- [ ] **Step 1: Run frontend tests**

Run: `task test:fe`

Expected: PASS.

- [ ] **Step 2: Run frontend lint/typecheck**

Run: `task lint:fe`

Expected: PASS.
