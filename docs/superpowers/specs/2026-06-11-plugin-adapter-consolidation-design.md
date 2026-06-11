# Plugin Adapter Consolidation Design

## Goal

Remove the three packages that only wrap concrete reader and writer constructors
with plugin metadata, while preserving the registry-based runtime selection model.

## Package Changes

- Move the Gmail `Plugin` implementation and tests from
  `pkg/plugins/readers/gmail` into `pkg/reader/gmail`.
- Move the Thunderbird `Plugin` implementation and tests from
  `pkg/plugins/readers/thunderbird` into `pkg/reader/thunderbird`.
- Move the PostgreSQL `Plugin` implementation and tests from
  `pkg/plugins/writers/postgres` into `pkg/writer/postgres`.
- Update `cmd/server` to register plugins from the concrete implementation
  packages.
- Remove the now-empty `pkg/plugins` tree.

The registry and its `ReaderPlugin` and `WriterPlugin` contracts remain in
`internal/plugins`. Concrete packages continue to implement those contracts;
only the one-to-one adapter package layer is removed.

## Guide Data

The adapter directories contain duplicate `guide.json` files that are not read
by production code. Runtime guide data is embedded from
`backend/cmd/server/content/readers`, which mirrors the canonical top-level
`content/readers` files. Removing the adapter directories also removes these
unused third copies.

## Behavior And Compatibility

Plugin names, metadata, configuration schemas, setup guides, constructor inputs,
and runtime registration remain unchanged. The old Go import paths are removed
without compatibility aliases because all consumers are inside this repository.

## Verification

Existing plugin tests move with their implementations. No new tests are needed
for this mechanical refactor. Verification includes:

- `task fmt:be`
- `task test:be`
- `task lint:be:prod`
- `task openapi:check`
- `task test`

## Deferred Work

This slice does not change `internal/plugins`, `pkg/api`, processed-message state,
migrations, PostgreSQL pools, or writer/store ownership.
