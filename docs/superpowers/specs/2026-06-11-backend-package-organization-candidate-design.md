# Backend Package Organization Candidate Design

## Goal

Create a small, behavior-neutral package organization candidate that makes three existing ownership boundaries explicit without redesigning plugins, persistence, or ingestion contracts.

## Scope

1. Rename `backend/internal/api` to `backend/internal/httpapi`.
   The package implements HTTP handlers, routing, middleware, request binding, and HTTP response models. The new name distinguishes it from `backend/pkg/api`, which currently holds ingestion contracts and data structures.
2. Move `backend/pkg/client` to `backend/internal/oauth`.
   The package contains only OAuth configuration and token-persisting HTTP client construction. It is application-specific and should not suggest a general-purpose client library.
3. Move `backend/pkg/extractor` to `backend/internal/extractor`.
   Extraction is shared by the Gmail and Thunderbird readers, so it remains a distinct package. Moving it under `internal` accurately reflects that it is not an external library contract.

## Non-Goals

- Rename or redesign `backend/pkg/api`.
- Merge plugin wrappers into reader or writer packages.
- Change `backend/pkg/state` or processed-message semantics.
- Move migrations or consolidate PostgreSQL pools.
- Change HTTP routes, JSON contracts, runtime behavior, or test behavior.

## Dependency Direction

The moved packages remain available throughout the backend module because they live under the module-level `internal` directory. Existing consumers update imports only:

- `cmd/server` imports `internal/httpapi` and `internal/oauth`.
- HTTP handlers import `internal/oauth`.
- Gmail, Thunderbird, and rule fixtures import `internal/extractor`.

No compatibility aliases will be retained because all consumers are in the same repository and the old paths are not intended public APIs.

## Verification

This refactor does not introduce new behavior, so existing tests are the appropriate safety net. Verification must include:

- `task fmt:be`
- `task test:be`
- `task lint:be:prod`
- `task openapi:check`
- `task test`

The OpenAPI check ensures the package rename does not alter generated API output.

## Deferred Review Questions

The PR should be reviewed as a candidate organization pattern. Follow-up work requires separate decisions about:

- the correct name and location for `pkg/api`;
- whether plugin wrappers should be merged into concrete implementations;
- whether `pkg/state` should become an internal deduplication policy package;
- whether PostgreSQL writer, store, migration, and pool ownership should be consolidated.
