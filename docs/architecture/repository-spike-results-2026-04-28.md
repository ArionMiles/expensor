# Repository Spike Results 2026-04-28

## Prototype

`LabelRepository` was extracted from `Store` while preserving the existing `Store.ListLabels`, `Store.CreateLabel`, `Store.UpdateLabel`, and `Store.DeleteLabel` forwarding methods. API handlers and the `Storer` interface did not need to change.

The repository prototype also wires the new query instrumentation helper around each label CRUD operation and records:

- operation name
- duration in milliseconds
- returned error

## What Worked

- Small CRUD areas can move behind repositories without changing handlers.
- Store forwarding keeps the existing API-facing contract stable during incremental migration.
- Repository tests can run against the existing container-backed store fixture when `Store` exposes the underlying pool through a focused test helper.
- Instrumentation is easiest to apply at repository method boundaries, where operation names map directly to product-level behavior.

## Risks

- Transaction list/search queries are substantially more complex than labels because they build dynamic filters, joins, sorting, and pagination. They need a dedicated query-builder design before extraction.
- Instrumentation currently logs at info level so tests using the default `slog.TextHandler` can assert output without custom handler options. Production wiring may want an explicit level policy before broad adoption.
- Creating a repository wrapper from each `Store` forwarding method is acceptable for the spike, but broader migration should decide whether repositories are long-lived `Store` fields.
- Test-only pool exposure should remain narrow and should not become part of production repository construction paths.

## Recommended Next Migration

Move low-risk runtime/config data next, then rules, then transactions last:

1. App config and reader runtime state.
2. Rule CRUD/import/export.
3. Label merchant mappings and category/bucket taxonomy.
4. Transaction list/search/update/mute behavior after a query-builder design is documented.

## Follow-Up Rules

- Keep handler dependencies small and stable while repositories are introduced.
- Preserve current `Store` forwarding methods until the API layer is deliberately moved to repository interfaces.
- Add repository tests before moving SQL.
- Wrap repository errors with operation context.
- Use instrumentation at repository method boundaries, not inside every query helper.
