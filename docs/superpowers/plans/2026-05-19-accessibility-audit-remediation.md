# Accessibility Audit And Remediation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Audit and remediate high-impact accessibility issues across the main Expensor UI.

**Status:** Complete. Implemented and merged to `main`; the audit harness, route checks, semantic/focus fixes, and keyboard follow-ups are in place.

**Architecture:** Add automated accessibility checks first, inventory failures by route/component, then remediate shared controls before page-specific issues. Keep visual design intact while making keyboard, focus, semantic, and label behavior reliable.

**Tech Stack:** React, Vitest, Testing Library, Playwright, axe-core or @axe-core/playwright, Tailwind focus-visible styles.

---

## Scope

Routes/components in scope:

- App shell: sidebar, command palette, daemon status.
- Dashboard charts and heatmaps.
- Transactions table, filters, bulk actions.
- Rules and rule editor.
- Diagnostics page after Slice 2.
- Settings tabs and custom combobox/select controls.
- Setup wizard.

Out of scope:

- Full WCAG certification.
- Screen-reader copy rewrites for every chart datum.
- Color palette redesign beyond high-impact contrast and focus fixes.

---

### Task 1: Add Automated Accessibility Harness

**Files:**
- Modify: `frontend/package.json`
- Modify: `frontend/playwright` config or create `frontend/playwright/accessibility.spec.ts`
- Create: `frontend/src/test/a11y.ts`

- [x] **Step 1: Add failing Playwright accessibility smoke**

Create `frontend/playwright/accessibility.spec.ts`:

```ts
import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

const routes = ['/', '/transactions', '/rules', '/settings', '/setup']

for (const route of routes) {
  test(`has no critical accessibility violations on ${route}`, async ({ page }) => {
    await page.goto(route)
    const results = await new AxeBuilder({ page })
      .disableRules(['color-contrast'])
      .analyze()

    const critical = results.violations.filter((v) => v.impact === 'critical' || v.impact === 'serious')
    expect(critical).toEqual([])
  })
}
```

- [x] **Step 2: Run to verify missing dependency/failures**

Run: `task test:fe:e2e`

Expected: FAIL because `@axe-core/playwright` is not installed or violations are present.

- [x] **Step 3: Install and wire axe dependency**

Run the repo’s package manager in `frontend/` to add `@axe-core/playwright` as a dev dependency. Update lockfile. Do not add global scripts unless the existing Playwright target cannot discover the new spec.

- [x] **Step 4: Add component helper**

Create `frontend/src/test/a11y.ts`:

```ts
import { axe } from 'vitest-axe'

export async function expectNoA11yViolations(container: HTMLElement) {
  const results = await axe(container)
  expect(results).toHaveNoViolations()
}
```

If `vitest-axe` is not installed, add it as a dev dependency and extend `frontend/src/test/setup.ts`:

```ts
import 'vitest-axe/extend-expect'
```

- [x] **Step 5: Commit harness**

```bash
git add frontend/package.json frontend/package-lock.json frontend/playwright/accessibility.spec.ts frontend/src/test/a11y.ts frontend/src/test/setup.ts
git commit --no-gpg-sign -m "test: add accessibility audit harness"
```

---

### Task 2: Inventory Accessibility Findings

**Files:**
- Create: `docs/accessibility/audit-2026-05-19.md`

- [x] **Step 1: Run route scan**

Run: `task test:fe:e2e`

Expected: FAIL if violations exist. Capture route, rule ID, impact, affected selector, and short remediation note.

- [x] **Step 2: Create audit document**

Create `docs/accessibility/audit-2026-05-19.md`:

```markdown
# Accessibility Audit 2026-05-19

## Scope

- Dashboard
- Transactions
- Rules
- Settings
- Setup

## Findings

| Priority | Route/component | Rule | Issue | Fix task |
|---|---|---|---|---|
| P1 | Command palette | dialog-name | Palette needs dialog semantics and label | Task 3 |
| P1 | Transactions filters | label | Filter inputs need accessible labels | Task 5 |

## Deferred

- Color contrast details that require palette redesign.
```

Add one row for every serious or critical finding from the scan. When the scan has no serious or critical issues, use this Findings table body:

```markdown
| Priority | Route/component | Rule | Issue | Fix task |
|---|---|---|---|---|
| None | All scoped routes | none | No serious or critical automated findings | Final verification |
```

- [x] **Step 3: Commit audit**

```bash
git add docs/accessibility/audit-2026-05-19.md
git commit --no-gpg-sign -m "docs: record accessibility audit"
```

---

### Task 3: Fix Shared Modal And Floating UI Semantics

**Files:**
- Modify: `frontend/src/components/CommandPalette.tsx`
- Modify: `frontend/src/components/ConfirmModal.tsx`
- Modify: `frontend/src/components/InlineSelect.tsx`
- Modify: `frontend/src/components/FilterCombobox.tsx`
- Test: related component tests

- [x] **Step 1: Write failing tests**

Add tests asserting dialog/listbox semantics:

```tsx
it('renders command palette as a named modal dialog', () => {
  render(<CommandPalette open targets={[]} onClose={vi.fn()} onNavigate={vi.fn()} />)

  expect(screen.getByRole('dialog', { name: 'Command palette' })).toHaveAttribute('aria-modal', 'true')
  expect(screen.getByRole('textbox', { name: 'Search destinations' })).toBeInTheDocument()
})
```

- [x] **Step 2: Run tests**

Run: `task test:fe`

Expected: FAIL where components lack roles/labels.

- [x] **Step 3: Implement semantics**

For modal surfaces:

```tsx
<div role="dialog" aria-modal="true" aria-label="Command palette">
```

For search inputs:

```tsx
aria-label="Search destinations"
```

For custom listbox controls, ensure the trigger has `role="combobox"`, `aria-expanded`, `aria-controls`, and the list has stable `id` with `role="listbox"` and option rows with `role="option"`.

- [x] **Step 4: Run tests**

Run: `task test:fe`

Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add frontend/src/components/CommandPalette.tsx frontend/src/components/ConfirmModal.tsx frontend/src/components/InlineSelect.tsx frontend/src/components/FilterCombobox.tsx frontend/src/components/*.test.tsx
git commit --no-gpg-sign -m "fix: improve shared accessibility semantics"
```

---

### Task 4: Fix Keyboard And Focus Behavior

**Files:**
- Modify: `frontend/src/components/CommandPalette.tsx`
- Modify: `frontend/src/components/DateRangePicker.tsx`
- Modify: `frontend/src/components/LabelCombobox.tsx`
- Modify: `frontend/src/components/FilterCombobox.tsx`
- Test: related component tests

- [x] **Step 1: Write keyboard tests**

Add tests for Escape, Tab, ArrowDown/ArrowUp, and Enter where applicable. For example:

```tsx
it('navigates command palette options by keyboard', async () => {
  const user = userEvent.setup()
  const onNavigate = vi.fn()
  render(<CommandPalette open targets={targets} onClose={vi.fn()} onNavigate={onNavigate} />)

  await user.keyboard('{ArrowDown}{Enter}')

  expect(onNavigate).toHaveBeenCalledWith(targets[1].path)
})
```

- [x] **Step 2: Run tests**

Run: `task test:fe`

Expected: FAIL for missing keyboard paths.

- [x] **Step 3: Implement focus behavior**

Implement:

- Escape closes each floating UI.
- Arrow keys move active option.
- Enter selects active option.
- Opening a modal/floating UI focuses the first useful input.
- Closing returns focus to the trigger when practical.
- Disabled buttons that need hover/tooltips are wrapped in non-disabled elements.

- [x] **Step 4: Run tests**

Run: `task test:fe`

Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add frontend/src/components frontend/src/pages
git commit --no-gpg-sign -m "fix: improve keyboard accessibility"
```

---

### Task 5: Fix Page-Level Labels And Table Semantics

**Files:**
- Modify: `frontend/src/pages/Transactions.tsx`
- Modify: `frontend/src/pages/Rules.tsx`
- Modify: `frontend/src/pages/Diagnostics.tsx`
- Modify: `frontend/src/pages/Settings.tsx`
- Test: related page tests

- [x] **Step 1: Add failing page tests**

Add tests for named checkboxes, tables, and filter inputs:

```tsx
expect(screen.getByRole('searchbox', { name: 'Search transactions' })).toBeInTheDocument()
expect(screen.getByRole('table', { name: 'Transactions' })).toBeInTheDocument()
expect(screen.getAllByRole('checkbox', { name: /select transaction/i }).length).toBeGreaterThan(0)
```

- [x] **Step 2: Run tests**

Run: `task test:fe`

Expected: FAIL for any missing accessible names.

- [x] **Step 3: Implement labels**

Apply concrete fixes:

- Add `role="searchbox"` or preserve textbox with `aria-label`.
- Add `aria-label` to data tables.
- Add row-specific checkbox labels.
- Add `scope="col"` to table headers.
- Add `aria-live="polite"` for toolbar count/selection changes where not disruptive.

- [x] **Step 4: Run tests**

Run: `task test:fe`

Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add frontend/src/pages frontend/src/pages/*.test.tsx
git commit --no-gpg-sign -m "fix: label primary page controls"
```

---

### Task 6: Final Accessibility Verification

**Files:**
- Modify: `docs/accessibility/audit-2026-05-19.md`

- [x] **Step 1: Re-run automated audits**

Run: `task test:fe`

Expected: PASS.

Run: `task test:fe:e2e`

Expected: PASS for axe critical/serious checks.

- [x] **Step 2: Update audit doc**

Mark fixed findings as resolved:

```markdown
| P1 | Command palette | dialog-name | Resolved in <commit> |
```

- [x] **Step 3: Commit**

```bash
git add docs/accessibility/audit-2026-05-19.md frontend
git commit --no-gpg-sign -m "test: verify accessibility remediation"
```

Only create this commit if files changed during verification.
