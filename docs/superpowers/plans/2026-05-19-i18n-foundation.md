# i18n Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Add a lightweight internationalization foundation without translating the whole app yet.

**Status:** Complete. Implemented and merged to `main`; English source messages, provider, formatting helpers, converted shared UI surfaces, and extraction guidance are in place.

**Architecture:** Introduce a typed message catalog, provider, and formatting helpers. Migrate shared shell and one dense page first so future work can move strings incrementally without blocking product work.

**Tech Stack:** React context/hooks, TypeScript message keys, Intl.DateTimeFormat, Intl.NumberFormat, Vitest.

---

## Scope

In scope:

- Locale provider and `useI18n()` hook.
- English catalog as the source locale.
- Currency/date/time formatting helpers.
- Convert app shell, command palette, sidebar, settings shell, and a small Transactions toolbar slice.

Out of scope:

- Adding non-English translations.
- Runtime translation download.
- Pluralization library unless existing tests prove the simple helper is insufficient.

---

### Task 1: Add I18n Core

**Files:**
- Create: `frontend/src/i18n/messages.ts`
- Create: `frontend/src/i18n/I18nProvider.tsx`
- Create: `frontend/src/i18n/format.ts`
- Create: `frontend/src/i18n/I18nProvider.test.tsx`
- Modify: `frontend/src/App.tsx`

- [x] **Step 1: Write failing tests**

```tsx
it('returns English messages by key', () => {
  render(
    <I18nProvider locale="en">
      <Probe />
    </I18nProvider>,
  )

  expect(screen.getByText('Dashboard')).toBeInTheDocument()
})

function Probe() {
  const { t } = useI18n()
  return <span>{t('nav.dashboard')}</span>
}
```

- [x] **Step 2: Run tests**

Run: `task test:fe`

Expected: FAIL because i18n core does not exist.

- [x] **Step 3: Implement catalog and provider**

Create `messages.ts`:

```ts
export const messages = {
  en: {
    'nav.dashboard': 'Dashboard',
    'nav.transactions': 'Transactions',
    'nav.rules': 'Rules',
    'nav.settings': 'Settings',
    'command.search': 'Search destinations',
  },
} as const

export type Locale = keyof typeof messages
export type MessageKey = keyof typeof messages.en
```

Create provider:

```tsx
const I18nContext = createContext<{ locale: Locale; t: (key: MessageKey) => string } | null>(null)

export function I18nProvider({ locale = 'en', children }: { locale?: Locale; children: React.ReactNode }) {
  const value = useMemo(() => ({ locale, t: (key: MessageKey) => messages[locale][key] ?? key }), [locale])
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}

export function useI18n() {
  const ctx = useContext(I18nContext)
  if (!ctx) throw new Error('useI18n must be used within I18nProvider')
  return ctx
}
```

Wrap `App` inside `I18nProvider`.

- [x] **Step 4: Add formatting helpers**

Create `format.ts`:

```ts
export function formatCurrencyForLocale(amount: number, currency: string, locale: string) {
  return new Intl.NumberFormat(locale, { style: 'currency', currency }).format(amount)
}

export function formatDateForLocale(value: Date, locale: string) {
  return new Intl.DateTimeFormat(locale, { dateStyle: 'medium' }).format(value)
}
```

- [x] **Step 5: Run tests and commit**

Run: `task test:fe`

Expected: PASS.

```bash
git add frontend/src/i18n frontend/src/App.tsx
git commit --no-gpg-sign -m "feat: add i18n foundation"
```

---

### Task 2: Migrate Navigation And Command Palette Strings

**Files:**
- Modify: `frontend/src/lib/navigation.ts`
- Modify: `frontend/src/components/Sidebar.tsx`
- Modify: `frontend/src/components/CommandPalette.tsx`
- Modify: `frontend/src/i18n/messages.ts`
- Test: existing component tests

- [x] **Step 1: Add failing tests**

Assert navigation labels still render through message keys:

```tsx
expect(screen.getByRole('link', { name: 'Transactions' })).toBeInTheDocument()
expect(screen.getByRole('textbox', { name: 'Search destinations' })).toBeInTheDocument()
```

- [x] **Step 2: Run tests**

Run: `task test:fe`

Expected: FAIL once navigation targets move from literal labels to keys.

- [x] **Step 3: Add keyed navigation model**

Update navigation targets:

```ts
export interface NavigationTarget {
  id: string
  titleKey: MessageKey
  subtitleKey?: MessageKey
  descriptionKey: MessageKey
  path: string
  keywords?: string[]
}
```

Resolve labels inside components with `t(target.titleKey)`.

- [x] **Step 4: Add catalog entries**

Add keys for all current navigation labels, subtitles, and descriptions. Keep English text identical to current UI unless Slice 3 already changed command palette descriptions.

- [x] **Step 5: Run tests and commit**

Run: `task test:fe`

Expected: PASS.

```bash
git add frontend/src/lib/navigation.ts frontend/src/components/Sidebar.tsx frontend/src/components/CommandPalette.tsx frontend/src/i18n/messages.ts
git commit --no-gpg-sign -m "feat: localize navigation shell"
```

---

### Task 3: Migrate Shared Formatting Boundaries

**Files:**
- Modify: `frontend/src/lib/utils.ts`
- Modify: `frontend/src/pages/Transactions.tsx`
- Modify: `frontend/src/pages/Dashboard.tsx`
- Test: page tests

- [x] **Step 1: Write tests for locale-aware formatting**

```ts
expect(formatCurrencyForLocale(1234.5, 'INR', 'en-IN')).toContain('1,234.50')
expect(formatDateForLocale(new Date('2026-05-19T00:00:00Z'), 'en-US')).toContain('Apr')
```

- [x] **Step 2: Run tests**

Run: `task test:fe`

Expected: FAIL if formatting is still scattered or helpers not imported.

- [x] **Step 3: Replace local formatting**

Use `const { locale } = useI18n()` in pages and call shared helpers instead of hardcoded locale strings such as `toLocaleString('en-IN')` where the value is user-facing.

- [x] **Step 4: Run tests and commit**

Run: `task test:fe`

Expected: PASS.

```bash
git add frontend/src/i18n frontend/src/lib frontend/src/pages/Transactions.tsx frontend/src/pages/Dashboard.tsx
git commit --no-gpg-sign -m "feat: centralize locale formatting"
```

---

### Task 4: Add Extraction Guardrail

**Files:**
- Create: `docs/i18n/string-extraction.md`
- Modify: no production code unless tests expose missing catalog entries.

- [x] **Step 1: Document string rules**

Create:

```markdown
# i18n String Rules

- Shared navigation, command, settings tab, and repeated control labels use `MessageKey`.
- Page-specific one-off copy may remain inline until the page is touched for feature work.
- Dates, times, currency, and counts use helpers from `frontend/src/i18n/format.ts`.
- Do not concatenate translated fragments around dynamic values. Add a complete message helper instead.
```

- [x] **Step 2: Add grep check note**

Add:

```markdown
Review command:

rg -n "toLocaleString\\('en|toLocaleDateString\\('en|Intl\\." frontend/src
```

- [x] **Step 3: Commit**

```bash
git add docs/i18n/string-extraction.md
git commit --no-gpg-sign -m "docs: add i18n extraction rules"
```

---

### Task 5: Final Verification

- [x] **Step 1: Run tests**

Run: `task test:fe`

Expected: PASS.

Run: `task lint:fe`

Expected: PASS.

- [x] **Step 2: Verify no unintended string churn**

Run:

```bash
git diff -- frontend/src | rg -n "Dashboard|Transactions|Settings|Rules|Search"
```

Expected: only intended navigation/shell/page formatting changes appear.

- [x] **Step 3: Commit fixes if needed**

```bash
git add frontend docs
git commit --no-gpg-sign -m "test: verify i18n foundation"
```

Only create this commit if verification changed files.
