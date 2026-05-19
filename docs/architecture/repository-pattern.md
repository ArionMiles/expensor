# Repository Pattern Direction

## Boundaries

- `TransactionRepository`: transaction list/search/detail/update/mute/label joins and merchant-wide transaction side effects.
- `ReportingRepository`: dashboard stats, charts, heatmaps, annual spend, monthly breakdowns, and facets when used only for reporting/filter metadata.
- `TaxonomyRepository`: labels, categories, buckets, and label mappings. A temporary `LabelRepository` may exist during migration while label CRUD is extracted first.
- `RuleRepository`: extraction rules, predefined rule seeding, and user-rule imports.
- `RuntimeRepository`: app config, active reader, reader runtime state, processed messages, sync status, and community URL.
- `DiagnosticRepository`: extraction diagnostic records and status transitions.
- `ContentRepository`: community content sync, MCC codes, merchant category mappings, and category resolver snapshots.

## Rules

- Handlers depend on small interfaces, not concrete repositories.
- `Store` may keep forwarding methods during migration so the API `Storer` interface changes only when a handler needs a new behavior.
- Dynamic query builders return SQL fragments plus parameter arrays.
- User input is never interpolated into SQL.
- Templates may only vary trusted structural clauses such as allowlisted sort columns, allowlisted JSON columns, or fixed optional joins.
- Every repository method wraps errors with operation context.
- Instrumentation wraps repository operations using stable names such as `labels.list` or `transactions.search`.
- Repository constructors accept dependencies explicitly: connection pool, logger/instrumentation, and clock only when needed.

## Migration Order

1. Label CRUD prototype.
2. Runtime state and app config.
3. Categories, buckets, and rules.
4. Diagnostics and community content.
5. Reporting.
6. Transaction list/search last, after a separate query-builder design.

## Non-Goals

- Do not add an ORM.
- Do not rewrite all of `internal/store/store.go` in one pass.
- Do not move SQL into string templates that can interpolate raw user input.
- Do not change handler contracts unless the repository boundary requires a new operation.
