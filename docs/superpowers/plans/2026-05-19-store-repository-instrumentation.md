# Store Repository Instrumentation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move all current database-backed `Store` behavior into repository files and instrument every repository entry point with debug-level query logs.

**Architecture:** `Store` remains the API-facing facade that satisfies existing handler interfaces, but each database-backed method becomes a forwarding wrapper to a focused repository. Repositories live in `backend/internal/store`, accept explicit dependencies from `Store`, and use `QueryInstrumentation.Observe` at public repository method boundaries with stable operation names. Transaction list/search dynamic SQL remains parameterized through existing builder helpers moved beside `TransactionRepository`; no raw user input is interpolated into SQL templates.

**Tech Stack:** Go, pgx/v5, slog, existing `backend/internal/store` tests, `task` commands, PostgreSQL testcontainers.

---

## Scope And Ground Rules

This plan intentionally changes the earlier spike direction. The spike recommended incremental migration and leaving transaction list/search until later; this plan implements the user's newer request: one broad migration phase for every current `Store` database method.

In scope:

- Change query instrumentation logs from info to debug.
- Set debug log level explicitly in tests that assert instrumentation output.
- Add focused instrumentation coverage for every repository group.
- Move database-backed `Store` method bodies out of `store.go` into repository files.
- Keep all existing `Store` method signatures unchanged as forwarding methods so API handlers and tests do not need broad rewrites.
- Keep existing SQL parameterization and allowlists.
- Update architecture docs to describe the broad migration result.

Out of scope:

- Adding an ORM.
- Replacing pgx with generated queries.
- Changing HTTP handler interfaces unless a missing `Storer` method is discovered by compile failure.
- Rewriting dynamic transaction filters into Go templates in this phase.

## Repository File Structure

Create these files:

- `backend/internal/store/repository_dependencies.go`
  Defines the shared dependency bundle used by repositories.

- `backend/internal/store/runtime_repository.go`
  Owns app config, active reader, reader runtime JSON, processed messages, sync status, and community URL.

- `backend/internal/store/taxonomy_repository.go`
  Owns labels, label merchant mappings, categories, and buckets. It absorbs the current `label_repository.go` prototype.

- `backend/internal/store/rule_repository.go`
  Owns extraction rule CRUD, predefined rule seeding, and user-rule import.

- `backend/internal/store/diagnostic_repository.go`
  Owns extraction diagnostic records and status transitions.

- `backend/internal/store/transaction_repository.go`
  Owns transaction list/search/detail/edit, labels on transactions, merchant mutes, merchant categorization, and transaction label loading.

- `backend/internal/store/reporting_repository.go`
  Owns stats, dashboard charts, heatmaps, annual spend, monthly breakdowns, facets, and reporting query helpers.

- `backend/internal/store/content_repository.go`
  Owns community content seed/load methods for MCC codes, merchant categories, category snapshots, and MCC categories.

- `backend/internal/store/store_repositories_test.go`
  Adds focused tests that call `Store` facade methods and assert debug instrumentation output for each repository group.

Modify these files:

- `backend/internal/store/store.go`
  Keep shared model types, `Store`, `New`, `Close`, init orchestration, and forwarding methods. Remove database-backed method bodies after moving them to repositories.

- `backend/internal/store/instrumentation.go`
  Log at debug level.

- `backend/internal/store/instrumentation_test.go`
  Configure the test logger at debug level.

- `backend/internal/store/label_repository.go`
  Delete after moving its code into `taxonomy_repository.go`, or keep only if the implementation chooses `TaxonomyRepository` to embed it. Prefer deletion to avoid duplicate taxonomy boundaries.

- `backend/internal/store/label_repository_test.go`
  Replace with taxonomy repository tests or keep and update constructor names if `LabelRepository` is retained as an internal sub-repository. Prefer moving coverage into `store_repositories_test.go`.

- `backend/internal/store/testing.go`
  Keep `PoolForTest` only if repository tests still need direct constructors. Prefer facade-level tests for broad instrumentation.

- `docs/architecture/repository-pattern.md`
  Update migration order and non-goals to reflect the broad migration decision.

- `docs/architecture/repository-spike-results-2026-05-19.md`
  Add a follow-up note that the spike was superseded by this broad migration plan.

## Operation Name Map

Use these exact operation names for instrumentation:

| Repository | Methods | Operation names |
|---|---|---|
| `RuntimeRepository` | `InitAppConfig`, `GetAppConfig`, `SetAppConfig`, `SetActiveReader`, `GetActiveReader`, reader secret/token/config methods, processed message methods, sync status, community URL | `runtime.init_app_config`, `runtime.get_app_config`, `runtime.set_app_config`, `runtime.set_active_reader`, `runtime.get_active_reader`, `runtime.set_reader_secret`, `runtime.get_reader_secret`, `runtime.set_reader_token`, `runtime.get_reader_token`, `runtime.delete_reader_token`, `runtime.set_reader_config`, `runtime.get_reader_config`, `runtime.delete_reader_runtime`, `runtime.is_message_processed`, `runtime.mark_message_processed`, `runtime.get_sync_status`, `runtime.set_sync_status`, `runtime.get_community_url`, `runtime.set_community_url` |
| `TaxonomyRepository` | label CRUD, label merchant mappings, categories, buckets | `taxonomy.init_labels`, `taxonomy.list_labels`, `taxonomy.create_label`, `taxonomy.update_label`, `taxonomy.delete_label`, `taxonomy.apply_label_by_merchant`, `taxonomy.remove_label_by_merchant`, `taxonomy.get_label_mappings`, `taxonomy.init_categories_buckets`, `taxonomy.list_categories`, `taxonomy.create_category`, `taxonomy.delete_category`, `taxonomy.list_buckets`, `taxonomy.create_bucket`, `taxonomy.delete_bucket` |
| `RuleRepository` | rule init, CRUD, seed, import | `rules.init`, `rules.list`, `rules.get`, `rules.create`, `rules.update`, `rules.delete`, `rules.seed_predefined`, `rules.import_user` |
| `DiagnosticRepository` | diagnostics CRUD/status | `diagnostics.record`, `diagnostics.list`, `diagnostics.get`, `diagnostics.update_status` |
| `TransactionRepository` | list/search/detail/edit, transaction labels, mutes, merchant category side effects | `transactions.list`, `transactions.search`, `transactions.get`, `transactions.update_description`, `transactions.add_label`, `transactions.add_labels`, `transactions.remove_label`, `transactions.update`, `transactions.mute`, `transactions.update_mute_reason`, `transactions.update_merchant_reason`, `transactions.mute_by_merchant`, `transactions.categorize_merchant`, `transactions.list_muted_merchants`, `transactions.get_muted_merchants_with_count`, `transactions.delete_muted_merchant`, `transactions.unmute_by_pattern`, `transactions.delete_muted_merchant_and_unmute`, `transactions.get_muted_merchant_patterns` |
| `ReportingRepository` | stats/charts/dashboard/heatmap/facets/monthly breakdown | `reporting.get_stats`, `reporting.get_chart_data`, `reporting.get_dashboard_data`, `reporting.get_spending_heatmap`, `reporting.get_annual_spend`, `reporting.get_facets`, `reporting.get_monthly_breakdown_spend` |
| `ContentRepository` | community content seed/load | `content.seed_mcc_codes`, `content.seed_merchant_categories`, `content.load_category_snapshot`, `content.seed_mcc_categories` |

Private helper methods may remain uninstrumented when they are only called inside an already observed repository method. If a helper is callable from multiple public repository methods and performs a meaningful standalone query, either keep it private and uninstrumented or instrument it with a helper-specific name only if logs would otherwise hide an expensive query. Do not double-wrap every inner helper by default.

## Task 1: Change Instrumentation To Debug With Explicit Test Level

**Files:**
- Modify: `backend/internal/store/instrumentation.go`
- Modify: `backend/internal/store/instrumentation_test.go`

- [ ] **Step 1: Write the failing test**

Update `TestInstrumentRecordsDurationAndError` so it first proves debug must be enabled explicitly:

```go
func TestInstrumentRecordsDebugDurationAndError(t *testing.T) {
	var out bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{Level: slog.LevelDebug}))
	metrics := store.NewQueryInstrumentation(logger)

	err := metrics.Observe(context.Background(), "labels.list", func(context.Context) error {
		return errors.New("boom")
	})

	if err == nil {
		t.Fatal("expected error")
	}
	got := out.String()
	if !strings.Contains(got, "level=DEBUG") {
		t.Fatalf("expected debug log output, got %q", got)
	}
	if !strings.Contains(got, "labels.list") {
		t.Fatalf("expected operation in log output, got %q", got)
	}
	if !strings.Contains(got, "boom") {
		t.Fatalf("expected error in log output, got %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
task test:be
```

Expected: FAIL because the current implementation emits `level=INFO`, not `level=DEBUG`.

- [ ] **Step 3: Implement debug logging**

Change `backend/internal/store/instrumentation.go`:

```go
q.logger.Debug(
	"store query",
	"operation", operation,
	"duration_ms", time.Since(start).Milliseconds(),
	"error", err,
)
```

- [ ] **Step 4: Run backend tests**

Run:

```bash
task test:be
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/store/instrumentation.go backend/internal/store/instrumentation_test.go
git commit --no-gpg-sign -m "chore: log store instrumentation at debug level"
```

## Task 2: Add Store Repository Dependency Wiring

**Files:**
- Create: `backend/internal/store/repository_dependencies.go`
- Modify: `backend/internal/store/store.go`

- [ ] **Step 1: Write the failing compile-time test**

Add this test to `backend/internal/store/store_repositories_test.go`:

```go
package store_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

func TestStoreRepositoriesEmitDebugInstrumentation(t *testing.T) {
	ts := newInstrumentedTestStore(t)
	defer ts.cleanup()

	ctx := context.Background()
	if _, err := ts.ListLabels(ctx); err != nil {
		t.Fatalf("ListLabels: %v", err)
	}

	ts.requireOperation(t, "taxonomy.list_labels")
}

type instrumentedTestStore struct {
	*testStore
	logs *bytes.Buffer
}

func newInstrumentedTestStore(t *testing.T) *instrumentedTestStore {
	t.Helper()
	ts := newTestStoreWithLogger(t, new(bytes.Buffer))
	return &instrumentedTestStore{testStore: ts, logs: ts.logs}
}

func (ts *instrumentedTestStore) requireOperation(t *testing.T, operation string) {
	t.Helper()
	got := ts.logs.String()
	if !strings.Contains(got, "level=DEBUG") {
		t.Fatalf("expected debug logs, got %q", got)
	}
	if !strings.Contains(got, operation) {
		t.Fatalf("expected operation %q in logs, got %q", operation, got)
	}
}
```

Update the existing test helper in `backend/internal/store/store_test.go` by extracting logger construction:

```go
type testStore struct {
	*store.Store
	cleanup func()
	logs    *bytes.Buffer
}

func newTestStore(t *testing.T) *testStore {
	t.Helper()
	return newTestStoreWithLogger(t, nil)
}

func newTestStoreWithLogger(t *testing.T, logs *bytes.Buffer) *testStore {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	// Keep the existing container setup unchanged.
	// Replace only the logger creation before store.New:
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	if logs != nil {
		logger = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	st, err := store.New(cfg, logger)
	// Keep existing error handling and cleanup unchanged.
	return &testStore{
		Store: st,
		logs:  logs,
		cleanup: func() {
			st.Close()
			_ = ctr.Terminate(context.Background())
		},
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
task test:be
```

Expected: FAIL because operation name is currently `labels.list`, not `taxonomy.list_labels`, and repository fields do not exist yet.

- [ ] **Step 3: Add dependency bundle and repository fields**

Create `backend/internal/store/repository_dependencies.go`:

```go
package store

import (
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type repositoryDependencies struct {
	pool    *pgxpool.Pool
	logger  *slog.Logger
	metrics *QueryInstrumentation
	now     func() time.Time
}
```

Modify `Store` in `backend/internal/store/store.go`:

```go
type Store struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
	now    func() time.Time

	runtime      RuntimeRepository
	taxonomy     TaxonomyRepository
	rules        RuleRepository
	diagnostics  DiagnosticRepository
	transactions TransactionRepository
	reporting    ReportingRepository
	content      ContentRepository
}
```

After `s := &Store{pool: pool, logger: logger, now: time.Now}` in `New`, call:

```go
s.initRepositories()
```

Add this method near `New`:

```go
func (s *Store) initRepositories() {
	deps := repositoryDependencies{
		pool:    s.pool,
		logger:  s.logger,
		metrics: NewQueryInstrumentation(s.logger),
		now:     s.now,
	}
	s.runtime = NewRuntimeRepository(deps)
	s.taxonomy = NewTaxonomyRepository(deps)
	s.rules = NewRuleRepository(deps)
	s.diagnostics = NewDiagnosticRepository(deps)
	s.transactions = NewTransactionRepository(deps)
	s.reporting = NewReportingRepository(deps)
	s.content = NewContentRepository(deps)
}
```

The `NewRuntimeRepository`, `NewTaxonomyRepository`, `NewRuleRepository`, `NewDiagnosticRepository`, `NewTransactionRepository`, `NewReportingRepository`, and `NewContentRepository` calls will not compile until repository files exist. In this task, create temporary repository interfaces and stub constructors in each new repository file with only the methods needed by existing forwarding methods as they are moved. Do not commit until all stubs compile.

- [ ] **Step 4: Run backend tests**

Run:

```bash
task test:be
```

Expected: PASS after the existing label repository is absorbed by taxonomy in Task 3. If this task is implemented before Task 3, keep it uncommitted and continue directly into Task 3.

## Task 3: Move Labels, Categories, Buckets, And Label Mappings To TaxonomyRepository

**Files:**
- Create: `backend/internal/store/taxonomy_repository.go`
- Delete: `backend/internal/store/label_repository.go`
- Modify: `backend/internal/store/label_repository_test.go`
- Modify: `backend/internal/store/store.go`

- [ ] **Step 1: Expand failing instrumentation test**

In `TestStoreRepositoriesEmitDebugInstrumentation`, add calls and assertions:

```go
if err := ts.CreateLabel(ctx, "utilities", "#38bdf8"); err != nil {
	t.Fatalf("CreateLabel: %v", err)
}
if err := ts.UpdateLabel(ctx, "utilities", "#f97316"); err != nil {
	t.Fatalf("UpdateLabel: %v", err)
}
if _, err := ts.ApplyLabelByMerchant(ctx, "utilities", "Power Co"); err != nil {
	t.Fatalf("ApplyLabelByMerchant: %v", err)
}
if _, err := ts.GetLabelMappings(ctx); err != nil {
	t.Fatalf("GetLabelMappings: %v", err)
}
if _, err := ts.ListCategories(ctx); err != nil {
	t.Fatalf("ListCategories: %v", err)
}
if err := ts.CreateCategory(ctx, "Test Category", "temporary"); err != nil {
	t.Fatalf("CreateCategory: %v", err)
}
if _, err := ts.ListBuckets(ctx); err != nil {
	t.Fatalf("ListBuckets: %v", err)
}
if err := ts.CreateBucket(ctx, "test-bucket", "temporary"); err != nil {
	t.Fatalf("CreateBucket: %v", err)
}

for _, operation := range []string{
	"taxonomy.list_labels",
	"taxonomy.create_label",
	"taxonomy.update_label",
	"taxonomy.apply_label_by_merchant",
	"taxonomy.get_label_mappings",
	"taxonomy.list_categories",
	"taxonomy.create_category",
	"taxonomy.list_buckets",
	"taxonomy.create_bucket",
} {
	ts.requireOperation(t, operation)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
task test:be
```

Expected: FAIL because taxonomy operations are not all instrumented.

- [ ] **Step 3: Implement TaxonomyRepository**

Create `backend/internal/store/taxonomy_repository.go` with:

```go
type TaxonomyRepository interface {
	InitLabels(ctx context.Context) error
	InitCategoriesBuckets(ctx context.Context) error
	ListLabels(ctx context.Context) ([]Label, error)
	CreateLabel(ctx context.Context, name, color string) error
	UpdateLabel(ctx context.Context, name, color string) error
	DeleteLabel(ctx context.Context, name string) error
	ApplyLabelByMerchant(ctx context.Context, label, pattern string) (int64, error)
	RemoveLabelByMerchant(ctx context.Context, label, pattern string) (int64, error)
	GetLabelMappings(ctx context.Context) (map[string][]string, error)
	ListCategories(ctx context.Context) ([]Category, error)
	CreateCategory(ctx context.Context, name, description string) error
	DeleteCategory(ctx context.Context, name string) error
	ListBuckets(ctx context.Context) ([]Bucket, error)
	CreateBucket(ctx context.Context, name, description string) error
	DeleteBucket(ctx context.Context, name string) error
}
```

Use this implementation shape for every method:

```go
func (r *pgTaxonomyRepository) ListLabels(ctx context.Context) ([]Label, error) {
	labels := []Label{}
	err := r.metrics.Observe(ctx, "taxonomy.list_labels", func(ctx context.Context) error {
		rows, err := r.pool.Query(ctx, `SELECT name, color, created_at FROM labels ORDER BY name`)
		if err != nil {
			return fmt.Errorf("querying labels: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var label Label
			if err := rows.Scan(&label.Name, &label.Color, &label.CreatedAt); err != nil {
				return fmt.Errorf("scanning label: %w", err)
			}
			labels = append(labels, label)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterating labels: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing labels: %w", err)
	}
	return labels, nil
}
```

Move the existing method bodies for labels, label mappings, categories, and buckets from `store.go` into this repository. Preserve SQL exactly unless a test proves a change is needed.

- [ ] **Step 4: Replace Store methods with forwarders**

In `backend/internal/store/store.go`, keep these method signatures and forward:

```go
func (s *Store) ListLabels(ctx context.Context) ([]Label, error) {
	return s.taxonomy.ListLabels(ctx)
}
```

Apply the same forwarding pattern for every method in the `TaxonomyRepository` interface.

- [ ] **Step 5: Run backend tests**

Run:

```bash
task test:be
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/store
git commit --no-gpg-sign -m "refactor: move taxonomy queries into repository"
```

## Task 4: Move Runtime State And App Config To RuntimeRepository

**Files:**
- Create: `backend/internal/store/runtime_repository.go`
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/store/store_repositories_test.go`

- [ ] **Step 1: Add failing instrumentation coverage**

Append to `TestStoreRepositoriesEmitDebugInstrumentation`:

```go
if err := ts.SetAppConfig(ctx, "test_key", "test_value"); err != nil {
	t.Fatalf("SetAppConfig: %v", err)
}
if _, err := ts.GetAppConfig(ctx, "test_key"); err != nil {
	t.Fatalf("GetAppConfig: %v", err)
}
if err := ts.SetActiveReader(ctx, "gmail"); err != nil {
	t.Fatalf("SetActiveReader: %v", err)
}
if _, err := ts.GetActiveReader(ctx); err != nil {
	t.Fatalf("GetActiveReader: %v", err)
}
if err := ts.SetReaderSecret(ctx, "gmail", []byte(`{"installed":{}}`)); err != nil {
	t.Fatalf("SetReaderSecret: %v", err)
}
if _, _, err := ts.GetReaderSecret(ctx, "gmail"); err != nil {
	t.Fatalf("GetReaderSecret: %v", err)
}
if err := ts.SetReaderToken(ctx, "gmail", []byte(`{"access_token":"a"}`)); err != nil {
	t.Fatalf("SetReaderToken: %v", err)
}
if _, _, err := ts.GetReaderToken(ctx, "gmail"); err != nil {
	t.Fatalf("GetReaderToken: %v", err)
}
if err := ts.DeleteReaderToken(ctx, "gmail"); err != nil {
	t.Fatalf("DeleteReaderToken: %v", err)
}
if err := ts.SetReaderConfig(ctx, "thunderbird", json.RawMessage(`{"config":{"mailboxes":"Inbox"}}`)); err != nil {
	t.Fatalf("SetReaderConfig: %v", err)
}
if _, _, err := ts.GetReaderConfig(ctx, "thunderbird"); err != nil {
	t.Fatalf("GetReaderConfig: %v", err)
}
if err := ts.DeleteReaderRuntime(ctx, "thunderbird"); err != nil {
	t.Fatalf("DeleteReaderRuntime: %v", err)
}
if _, err := ts.IsMessageProcessed(ctx, "msg-1"); err != nil {
	t.Fatalf("IsMessageProcessed: %v", err)
}
if err := ts.MarkMessageProcessed(ctx, "msg-1", time.Now()); err != nil {
	t.Fatalf("MarkMessageProcessed: %v", err)
}
if _, err := ts.GetSyncStatus(ctx); err != nil {
	t.Fatalf("GetSyncStatus: %v", err)
}
if err := ts.SetSyncStatus(ctx, store.SyncStatus{LastSyncAt: time.Now()}); err != nil {
	t.Fatalf("SetSyncStatus: %v", err)
}
if _, err := ts.GetCommunityURL(ctx); err != nil {
	t.Fatalf("GetCommunityURL: %v", err)
}
if err := ts.SetCommunityURL(ctx, "https://example.com/community.json"); err != nil {
	t.Fatalf("SetCommunityURL: %v", err)
}

for _, operation := range []string{
	"runtime.set_app_config",
	"runtime.get_app_config",
	"runtime.set_active_reader",
	"runtime.get_active_reader",
	"runtime.set_reader_secret",
	"runtime.get_reader_secret",
	"runtime.set_reader_token",
	"runtime.get_reader_token",
	"runtime.delete_reader_token",
	"runtime.set_reader_config",
	"runtime.get_reader_config",
	"runtime.delete_reader_runtime",
	"runtime.is_message_processed",
	"runtime.mark_message_processed",
	"runtime.get_sync_status",
	"runtime.set_sync_status",
	"runtime.get_community_url",
	"runtime.set_community_url",
} {
	ts.requireOperation(t, operation)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
task test:be
```

Expected: FAIL because runtime operations are not instrumented.

- [ ] **Step 3: Implement RuntimeRepository**

Create `RuntimeRepository` with methods matching the operation map. Move existing method bodies from `store.go`:

- `initAppConfig`
- `GetAppConfig`
- `SetAppConfig`
- `SetActiveReader`
- `GetActiveReader`
- all reader secret/token/config methods
- `DeleteReaderRuntime`
- `IsMessageProcessed`
- `MarkMessageProcessed`
- private `setReaderJSON`
- private `getReaderJSON`
- `GetSyncStatus`
- `SetSyncStatus`
- `GetCommunityURL`
- `SetCommunityURL`

Preserve the reader JSON column allowlist exactly:

```go
switch column {
case "client_secret_json", "token_json", "config_json":
default:
	return fmt.Errorf("invalid reader runtime column %q", column)
}
```

Wrap public repository methods with `metrics.Observe`. Keep private `setReaderJSON` and `getReaderJSON` uninstrumented because callers already use operation-specific wrappers.

- [ ] **Step 4: Forward Store methods**

Replace each moved `Store` body with:

```go
func (s *Store) GetAppConfig(ctx context.Context, key string) (string, error) {
	return s.runtime.GetAppConfig(ctx, key)
}
```

Apply the same pattern for every runtime method.

- [ ] **Step 5: Run backend tests and commit**

```bash
task test:be
git add backend/internal/store
git commit --no-gpg-sign -m "refactor: move runtime queries into repository"
```

Expected: tests PASS before commit.

## Task 5: Move Rules To RuleRepository

**Files:**
- Create: `backend/internal/store/rule_repository.go`
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/store/store_repositories_test.go`

- [ ] **Step 1: Add failing instrumentation coverage**

Add calls to create, list, get, update, seed, import, and delete rules using existing `RuleRow` fields from current tests. Assert operations:

```go
for _, operation := range []string{
	"rules.list",
	"rules.create",
	"rules.get",
	"rules.update",
	"rules.seed_predefined",
	"rules.import_user",
	"rules.delete",
} {
	ts.requireOperation(t, operation)
}
```

Use concrete test data:

```go
rule := store.RuleRow{
	Name:              "Test Rule",
	SenderEmail:       "alerts@example.com",
	SubjectContains:   "spent",
	AmountRegex:       `INR\s+([0-9.]+)`,
	MerchantRegex:     `at\s+(.+)$`,
	CurrencyRegex:     `INR`,
	TransactionSource: "Test Source",
}
```

- [ ] **Step 2: Run test to verify it fails**

Run `task test:be`.

Expected: FAIL because rule repository instrumentation is absent.

- [ ] **Step 3: Implement RuleRepository**

Move these methods from `store.go`:

- `initRules`
- `ListRules`
- `GetRule`
- `CreateRule`
- `UpdateRule`
- `DeleteRule`
- `SeedPredefinedRules`
- `ImportUserRules`

Keep existing helpers used only by rules in `rule_repository.go`, including rule scanning SQL fragments if they are local to the moved methods.

- [ ] **Step 4: Forward Store methods**

Replace moved `Store` bodies with calls to `s.rules`.

- [ ] **Step 5: Run backend tests and commit**

```bash
task test:be
git add backend/internal/store
git commit --no-gpg-sign -m "refactor: move rule queries into repository"
```

Expected: PASS.

## Task 6: Move Diagnostics To DiagnosticRepository

**Files:**
- Create: `backend/internal/store/diagnostic_repository.go`
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/store/store_repositories_test.go`

- [ ] **Step 1: Add failing instrumentation coverage**

Use an `api.ExtractionDiagnostic` fixture and call:

```go
diagnostic := api.ExtractionDiagnostic{
	MessageID: "diag-message",
	Reader:    "gmail",
	Sender:    "alerts@example.com",
	Subject:   "Payment alert",
	Body:      "spent INR 123",
	Reason:    "no_matching_rule",
	Status:    "open",
	CreatedAt: time.Now(),
}
if err := ts.RecordExtractionDiagnostic(ctx, diagnostic); err != nil {
	t.Fatalf("RecordExtractionDiagnostic: %v", err)
}
rows, err := ts.ListExtractionDiagnostics(ctx, store.DiagnosticFilter{Status: "open", Limit: 10})
if err != nil {
	t.Fatalf("ListExtractionDiagnostics: %v", err)
}
if len(rows) == 0 {
	t.Fatal("expected at least one diagnostic")
}
if _, err := ts.GetExtractionDiagnostic(ctx, rows[0].ID); err != nil {
	t.Fatalf("GetExtractionDiagnostic: %v", err)
}
if _, err := ts.UpdateExtractionDiagnosticStatus(ctx, rows[0].ID, "ignored"); err != nil {
	t.Fatalf("UpdateExtractionDiagnosticStatus: %v", err)
}
```

Assert:

```go
for _, operation := range []string{
	"diagnostics.record",
	"diagnostics.list",
	"diagnostics.get",
	"diagnostics.update_status",
} {
	ts.requireOperation(t, operation)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run `task test:be`.

Expected: FAIL because diagnostics operations are not instrumented.

- [ ] **Step 3: Implement DiagnosticRepository**

Move diagnostic methods and their scan helper logic from `store.go` into `diagnostic_repository.go`. Keep dynamic diagnostic filters parameterized exactly as they are.

- [ ] **Step 4: Forward Store methods**

Replace moved `Store` bodies with calls to `s.diagnostics`.

- [ ] **Step 5: Run backend tests and commit**

```bash
task test:be
git add backend/internal/store
git commit --no-gpg-sign -m "refactor: move diagnostic queries into repository"
```

Expected: PASS.

## Task 7: Move Community Content To ContentRepository

**Files:**
- Create: `backend/internal/store/content_repository.go`
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/store/store_repositories_test.go`

- [ ] **Step 1: Add failing instrumentation coverage**

Call:

```go
if err := ts.SeedMCCCodes(ctx, []store.MCCEntry{{Code: "5411", Description: "Grocery Stores"}}); err != nil {
	t.Fatalf("SeedMCCCodes: %v", err)
}
if _, err := ts.SeedMerchantCategories(ctx, []store.MerchantCategoryEntry{{
	MerchantPattern: "Test Grocery",
	Category:        "Food",
	Bucket:          "needs",
}}); err != nil {
	t.Fatalf("SeedMerchantCategories: %v", err)
}
if _, err := ts.LoadCategorySnapshot(ctx); err != nil {
	t.Fatalf("LoadCategorySnapshot: %v", err)
}
if err := ts.SeedMCCCategories(ctx, []string{"Food"}); err != nil {
	t.Fatalf("SeedMCCCategories: %v", err)
}
```

Assert:

```go
for _, operation := range []string{
	"content.seed_mcc_codes",
	"content.seed_merchant_categories",
	"content.load_category_snapshot",
	"content.seed_mcc_categories",
} {
	ts.requireOperation(t, operation)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run `task test:be`.

Expected: FAIL because content operations are not instrumented.

- [ ] **Step 3: Implement ContentRepository**

Move these methods from `store.go`:

- `SeedMCCCodes`
- `SeedMerchantCategories`
- `LoadCategorySnapshot`
- `SeedMCCCategories`

Move content-only helper structs or helper functions if needed. Leave shared public DTO types in `store.go` unless moving them avoids import cycles.

- [ ] **Step 4: Forward Store methods**

Replace moved `Store` bodies with calls to `s.content`.

- [ ] **Step 5: Run backend tests and commit**

```bash
task test:be
git add backend/internal/store
git commit --no-gpg-sign -m "refactor: move content queries into repository"
```

Expected: PASS.

## Task 8: Move Reporting To ReportingRepository

**Files:**
- Create: `backend/internal/store/reporting_repository.go`
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/store/store_repositories_test.go`

- [ ] **Step 1: Add failing instrumentation coverage**

Seed at least one transaction using existing `InsertForTest`, then call:

```go
_, err := ts.GetStats(ctx, "INR")
if err != nil {
	t.Fatalf("GetStats: %v", err)
}
if _, err := ts.GetChartData(ctx); err != nil {
	t.Fatalf("GetChartData: %v", err)
}
if _, err := ts.GetDashboardData(ctx); err != nil {
	t.Fatalf("GetDashboardData: %v", err)
}
if _, err := ts.GetSpendingHeatmap(ctx, nil, nil); err != nil {
	t.Fatalf("GetSpendingHeatmap: %v", err)
}
if _, err := ts.GetAnnualSpend(ctx, time.Now().Year()); err != nil {
	t.Fatalf("GetAnnualSpend: %v", err)
}
if _, err := ts.GetFacets(ctx); err != nil {
	t.Fatalf("GetFacets: %v", err)
}
if _, err := ts.GetMonthlyBreakdownSpend(ctx, "category", 3); err != nil {
	t.Fatalf("GetMonthlyBreakdownSpend: %v", err)
}
```

Assert:

```go
for _, operation := range []string{
	"reporting.get_stats",
	"reporting.get_chart_data",
	"reporting.get_dashboard_data",
	"reporting.get_spending_heatmap",
	"reporting.get_annual_spend",
	"reporting.get_facets",
	"reporting.get_monthly_breakdown_spend",
} {
	ts.requireOperation(t, operation)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run `task test:be`.

Expected: FAIL because reporting operations are not instrumented.

- [ ] **Step 3: Implement ReportingRepository**

Move reporting methods and private reporting helpers from `store.go`:

- `GetStats`
- `GetChartData`
- `getChartDataAt`
- `GetDashboardData`
- `getStatsBetween`
- `getChartDataBetween`
- `queryCategoryMonthly`
- `queryCategoryMonthlyAt`
- `queryCategoryMonthlyBetween`
- `GetSpendingHeatmap`
- `GetAnnualSpend`
- `GetFacets`
- `GetMonthlyBreakdownSpend`
- `queryTimeBuckets`
- `queryStringFloat`
- `appTimezone`
- `dashboardBaseCurrency`
- `dashboardMonthBounds`

Keep non-query helper functions private to `reporting_repository.go` when only reporting uses them.

- [ ] **Step 4: Forward Store methods**

Replace exported moved `Store` bodies with calls to `s.reporting`.

- [ ] **Step 5: Run backend tests and commit**

```bash
task test:be
git add backend/internal/store
git commit --no-gpg-sign -m "refactor: move reporting queries into repository"
```

Expected: PASS.

## Task 9: Move Transactions And Muting To TransactionRepository

**Files:**
- Create: `backend/internal/store/transaction_repository.go`
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/store/store_repositories_test.go`

- [ ] **Step 1: Add failing instrumentation coverage**

Seed transactions with `InsertForTest`, then call:

```go
id, err := ts.InsertForTest(ctx, store.InsertParams{
	MessageID:    "repo-txn-1",
	Amount:       100,
	Currency:     "INR",
	MerchantInfo: "Repository Shop",
	Category:     "Shopping",
	Timestamp:    time.Now(),
})
if err != nil {
	t.Fatalf("InsertForTest: %v", err)
}
if _, _, err := ts.ListTransactions(ctx, store.ListFilter{Page: 1, PageSize: 10}); err != nil {
	t.Fatalf("ListTransactions: %v", err)
}
if _, _, err := ts.SearchTransactions(ctx, "Repository", store.ListFilter{Page: 1, PageSize: 10}); err != nil {
	t.Fatalf("SearchTransactions: %v", err)
}
if _, err := ts.GetTransaction(ctx, id); err != nil {
	t.Fatalf("GetTransaction: %v", err)
}
if err := ts.UpdateDescription(ctx, id, "updated"); err != nil {
	t.Fatalf("UpdateDescription: %v", err)
}
if err := ts.AddLabel(ctx, id, "reviewed"); err != nil {
	t.Fatalf("AddLabel: %v", err)
}
if err := ts.AddLabels(ctx, id, []string{"tax", "manual"}); err != nil {
	t.Fatalf("AddLabels: %v", err)
}
if err := ts.RemoveLabel(ctx, id, "tax"); err != nil {
	t.Fatalf("RemoveLabel: %v", err)
}
if err := ts.UpdateTransaction(ctx, id, store.TransactionUpdate{Description: ptrString("final")}); err != nil {
	t.Fatalf("UpdateTransaction: %v", err)
}
if err := ts.MuteTransaction(ctx, id, true, "test mute"); err != nil {
	t.Fatalf("MuteTransaction: %v", err)
}
if err := ts.UpdateMuteReason(ctx, id, "updated reason"); err != nil {
	t.Fatalf("UpdateMuteReason: %v", err)
}
if err := ts.MuteByMerchant(ctx, "Repository Shop", "merchant mute"); err != nil {
	t.Fatalf("MuteByMerchant: %v", err)
}
if _, err := ts.CategorizeMerchant(ctx, "Repository Shop", "Shopping", "wants"); err != nil {
	t.Fatalf("CategorizeMerchant: %v", err)
}
if _, err := ts.ListMutedMerchants(ctx); err != nil {
	t.Fatalf("ListMutedMerchants: %v", err)
}
if _, err := ts.GetMutedMerchantsWithCount(ctx); err != nil {
	t.Fatalf("GetMutedMerchantsWithCount: %v", err)
}
if _, err := ts.GetMutedMerchantPatterns(ctx); err != nil {
	t.Fatalf("GetMutedMerchantPatterns: %v", err)
}
```

Add helper in the test file:

```go
func ptrString(v string) *string {
	return &v
}
```

Assert:

```go
for _, operation := range []string{
	"transactions.list",
	"transactions.search",
	"transactions.get",
	"transactions.update_description",
	"transactions.add_label",
	"transactions.add_labels",
	"transactions.remove_label",
	"transactions.update",
	"transactions.mute",
	"transactions.update_mute_reason",
	"transactions.mute_by_merchant",
	"transactions.categorize_merchant",
	"transactions.list_muted_merchants",
	"transactions.get_muted_merchants_with_count",
	"transactions.get_muted_merchant_patterns",
} {
	ts.requireOperation(t, operation)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run `task test:be`.

Expected: FAIL because transaction operations are not instrumented.

- [ ] **Step 3: Implement TransactionRepository**

Move these methods and helpers from `store.go`:

- `queryTransactionTotals`
- `ListTransactions`
- `GetTransaction`
- `UpdateDescription`
- `AddLabel`
- `AddLabels`
- `RemoveLabel`
- `SearchTransactions`
- `UpdateTransaction`
- `MuteTransaction`
- `UpdateMuteReason`
- `UpdateMerchantReason`
- `MuteByMerchant`
- `CategorizeMerchant`
- `ListMutedMerchants`
- `GetMutedMerchantsWithCount`
- `DeleteMutedMerchant`
- `UnmuteByPattern`
- `DeleteMutedMerchantAndUnmute`
- `GetMutedMerchantPatterns`
- `loadLabels`
- `buildListWhere`
- `joinLabel`
- `scanTransactions`
- transaction-list helper functions used only by the moved methods

Preserve all current dynamic SQL safeguards:

- Keep parameter placeholders generated through the existing `next(...)` helper pattern.
- Keep sort column and sort direction allowlists.
- Keep JSON/column allowlists unchanged.
- Do not introduce text/template for transaction filters in this task.

- [ ] **Step 4: Forward Store methods**

Replace exported moved `Store` bodies with calls to `s.transactions`.

- [ ] **Step 5: Run backend tests and commit**

```bash
task test:be
git add backend/internal/store
git commit --no-gpg-sign -m "refactor: move transaction queries into repository"
```

Expected: PASS.

## Task 10: Remove Residual Store SQL And Enforce Facade Boundary

**Files:**
- Modify: `backend/internal/store/store.go`
- Modify: repository files as needed

- [ ] **Step 1: Add static guard test**

Create `backend/internal/store/store_facade_test.go` in package `store_test`:

```go
package store_test

import (
	"os"
	"strings"
	"testing"
)

func TestStoreFacadeDoesNotContainDatabaseQueries(t *testing.T) {
	data, err := os.ReadFile("store.go")
	if err != nil {
		t.Fatalf("ReadFile store.go: %v", err)
	}
	source := string(data)
	for _, forbidden := range []string{".Query(", ".QueryRow(", ".Exec("} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("store.go should be a repository facade; found %s", forbidden)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails if SQL remains**

Run:

```bash
task test:be
```

Expected: FAIL if any direct query calls remain in `store.go`; PASS only after all database-backed methods are moved.

- [ ] **Step 3: Move remaining SQL**

Run:

```bash
rg -n "\\.(Query|QueryRow|Exec)\\(" backend/internal/store/store.go
```

Expected after cleanup: no output.

If output remains, move the containing method to the correct repository and keep the `Store` method as a forwarding wrapper.

- [ ] **Step 4: Run backend tests and commit**

```bash
task test:be
git add backend/internal/store
git commit --no-gpg-sign -m "test: enforce store repository facade"
```

Expected: PASS.

## Task 11: Update Architecture Docs

**Files:**
- Modify: `docs/architecture/repository-pattern.md`
- Modify: `docs/architecture/repository-spike-results-2026-05-19.md`
- Modify: `docs/superpowers/plans/2026-05-19-open-work-index.md`

- [ ] **Step 1: Update repository pattern doc**

Change the migration section to state that the broad migration is complete after this plan, and keep a future note that transaction query templating/query-builder redesign remains separate.

Use this wording:

```markdown
## Migration State

`Store` is the API-facing facade. Database-backed behavior lives in focused repositories:

- `RuntimeRepository`
- `TaxonomyRepository`
- `RuleRepository`
- `DiagnosticRepository`
- `TransactionRepository`
- `ReportingRepository`
- `ContentRepository`

Every public repository operation is wrapped by debug-level query instrumentation with a stable operation name.

## Remaining Work

Transaction list/search still uses parameterized dynamic SQL builders. A separate query-template/query-builder design is required before changing that SQL construction model.
```

- [ ] **Step 2: Update spike results**

Add:

```markdown
## Follow-Up 2026-05-19

The original incremental migration recommendation was superseded by the broad repository instrumentation migration. The core spike finding still applies: transaction query construction remains a separate design problem from repository placement and instrumentation.
```

- [ ] **Step 3: Update open work index**

If this plan is implemented immediately, add it to completed feedback triage work. If it is only written and not implemented, do not mark it complete.

- [ ] **Step 4: Commit docs**

```bash
git add docs/architecture/repository-pattern.md docs/architecture/repository-spike-results-2026-05-19.md docs/superpowers/plans/2026-05-19-open-work-index.md
git commit --no-gpg-sign -m "docs: update repository migration status"
```

## Task 12: Final Verification

**Files:**
- No planned source edits unless verification finds issues.

- [ ] **Step 1: Format backend**

Run:

```bash
task fmt:be
```

Expected: PASS and no unexpected file churn outside `backend/internal/store`.

- [ ] **Step 2: Run production backend lint**

Run:

```bash
task lint:be:prod
```

Expected: `0 issues.`

- [ ] **Step 3: Run backend tests**

Run:

```bash
task test:be
```

Expected: PASS.

- [ ] **Step 4: Inspect remaining Store SQL**

Run:

```bash
rg -n "\\.(Query|QueryRow|Exec)\\(" backend/internal/store/store.go
```

Expected: no output.

- [ ] **Step 5: Inspect instrumentation coverage**

Run:

```bash
rg -n "metrics\\.Observe\\(" backend/internal/store
```

Expected: repository files contain observe calls for every operation in the operation name map.

- [ ] **Step 6: Inspect working tree**

Run:

```bash
git status --short
```

Expected: no output.

If formatting changed files after the last commit, commit them:

```bash
git add backend/internal/store docs/architecture docs/superpowers/plans
git commit --no-gpg-sign -m "chore: finalize repository instrumentation migration"
```

## Self-Review

- Spec coverage: the plan changes instrumentation to debug, explicitly configures debug tests, moves every current database-backed `Store` method into a repository, keeps facade method signatures, and adds verification that `store.go` has no direct pgx query calls.
- Placeholder scan: no task relies on unspecified future behavior. Transaction query templating is explicitly out of scope and preserved as a separate future design.
- Type consistency: repository names and operation names are fixed in the operation map and reused in tests and implementation steps.
