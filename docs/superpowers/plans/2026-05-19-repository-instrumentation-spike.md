# Repository Pattern And Instrumentation Spike Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Produce a bounded architecture spike for repository-pattern migration, query templates, and instrumentation middleware.

**Status:** Complete. Implemented and merged to `main`; query inventory, repository boundary notes, instrumentation helper, prototype, and spike results are documented.

**Architecture:** Inventory current query shapes, define repository boundaries, prototype one low-risk repository, and document migration rules before broad refactoring.

**Tech Stack:** Go, pgx/v5, slog, existing `internal/store`, handler `Storer` interface, unit tests.

---

## Scope

In scope:

- Query inventory.
- Repository boundary proposal.
- Instrumentation middleware interface.
- Prototype one bounded area, preferably labels or app config.

Out of scope:

- Rewriting all `internal/store/store.go`.
- Adding an ORM.
- Generating SQL with raw user input.

---

### Task 1: Query Inventory

**Files:**
- Create: `docs/architecture/query-inventory-2026-05-19.md`

- [x] **Step 1: Generate inventory**

Run:

```bash
rg -n "func \\(s \\*Store\\)|Query\\(|QueryRow\\(|Exec\\(" backend/internal/store/store.go
```

- [x] **Step 2: Classify query groups**

Create:

```markdown
# Store Query Inventory 2026-05-19

| Area | Methods | Query style | Risk | Suggested repository |
|---|---|---|---|---|
| Transactions | ListTransactions, SearchTransactions, UpdateTransaction | dynamic filters | High | TransactionRepository |
| Labels | ListLabels, CreateLabel, UpdateLabel, DeleteLabel | CRUD | Low | LabelRepository |
| App config | GetAppConfig, SetAppConfig | key/value | Low | ConfigRepository |
```

Fill the table with actual methods from `store.go`.

- [x] **Step 3: Commit inventory**

```bash
git add docs/architecture/query-inventory-2026-05-19.md
git commit --no-gpg-sign -m "docs: inventory store query shapes"
```

---

### Task 2: Define Repository Interfaces

**Files:**
- Create: `docs/architecture/repository-pattern.md`

- [x] **Step 1: Write architecture note**

Create:

```markdown
# Repository Pattern Direction

## Boundaries

- `TransactionRepository`: transaction list/search/update/mute/label joins.
- `TaxonomyRepository`: labels, categories, buckets.
- `RuleRepository`: extraction rules.
- `RuntimeRepository`: app config, reader runtime state, processed messages.
- `ContentRepository`: community content sync and imported reference data.

## Rules

- Handlers depend on small interfaces, not concrete repositories.
- Dynamic query builders return SQL fragments plus parameter arrays.
- User input is never interpolated into SQL.
- Templates may only vary trusted structural clauses such as sort column allowlists.
- Every repository method wraps errors with operation context.
```

- [x] **Step 2: Commit**

```bash
git add docs/architecture/repository-pattern.md
git commit --no-gpg-sign -m "docs: define repository migration boundaries"
```

---

### Task 3: Add Instrumentation Middleware Prototype

**Files:**
- Create: `backend/internal/store/instrumentation.go`
- Create: `backend/internal/store/instrumentation_test.go`

- [x] **Step 1: Write failing tests**

```go
func TestInstrumentRecordsDurationAndError(t *testing.T) {
	var out bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&out, nil))
	metrics := store.NewQueryInstrumentation(logger)

	err := metrics.Observe(context.Background(), "labels.list", func(context.Context) error {
		return errors.New("boom")
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(out.String(), "labels.list") {
		t.Fatalf("expected operation in log output, got %q", out.String())
	}
}
```

- [x] **Step 2: Run backend tests**

Run: `task test:be`

Expected: FAIL because instrumentation helper does not exist.

- [x] **Step 3: Implement minimal instrumentation helper**

Create:

```go
type QueryInstrumentation struct {
	logger *slog.Logger
	now    func() time.Time
}

func NewQueryInstrumentation(logger *slog.Logger) *QueryInstrumentation {
	if logger == nil {
		logger = slog.Default()
	}
	return &QueryInstrumentation{logger: logger, now: time.Now}
}

func (q *QueryInstrumentation) Observe(ctx context.Context, operation string, fn func(context.Context) error) error {
	start := q.now()
	err := fn(ctx)
	q.logger.Debug("store query", "operation", operation, "duration_ms", time.Since(start).Milliseconds(), "error", err)
	return err
}
```

Keep it unused until the prototype repository task to avoid broad churn.

- [x] **Step 4: Run tests and commit**

Run: `task test:be`

Expected: PASS.

```bash
git add backend/internal/store/instrumentation.go backend/internal/store/instrumentation_test.go
git commit --no-gpg-sign -m "feat: add store instrumentation helper"
```

---

### Task 4: Prototype Label Repository

**Files:**
- Create: `backend/internal/store/label_repository.go`
- Create: `backend/internal/store/label_repository_test.go`
- Modify: `backend/internal/store/store.go`

- [x] **Step 1: Write failing repository tests**

```go
func TestLabelRepositoryListCreatesNonNilSlice(t *testing.T) {
	ts := newTestStore(t)
	repo := store.NewLabelRepository(ts.PoolForTest(), slog.Default())

	labels, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if labels == nil {
		t.Fatal("labels should be non-nil")
	}
}
```

- [x] **Step 2: Run backend tests**

Run: `task test:be`

Expected: FAIL because repository does not exist.

- [x] **Step 3: Implement repository**

Move label CRUD SQL into `label_repository.go` behind:

```go
type LabelRepository interface {
	List(ctx context.Context) ([]Label, error)
	Create(ctx context.Context, name, color string) error
	Update(ctx context.Context, name, color string) error
	Delete(ctx context.Context, name string) error
}
```

Keep `Store.ListLabels`, `Store.CreateLabel`, `Store.UpdateLabel`, and `Store.DeleteLabel` as forwarding methods so API code does not change in this spike.

- [x] **Step 4: Run tests and commit**

Run: `task test:be`

Expected: PASS.

```bash
git add backend/internal/store/store.go backend/internal/store/label_repository.go backend/internal/store/label_repository_test.go
git commit --no-gpg-sign -m "refactor: prototype label repository"
```

---

### Task 5: Spike Evaluation

**Files:**
- Create: `docs/architecture/repository-spike-results-2026-05-19.md`

- [x] **Step 1: Document results**

Create:

```markdown
# Repository Spike Results 2026-05-19

## Prototype

LabelRepository was extracted while preserving Store forwarding methods.

## What Worked

- Small CRUD areas can move without handler changes.
- Store forwarding keeps `Storer` stable.

## Risks

- Transaction search/list dynamic filters need a separate query-builder design.
- Instrumentation should wrap repository methods, not every helper.

## Recommended Next Migration

Move app config/runtime state next, then rules, then transactions last.
```

- [x] **Step 2: Commit**

```bash
git add docs/architecture/repository-spike-results-2026-05-19.md
git commit --no-gpg-sign -m "docs: record repository spike results"
```

---

### Task 6: Final Verification

- [x] **Step 1: Run backend verification**

Run: `task fmt:be`

Expected: PASS.

Run: `task lint:be:prod`

Expected: `0 issues`.

Run: `task test:be`

Expected: PASS.

- [x] **Step 2: Commit final fixes if needed**

```bash
git add backend docs
git commit --no-gpg-sign -m "test: verify repository spike"
```

Only create this commit if verification changed files.
