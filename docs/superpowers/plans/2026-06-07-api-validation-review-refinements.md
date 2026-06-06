# API Validation Review Refinements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refine candidate API validation with product-aligned pagination limits, checked offset arithmetic, and a type-safe generic query decode-and-validate helper.

**Architecture:** Candidate query handlers call one package-level generic helper that decodes a concrete DTO with `form/v4` and validates it with `validator/v10`. Transaction pagination overflow is rejected by a struct-level HTTP validator and independently guarded in the Postgres repository; query conversion metadata is cached by concrete DTO type.

**Tech Stack:** Go 1.26, `github.com/go-playground/form/v4`, `github.com/go-playground/validator/v10`, PostgreSQL/pgx, Swag/OpenAPI.

---

### Task 1: Correct Transaction Pagination Validation

**Files:**
- Modify: `backend/internal/api/handlers_test.go`
- Modify: `backend/internal/api/handlers_transactions.go`
- Modify: `backend/internal/api/validation.go`

- [ ] **Step 1: Replace the old page-size boundary test and add accepted pagination tests**

In `TestListTransactions_RejectsInvalidQuery`, replace the `page_size=501`
case with:

```go
{
	name:    "page size too large",
	query:   "page_size=101",
	field:   "page_size",
	message: "must be at most 100",
},
```

Add a success test proving the removed page maximum and supported maximum page
size reach the store unchanged:

```go
func TestListTransactions_AcceptsLargePageAndMaximumPageSize(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(h.ListTransactions, "/api/transactions?page=10001&page_size=100")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if st.listFilter.Page != 10001 || st.listFilter.PageSize != 100 {
		t.Fatalf("pagination = page:%d page_size:%d", st.listFilter.Page, st.listFilter.PageSize)
	}
}
```

Add an overflow test using the platform maximum integer:

```go
func TestListTransactions_RejectsOffsetOverflow(t *testing.T) {
	st := &mockStore{}
	h := newTestHandlers(t, st, &mockDaemon{})
	maxInt := int(^uint(0) >> 1)

	rr := get(h.ListTransactions, fmt.Sprintf(
		"/api/transactions?page=%d&page_size=100",
		maxInt,
	))

	assertValidationError(t, rr, "page", "query", "is too large for page_size")
	if st.listCalls != 0 || st.searchCalls != 0 {
		t.Fatalf("store calls = list:%d search:%d", st.listCalls, st.searchCalls)
	}
}
```

- [ ] **Step 2: Run the handler tests and verify they fail**

Run:

```bash
task test:be
```

Expected: FAIL in the new transaction handler cases because `page` still has a
maximum, `page_size` still permits 500, and offset overflow has no semantic
rule.

- [ ] **Step 3: Update transaction query tags and OpenAPI annotations**

Change the annotations in `handlers_transactions.go` to:

```go
// @Param page query int false "1-based page number" default(1) minimum(1)
// @Param page_size query int false "Page size" default(20) minimum(1) maximum(100)
```

Change the DTO fields to:

```go
Page     *int `form:"page" validate:"omitempty,min=1"`
PageSize *int `form:"page_size" validate:"omitempty,min=1,max=100"`
```

- [ ] **Step 4: Add a transaction pagination struct validator**

Register the rule in `newRequestValidator`:

```go
validate.RegisterStructValidation(validateTransactionPagination, transactionListQuery{})
```

Add:

```go
func validateTransactionPagination(level validator.StructLevel) {
	query := level.Current().Interface().(transactionListQuery)
	if query.Page == nil || *query.Page <= 1 {
		return
	}

	pageSize := 20
	if query.PageSize != nil {
		pageSize = *query.PageSize
	}
	if pageSize < 1 {
		return
	}
	maxInt := int(^uint(0) >> 1)
	if *query.Page-1 > maxInt/pageSize {
		level.ReportError(query.Page, "Page", "page", "page_offset", "")
	}
}
```

Add this `validationMessage` case:

```go
case "page_offset":
	return "is too large for page_size"
```

- [ ] **Step 5: Run the focused handler tests**

Run:

```bash
task test:be
```

Expected: PASS.

- [ ] **Step 6: Commit the pagination contract**

```bash
git add backend/internal/api/handlers_test.go backend/internal/api/handlers_transactions.go backend/internal/api/validation.go
git commit --no-gpg-sign -m "Correct transaction pagination validation"
```

### Task 2: Guard Repository Offset Arithmetic

**Files:**
- Modify: `backend/internal/store/errors.go`
- Modify: `backend/internal/store/store_test.go`
- Modify: `backend/internal/store/transactions_repository.go`

- [ ] **Step 1: Add repository overflow tests for list and search**

Add:

```go
func TestListTransactions_RejectsOffsetOverflow(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	maxInt := int(^uint(0) >> 1)

	_, _, err := ts.ListTransactions(
		context.Background(),
		store.ListFilter{Page: maxInt, PageSize: 100},
	)
	if !errors.Is(err, store.ErrPaginationOverflow) {
		t.Fatalf("expected ErrPaginationOverflow, got %v", err)
	}
}

func TestSearchTransactions_RejectsOffsetOverflow(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	maxInt := int(^uint(0) >> 1)

	_, _, err := ts.SearchTransactions(
		context.Background(),
		"coffee",
		store.ListFilter{Page: maxInt, PageSize: 100},
	)
	if !errors.Is(err, store.ErrPaginationOverflow) {
		t.Fatalf("expected ErrPaginationOverflow, got %v", err)
	}
}
```

- [ ] **Step 2: Run the repository tests and verify they fail**

Run:

```bash
task test:be
```

Expected: FAIL to compile because `store.ErrPaginationOverflow` does not exist.

- [ ] **Step 3: Implement checked offset calculation before database work**

Add the exported sentinel to `errors.go`:

```go
// ErrPaginationOverflow is returned when a requested page cannot be represented as a SQL offset.
var ErrPaginationOverflow = errors.New("pagination offset overflow")
```

Add the checked helper to `transactions_repository.go`:

```go
func transactionOffset(filter ListFilter) (int, error) {
	maxInt := int(^uint(0) >> 1)
	if filter.Page-1 > maxInt/filter.PageSize {
		return 0, fmt.Errorf(
			"%w: page=%d page_size=%d",
			ErrPaginationOverflow,
			filter.Page,
			filter.PageSize,
		)
	}
	return (filter.Page - 1) * filter.PageSize, nil
}
```

In `queryTransactions`, calculate the offset immediately after normalizing the
filter and return before building or running either query:

```go
f := normalizeTransactionListFilter(request.filter)
offset, err := transactionOffset(f)
if err != nil {
	return nil, TransactionListResult{}, err
}
query := strings.TrimSpace(request.search)
```

Remove the existing unchecked offset assignment later in the method.

- [ ] **Step 4: Run the repository tests**

Run:

```bash
task test:be
```

Expected: PASS.

- [ ] **Step 5: Commit repository protection**

```bash
git add backend/internal/store/errors.go backend/internal/store/store_test.go backend/internal/store/transactions_repository.go
git commit --no-gpg-sign -m "Guard transaction pagination offsets"
```

### Task 3: Introduce the Generic Query Validation Helper

**Files:**
- Modify: `backend/internal/api/query_binding_test.go`
- Modify: `backend/internal/api/query_binding.go`
- Modify: `backend/internal/api/handlers_transactions.go`
- Modify: `backend/internal/api/handlers_diagnostics.go`

- [ ] **Step 1: Rewrite binding tests around the desired generic API**

Add semantic tags to the fixture:

```go
type queryBindingFixture struct {
	Page     *int       `form:"page" validate:"omitempty,min=1"`
	DateFrom *time.Time `form:"date_from"`
	Ignored  string
}
```

Replace calls that allocate a target and invoke `h.decodeQuery` with:

```go
query, ok := decodeAndValidateQuery[queryBindingFixture](h, rr, req)
if !ok {
	t.Fatalf("query validation failed: status=%d body=%s", rr.Code, rr.Body.String())
}
```

For failure tests, discard the returned zero value and assert `ok == false`.
Rename the tests from `TestDecodeQuery_*` to `TestDecodeAndValidateQuery_*`.
Add a semantic failure case:

```go
{
	name:    "range",
	query:   "page=0",
	field:   "page",
	message: "must be at least 1",
}
```

- [ ] **Step 2: Run focused tests and verify compilation fails**

Run:

```bash
task test:be
```

Expected: FAIL to compile because `decodeAndValidateQuery` does not exist.

- [ ] **Step 3: Implement the generic helper and metadata cache**

Replace the `Handlers.decodeQuery` method with:

```go
var queryFieldTypesCache sync.Map

func decodeAndValidateQuery[T any](
	h *Handlers,
	w http.ResponseWriter,
	r *http.Request,
) (T, bool) {
	var query T
	err := h.queryDecoder.Decode(&query, r.URL.Query())
	if err != nil {
		writeQueryDecodeError[T](h, w, err)
		return query, false
	}
	if !h.validateRequest(w, "query", query) {
		return query, false
	}
	return query, true
}
```

Implement the decode-error translation as:

```go
func writeQueryDecodeError[T any](h *Handlers, w http.ResponseWriter, err error) {
	var decodeErrors form.DecodeErrors
	if !errors.As(err, &decodeErrors) {
		h.logger.Error("decode query", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to decode query")
		return
	}

	fieldTypes := queryFieldTypes[T]()
	fields := make([]string, 0, len(decodeErrors))
	for field := range decodeErrors {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	violations := make([]ValidationErrorDetail, 0, len(fields))
	for _, field := range fields {
		violations = append(violations, ValidationErrorDetail{
			Field:    field,
			Location: "query",
			Message:  queryConversionMessage(fieldTypes[field]),
		})
	}
	writeValidationErrors(w, violations)
}
```

Replace `queryFieldTypes(target any)` with:

```go
func queryFieldTypes[T any]() map[string]reflect.Type {
	targetType := reflect.TypeFor[T]()
	if cached, ok := queryFieldTypesCache.Load(targetType); ok {
		return cached.(map[string]reflect.Type)
	}

	fieldTypes := buildQueryFieldTypes(targetType)
	queryFieldTypesCache.Store(targetType, fieldTypes)
	return fieldTypes
}
```

Move the existing field traversal into `buildQueryFieldTypes(reflect.Type)`.
Use this implementation so pointers are unwrapped consistently and the cached
map is immutable after construction:

```go
func buildQueryFieldTypes(targetType reflect.Type) map[string]reflect.Type {
	for targetType.Kind() == reflect.Pointer {
		targetType = targetType.Elem()
	}
	if targetType.Kind() != reflect.Struct {
		return nil
	}

	fieldTypes := make(map[string]reflect.Type, targetType.NumField())
	for index := range targetType.NumField() {
		field := targetType.Field(index)
		name := strings.Split(field.Tag.Get("form"), ",")[0]
		if name == "" || name == "-" {
			name = field.Name
		}
		fieldType := field.Type
		for fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		fieldTypes[name] = fieldType
	}
	return fieldTypes
}
```

- [ ] **Step 4: Migrate both candidate query handlers**

Change transactions to:

```go
query, ok := decodeAndValidateQuery[transactionListQuery](h, w, r)
if !ok {
	return
}
```

Change diagnostics to:

```go
query, ok := decodeAndValidateQuery[diagnosticListQuery](h, w, r)
if !ok {
	return
}
if query.Status == "" {
	query.Status = store.DiagnosticStatusOpen
}
```

Because absence is valid and defaulted after validation, change its tag to:

```go
Status string `form:"status" validate:"omitempty,oneof=open resolved ignored all"`
```

Remove the now-duplicated explicit `validateRequest` calls from both handlers.

- [ ] **Step 5: Run query and candidate handler tests**

Run:

```bash
task test:be
```

Expected: PASS.

- [ ] **Step 6: Commit the generic helper**

```bash
git add backend/internal/api/query_binding.go backend/internal/api/query_binding_test.go backend/internal/api/handlers_transactions.go backend/internal/api/handlers_diagnostics.go
git commit --no-gpg-sign -m "Type candidate query validation"
```

### Task 4: Update Documentation and Remove Stale Schema

**Files:**
- Modify: `CLAUDE.md`
- Modify: `backend/internal/api/openapi_types.go`
- Modify: `api/openapi/expensor.openapi.yaml`

- [ ] **Step 1: Remove the obsolete OpenAPI response type**

Delete `TransactionsSearchResponse` from `openapi_types.go`. Also update the
`ValidationErrorDetail.Message` example:

```go
Message string `json:"message" example:"must be at most 100"`
```

Confirm the stale type has no remaining source references:

```bash
rg -n "TransactionsSearchResponse" backend
```

Expected: no output.

- [ ] **Step 2: Add generics guidance**

Under `### Backend code health` in `CLAUDE.md`, add:

```markdown
Use Go generics when they improve compile-time type safety or remove meaningful
repeated algorithms across concrete types. Do not introduce generics solely to
replace `any` when the underlying library still relies on reflection or when
call sites do not become clearer. Do not claim a generics performance benefit
without measuring the relevant path. For tag-driven decoders and validators,
cache unavoidable local reflection metadata by concrete DTO type when it is
reused.
```

- [ ] **Step 3: Regenerate and verify OpenAPI**

Run:

```bash
task openapi:generate
```

Verify:

```bash
rg -n "maximum: 10000|maximum: 500|TransactionsSearchResponse" api/openapi/expensor.openapi.yaml
rg -n -A8 "name: page$|name: page_size$" api/openapi/expensor.openapi.yaml
```

Expected: the first command has no output; the page parameter has no maximum;
the page-size parameter has `maximum: 100`.

- [ ] **Step 4: Commit documentation and schema cleanup**

```bash
git add CLAUDE.md backend/internal/api/openapi_types.go api/openapi/expensor.openapi.yaml
git commit --no-gpg-sign -m "Document validation generics guidance"
```

### Task 5: Full Verification and PR Update

**Files:**
- Modify: `docs/superpowers/plans/2026-06-07-api-validation-review-refinements.md`

- [ ] **Step 1: Format backend code**

Run:

```bash
task fmt:be
```

Expected: exit 0.

- [ ] **Step 2: Run full backend tests**

Run:

```bash
task test:be
```

Expected: all backend packages pass, including Postgres integration tests.

- [ ] **Step 3: Run strict production lint**

Run:

```bash
task lint:be:prod
```

Expected: `0 issues.`

- [ ] **Step 4: Check OpenAPI drift**

Run:

```bash
task openapi:check
```

Expected: exit 0 with no generated artifact diff.

- [ ] **Step 5: Check the final worktree**

Run:

```bash
git diff --check
git status --short
```

Expected: no whitespace errors; only the intentionally untracked
`CODE_QUALITY_REVIEW.md` remains outside the committed changes.

- [ ] **Step 6: Push the feature branch**

```bash
git push origin pr/api-validation-candidates
```

Expected: PR #23 updates with the refinement commits.
