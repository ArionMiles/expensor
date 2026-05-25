# Backend Observability and Code Health — Design Spec

**Date:** 2026-05-24
**Status:** Observability/store instrumentation slice complete; broader backend code-health work remains.

---

## Goal

Improve backend maintainability before adding new reader/search workflows. The refactor should make backend abstractions easier to understand, reduce feature accretion around overloaded files and interfaces, and introduce first-class observability for local self-hosted operation.

The work has two equal outcomes:

1. Better backend code health: smaller ownership boundaries, clearer interfaces, less inline instrumentation noise, and safer extension points.
2. Updated `AGENTS.md` guidance so future agentic sessions do not repeat the same architectural mistakes.

## Program Status

| Slice | Status | Notes |
|-------|--------|-------|
| Observability package and store instrumentation | Complete | Implemented in `pr/backend-observability-code-health`; see `docs/superpowers/plans/2026-05-24-observability-store-instrumentation.md`. |
| Plugin registry cleanup | Pending | Requires a separate implementation plan. |
| API handler decomposition | Pending | Requires a separate implementation plan. |
| Store package ownership cleanup beyond instrumentation | Pending | Remaining work includes read-model ownership and `store.go` model/helper split. |
| Processed-message state context cleanup | Pending | Requires a separate implementation plan. |
| Provider-specific domain cleanup | Pending | Includes moving Gmail query construction out of `pkg/api`. |

---

## Non-goals

- Do not implement email search or rule workbench search in this refactor.
- Do not replace `slog` with OpenTelemetry logs.
- Do not require an OpenTelemetry collector for normal self-hosted use.
- Do not change user-facing API behavior except where a refactor exposes an existing bug that tests capture first.
- Do not rewrite the whole backend. Prefer targeted, test-backed slices.

---

## Current Problems

### Plugin Registry Drift

`backend/internal/plugins/registry.go` is both a plugin catalog and an application factory. `ReaderPlugin.NewReader` has a long flattened argument list with HTTP client, global config, rules, category resolver, processed-message state, diagnostic sink, and logger. `GuideProvider` and `ConfigApplier` were added as optional interfaces even though setup guides are required metadata and persisted config decoding is a core reader concern.

This makes adding a new reader or search capability harder because every new concern is likely to become another optional interface or constructor argument.

### Store Instrumentation Is Inline

`internal/store` currently wraps DB operations by calling `QueryInstrumentation.Observe` inside each repository method. This scatters observability mechanics through concrete repository code. The repositories should express database behavior; instrumentation should wrap repository/store interfaces from the outside.

### Store Repository Split Is Incomplete

`read_model_repository.go` mostly delegates to functions still implemented in `store.go` via a `legacy *Store` back-reference. `store.go` also contains shared models, helper scanners, read-model query implementations, facade methods, and sentinel errors. The package has repository names, but ownership is still blurred.

### API Handler File Is Overloaded

`internal/api/handlers.go` combines OpenAPI doc structs, reader setup and OAuth, daemon control, transactions, diagnostics, taxonomy, rules, dashboard/config endpoints, sync, and helper conversion logic. New features are easy to bolt onto the file because everything is already nearby.

### Domain Package Contains Provider-Specific Logic

`pkg/api.Rule` contains Gmail query construction. The core API package should not know Gmail syntax. Reader-specific behavior belongs in reader packages.

### Processed Message State Ignores Caller Context

`pkg/state.Manager` supports both legacy file state and DB-backed state in one type and uses `context.Background()` internally for DB calls. Daemon cancellation should propagate into state calls.

---

## Target Architecture

### Observability Package

Rename `backend/pkg/logging` to `backend/pkg/observability`.

`observability` owns:

- `slog` setup and default logger installation.
- OpenTelemetry tracer provider setup.
- OpenTelemetry meter provider setup.
- Optional OTLP exporter for traces and metrics.
- Shutdown flushing.
- Helpers for attaching trace/span IDs to logs.
- Shared low-cardinality attribute conventions.

`slog` remains the only logging API and normal log output path. We will not configure OTel log export.

Default behavior:

- Logs go to stderr as they do today.
- Tracing and metrics are disabled unless explicitly enabled.
- No OTel collector is required.

Configuration:

- Preserve `LOG_LEVEL` compatibility.
- Add `EXPENSOR_OBSERVABILITY_ENABLED`.
- Add `EXPENSOR_OBSERVABILITY_EXPORTER` with values `none` and `otlp`.
- Add `EXPENSOR_OBSERVABILITY_OTLP_ENDPOINT`.
- Add signal toggles only if needed for clarity: `EXPENSOR_TRACES_ENABLED`, `EXPENSOR_METRICS_ENABLED`.
- Standard `OTEL_*` environment variables may be honored where the OTel SDK already supports them, but Expensor's own config must remain explicit and documented.

### Store and Repository Decorators

Move instrumentation out of concrete repository methods.

Define capability interfaces in `internal/store` for meaningful boundaries:

- `TransactionStore`
- `ReadModelStore`
- `RuleStore`
- `RuntimeStore`
- `TaxonomyStore`
- `CommunityStore`
- `DiagnosticStore`

`*store.Store` remains the concrete facade used by application code and should satisfy these capability interfaces.

Create instrumentation decorators that implement the same interfaces and delegate to `next`:

```go
type InstrumentedTransactions struct {
    next   TransactionStore
    obs    *observability.Scope
    logger *slog.Logger
}

func (i *InstrumentedTransactions) ListTransactions(ctx context.Context, f ListFilter) ([]Transaction, TransactionListResult, error) {
    ctx, span := i.obs.Start(ctx, "store.transactions.list")
    defer span.End()

    txns, result, err := i.next.ListTransactions(ctx, f)
    i.obs.RecordOperation(ctx, observability.Operation{
        Namespace: "store",
        Name:      "transactions.list",
        Err:       err,
    })
    return txns, result, err
}
```

Instrumentation records:

- A span per store operation.
- An operation counter tagged with status.
- Error logging through `slog`, using trace/span IDs when present.

Implementation note from the completed observability slice: store decorators should start spans inline in the instrumented method, call `next` directly, and then record the operation result. Do not hide the delegated call inside callback helpers like `observe1` or `observe2`. Store operation duration is represented by the span; do not add separate `time.Now()` timing unless a future metrics requirement explicitly needs a histogram.

Store operation errors must be sanitized before entering OTel data. Do not call `span.RecordError` with raw store errors or use `err.Error()` as a span status description; use a stable low-cardinality class such as `error` for trace status and keep raw error text in `slog` only.

Use low-cardinality metrics attributes only:

- `repository`
- `operation`
- `status`

Avoid sensitive or high-cardinality attributes:

- transaction IDs
- message IDs
- merchant names
- sender emails
- rule names
- raw SQL
- raw error strings
- span events or status descriptions derived from raw errors
- email bodies or snippets

Composition should happen at wiring boundaries. Concrete repositories should not import or call observability helpers.

### Plugin Registry Cleanup

Keep the registry as a catalog. It should register and return plugins by name; it should not hide application assembly behind `CreateReader` or `CreateWriter`.

Replace scattered metadata methods with explicit metadata structs:

```go
type AuthSpec struct {
    Type                      AuthType
    RequiredScopes            []string
    RequiresCredentialsUpload bool
}

type ReaderMetadata struct {
    Name         string
    Description  string
    Auth         AuthSpec
    ConfigSchema []ConfigField
    SetupGuide   json.RawMessage
}
```

Reader plugins expose required metadata directly. `GuideProvider` should be removed because setup guides are required. `ConfigApplier` should be removed because reader-specific config should be decoded by the reader/plugin construction path instead of mutating global `config.Config`.

Reader construction should use one input struct if kept on plugin types:

```go
type ReaderInput struct {
    HTTPClient     *http.Client
    AppConfig      config.Config
    ReaderConfig   json.RawMessage
    Rules          []api.Rule
    Resolver       api.CategoryResolver
    State          ProcessedMessageState
    DiagnosticSink api.DiagnosticSink
    Logger         *slog.Logger
}
```

The exact type names can change during planning. The core requirement is to remove long flattened constructors and remove optional interfaces for required behavior.

### API Handler Decomposition

Split `internal/api/handlers.go` by resource:

- `handlers_readers.go`
- `handlers_daemon.go`
- `handlers_transactions.go`
- `handlers_diagnostics.go`
- `handlers_taxonomy.go`
- `handlers_rules.go`
- `handlers_config.go`
- `handlers_sync.go`
- `openapi_types.go`
- `http_helpers.go`

Keep handler behavior stable. The main improvement is ownership clarity and simpler future changes.

Add small HTTP helpers where they reduce real duplication:

- store availability guard
- JSON decode with max body size where relevant
- UUID parse and error response
- common `ErrNotFound` and conflict mapping
- query parsing for pagination/date filters

### Store Package Cleanup

Move model and helper definitions out of `store.go` into focused files:

- `models.go`
- `filters.go`
- `errors.go`
- `scan.go`

Move all read-model implementation into `read_model_repository.go` or focused read-model files. Remove `legacy *Store` from `pgReadModelRepository`; inject the dependencies it actually needs, such as pool, runtime config reader, clock, and metrics/observability wrapper.

Inside `internal/store`, prefer concrete private repository fields over interfaces unless there is a second implementation or a consumer boundary requires an interface.

### State Cleanup

Introduce a context-aware processed-message state interface:

```go
type ProcessedMessageState interface {
    IsProcessed(ctx context.Context, key string) bool
    MarkProcessed(ctx context.Context, key string) error
}
```

Readers should pass their daemon context to state calls. Legacy file-backed state can remain while needed, but DB-backed state should not call the store with `context.Background()`.

### Domain Boundary Cleanup

Move Gmail query construction out of `pkg/api.Rule` into the Gmail reader package. Keep core rule data and provider-neutral matching logic only if it is used by more than one reader.

Treat `pkg` as an application package area unless a package is deliberately reusable. If concrete readers/writers continue to import internal app packages, document that `pkg` is not a public reusable SDK boundary, or move concrete implementations under `internal`.

---

## Observability Instrumentation Targets

Implement traces and metrics for these boundaries:

| Area | Trace | Metrics |
|------|-------|---------|
| HTTP middleware | request span with method, route, status | request count and duration |
| Store decorators | one span per operation | operation count |
| Daemon runner | run lifecycle span | daemon starts/stops/errors |
| Gmail reader | scan iteration, list/get message spans | messages found, skipped, extraction failures |
| Thunderbird reader | mailbox scan spans | mailboxes scanned, messages scanned/skipped |
| Postgres writer | batch write span | batch size, write duration, write failures |
| Diagnostics | record diagnostic span | diagnostics recorded/skipped/errors |
| Community sync | sync span | sync duration, entries updated, sync errors |

Sensitive email contents must never be added to metrics or span attributes.

---

## Testing Strategy

Use TDD for implementation slices.

Focused tests:

- `observability` config parsing and no-op/default behavior.
- Trace/span ID enrichment behavior for logs.
- Store instrumentation decorators call `next`, propagate return values, record success/error status, and do not swallow errors.
- Handler split preserves existing handler tests.
- Plugin metadata refactor preserves reader listing, auth status, setup guide, daemon startup, and plugin registration behavior.
- State cleanup propagates caller context through DB-backed state.

Verification commands:

- `task test:be`
- `task lint:be:prod`
- `task openapi:check` if handler/OpenAPI type movement changes generated output or annotations.

Run component or contract tests only for slices that touch store behavior, daemon wiring, or OpenAPI behavior.

---

## Implementation Order

1. Introduce `pkg/observability` and migrate existing logging tests/imports. **Complete.**
2. Add OTel trace/metric setup and no-op/default behavior. **Complete.**
3. Replace store inline `QueryInstrumentation` with interface decorators. **Complete.**
4. Split store capability interfaces and clean up `store.go` ownership. **Partially complete for instrumentation boundaries; remaining ownership cleanup is deferred.**
5. Refactor plugin metadata and construction boundary.
6. Split `internal/api/handlers.go` by resource without behavior changes.
7. Make processed-message state context-aware.
8. Move Gmail-specific rule query construction out of `pkg/api`.
9. Update `AGENTS.md` with backend code-health rules.

The exact implementation plan may split these further, but each slice should be independently testable.

---

## AGENTS.md Guidance To Add

Add a backend architecture section with these rules:

- Do not add optional plugin interfaces for required metadata. If every reader must provide it, put it in the main metadata struct.
- Do not add long constructor signatures. Use a small input/deps struct once a constructor exceeds 4-5 parameters.
- `Registry` is a catalog, not an application assembler. Daemon/runtime wiring belongs outside the registry.
- Define interfaces at consumer boundaries. Do not create package-local interfaces beside a single implementation unless a decorator or test boundary needs it.
- Do not call instrumentation helpers from inside repository implementations. Wrap store/repository interfaces with decorators that own logging, metrics, and tracing.
- Keep concrete Postgres repositories focused on database behavior.
- Do not leave new repository implementations in `store.go`; new store behavior must live in the owning repository file.
- Do not add provider-specific helpers to `pkg/api`; Gmail/Thunderbird-specific behavior belongs in that reader package.
- Do not use `context.Background()` inside request or daemon paths when a caller context is available.
- Keep `slog` as the application logging API. Use OpenTelemetry for traces and metrics, not log export.
- Do not put high-cardinality or sensitive values in metrics or trace attributes, including email bodies, snippets, sender addresses, message IDs, transaction IDs, merchant names, and raw SQL.
- When a file grows because a new feature was convenient to bolt on, split by ownership before adding the feature.

---

## Open Questions For Implementation Planning

1. Should concrete reader and writer implementations move from `backend/pkg` to `backend/internal`, or should we document `pkg` as app-internal despite the name?
2. Should HTTP middleware use `otelhttp`, manual spans, or a small wrapper that preserves the existing mux route names?
3. Should store instrumentation wrap one facade interface or multiple capability interfaces first? The recommendation is capability interfaces to avoid an unmaintainable giant decorator.
