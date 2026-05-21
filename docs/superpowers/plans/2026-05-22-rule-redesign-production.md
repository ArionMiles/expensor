# Rule Redesign Production Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the approved rules redesign in production: versioned rule JSON, exact multi-sender matching, source type/bank support, self-contained rule fixtures, revamped Rules list/editor, diagnostic fix routing, transaction filters, and dashboard source donuts.

**Architecture:** Introduce `api.Source` as the shared domain value and keep API/storage JSON flat enough for the current handlers while exposing structured source fields to the UI. Parse embedded and imported rules through a versioned rule document adapter, persist normalized sender lists and source columns, and keep extraction tests fixture-driven so each email fixture is self-contained and automatically discovered. Replace the temporary prototype with production React components that reuse the app's dark design system, URL state persistence, and custom combobox/dropdown patterns.

**Tech Stack:** Go 1.26, pgx/PostgreSQL migrations, YAML test fixtures via `gopkg.in/yaml.v3`, React/Vite/TypeScript/Tailwind, TanStack Query, Vitest, Playwright.

---

## File Map

**Backend domain and parsing**
- Modify `backend/pkg/api/api.go`: add `Source`, update `Rule`, `TransactionDetails`, diagnostics, Gmail query, and exact sender matching.
- Create `backend/pkg/rules/document.go`: parse v2 rule documents and legacy array documents into `api.Rule`.
- Modify `backend/pkg/rules/rules_test.go`: cover v2 parse, legacy parse, exact sender matching, and Gmail query OR sender behavior.
- Modify `backend/cmd/server/main.go`: load embedded v2 rules through `rules.ParseDocument`.
- Modify `content/rules.json` and `backend/cmd/server/content/rules.json`: convert to v2 versioned object.

**Backend fixtures**
- Create `tests/data/rule-emails/hdfc_credit-card_classic-spend.yaml`: first fixture using the final readable format.
- Create `backend/pkg/rules/fixtures.go`: discover and load fixture files from `tests/data/rule-emails`.
- Create `backend/pkg/rules/fixtures_test.go`: table-driven runner that uses fixture basename as subtest name and runs against actual rules loaded from `content/rules.json`.
- Update `.github/CONTRIBUTING.md` and `AGENTS.md`: document fixture convention and naming pattern `<bank>_<source-type>_<case>.yaml`.

**Backend storage/API**
- Modify `backend/migrations/001_init.sql`: fresh install schema for `transactions.source_type`, `transactions.source_label`, `transactions.bank`, `rules.sender_emails`, `rules.source_type`, `rules.source_label`, `rules.bank`.
- Create `backend/migrations/002_source_struct_and_rule_senders.sql`: idempotent upgrade path from existing `source` and `sender_email` fields.
- Modify `backend/internal/store/store.go`: update `Transaction`, `RuleRow`, `ListFilter`, `ChartData`, `Facets`.
- Modify `backend/internal/store/rules_repository.go`: read/write `sender_emails`, source fields, and seed presets.
- Modify `backend/internal/store/transactions_repository.go`: list/get/filter on source type and bank.
- Modify `backend/internal/store/testing.go`: seed source type/bank fields.
- Modify `backend/pkg/writer/postgres/postgres.go` and `backend/pkg/writer/postgres/001_create_transactions.sql`: write structured source values.
- Modify `backend/internal/api/handlers.go`, `backend/internal/api/store.go`, `backend/internal/api/handlers_test.go`: API payloads, validation, import/export, facets, transactions query params.
- Modify `api/openapi/expensor.openapi.yaml` or regenerate with `task openapi:generate` after handler annotations are updated.

**Readers/extraction/diagnostics**
- Modify `backend/pkg/reader/gmail/gmail.go`: evaluate each sender exactly, set transaction source struct, and record source in diagnostics.
- Modify `backend/pkg/reader/gmail/gmail_test.go`: query and source propagation tests.
- Modify `backend/pkg/reader/thunderbird/thunderbird.go`: exact parsed sender matching and source propagation.
- Modify `backend/pkg/reader/thunderbird/thunderbird_test.go`: exact sender matching tests.
- Modify `backend/pkg/extractor/extractor.go` only if source needs to be injected at extraction time; otherwise keep extraction source-neutral.

**Frontend API and mocks**
- Modify `frontend/src/api/types.ts`: add `Source`, rule sender/source fields, facets, dashboard charts.
- Modify `frontend/src/api/client.ts`, `frontend/src/api/queries.ts`: update create/update/import rule types.
- Modify `frontend/src/test/handlers/index.ts`, `frontend/src/mocks/handlers.ts`, `frontend/src/mocks/fixtures/transactions.ts`, and `frontend/src/test/fixtures/transactions.ts`: align mocked API data.

**Frontend Rules**
- Modify `frontend/src/pages/Rules.tsx`: replace old table with approved list view, URL-backed filters, column order Bank, Name, Subject, Senders, Type, Origin.
- Modify `frontend/src/pages/Rules.test.tsx`: list filters, column order, create/edit links, custom chevrons.
- Replace `frontend/src/pages/rules/RuleForm.tsx`: production workbench matching prototype, including editable title, sender chips, custom type/bank comboboxes, sample tabs, live result, and diagnostic prefill.
- Modify `frontend/src/pages/rules/RuleForm.test.tsx`: title validation/revert, sender Enter behavior, type/bank add behavior, sample add, live result failures, diagnostic prefill.
- Modify `frontend/src/pages/Diagnostics.tsx` and `frontend/src/pages/Diagnostics.test.tsx`: "Fix rule" routes to the workbench with diagnostic context.
- Modify `frontend/src/App.tsx`: keep `/rules`, `/rules/new`, and `/rules/:id`; add query support only where route code needs it.

**Frontend Transactions/Dashboard**
- Modify `frontend/src/pages/Transactions.tsx` and `frontend/src/pages/Transactions.test.tsx`: replace source column/filter with Type and Bank columns/filters.
- Modify `frontend/src/pages/Dashboard.tsx` and `frontend/src/pages/Dashboard.test.tsx`: replace source donut with two donuts, by type and by bank, with slice clicks targeting transaction filters.
- Modify `frontend/src/lib/utils.ts` if color helpers need structured source colors.

---

## Task 1: Domain Model and Rule Document Parser

**Files:**
- Modify: `backend/pkg/api/api.go`
- Create: `backend/pkg/rules/document.go`
- Modify: `backend/pkg/rules/rules_test.go`

- [x] **Step 1: Write failing domain/parser tests**

Add tests that assert exact sender behavior and v2 parsing:

```go
func TestParseDocumentV2(t *testing.T) {
	body := []byte(`{
	  "version": 2,
	  "presets": {"source_types": ["Credit Card"], "banks": ["HDFC"]},
	  "rules": [{
	    "name": "HDFC Credit Card",
	    "sender_emails": ["alerts@hdfcbank.net", "alerts@hdfcbank.bank.in"],
	    "subject_contains": "HDFC Bank Credit Card",
	    "amount_regex": "Rs\\.\\s*([\\d,]+(?:\\.\\d+)?)",
	    "merchant_regex": "\\bat\\b (.*?) on",
	    "currency_regex": "",
	    "source": {"type": "Credit Card", "label": "HDFC Credit Card", "bank": "HDFC"}
	  }]
	}`)
	doc, err := ParseDocument(body)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if doc.Version != 2 {
		t.Fatalf("version = %d", doc.Version)
	}
	rule := doc.Rules[0]
	if got := rule.SenderEmails; !reflect.DeepEqual(got, []string{"alerts@hdfcbank.net", "alerts@hdfcbank.bank.in"}) {
		t.Fatalf("sender emails = %#v", got)
	}
	if rule.Source.Type != "Credit Card" || rule.Source.Bank != "HDFC" || rule.Source.Label != "HDFC Credit Card" {
		t.Fatalf("source = %#v", rule.Source)
	}
}

func TestRuleMatchesEmailExactSenderAddress(t *testing.T) {
	rule := api.Rule{SenderEmails: []string{"alerts@hdfcbank.net"}, SubjectContains: "statement"}
	if !rule.MatchesEmail("HDFC <alerts@hdfcbank.net>", "Monthly statement") {
		t.Fatal("expected exact parsed address match")
	}
	if rule.MatchesEmail("alerts@hdfcbank.net.evil.example", "Monthly statement") {
		t.Fatal("expected substring sender mismatch")
	}
}
```

- [x] **Step 2: Run tests to confirm failure**

Run: `task test:be -- ./pkg/rules ./pkg/api`

Expected: fails because `ParseDocument`, `Source`, and `SenderEmails` do not exist.

- [x] **Step 3: Implement domain and parser**

Add:

```go
type Source struct {
	Type  string `json:"type"`
	Label string `json:"label"`
	Bank  string `json:"bank"`
}

func (s Source) Display() string {
	parts := []string{}
	if strings.TrimSpace(s.Bank) != "" {
		parts = append(parts, strings.TrimSpace(s.Bank))
	}
	if strings.TrimSpace(s.Type) != "" {
		parts = append(parts, strings.TrimSpace(s.Type))
	}
	if strings.TrimSpace(s.Label) != "" && strings.TrimSpace(s.Label) != strings.Join(parts, " ") {
		parts = append(parts, strings.TrimSpace(s.Label))
	}
	return strings.Join(parts, " ")
}
```

Update `api.Rule` to use `SenderEmails []string` and `Source Source`. Keep `SenderEmail string` only as a compatibility helper during the same task if needed by untouched call sites, then remove it when call sites are migrated.

Create `rules.ParseDocument` that accepts:

```go
type Document struct {
	Version int
	Presets Presets
	Rules []api.Rule
}

type Presets struct {
	SourceTypes []PresetValue
	Banks []PresetValue
}

type PresetValue struct {
	Value string `json:"value"`
	Origin string `json:"origin"`
}
```

The parser must normalize lower/trim sender emails, compile regexes, reject blank names, reject rules with no senders, and support legacy array input by mapping `senderEmail` into one sender and splitting legacy `source` into best-effort type/bank/label.

- [x] **Step 4: Run tests to confirm pass**

Run: `task test:be -- ./pkg/rules ./pkg/api`

Expected: PASS.

- [x] **Step 5: Commit**

Run:

```bash
git add backend/pkg/api/api.go backend/pkg/rules/document.go backend/pkg/rules/rules_test.go
git commit --no-gpg-sign -m "feat: parse versioned rule documents"
```

## Task 2: Rule Fixtures

**Files:**
- Create: `tests/data/rule-emails/hdfc_credit-card_classic-spend.yaml`
- Create: `backend/pkg/rules/fixtures.go`
- Create: `backend/pkg/rules/fixtures_test.go`
- Modify: `backend/go.mod`, `backend/go.sum`

- [x] **Step 1: Write the fixture and failing fixture runner test**

Fixture shape:

```yaml
rule: HDFC Credit Card
sender: alerts@hdfcbank.net
subject: "Alert : Update on your HDFC Bank Credit Card"
body: |
  Dear Customer,
  Rs.999.00 spent at SWIGGY on your HDFC Credit Card on 12-Apr-2026.
expected:
  amount: 999.00
  merchant: SWIGGY
  currency: INR
```

Test behavior:

```go
func TestRuleEmailFixtures(t *testing.T) {
	doc := loadRulesDocument(t, "../../../content/rules.json")
	fixtures := LoadFixtures(t, "../../../tests/data/rule-emails")
	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.TestName, func(t *testing.T) {
			rule := findRule(t, doc.Rules, fixture.Rule)
			if !rule.MatchesEmail(fixture.Sender, fixture.Subject) {
				t.Fatalf("fixture did not match rule sender/subject")
			}
			tx := extractor.ExtractTransactionDetails(fixture.Body, rule.Amount, rule.MerchantInfo, rule.Currency, fixedFixtureTime)
			if tx.Amount != fixture.Expected.Amount {
				t.Fatalf("amount = %v, want %v", tx.Amount, fixture.Expected.Amount)
			}
			if tx.MerchantInfo != fixture.Expected.Merchant {
				t.Fatalf("merchant = %q, want %q", tx.MerchantInfo, fixture.Expected.Merchant)
			}
			if tx.Currency != fixture.Expected.Currency {
				t.Fatalf("currency = %q, want %q", tx.Currency, fixture.Expected.Currency)
			}
		})
	}
}
```

- [x] **Step 2: Run tests to confirm failure**

Run: `task test:be -- ./pkg/rules`

Expected: fails because fixture loader and v2 content are absent.

- [x] **Step 3: Implement fixture loader**

Use `filepath.WalkDir`, load only `.yaml`/`.yml`, set `TestName` to `strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))`, and validate the basename with:

```go
var fixtureNamePattern = regexp.MustCompile(`^[a-z0-9-]+_[a-z0-9-]+_[a-z0-9-]+$`)
```

Reject fixtures that define regexes or timestamps by not supporting those fields in the struct.

- [x] **Step 4: Run tests to confirm pass**

Run: `task test:be -- ./pkg/rules`

Expected: PASS.

- [x] **Step 5: Commit**

Run:

```bash
git add backend/go.mod backend/go.sum backend/pkg/rules/fixtures.go backend/pkg/rules/fixtures_test.go tests/data/rule-emails
git commit --no-gpg-sign -m "test: add self-contained rule fixtures"
```

## Task 3: Storage Migration and Repository Shape

**Files:**
- Modify: `backend/migrations/001_init.sql`
- Create: `backend/migrations/002_source_struct_and_rule_senders.sql`
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/store/rules_repository.go`
- Modify: `backend/internal/store/transactions_repository.go`
- Modify: `backend/internal/store/testing.go`
- Modify: `backend/internal/store/store_test.go`

- [x] **Step 1: Write failing store tests**

Add tests that create a rule with two sender emails and source fields, then assert `ListRules` returns them. Add transaction fixture rows with `source_type` and `bank`, then assert facets include `source_types` and `banks`, and filters `source_type=Credit Card&bank=HDFC` return only matching rows.

- [x] **Step 2: Run tests to confirm failure**

Run: `task test:be -- ./internal/store -run 'Test.*Rule.*Sender|Test.*Source.*Facet|Test.*Source.*Filter'`

Expected: fails on missing columns/fields.

- [x] **Step 3: Implement migration and repository changes**

The upgrade migration must:

```sql
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS source_type TEXT NOT NULL DEFAULT '';
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS source_label TEXT NOT NULL DEFAULT '';
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS bank TEXT NOT NULL DEFAULT '';
UPDATE transactions
SET source_label = COALESCE(NULLIF(source_label, ''), source)
WHERE COALESCE(source_label, '') = '';

ALTER TABLE rules ADD COLUMN IF NOT EXISTS sender_emails TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE rules ADD COLUMN IF NOT EXISTS source_type TEXT NOT NULL DEFAULT '';
ALTER TABLE rules ADD COLUMN IF NOT EXISTS source_label TEXT NOT NULL DEFAULT '';
ALTER TABLE rules ADD COLUMN IF NOT EXISTS bank TEXT NOT NULL DEFAULT '';
UPDATE rules SET sender_emails = ARRAY[sender_email] WHERE cardinality(sender_emails) = 0 AND sender_email <> '';
UPDATE rules SET source_label = transaction_source WHERE source_label = '' AND transaction_source <> '';
```

Keep legacy `source`, `sender_email`, and `transaction_source` columns populated during this task so older code paths do not break while the rest of the branch is migrated.

- [x] **Step 4: Run store tests**

Run: `task test:be -- ./internal/store`

Expected: PASS.

- [x] **Step 5: Commit**

Run:

```bash
git add backend/migrations backend/internal/store
git commit --no-gpg-sign -m "feat: store structured rule sources"
```

## Task 4: API Payloads, Import, Export, and OpenAPI

**Files:**
- Modify: `backend/internal/api/handlers.go`
- Modify: `backend/internal/api/store.go`
- Modify: `backend/internal/api/handlers_test.go`
- Modify: `api/openapi/expensor.openapi.yaml`

- [x] **Step 1: Write failing handler tests**

Cover `POST /api/rules` with `sender_emails` and `source`, `GET /api/rules` returning `source`, import/export using the v2 versioned object, and `GET /api/transactions` query parsing for `source_type`, `bank`, `exclude_source_types`, and `exclude_banks`.

- [x] **Step 2: Run tests to confirm failure**

Run: `task test:be -- ./internal/api -run 'TestHandle.*Rule|TestHandleListTransactions_QueryCSVFilters'`

Expected: fails on old JSON fields and missing query filters.

- [x] **Step 3: Implement handlers**

Use request/response structs with JSON names:

```go
type ruleRequest struct {
	Name string `json:"name"`
	SenderEmails []string `json:"sender_emails"`
	SubjectContains string `json:"subject_contains"`
	AmountRegex string `json:"amount_regex"`
	MerchantRegex string `json:"merchant_regex"`
	CurrencyRegex string `json:"currency_regex"`
	Source api.Source `json:"source"`
}
```

Validate nonblank rule name, at least one exact sender email, amount regex, merchant regex, source type, and bank. Import/export must use the same versioned document format as `content/rules.json`.

- [x] **Step 4: Regenerate/check OpenAPI**

Run: `task openapi:generate` then `task openapi:check`.

Expected: generated spec is current and check passes.

- [x] **Step 5: Commit**

Run:

```bash
git add backend/internal/api api/openapi/expensor.openapi.yaml
git commit --no-gpg-sign -m "feat: expose structured rules api"
```

## Task 5: Readers, Writer, and Diagnostics

**Files:**
- Modify: `backend/pkg/reader/gmail/gmail.go`
- Modify: `backend/pkg/reader/gmail/gmail_test.go`
- Modify: `backend/pkg/reader/thunderbird/thunderbird.go`
- Modify: `backend/pkg/reader/thunderbird/thunderbird_test.go`
- Modify: `backend/pkg/writer/postgres/postgres.go`
- Modify: `backend/pkg/writer/postgres/001_create_transactions.sql`
- Modify: `backend/pkg/writer/postgres/postgres_test.go`

- [x] **Step 1: Write failing tests**

Gmail tests must assert one rule with two senders produces sender-specific queries and exact match semantics. Thunderbird tests must assert `alerts@hdfcbank.net.evil.example` does not match `alerts@hdfcbank.net`. Writer tests must assert inserted rows populate `source_type`, `source_label`, and `bank`.

- [x] **Step 2: Run tests to confirm failure**

Run: `task test:be -- ./pkg/reader/gmail ./pkg/reader/thunderbird ./pkg/writer/postgres`

Expected: fails on source fields and sender semantics.

- [x] **Step 3: Implement runtime propagation**

Set `transaction.Source = rule.Source`. Use `mail.ParseAddress` for sender extraction where a From header may include display names. Gmail query construction should produce one query per exact sender for a multi-sender rule, each retaining the subject filter.

- [x] **Step 4: Run tests**

Run: `task test:be -- ./pkg/reader/gmail ./pkg/reader/thunderbird ./pkg/writer/postgres`

Expected: PASS.

- [x] **Step 5: Commit**

Run:

```bash
git add backend/pkg/reader backend/pkg/writer
git commit --no-gpg-sign -m "feat: match exact rule senders"
```

## Task 6: Production Frontend Types and Mocks

**Files:**
- Modify: `frontend/src/api/types.ts`
- Modify: `frontend/src/api/queries.ts`
- Modify: `frontend/src/api/client.ts`
- Modify: `frontend/src/test/handlers/index.ts`
- Modify: `frontend/src/mocks/handlers.ts`
- Modify: `frontend/src/mocks/fixtures/transactions.ts`
- Modify: `frontend/src/test/fixtures/transactions.ts`

- [x] **Step 1: Write failing frontend type/mock tests**

Update existing tests that consume mocked rules and transactions so TypeScript expects:

```ts
interface Source {
  type: string
  label: string
  bank: string
}
interface Rule {
  sender_emails: string[]
  source: Source
  predefined: boolean
}
```

- [x] **Step 2: Run tests to confirm failure**

Run: `task lint:fe`

Expected: TypeScript fails on old `sender_email`, `transaction_source`, and `source: string` usage.

- [x] **Step 3: Update types and mocks**

Keep mock data realistic: include HDFC Credit Card, ICICI UPI, and custom Mobile App examples with `predefined` and `custom` origins.

- [x] **Step 4: Run frontend typecheck**

Run: `task lint:fe`

Expected: PASS.

- [x] **Step 5: Commit**

Run:

```bash
git add frontend/src/api frontend/src/mocks frontend/src/test
git commit --no-gpg-sign -m "feat: update frontend source types"
```

## Task 7: Rules List View

**Files:**
- Modify: `frontend/src/pages/Rules.tsx`
- Modify: `frontend/src/pages/Rules.test.tsx`

- [x] **Step 1: Write failing component tests**

Assert URL-backed filters for type, bank, and origin; assert column order text appears as Bank, Name, Subject, Senders, Type, Origin; assert selecting a filter updates `?type=Credit%20Card`; assert no native `<select>` appears.

- [x] **Step 2: Run tests to confirm failure**

Run: `task test:fe -- Rules.test.tsx`

Expected: fails on old table and filters.

- [x] **Step 3: Implement list view**

Port the approved `prototypes/rule-workbench/list.html` structure to React/Tailwind. Use custom buttons/popovers, CSS chevrons, and `useSearchParams` for filters. Origin values are `all`, `predefined`, `custom`.

- [x] **Step 4: Run tests**

Run: `task test:fe -- Rules.test.tsx`

Expected: PASS.

- [x] **Step 5: Commit**

Run:

```bash
git add frontend/src/pages/Rules.tsx frontend/src/pages/Rules.test.tsx
git commit --no-gpg-sign -m "feat: revamp rules list"
```

## Task 8: Rule Workbench Editor and Diagnostic Fix Routing

**Files:**
- Modify: `frontend/src/pages/rules/RuleForm.tsx`
- Modify: `frontend/src/pages/rules/RuleForm.test.tsx`
- Modify: `frontend/src/pages/Diagnostics.tsx`
- Modify: `frontend/src/pages/Diagnostics.test.tsx`

- [x] **Step 1: Write failing component tests**

Cover editable rule title, blank title blur reverting to last saved value, Enter-to-add sender, no Add Sender button, type/bank combobox add behavior only when no matches exist, `+ Add sample`, red live-result failures, and diagnostic "Fix rule" opening `/rules/new?diagnostic=<id>`.

- [x] **Step 2: Run tests to confirm failure**

Run: `task test:fe -- RuleForm.test.tsx Diagnostics.test.tsx`

Expected: fails on old single-column editor and old diagnostic routing.

- [x] **Step 3: Implement workbench**

Port the approved prototype behavior, but use production APIs and no native `title` attributes. The type/bank floating menu must use `position: fixed` plus `createPortal` if it can be clipped by a scrollable pane. Use `data-1p-ignore`, `data-lpignore`, `data-form-type="other"`, and readonly-until-focus on combobox text inputs to avoid password-manager overlays.

- [x] **Step 4: Run tests**

Run: `task test:fe -- RuleForm.test.tsx Diagnostics.test.tsx`

Expected: PASS.

- [x] **Step 5: Commit**

Run:

```bash
git add frontend/src/pages/rules/RuleForm.tsx frontend/src/pages/rules/RuleForm.test.tsx frontend/src/pages/Diagnostics.tsx frontend/src/pages/Diagnostics.test.tsx
git commit --no-gpg-sign -m "feat: add rule workbench"
```

## Task 9: Transactions and Dashboard Source Analytics

**Files:**
- Modify: `frontend/src/pages/Transactions.tsx`
- Modify: `frontend/src/pages/Transactions.test.tsx`
- Modify: `frontend/src/pages/Dashboard.tsx`
- Modify: `frontend/src/pages/Dashboard.test.tsx`
- Modify: `frontend/src/lib/utils.ts`

- [x] **Step 1: Write failing UI tests**

Transactions tests must assert Type and Bank columns/filters replace Source. Dashboard tests must assert two donuts render: By type and By bank, and clicking a slice navigates with `source_type` or `bank`.

- [x] **Step 2: Run tests to confirm failure**

Run: `task test:fe -- Transactions.test.tsx Dashboard.test.tsx`

Expected: fails on old source column and single source donut.

- [x] **Step 3: Implement transactions/dashboard updates**

Use API fields `tx.source.type`, `tx.source.bank`, chart fields `by_source_type`, and `by_bank`. Update filter query keys to `source_type`, `bank`, `exclude_source_types`, and `exclude_banks`.

- [x] **Step 4: Run tests**

Run: `task test:fe -- Transactions.test.tsx Dashboard.test.tsx`

Expected: PASS.

- [x] **Step 5: Commit**

Run:

```bash
git add frontend/src/pages/Transactions.tsx frontend/src/pages/Transactions.test.tsx frontend/src/pages/Dashboard.tsx frontend/src/pages/Dashboard.test.tsx frontend/src/lib/utils.ts
git commit --no-gpg-sign -m "feat: split source analytics"
```

## Task 10: Docs, Formatting, and Full Verification

**Files:**
- Modify: `.github/CONTRIBUTING.md`
- Modify: `AGENTS.md`
- Modify: `frontend/README.md` if new combobox guidance is worth documenting.

- [x] **Step 1: Update docs**

Document:
- Rule fixture directory: `tests/data/rule-emails`.
- Fixture naming: `<bank>_<source-type>_<case>.yaml`, lowercase kebab sections.
- One email per file, positive assertions only.
- Fixtures do not include regexes, rule runner loads regexes from `content/rules.json`.
- Rule JSON v2 object shape and source fields.

- [x] **Step 2: Format**

Run: `task fmt`

Expected: no unformatted changes after rerun.

- [ ] **Step 3: Backend verification**

Run: `task test:be`, `task lint:be:prod`, and `task openapi:check`.

Expected: PASS / 0 issues.

- [ ] **Step 4: Frontend verification**

Run: `task test:fe`, `task lint:fe`, and `task test:fe:e2e`.

Expected: PASS.

- [ ] **Step 5: Inspect final diff and commit docs/cleanup**

Run:

```bash
git status --short
git diff --stat
git add .github/CONTRIBUTING.md AGENTS.md frontend/README.md
git commit --no-gpg-sign -m "docs: document rule fixtures"
```

---

## Self-Review

**Spec coverage:** This plan covers v2 rules, exact multi-sender matching, source type/bank/label, transaction filters, dashboard two donuts, fixture format and naming, diagnostics workbench routing, Rules list revamp, and contributor/agent documentation.

**Placeholder scan:** The plan avoids deferred placeholders; each task has concrete files, expected behavior, commands, and commit points.

**Type consistency:** The canonical names are `api.Source{Type, Label, Bank}`, JSON `source: {type,label,bank}`, `sender_emails`, filter keys `source_type` and `bank`, chart keys `by_source_type` and `by_bank`, and fixture key `rule`.
