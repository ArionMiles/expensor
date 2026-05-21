# Rule Redesign Design

## Goal

Redesign extraction rules, rule tests, and the rule authoring UI so rules are easier to maintain, test, and analyze. The redesign should remove sender substring matching, split transaction source into source type and bank, make rule fixtures self-contained, and provide a more useful workbench-style rule editor before the production implementation begins.

Backward compatibility is not a primary constraint. Existing source strings should still be migrated when they can be split confidently.

## Decisions

- Use a clean v2 rules schema instead of extending the current flat rule array.
- Use exact normalized sender email matching. Wildcards, domain matching, and substring matching are out of scope for v2.
- Allow multiple sender email addresses per rule.
- Represent source as a first-class object with a stable type key, display label, and bank.
- Store source fields as flattened database columns for queryability.
- Replace transaction and dashboard source views with source type and bank views.
- Use one positive rule fixture per email file.
- Keep rule fixture regexes out of test files; fixtures reference the real rule from bundled rules JSON.
- Build a temporary live Rule creator prototype first. Production implementation follows after prototype approval and must include tests.

## Rules V2 Format

`rules.json` should become a versioned object:

```json
{
  "version": 2,
  "rules": [
    {
      "name": "HDFC Credit Card",
      "match": {
        "senders": ["alerts@hdfcbank.bank.in", "alerts@hdfcbank.net"],
        "subjectContains": "Alert : Update on your HDFC Bank Credit Card"
      },
      "source": {
        "type": "credit_card",
        "label": "Credit Card",
        "bank": "HDFC"
      },
      "extract": {
        "amount": "Rs\\.\\s*([\\d,]+(?:\\.\\d+)?)",
        "merchant": "\\bat\\b (.*?) on",
        "currency": ""
      }
    }
  ]
}
```

Field semantics:

- `version`: required; v2 is the new canonical format.
- `rules`: ordered list of rules.
- `match.senders`: exact email addresses after normalization. The runtime should parse the email address from `From` headers and compare normalized email addresses, not the full header string.
- `match.subjectContains`: case-insensitive subject substring match.
- `source.type`: stable machine key, such as `credit_card`, `debit_card`, `upi`, `netbanking`, `mobile_app`, `wallet`, or `other`.
- `source.label`: display label for `source.type`, such as `Credit Card`.
- `source.bank`: display bank/provider name, such as `HDFC`.
- `extract.amount`: Go regexp used to extract amount.
- `extract.merchant`: Go regexp used to extract merchant. The first non-empty capture group remains the extracted value.
- `extract.currency`: optional Go regexp used to extract ISO currency.

Rules import/export should use this v2 object only for the first implementation. A v1 import adapter is out of scope.

## Domain And API Model

The Go domain model should use a first-class source struct:

```go
type Source struct {
	Type  string `json:"type"`
	Label string `json:"label"`
	Bank  string `json:"bank"`
}
```

`api.Rule` should use:

```go
type Rule struct {
	ID              string
	Name            string
	SenderEmails    []string
	SubjectContains string
	Amount          *regexp.Regexp
	MerchantInfo    *regexp.Regexp
	Currency        *regexp.Regexp
	Source          Source
}
```

`api.TransactionDetails` should also use `Source Source`.

External JSON should expose nested source objects for row-level data:

```json
"source": {
  "type": "credit_card",
  "label": "Credit Card",
  "bank": "HDFC"
}
```

This applies to rule API payloads, transaction API payloads, diagnostics payloads, and rule import/export.

Aggregated APIs should split source dimensions:

- Facets: `source_types`, `banks`.
- Charts: `by_source_type`, `by_bank`.
- Dashboard: separate type and bank breakdowns.

## Database And Migration

Database tables should store source fields as flattened columns:

- `source_type`
- `source_label`
- `bank`

Rules should replace `sender_email` with `sender_emails TEXT[] NOT NULL DEFAULT '{}'`. Senders are part of one rule's match definition and do not require their own lifecycle.

Rules should replace `transaction_source` with `source_type`, `source_label`, and `bank`.

Transactions should replace `source` with `source_type`, `source_label`, and `bank`.

The migration should backfill new columns from legacy columns, then drop the replaced legacy columns in the same migration.

Diagnostics should retain the source object fields and existing regex snapshots so failed extraction remains actionable.

Migration should split known legacy source strings:

| Legacy source | Type | Label | Bank |
| --- | --- | --- | --- |
| `Credit Card - HDFC` | `credit_card` | `Credit Card` | `HDFC` |
| `Credit Card - ICICI` | `credit_card` | `Credit Card` | `ICICI` |
| `Credit Card - Axis` | `credit_card` | `Credit Card` | `Axis` |
| `Debit Card - ICICI` | `debit_card` | `Debit Card` | `ICICI` |
| `UPI - HDFC` | `upi` | `UPI` | `HDFC` |
| `iMobile - ICICI` | `mobile_app` | `Mobile App` | `ICICI` |

Unknown source strings should migrate to:

- `type`: `other`
- `label`: `Other`
- `bank`: the old source string

This preserves visibility without inventing unreliable splits.

## Source Type And Bank Values

Source types and banks are derived from rules, not from independent taxonomy tables.

The UI should distinguish:

- Predefined values: values referenced by bundled predefined rules.
- Custom values: values introduced by user-created or imported rules.

Types and banks only exist while referenced by at least one rule. Renames happen by editing rules that use the value. Independent delete/rename screens are out of scope.

If a new bank or type is introduced by changing bundled `rules.json`, that value automatically becomes part of the predefined suggestions.

## Rule Fixture Format

Rule fixtures should live in:

```text
tests/data/rule-emails/
```

Use one email per fixture file. Fixtures are positive-only: every fixture must match its named rule and produce the expected extraction.

Use `.yaml` as the standard extension. The test runner should scan for fixture files automatically.

Fixture filenames should follow:

```text
<bank>_<source_type>_<case>.yaml
```

Examples:

```text
hdfc_credit_card_payment_made_alert.yaml
hdfc_credit_card_standing_instruction.yaml
icici_credit_card_usd_international.yaml
axis_credit_card_html_amount_block.yaml
```

The Go test name should use only the file basename without extension:

```text
TestRuleFixtures/hdfc_credit_card_payment_made_alert
```

Fixture format:

```yaml
---
rule: HDFC Credit Card (payment made alert)
sender: HDFC Alerts <alerts@hdfcbank.net>
subject: A payment was made using your Credit Card
want:
  amount: 1007.00
  merchant: WWW SWIGGY IN
  currency: INR
  source:
    type: credit_card
    label: Credit Card
    bank: HDFC
---
Dear Customer,
Rs.1007.00 debited towards WWW SWIGGY IN on ...
```

The fixture must not contain regex patterns. The test runner loads bundled rules from `backend/cmd/server/content/rules.json`, finds the named rule, asserts the sender and subject match, runs extraction, and asserts amount, merchant, currency, and source.

The fixture should not include a timestamp. The runner can provide an internal fixed timestamp because timestamp extraction is not part of these rule fixture assertions.

## Rule Fixture Runner

The runner should:

1. Discover all `*.yaml` files under the fixture directory.
2. Parse YAML front matter and raw body.
3. Use the filename basename as the subtest name.
4. Load and compile rules from the v2 bundled rules file.
5. Find the rule named by `rule`.
6. Parse and normalize the fixture `sender`.
7. Assert exact sender email match through the rule matcher.
8. Assert subject match.
9. Run extraction with the rule regexes and a fixed internal timestamp.
10. Assert amount, merchant, currency, and nested source.

Adding or changing a bundled rule should include at least one matching fixture unless there is a clear reason not to.

## Rule Creator UI

The Rule creator should move to a workbench layout.

All paths that currently open the rule editor should use this workbench:

- `Rules` list create/edit actions.
- `Diagnostics` row "Fix rule" actions.
- Direct rule edit URLs.
- Diagnostic-prefilled create URLs when a diagnostic cannot be associated with an existing rule.

Left panel:

- Identity: name and predefined/custom badge.
- Source: type combobox and bank combobox.
- Match: exact sender email chips/list and subject contains.
- Extract: amount, merchant, and currency regex fields.

Right panel:

- Large email sample body editor/viewer.
- Sample sender and subject.
- Expected amount, merchant, currency, and source.
- Live extraction results and pass/fail indicators.
- Sample tabs/list for multiple scratch samples.

The app database does not need to persist samples. The workbench can treat samples as temporary scratch data. A future export path can convert samples into fixture files, but bundled unit-test fixtures remain file-based.

Regex help should be shown once for the extraction group, not repeated on every regex field. It should subtly communicate that Go regexp syntax is required.

The workbench must follow the frontend design constraints:

- No native `<select>`.
- No `<datalist>`.
- No `confirm()`, `alert()`, or `prompt()`.
- No native `title` tooltips.
- Floating dropdowns/tooltips inside clipped or scrollable containers must use `position: fixed`, `getBoundingClientRect()`, and `createPortal`.
- New user-facing strings should go through the i18n catalog.

## Rules List UI

The Rules list should add columns:

- Name
- Type
- Bank
- Senders
- Subject
- Origin

Filters should persist in URL search params:

- search
- type
- bank
- origin

Type and bank filter values should be derived from rules and should show predefined/custom origin where helpful.

The `/rules` route should open on this revamped list view by default. Creating or editing a rule should be a secondary route/state from that list, not the first surface users see when they visit Rules.

## Transactions UI

The Transactions page should replace the Source column and filter with:

- Type column
- Bank column
- Type filter
- Bank filter

Filter state must persist in URL params. Existing source exclusion behavior should be translated to `exclude_source_types` and `exclude_banks` so saved exclusion workflows continue to work with the split fields.

## Dashboard UI

The Dashboard should replace the single source donut with two donuts:

- By type
- By bank

Clicking a type or bank slice should drill into the Transactions page using the corresponding URL filter.

## Prototype Phase

Before production implementation, create a temporary live prototype for the Rule creator workbench. The prototype should optimize for fast UI feedback and does not require production tests.

The production Rule creator implementation starts only after the prototype is approved.

## Testing Strategy

Follow the repo's TDD workflow.

Backend tests:

- v2 rules JSON parsing and validation.
- Exact sender matching with multiple sender emails.
- Auto-discovered rule fixture runner.
- Store/migration tests for rules and transactions source fields.
- API handler tests for nested source payloads.
- Transaction filters for source type and bank.
- Facets and charts for `source_types`, `banks`, `by_source_type`, and `by_bank`.
- Diagnostics source object persistence and responses.

Frontend tests:

- Rule workbench layout basics.
- Diagnostics "Fix rule" navigation opens the workbench with the diagnostic sample loaded.
- Sender chip editing.
- Source type and bank combobox behavior.
- Shared Go regexp help affordance.
- Rules route opens the revamped list view by default.
- Rules list URL-persisted filters.
- Transactions type/bank columns and filters.
- Dashboard type and bank donuts.

Use the narrowest test that proves behavior first, then run the broader relevant suite before finishing implementation.

## Documentation Updates

Update `.github/CONTRIBUTING.md` to document:

- v2 `rules.json` format.
- Exact multi-sender matching.
- Source type/bank structure.
- Rule fixture format.
- Fixture filename convention: `<bank>_<source_type>_<case>.yaml`.
- Requirement to add or update fixtures when adding or changing bundled rules.
- Correct current rules path, replacing stale references such as `cmd/expensor/config/rules.json`.

Update `AGENTS.md` with the same operational rules so future coding agents follow the fixture and rules conventions.

Update backend README or other docs that describe the old rule format.

## Out Of Scope

- Negative rule fixture tests.
- Wildcard or domain sender matching.
- Independent bank/type taxonomy management screens.
- Persisting rule workbench scratch samples in the app database.
- Timestamp assertions in rule fixtures.
- v1 rule import compatibility.

## Implementation Notes

- Add a shared rules v2 parser package so API handlers, predefined rule seeding, and fixture tests use the same validation and compile path.
- Treat `backend/cmd/server/content/rules.json` as the canonical bundled rules file used by the binary and rule fixture tests. If the repo keeps another copy under `content/`, update or remove the duplicate during implementation so rule tests cannot drift.
