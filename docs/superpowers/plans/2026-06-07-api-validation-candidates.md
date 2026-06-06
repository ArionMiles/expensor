# API Validation Candidates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add typed HTTP validation and structured validation errors to the approved transaction, extraction-diagnostic, and preference endpoints.

**Architecture:** `Handlers` owns one configured `validator.Validate`. Focused query decoders convert URL values into typed DTOs and report syntax errors, then validator tags and small custom validators enforce semantic constraints. A shared response mapper exposes only public field names, locations, and consumer-facing messages.

**Tech Stack:** Go 1.26, `github.com/go-playground/validator/v10`, `net/http`, existing handler unit tests, swag/OpenAPI generation, Task.

---

## File Map

- Create `backend/internal/api/validation.go`: validator construction, custom validators, structured error types, field-name/message mapping, and response writer.
- Create `backend/internal/api/query_validation.go`: reusable optional query parsing helpers that distinguish absent values from invalid values.
- Modify `backend/internal/api/handlers.go`: store the configured validator on `Handlers`.
- Modify `backend/internal/api/handlers_transactions.go`: typed transaction query DTO, decoding, validation, and conversion to `store.ListFilter`.
- Modify `backend/internal/api/handlers_diagnostics.go`: typed list query and validator-backed status patch.
- Modify `backend/internal/api/handlers_config.go`: preference normalization followed by validator-backed validation.
- Modify `backend/internal/api/openapi_types.go`: structured validation response schema and validation tags on public request DTOs.
- Modify `backend/internal/api/handlers_test.go`: candidate behavior tests.
- Modify `backend/go.mod` and `backend/go.sum`: validator dependency.
- Regenerate `api/openapi/expensor.openapi.yaml`.

### Task 1: Shared Validation Boundary

**Files:**
- Create: `backend/internal/api/validation.go`
- Modify: `backend/internal/api/handlers.go`
- Modify: `backend/internal/api/openapi_types.go`
- Modify: `backend/internal/api/handlers_test.go`
- Modify: `backend/go.mod`
- Modify: `backend/go.sum`

- [ ] **Step 1: Add a failing handler test for the structured response**

Add a helper assertion and extend an existing invalid diagnostic-status test:

```go
func assertValidationError(
	t *testing.T,
	rr *httptest.ResponseRecorder,
	field string,
	location string,
	message string,
) {
	t.Helper()
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	var response ValidationErrorResponse
	decodeJSON(t, rr.Body.String(), &response)
	if response.Error != "request validation failed" {
		t.Fatalf("error = %q", response.Error)
	}
	want := []ValidationErrorDetail{{Field: field, Location: location, Message: message}}
	if !reflect.DeepEqual(response.Details, want) {
		t.Fatalf("details = %#v, want %#v", response.Details, want)
	}
}
```

Change `TestUpdateExtractionDiagnosticStatus_InvalidStatus` to expect:

```go
assertValidationError(t, rr, "status", "body", "must be one of: open, resolved, ignored")
```

- [ ] **Step 2: Run the narrow test and verify the red state**

Run:

```bash
task test:be
```

Expected: FAIL to compile because `ValidationErrorResponse` and
`ValidationErrorDetail` do not exist.

- [ ] **Step 3: Add the validator dependency**

From `backend/`, run:

```bash
go get github.com/go-playground/validator/v10
```

Expected: `backend/go.mod` and `backend/go.sum` add validator and its required transitive dependencies.

- [ ] **Step 4: Implement validator ownership and response types**

Add to `Handlers`:

```go
validate *validator.Validate
```

Initialize it in `NewHandlers` with `newRequestValidator()`.

Define public OpenAPI response types:

```go
type ValidationErrorDetail struct {
	Field    string `json:"field" example:"page_size"`
	Location string `json:"location" example:"query"`
	Message  string `json:"message" example:"must be at most 500"`
}

type ValidationErrorResponse struct {
	Error   string                  `json:"error" example:"request validation failed"`
	Details []ValidationErrorDetail `json:"details"`
}
```

Create `validation.go` with:

```go
func newRequestValidator() *validator.Validate {
	validate := validator.New()
	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		for _, tagName := range []string{"json", "query"} {
			if name := strings.Split(field.Tag.Get(tagName), ",")[0]; name != "" && name != "-" {
				return name
			}
		}
		return field.Name
	})
	return validate
}

func (h *Handlers) validateRequest(
	w http.ResponseWriter,
	location string,
	request any,
) bool {
	err := h.validate.Struct(request)
	if err == nil {
		return true
	}
	var validationErrors validator.ValidationErrors
	if !errors.As(err, &validationErrors) {
		h.logger.Error("validate request", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to validate request")
		return false
	}
	details := make([]ValidationErrorDetail, 0, len(validationErrors))
	for _, fieldError := range validationErrors {
		details = append(details, ValidationErrorDetail{
			Field:    fieldError.Field(),
			Location: location,
			Message:  validationMessage(fieldError),
		})
	}
	writeValidationErrors(w, details)
	return false
}

func writeValidationErrors(w http.ResponseWriter, details []ValidationErrorDetail) {
	writeJSON(w, http.StatusUnprocessableEntity, ValidationErrorResponse{
		Error:   "request validation failed",
		Details: details,
	})
}
```

Implement `validationMessage` as an explicit switch for the tags used in this PR (`required`, `oneof`, `min`, `max`, `len`, and registered custom tags). It must return consumer-facing text and never expose the tag name.

- [ ] **Step 5: Temporarily add validator tags to the diagnostic request**

```go
type ExtractionDiagnosticStatusRequest struct {
	Status string `json:"status" example:"resolved" validate:"required,oneof=open resolved ignored" enums:"open,resolved,ignored"`
}
```

Call `h.validateRequest(w, "body", body)` in the patch handler so the narrow test can pass. The complete diagnostic refactor occurs in Task 3.

- [ ] **Step 6: Run the narrow test and verify green**

Run:

```bash
task test:be
```

Expected: PASS, including `TestUpdateExtractionDiagnosticStatus_InvalidStatus`.

- [ ] **Step 7: Format and commit**

Run:

```bash
task fmt:be
git add backend/internal/api/validation.go backend/internal/api/handlers.go backend/internal/api/openapi_types.go \
  backend/internal/api/handlers_diagnostics.go backend/internal/api/handlers_test.go backend/go.mod backend/go.sum
git commit --no-gpg-sign -m "Add structured request validation"
```

### Task 2: Transaction Query DTO

**Files:**
- Create: `backend/internal/api/query_validation.go`
- Modify: `backend/internal/api/handlers_transactions.go`
- Modify: `backend/internal/api/handlers_test.go`

- [ ] **Step 1: Replace fallback expectations with failing validation tests**

Replace `TestListTransactions_HugePaginationFallsBackToDefaults` with table-driven cases covering:

```go
tests := []struct {
	name     string
	query    string
	field    string
	message  string
}{
	{name: "page overflow", query: "page=576460752303423488", field: "page", message: "must be an integer"},
	{name: "page size too large", query: "page_size=501", field: "page_size", message: "must be at most 500"},
	{name: "invalid date", query: "date_from=yesterday", field: "date_from", message: "must be an RFC3339 timestamp"},
	{name: "invalid weekday", query: "weekday=7", field: "weekday", message: "must be at most 6"},
	{name: "invalid hour", query: "hour_from=24", field: "hour_from", message: "must be at most 23"},
	{name: "invalid boolean flag", query: "show_muted=true", field: "show_muted", message: "must be 1 when present"},
	{name: "invalid sort", query: "sort_dir=sideways", field: "sort_dir", message: "must be one of: asc, desc"},
	{name: "invalid timezone", query: "tz=Mars/Olympus", field: "tz", message: "must be a valid IANA timezone"},
	{name: "control character", query: "currency=%00bad", field: "currency", message: "must not contain control characters"},
}
```

Each case calls `assertValidationError(t, rr, field, "query", message)` and asserts neither list nor search store operation was invoked.

Add or retain a success test proving absent pagination defaults to page `1` and page size `20`.

- [ ] **Step 2: Run transaction tests and verify red**

Run:

```bash
task test:be
```

Expected: FAIL in the new transaction validation cases because invalid values
still fall back or return the old error shape/status.

- [ ] **Step 3: Implement reusable optional query parsing**

Create `query_validation.go` with helpers shaped as:

```go
func optionalQueryInt(values url.Values, key string) (*int, *ValidationErrorDetail)
func optionalQueryTime(values url.Values, key string) (*time.Time, *ValidationErrorDetail)
```

Absent or blank values return `(nil, nil)`. Present malformed values return a detail with `Location: "query"` and messages `must be an integer` or `must be an RFC3339 timestamp`.

- [ ] **Step 4: Implement the transaction query DTO**

Define a private DTO with public query tags:

```go
type transactionListQuery struct {
	Page               *int       `query:"page" validate:"omitempty,min=1,max=10000"`
	PageSize           *int       `query:"page_size" validate:"omitempty,min=1,max=500"`
	Merchant           string     `query:"merchant" validate:"no_control_chars"`
	Category           string     `query:"category" validate:"no_control_chars"`
	CategoryMissing    string     `query:"category_missing" validate:"omitempty,oneof=1"`
	Currency           string     `query:"currency" validate:"no_control_chars"`
	Source             string     `query:"source" validate:"no_control_chars"`
	SourceType         string     `query:"source_type" validate:"no_control_chars"`
	Bank               string     `query:"bank" validate:"no_control_chars"`
	Label              string     `query:"label" validate:"no_control_chars"`
	Bucket             string     `query:"bucket" validate:"no_control_chars"`
	DateFrom           *time.Time `query:"date_from"`
	DateTo             *time.Time `query:"date_to"`
	ShowMuted          string     `query:"show_muted" validate:"omitempty,oneof=1"`
	MutedOnly          string     `query:"muted_only" validate:"omitempty,oneof=1"`
	IndividualOnly     string     `query:"individual_only" validate:"omitempty,oneof=1"`
	Weekday            *int       `query:"weekday" validate:"omitempty,min=0,max=6"`
	HourFrom           *int       `query:"hour_from" validate:"omitempty,min=0,max=23"`
	HourTo             *int       `query:"hour_to" validate:"omitempty,min=0,max=23"`
	Timezone           string     `query:"tz" validate:"omitempty,iana_timezone"`
	Query              string     `query:"q" validate:"no_control_chars"`
	SortBy             string     `query:"sort_by" validate:"omitempty,oneof=timestamp"`
	SortDir            string     `query:"sort_dir" validate:"omitempty,oneof=asc desc"`
}
```

Include the existing exclusion CSV fields and missing flags in the real DTO. Register `no_control_chars` and `iana_timezone` in `newRequestValidator`.

Implement:

```go
func decodeTransactionListQuery(values url.Values) (transactionListQuery, []ValidationErrorDetail)
func (query transactionListQuery) listFilter(defaultTimezone string) store.ListFilter
```

The decoder parses typed fields and collects all syntax failures. The converter applies pagination defaults and preserves existing CSV, missing-flag, and timezone fallback behavior.

- [ ] **Step 5: Wire the DTO into `ListTransactions`**

Replace `invalidTransactionFilter`, `transactionListFilter`, `queryInt`, `queryHour`, and `queryWeekday` usage with:

```go
query, details := decodeTransactionListQuery(r.URL.Query())
if len(details) > 0 {
	writeValidationErrors(w, details)
	return
}
if !h.validateRequest(w, "query", query) {
	return
}
filter := query.listFilter(h.resolveTimezone(r.Context(), query.Timezone))
```

Use `strings.TrimSpace(query.Query)` to select list versus search. Delete helpers made unused by the new path.

- [ ] **Step 6: Run transaction tests and verify green**

Run:

```bash
task test:be
```

Expected: PASS, including transaction search-plus-filter and default-pagination
coverage.

- [ ] **Step 7: Format and commit**

Run:

```bash
task fmt:be
git add backend/internal/api/query_validation.go backend/internal/api/handlers_transactions.go \
  backend/internal/api/http_helpers.go backend/internal/api/handlers_test.go
git commit --no-gpg-sign -m "Validate transaction collection queries"
```

### Task 3: Extraction Diagnostic DTOs

**Files:**
- Modify: `backend/internal/api/handlers_diagnostics.go`
- Modify: `backend/internal/api/handlers_test.go`

- [ ] **Step 1: Add failing structured-error tests**

Update or add tests for:

```go
GET /api/extraction-diagnostics?status=pending
GET /api/extraction-diagnostics?limit=bad
GET /api/extraction-diagnostics?limit=0
PATCH /api/extraction-diagnostics/{id} with {}
PATCH /api/extraction-diagnostics/{id} with not-json
```

Expect structured `422` details for semantic failures and `400` with `{"error":"invalid JSON body"}` for malformed JSON.

- [ ] **Step 2: Run diagnostic tests and verify red**

Run:

```bash
task test:be
```

Expected: FAIL in the diagnostic tests on structured list-query errors,
required status, and malformed JSON status.

- [ ] **Step 3: Implement list query decoding and validation**

Add:

```go
type diagnosticListQuery struct {
	Status string `query:"status" validate:"required,oneof=open resolved ignored all"`
	Limit  *int   `query:"limit" validate:"omitempty,min=1"`
}
```

Default status to `open`, parse `limit` with `optionalQueryInt`, return parsing details before validation, then map the DTO to `store.DiagnosticFilter`.

- [ ] **Step 4: Complete patch validation**

Change malformed JSON to:

```go
writeError(w, http.StatusBadRequest, "invalid JSON body")
```

Validate `ExtractionDiagnosticStatusRequest` through `h.validateRequest` and delete `validDiagnosticUpdateStatus`. Keep `store.ValidateDiagnosticFilterStatus` and store update validation unchanged.

- [ ] **Step 5: Run diagnostic tests and verify green**

Run:

```bash
task test:be
```

Expected: PASS, including all extraction diagnostic tests.

- [ ] **Step 6: Format and commit**

Run:

```bash
task fmt:be
git add backend/internal/api/handlers_diagnostics.go backend/internal/api/handlers_test.go
git commit --no-gpg-sign -m "Validate extraction diagnostic requests"
```

### Task 4: Preference Patch Validation

**Files:**
- Modify: `backend/internal/api/handlers_config.go`
- Modify: `backend/internal/api/openapi_types.go`
- Modify: `backend/internal/api/validation.go`
- Modify: `backend/internal/api/handlers_test.go`

- [ ] **Step 1: Add failing preference error-contract tests**

Update `TestPatchPreferencesValidatesBeforeWriting` to expect:

```go
assertValidationError(
	t,
	rr,
	"scan_interval",
	"body",
	"must be at least 10",
)
```

Add cases for invalid currency, lookback days, timezone, and time format. For every case assert `ms.appConfig` remains empty. Add malformed JSON coverage expecting `400`.

- [ ] **Step 2: Run preference tests and verify red**

Run:

```bash
task test:be
```

Expected: FAIL in preference tests because semantic errors currently return an
unstructured `400` and malformed JSON returns `422`.

- [ ] **Step 3: Add DTO tags and custom validators**

Update:

```go
type PreferencesPatchRequest struct {
	BaseCurrency *string `json:"base_currency,omitempty" validate:"omitempty,currency_code"`
	ScanInterval *int    `json:"scan_interval,omitempty" validate:"omitempty,min=10,max=3600"`
	LookbackDays *int    `json:"lookback_days,omitempty" validate:"omitempty,min=1,max=3650"`
	Timezone     *string `json:"timezone,omitempty" validate:"omitempty,iana_timezone"`
	TimeFormat   *string `json:"time_format,omitempty" validate:"omitempty,oneof='HH:mm' 'HH:mm:ss' 'h:mm a' 'h:mm:ss a'"`
}
```

Register `currency_code` and ensure custom validation messages are:

```text
must be a 3-letter ISO 4217 code
must be a valid IANA timezone
```

- [ ] **Step 4: Separate normalization from validation**

Replace `normalizePreferencesPatch` with a no-error normalizer:

```go
func normalizePreferencesPatch(body *PreferencesPatchRequest) {
	if body.BaseCurrency != nil {
		value := strings.ToUpper(strings.TrimSpace(*body.BaseCurrency))
		body.BaseCurrency = &value
	}
	if body.Timezone != nil {
		value := strings.TrimSpace(*body.Timezone)
		body.Timezone = &value
	}
	if body.TimeFormat != nil {
		value := strings.TrimSpace(*body.TimeFormat)
		body.TimeFormat = &value
	}
}
```

In `PatchPreferences`, return `400` for malformed JSON, normalize, call `h.validateRequest(w, "body", body)`, and persist only after validation succeeds. Delete `validTimeFormats` and validation branches replaced by tags.

- [ ] **Step 5: Run preference tests and verify green**

Run:

```bash
task test:be
```

Expected: PASS, including preference normalization and no-partial-write
assertions.

- [ ] **Step 6: Format and commit**

Run:

```bash
task fmt:be
git add backend/internal/api/handlers_config.go backend/internal/api/openapi_types.go \
  backend/internal/api/validation.go backend/internal/api/handlers_test.go
git commit --no-gpg-sign -m "Validate application preference updates"
```

### Task 5: OpenAPI and Full Verification

**Files:**
- Modify: handler Swagger annotations in candidate handler files
- Modify: `api/openapi/expensor.openapi.yaml`

- [ ] **Step 1: Document validation failures**

Add `@Failure 422 {object} ValidationErrorResponse` to all candidate endpoints. Keep `@Failure 400 {object} ErrorResponse` for malformed JSON and malformed path syntax. Ensure query parameter minima, maxima, and enumerations match the implementation.

- [ ] **Step 2: Generate OpenAPI artifacts**

Run:

```bash
task openapi:generate
```

Expected: generated artifacts include `ValidationErrorResponse` and `ValidationErrorDetail`, and candidate `422` responses reference the structured schema.

- [ ] **Step 3: Run focused and broad verification**

Run:

```bash
task fmt:be
task test:be
task lint:be:prod
task openapi:check
```

Expected: all commands pass and the production linter reports `0 issues`.

- [ ] **Step 4: Review the diff**

Run:

```bash
git diff main...HEAD --check
git diff main...HEAD --stat
git status --short
```

Expected: no whitespace errors; only candidate validation, documentation, dependency, and generated OpenAPI files are changed. `CODE_QUALITY_REVIEW.md` remains untracked and is not committed.

- [ ] **Step 5: Commit generated documentation**

```bash
git add backend/internal/api/handlers_transactions.go backend/internal/api/handlers_diagnostics.go \
  backend/internal/api/handlers_config.go api/openapi/expensor.openapi.yaml
git commit --no-gpg-sign -m "Document API validation errors"
```

### Task 6: Push and Open the Candidate PR

**Files:**
- Read: `.github/PULL_REQUEST_TEMPLATE.md`

- [ ] **Step 1: Push the feature branch**

Run:

```bash
git push -u origin pr/api-validation-candidates
```

Expected: remote branch is created or updated successfully.

- [ ] **Step 2: Create the PR from the repository template**

Use `.github/PULL_REQUEST_TEMPLATE.md` unchanged as the section structure. Describe the bounded candidate scope, structured error contract, intentional transaction-query behavior change, and verification commands. Mark this as a breaking API behavior change for clients that relied on invalid query values being silently ignored.

Run:

```bash
gh pr create --base main --head pr/api-validation-candidates --title "Add structured API request validation" --body-file /tmp/expensor-validation-pr.md
```

Expected: GitHub returns the new PR URL.
