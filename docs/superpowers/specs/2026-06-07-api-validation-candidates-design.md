# API Validation Candidates Design

## Objective

Introduce a reusable HTTP request-validation boundary using
`github.com/go-playground/validator/v10` and apply it to a deliberately limited
set of candidate APIs:

- `GET /api/transactions`
- `GET /api/extraction-diagnostics`
- `PATCH /api/extraction-diagnostics/{id}`
- `PATCH /api/config/preferences`

The candidate set covers a complex query DTO, a simple query DTO, a simple JSON
body, and a normalized multi-field JSON body. Other APIs remain unchanged until
these candidates are reviewed and approved. Extending the same validation
layer to them will remain part of this PR after that approval.

## Architecture

`Handlers` will own one initialized `*validator.Validate`. The validator is an
HTTP-layer dependency because it validates transport DTOs rather than domain or
storage models.

Handlers will:

1. Decode JSON or parse query parameters into a typed request DTO.
2. Normalize fields whose accepted representation is intentionally flexible.
3. Validate the complete DTO.
4. Convert failures to the shared structured error response.
5. Call the store only after parsing and validation succeed.

Validation will not be implemented as generic HTTP middleware. Middleware does
not know which DTO a handler expects, whether normalization is required, or how
query strings should be decoded. Keeping validation near the handler makes the
request contract and data flow visible.

DTOs will not be registered globally. The validator will validate concrete DTO
values at their point of use. Shared custom validators may be registered once
when `Handlers` is constructed.

## Error Contract

Malformed request syntax returns `400 Bad Request`. This includes malformed
JSON and path values that cannot be parsed into their required type.

Well-formed requests that fail field or cross-field semantics return
`422 Unprocessable Entity`:

```json
{
  "error": "request validation failed",
  "details": [
    {
      "field": "page_size",
      "location": "query",
      "message": "must be at most 500"
    }
  ]
}
```

Each detail contains:

- `field`: the public JSON or query parameter name.
- `location`: `query`, `body`, or `path`.
- `message`: a consumer-facing description of the accepted constraint.

Validator tag names and implementation-specific codes will not be exposed.
Multiple independent validation failures will be returned together in stable
DTO field order where practical.

Existing non-validation errors retain their current status codes and response
shape. Store conflicts remain `409`, missing resources remain `404`, and
unexpected store failures remain `500`.

## Transaction Collection Query

A typed transaction-list query DTO will represent the collection parameters.
Absent `page` and `page_size` values retain their existing defaults of `1` and
`20`.

Present values will be validated instead of silently falling back or being
ignored:

- `page`: integer from 1 through 10000.
- `page_size`: integer from 1 through 500.
- `category_missing`, `label_missing`, `bucket_missing`, `show_muted`,
  `muted_only`, and `individual_only`: `1` when present.
- `weekday`: integer from 0 through 6.
- `hour_from` and `hour_to`: integer from 0 through 23.
- `date_from` and `date_to`: RFC3339 timestamps, including fractional seconds.
- `sort_by`: `timestamp` when present.
- `sort_dir`: `asc` or `desc` when present.
- `tz`: a valid IANA timezone when present.
- Free-text and CSV filter values: no control characters.

The free-text `q` parameter will be part of the same DTO, so list and search
requests use one validation path. After validation, the DTO will be converted
to the existing `store.ListFilter`; store list/search methods and SQL query
construction remain unchanged.

The candidate does not introduce new business constraints between otherwise
valid fields. In particular, it will not invent ordering rules for dates or
hours unless the current API already documents and enforces them.

## Extraction Diagnostics

The diagnostics list query DTO will validate:

- `status`: `open`, `resolved`, `ignored`, or `all`; absent values default to
  `open`.
- `limit`: a positive integer when present.

The diagnostic patch DTO will declare `status` as required and restrict it to
`open`, `resolved`, or `ignored`. This replaces the handler-local status switch.
The store validation remains in place because stores may be called outside HTTP
and must preserve their own invariants.

Malformed JSON changes from `422` to `400`. A missing or unsupported status
returns the structured `422` response.

## Application Preferences

The preferences patch retains its current all-fields-before-write behavior.
Normalization occurs before validation:

- `base_currency` is trimmed and uppercased.
- `timezone` and `time_format` are trimmed.

Validation then enforces:

- `base_currency`: exactly three ASCII uppercase letters.
- `scan_interval`: 10 through 3600 seconds.
- `lookback_days`: 1 through 3650.
- `timezone`: valid IANA timezone.
- `time_format`: `HH:mm`, `HH:mm:ss`, `h:mm a`, or `h:mm:ss a`.

All supplied fields are validated before `persistPreferences` is called. The
store will not be called for any field when the request contains a validation
failure. Malformed JSON changes from `422` to `400`; semantic failures use the
structured `422` response.

## Store and Security Boundaries

HTTP validation improves API contracts and client feedback. It is not an
SQL-injection defense because accepted input can still contain ordinary SQL
metacharacters and because requests may reach stores through non-HTTP callers.
Repositories will continue using parameterized SQL.

Store and domain validation will remain where it protects data integrity,
supports non-HTTP callers, or expresses persistence constraints. HTTP-layer
validation may replace only duplicated transport-specific checks, such as a
handler-local enumeration switch.

## Testing

Implementation will follow TDD:

1. Add focused handler tests for structured query/body validation failures and
   confirm they fail before production changes.
2. Add success tests proving valid DTOs still reach the existing store methods
   with the same normalized filter or request data.
3. Confirm invalid preference patches perform no writes.
4. Update existing tests that currently assert silent query fallback or the old
   malformed-JSON status.
5. Update OpenAPI response schemas and endpoint failure responses.
6. Run the narrow backend handler suite, then the repository-required backend
   lint, tests, and OpenAPI drift checks.

Tests will assert public behavior rather than validator internals. They will not
depend on validator tag names.

## Out of Scope Until Candidate Review

- Migrating request DTOs or query parameters for APIs outside the candidate set.
- Removing store/domain invariants that protect non-HTTP callers or persistence.
- Adding generic validation middleware.
- Changing transaction SQL or store query interfaces.
- Adding validator-specific error codes to the response.
