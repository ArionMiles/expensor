# Extraction Diagnostics Workflow Design

**Date:** 2026-05-19
**Status:** Approved for implementation planning

## Purpose

Make failed transaction extraction visible and actionable. When a reader matches an email with a rule but the regex extraction yields no merchant or a zero amount, Expensor should keep enough context to fix the rule from the UI instead of silently dropping or storing unusable transactions.

This is Slice 2 of the feedback triage queue from `2026-05-19-feedback-triage-design.md`.

## Scope

In scope:

- Gmail diagnostics.
- Thunderbird diagnostics.
- DB persistence for extraction diagnostics.
- API endpoints for listing diagnostics and changing diagnostic status.
- A diagnostics page in the frontend.
- A rule-editor entry point that opens the failed email body as a test sample.

Out of scope:

- Automatically generating regexes.
- Blocking daemon scans when diagnostic persistence fails.
- Reprocessing a diagnostic directly from the diagnostics page.
- Changing the transaction writer contract.

## Diagnostic Trigger

A diagnostic is created when a reader has matched an email to a rule and `extractor.ExtractTransactionDetails` returns either:

- `MerchantInfo == ""`
- `Amount == 0`

The transaction should still follow the current reader flow unless implementation discovers an existing explicit guard. This slice is about observability and repair workflow, not changing whether questionable extracted transactions are sent to the writer.

## Data Model

Add a new `extraction_diagnostics` table. Each row stores:

- `id` UUID primary key.
- `status` text constrained to `open`, `resolved`, or `ignored`.
- `reader` text, for example `gmail` or `thunderbird`.
- `message_id` nullable text. Gmail has a stable message ID; Thunderbird may not.
- `sender_email` text.
- `subject` text.
- `email_body` text.
- `rule_name` text.
- `rule_id` nullable UUID. Existing runtime rules do not reliably carry IDs for predefined rules, so the rule name remains the durable repair hint.
- `amount_regex`, `merchant_regex`, and `currency_regex` text snapshots.
- `failure_reasons` text array containing `amount_zero`, `merchant_empty`, or both.
- `created_at`, `updated_at`, and `resolved_at`.

Create a partial unique index for open diagnostics to avoid unbounded duplicates during repeated scans:

```sql
CREATE UNIQUE INDEX IF NOT EXISTS extraction_diagnostics_open_unique
ON extraction_diagnostics (reader, message_id, rule_name)
WHERE status = 'open' AND message_id IS NOT NULL;
```

For Thunderbird rows without a message ID, do not force deduplication in the first version. The mbox data does not currently expose a reliable stable identifier in the reader contract, and false deduplication would hide real failures.

## Backend Architecture

Add reader-agnostic diagnostics types in `pkg/api`:

- `ExtractionDiagnostic` for the payload emitted by readers.
- `DiagnosticSink` with `RecordExtractionDiagnostic(context.Context, ExtractionDiagnostic) error`.

Readers receive an optional `DiagnosticSink` through their config. The sink is best-effort:

- If no sink is configured, readers continue normally.
- If recording fails, readers log a warning with reader, rule, subject, and error, then continue.

The production sink should be implemented by `internal/store.Store`, alongside list and status-update methods used by HTTP handlers.

Gmail should capture sender from the message headers, subject, body, Gmail message ID, and regex snapshots from the matched rule. Thunderbird should capture sender from the `From` header, subject, body, and regex snapshots from the matched rule.

## API

Add:

- `GET /api/extraction-diagnostics?status=open|resolved|ignored|all`
- `GET /api/extraction-diagnostics/{id}`
- `PUT /api/extraction-diagnostics/{id}/status`

The list endpoint defaults to `open`, newest first. `status=all` returns all statuses. The status endpoint accepts:

```json
{ "status": "resolved" }
```

Valid target statuses are `open`, `resolved`, and `ignored`. Moving to `resolved` or `ignored` sets `resolved_at` to `NOW()`. Moving back to `open` clears `resolved_at`.

Handlers should follow the existing `Storer` pattern and use `mockStore` in handler tests.

## Frontend Workflow

Add a `/diagnostics` page and sidebar navigation entry.

The page shows a dense table of diagnostics with:

- Status.
- Reader.
- Sender.
- Subject.
- Rule name.
- Failure reasons.
- Created time.
- Actions: fix rule, mark resolved, ignore.

The page should persist `status` filter state in the URL with `useSearchParams`.

The “Fix rule” action opens the rule editor:

- If the diagnostic can be matched to a rule ID, use `/rules/{id}?diagnostic=<id>`.
- Otherwise use `/rules/new?diagnostic=<id>`.

`RuleForm` should load the diagnostic by ID, prefill rule fields from the diagnostic snapshot where appropriate, and load `email_body` into the first test sample. Existing edit-mode rule fields remain authoritative when a rule exists; diagnostic regex snapshots should only fill empty fields or create-mode fields.

The page should use project-styled controls only. No native `<select>`, `alert`, `confirm`, `prompt`, or clipped absolute-position popovers.

## Error Handling

Diagnostic recording failures must not interrupt the daemon or prevent transaction writes. The reader logs the failed diagnostic write and continues.

API failures return the existing JSON error shape. Invalid status values return `422`. Missing diagnostics return `404`.

## Testing

Backend:

- Store integration tests for inserting diagnostics, listing by status, deduping Gmail open diagnostics by `(reader, message_id, rule_name)`, and status transitions.
- Handler unit tests for list/status endpoints, nil store, invalid status, and missing diagnostic.
- Gmail reader unit test proving failed extraction records a diagnostic without returning an error.
- Thunderbird reader unit test proving failed extraction records a diagnostic without returning an error.

Frontend:

- Component test for diagnostics page list rendering and status filter URL state.
- Component test for status actions calling mutations.
- RuleForm test proving a diagnostic ID preloads the email body as the first sample.

## Rollout Notes

This slice can ship independently. Existing users get the new table via migration on startup. Diagnostics start accumulating only after the new reader configuration wires the sink into Gmail and Thunderbird.
