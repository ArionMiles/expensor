# API Consolidation Design

## Objective

Consolidate redundant and action-oriented API routes before introducing request validation. Validation will be implemented in a separate PR after this consolidation PR is merged.

## Transaction Collection

Replace:

```http
GET /api/transactions
GET /api/transactions/search?q=instamart
```

with:

```http
GET /api/transactions
GET /api/transactions?q=instamart
```

The collection handler lists transactions when `q` is absent or blank and invokes search behavior when `q` is non-empty. All existing structured filters continue to compose with `q`.

The store may retain separate list and search methods because those are distinct query operations below the HTTP boundary.

## Partial Resource Updates

Use `PATCH` for partial updates:

```http
PATCH /api/transactions/{id}
PATCH /api/extraction-diagnostics/{id}
PATCH /api/muted-merchants/{id}
```

The transaction patch accepts the existing optional transaction fields plus `muted` and `mute_reason`. This replaces:

```http
PUT /api/transactions/{id}
PUT /api/transactions/{id}/mute
PUT /api/transactions/{id}/mute-reason
```

The diagnostic patch accepts `status`, replacing `/status`. The muted-merchant patch accepts `reason`, replacing `/reason`.

## Application Preferences

Replace five singleton getter/setter pairs with:

```http
GET   /api/config/preferences
PATCH /api/config/preferences
```

The resource contains:

```json
{
  "base_currency": "INR",
  "scan_interval": 60,
  "lookback_days": 180,
  "timezone": "Asia/Kolkata",
  "time_format": "HH:mm"
}
```

`GET` returns all preferences. `PATCH` accepts any subset, validates all supplied fields before writing, and then returns the complete resource. Existing persistence keys remain unchanged.

## Reader Configuration

Change reader configuration updates from:

```http
POST /api/readers/{name}/config
```

to:

```http
PUT /api/readers/{name}/config
```

The request replaces the singleton configuration document for that reader, so `PUT` matches existing behavior.

## Annual Heatmap

Replace:

```http
GET /api/stats/heatmap/annual?year=2026
```

with:

```http
GET /api/stats/heatmap?year=2026
```

When `year` is supplied, the endpoint returns the existing annual response. Otherwise it returns the existing ranged/all-time heatmap response. `year` cannot be combined with `from` or `to`.

## Taxonomy Merchant Mappings

Replace action routes and DELETE request bodies with merchant-mapping subresources:

```http
PUT    /api/config/labels/{name}/merchant-mappings/{pattern}
DELETE /api/config/labels/{name}/merchant-mappings/{pattern}

PUT    /api/config/categories/{name}/merchant-mappings/{pattern}
DELETE /api/config/categories/{name}/merchant-mappings/{pattern}

PUT    /api/config/buckets/{name}/merchant-mappings/{pattern}
DELETE /api/config/buckets/{name}/merchant-mappings/{pattern}
```

`{name}` and `{pattern}` are URL-encoded path segments. For example:

```http
PUT /api/config/categories/Food%20%26%20Dining/merchant-mappings/Swiggy%20Instamart
DELETE /api/config/categories/Food%20%26%20Dining/merchant-mappings/Swiggy%20Instamart
```

`PUT` creates or retains the mapping and applies it to matching transactions. `DELETE` removes the mapping and reverses its mapping-owned transaction effects. The routes are idempotent at the resource level.

Mapping IDs are deliberately not introduced. Patterns containing encoded slashes may be problematic with some routers or proxies; stable IDs can be reconsidered if that becomes a real limitation.

## Routes Kept Separate

- `/api/transactions/facets` is an aggregate representation, not a transaction list duplicate.
- Dashboard, charts, and heatmap responses are distinct read models.
- OAuth and daemon endpoints represent commands or protocol transitions.
- Import and export endpoints use distinct upload/download representations.

## Compatibility

This is an internal application API with the frontend updated in the same PR. Removed routes will not be retained as aliases. Backend handlers, frontend client calls, mocks, component tests, OpenAPI, contract allowlists, and API testing documentation will move together.

## Testing

- Backend handler tests prove the new methods and routes invoke existing store behavior.
- Frontend API tests prove generated URLs use consolidated routes.
- Existing behavior tests are moved rather than duplicated.
- OpenAPI generation and drift checks must pass.
- Backend, frontend, contract, and relevant Playwright suites run before the PR is opened.

## Validation Follow-Up

After this PR is merged, a separate validation PR will introduce the candidate validation layer and structured errors:

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

The structured detail intentionally omits validator-specific codes.
