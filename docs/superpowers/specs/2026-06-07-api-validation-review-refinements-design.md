# API Validation Review Refinements Design

## Objective

Refine the candidate HTTP validation implementation after review without
expanding validation to additional APIs. The changes cover transaction
pagination constraints, typed query helper ergonomics, validation-layer file
boundaries, request DTO placement, stale OpenAPI types, and repository guidance
for future uses of Go generics.

## Transaction Pagination

`GET /api/transactions` will retain the existing defaults of `page=1` and
`page_size=20`.

The `page` parameter will require only a positive integer. The arbitrary
maximum of 10,000 will be removed because the page number describes an offset
into user-owned data and valid later pages should remain addressable.

The `page_size` parameter will accept integers from 1 through 100. Current UI
consumers request 5, 20, 50, or 100 rows. The main Transactions page exposes
20, 50, and 100 and normalizes unsupported URL values back to 20. Therefore,
500 is not a supported product behavior and unnecessarily permits larger
database responses.

Pagination arithmetic must not overflow:

- The HTTP layer will reject a valid integer `page` when `(page - 1) *
  page_size` cannot be represented as a Go `int`. The response will use the
  existing structured `422` schema and identify `page` as the invalid query
  field.
- The transaction repository will independently use checked offset arithmetic
  and return an error for an overflowing `store.ListFilter`. This preserves
  safety for non-HTTP callers.

OpenAPI annotations and examples will describe the new constraints.

## Typed Query Helper

The candidate query handlers will use a package-level generic function that
creates and returns the concrete query DTO:

```go
query, ok := decodeAndValidateQuery[transactionListQuery](h, w, r)
```

This removes handler-managed target pointers and makes the concrete result type
visible at the call site. It also combines the standard candidate flow of form
decoding followed by semantic validation.

Generics do not remove reflection from this implementation.
`go-playground/form/v4` decodes tagged structs through reflection, and
`validator/v10` validates tagged structs through reflection. Go also does not
permit methods with their own type parameters, so the generic helper must be a
package-level function rather than a `Handlers` method.

The small amount of local reflection used to map form fields to public
conversion messages will be cached per DTO type. The generic helper is
therefore an API-safety and readability improvement; no performance claim will
be made without a benchmark.

Body validation will keep its current non-generic handler method. It already
accepts decoded concrete values, and a generic wrapper would neither remove
reflection nor simplify its call sites meaningfully.

## Validation File Boundaries

`query_binding.go` and `validation.go` will remain separate:

- `query_binding.go` owns query transport decoding and conversion failures.
- `validation.go` owns semantic validation and validator-message translation.

They form one HTTP validation layer while retaining separate responsibilities.
Combining them would produce a larger mixed-purpose file without reducing an
interface or dependency boundary.

## Request DTO Placement

`transactionListQuery` and `diagnosticListQuery` will remain next to their
handlers. They are private runtime transport DTOs used to decode query
parameters and are not referenced as OpenAPI body schemas. Their query
contracts are documented through endpoint `@Param` annotations.

`openapi_types.go` will remain limited to exported request and response schemas
that Swagger references as named body or response models. Moving private query
DTOs there would conflate runtime binding types with documentation-only schema
types.

The existing `TransactionsSearchResponse` will be removed because the separate
transaction search endpoint was consolidated into `GET /api/transactions` and
the type has no remaining references.

Other handler-local types will not be moved as part of this refinement. The
scan found internal transformation and orchestration types such as
`oauthTokenState`, `ruleHTTPJSON`, and taxonomy export helpers; they are not
documentation-only OpenAPI models and moving them would be unrelated churn.

## Repository Guidance

`CLAUDE.md` will record these rules:

- Use generics when they improve compile-time type safety or remove meaningful
  repeated algorithms across concrete types.
- Do not introduce generics solely to replace `any` when the underlying
  library still requires reflection or when call sites do not become clearer.
- Do not claim a performance benefit from generics without measuring the
  relevant path.
- For tag-driven decoders and validators, cache unavoidable local reflection
  metadata by concrete DTO type when the metadata is reused.

## Testing

Implementation will follow TDD:

1. Add handler tests proving `page` values above 10,000 are accepted when their
   offset is representable.
2. Change the page-size boundary test to reject 101 and accept 100.
3. Add a handler test for page/page-size offset overflow and its structured
   `422` response.
4. Add repository tests proving checked offset arithmetic rejects overflow for
   non-HTTP callers.
5. Add focused tests for the generic decode-and-validate helper, including
   conversion and semantic failures.
6. Regenerate OpenAPI and verify the page maximum is absent, page-size maximum
   is 100, and the obsolete search response schema is gone.
7. Run backend formatting, tests, strict production lint, and OpenAPI drift
   checks before committing the implementation.

## Scope

This refinement applies only to the already approved candidate validation
APIs. It does not migrate other query parameters or request bodies, merge the
two validation source files, move private query DTOs into
`openapi_types.go`, or replace reflection-based third-party libraries.
