# OpenAPI Contract Complete Coverage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Document every registered backend API route in OpenAPI and contract-test every deterministic route in one PR.

**Architecture:** Add machine-checked coverage guards that compare `backend/internal/api/server.go` registrations against the committed OpenAPI artifact, `tests/contract/allowlist.tsv`, and a small explicit contract exclusion list. Then fill OpenAPI annotations, request/response DTOs, and Schemathesis examples in resource-oriented phases until every route is either allowlisted or intentionally excluded for external OAuth/filesystem/live-reader requirements.

**Tech Stack:** Go 1.26, Swaggo v1.16.4, Schemathesis Docker runner, existing `task openapi:check`, `task test:be`, and `task test:be:contract` targets.

---

## Scope Boundary

This PR targets complete OpenAPI documentation for all routes registered by `registerRoutes`.

Contract coverage targets every route except routes that need external OAuth, filesystem credential upload/profile discovery, or live reader state. Destructive and stateful application routes are in scope when they can run deterministically against `tests/component/fixtures/seed.sql` or with explicit OpenAPI examples.

Initial contract exclusions:

```tsv
GET	/auth/callback	external OAuth redirect state
POST	/readers/{name}/credentials	filesystem-style credential upload and OAuth client JSON
GET	/readers/thunderbird/discover/profiles	filesystem profile discovery
GET	/readers/thunderbird/discover/mailboxes	filesystem mailbox discovery
POST	/readers/{name}/auth/start	external OAuth credentials and consent URL
POST	/readers/{name}/auth/exchange	external OAuth callback state and token exchange
DELETE	/readers/{name}/auth/token	external OAuth token state
GET	/readers/{name}/auth/status	external OAuth token state
DELETE	/readers/{name}	live reader runtime disconnect state
GET	/readers/{name}/status	live reader readiness state
GET	/readers/{name}/config	live reader runtime config
POST	/readers/{name}/config	live reader runtime config
GET	/readers/{name}/credentials/status	external OAuth credential state
```

If implementation proves any excluded route can be deterministic in the contract environment without external dependencies, move it into `allowlist.tsv` in the same PR.

## Current Inventory

Captured from the branch before implementation:

- Registered `/api` operations in `backend/internal/api/server.go`: 88
- Swaggo `@Router` annotations in `backend/internal/api`: 46
- Committed OpenAPI paths in `api/openapi/expensor.openapi.yaml`: 35
- Contract allowlist entries in `tests/contract/allowlist.tsv`: 18

The first task makes these counts enforceable so progress is measurable and future route drift fails tests.

## File Structure

Create:

- `backend/internal/api/openapi_coverage_test.go`: parses registered routes, committed OpenAPI operations, contract allowlist entries, and contract exclusions; fails on undocumented routes or contract coverage gaps.
- `tests/contract/exclusions.tsv`: tab-separated list of routes intentionally excluded from Schemathesis with short reasons.

Modify:

- `backend/internal/api/openapi_types.go`: add DTOs for plugin metadata, readers, rules, taxonomy export/mappings, stats, heatmaps, muted merchants, categorization, auth/config payloads, and generic count/status responses.
- `backend/internal/api/handlers_readers.go`: add Swaggo annotations for reader/plugin routes and examples for deterministic reader metadata routes.
- `backend/internal/api/handlers_rules.go`: add Swaggo annotations and DTO references for rules CRUD/import/export.
- `backend/internal/api/handlers_stats.go`: add Swaggo annotations for stats/charts/heatmaps/monthly breakdown.
- `backend/internal/api/handlers_taxonomy.go`: add missing annotations for taxonomy exports, category/bucket mappings, apply/remove merchant routes, and category/bucket apply routes.
- `backend/internal/api/handlers_transactions.go`: add missing annotations for muted merchant and merchant categorization routes.
- `backend/internal/api/handlers_config.go`: verify config route examples cover path/query/body values used by Schemathesis.
- `backend/internal/api/handlers_daemon.go`: verify daemon examples are deterministic for contract execution.
- `api/openapi/expensor.openapi.yaml`: regenerate through `task openapi:generate`.
- `tests/contract/allowlist.tsv`: expand to all deterministic routes.
- `tests/contract/README.md`: document the complete-coverage rule and exclusion process.

## Deterministic Examples

Use these stable values in OpenAPI examples for path/body parameters that Schemathesis can replay against the contract seed:

- Existing transaction ID: `00000000-0000-0000-0000-000000000001`
- Existing transaction label: `Weekend`
- Existing label: `Online`
- Existing category: `Food & Dining`
- Existing bucket: `Needs`
- Reader name: `thunderbird`
- Existing diagnostic ID: `00000000-0000-0000-0000-000000000001` only if the seed contains a diagnostic with that ID; otherwise keep diagnostic detail/update out of allowlist until a deterministic diagnostic seed is added.
- New label mutation name: `Contract Label`
- New category mutation name: `Contract Category`
- New bucket mutation name: `Contract Bucket`
- Merchant pattern: `Swiggy`
- Mute reason: `contract check`
- Categorization body: `{"merchant":"Swiggy","category":"Food & Dining","bucket":"Wants"}`
- Valid rule body:

```json
{
  "name": "Contract Rule",
  "sender_emails": ["contract@example.com"],
  "subject_contains": "Contract transaction",
  "amount_regex": "INR\\s+([0-9,.]+)",
  "merchant_regex": "at\\s+(.+)$",
  "currency_regex": "(INR)",
  "source": {
    "type": "Email",
    "label": "Contract",
    "bank": "Contract Bank"
  }
}
```

When a mutation would be order-sensitive if fuzzed repeatedly, prefer an example that is idempotent in the handler/store layer. If no idempotent example exists, add a deterministic seed record and use update/delete examples against that record.

## Task 1: Add Coverage Guard Tests

**Files:**

- Create: `backend/internal/api/openapi_coverage_test.go`
- Create: `tests/contract/exclusions.tsv`
- Modify: `tests/contract/README.md`

- [ ] **Step 1: Write the failing OpenAPI and contract coverage tests**

Create `backend/internal/api/openapi_coverage_test.go`:

```go
package api

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type routeOperation struct {
	Method string
	Path   string
}

func (op routeOperation) key() string {
	return op.Method + " " + op.Path
}

func TestRegisteredRoutesAreDocumentedInOpenAPI(t *testing.T) {
	registered := registeredRouteOperations(t)
	documented := openAPIOperations(t)

	var missing []string
	for _, op := range registered {
		if !documented[op.key()] {
			missing = append(missing, op.key())
		}
	}
	if len(missing) > 0 {
		t.Fatalf("registered routes missing from OpenAPI:\n%s", strings.Join(missing, "\n"))
	}
}

func TestRegisteredRoutesHaveContractDecision(t *testing.T) {
	registered := registeredRouteOperations(t)
	allowlisted := routeSetFromTSV(t, "../../../tests/contract/allowlist.tsv")
	excluded := routeSetFromTSV(t, "../../../tests/contract/exclusions.tsv")

	var missing []string
	for _, op := range registered {
		key := op.key()
		if !allowlisted[key] && !excluded[key] {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("registered routes missing contract allowlist/exclusion decision:\n%s", strings.Join(missing, "\n"))
	}

	for key := range allowlisted {
		if excluded[key] {
			t.Fatalf("route %s is both allowlisted and excluded from contract coverage", key)
		}
	}
}

func registeredRouteOperations(t *testing.T) []routeOperation {
	t.Helper()
	data, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatalf("read server.go: %v", err)
	}
	re := regexp.MustCompile(`mux\.HandleFunc\("([A-Z]+) /api([^"]+)"`)
	matches := re.FindAllStringSubmatch(string(data), -1)
	ops := make([]routeOperation, 0, len(matches))
	for _, match := range matches {
		ops = append(ops, routeOperation{Method: match[1], Path: match[2]})
	}
	sortRouteOperations(ops)
	return ops
}

func openAPIOperations(t *testing.T) map[string]bool {
	t.Helper()
	file, err := os.Open(filepath.Clean("../../../api/openapi/expensor.openapi.yaml"))
	if err != nil {
		t.Fatalf("open OpenAPI artifact: %v", err)
	}
	defer file.Close()

	ops := map[string]bool{}
	var currentPath string
	pathRE := regexp.MustCompile(`^  (/[^:]+):$`)
	methodRE := regexp.MustCompile(`^    (get|post|put|delete):$`)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if match := pathRE.FindStringSubmatch(line); match != nil {
			currentPath = match[1]
			continue
		}
		if currentPath == "" {
			continue
		}
		if match := methodRE.FindStringSubmatch(line); match != nil {
			ops[strings.ToUpper(match[1])+" "+currentPath] = true
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan OpenAPI artifact: %v", err)
	}
	return ops
}

func routeSetFromTSV(t *testing.T, path string) map[string]bool {
	t.Helper()
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer file.Close()

	routes := map[string]bool{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			t.Fatalf("%s line %q must be METHOD<TAB>PATH<TAB>REASON", path, line)
		}
		routes[strings.ToUpper(parts[0])+" "+parts[1]] = true
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan %s: %v", path, err)
	}
	return routes
}

func sortRouteOperations(ops []routeOperation) {
	sort.Slice(ops, func(i, j int) bool {
		return ops[i].key() < ops[j].key()
	})
}
```

Create `tests/contract/exclusions.tsv` with the initial exclusions from the Scope Boundary section.

Update `tests/contract/README.md` so it states that every registered route must appear in either `allowlist.tsv` or `exclusions.tsv`, and exclusions are limited to external OAuth, filesystem credential/profile discovery, or live reader state.

- [ ] **Step 2: Run the new tests to verify they fail**

Run:

```bash
task test:be -- ./internal/api
```

Expected: FAIL with missing OpenAPI routes and missing contract decisions.

- [ ] **Step 3: Do not broaden exclusions to make the test pass**

Only the routes listed in Scope Boundary should be in `exclusions.tsv` unless a route demonstrably requires the same external dependency class. All other failures should be fixed by OpenAPI annotations and `allowlist.tsv` entries in later tasks.

- [ ] **Step 4: Commit the guard test and exclusion baseline**

```bash
git add backend/internal/api/openapi_coverage_test.go tests/contract/exclusions.tsv tests/contract/README.md
git commit --no-gpg-sign -m "test: guard api contract coverage decisions"
```

## Task 2: Document Plugin, Reader Metadata, and Stats GET Routes

**Files:**

- Modify: `backend/internal/api/openapi_types.go`
- Modify: `backend/internal/api/handlers_readers.go`
- Modify: `backend/internal/api/handlers_stats.go`
- Modify: `tests/contract/allowlist.tsv`
- Modify: `api/openapi/expensor.openapi.yaml`

- [ ] **Step 1: Add DTOs for plugin, reader metadata, and stats payloads**

Add these DTOs to `backend/internal/api/openapi_types.go`:

```go
type ConfigFieldResponse struct {
	Name        string   `json:"name" example:"profilePath"`
	Label       string   `json:"label" example:"Profile path"`
	Type        string   `json:"type" example:"text"`
	Required    bool     `json:"required"`
	Description string   `json:"description,omitempty" example:"Path to the Thunderbird profile"`
	Options     []string `json:"options,omitempty"`
}

type ReaderInfoResponse struct {
	Name                      string                `json:"name" example:"thunderbird"`
	Description               string                `json:"description" example:"Read transaction emails from Thunderbird"`
	AuthType                  string                `json:"auth_type" example:"config"`
	RequiresCredentialsUpload bool                  `json:"requires_credentials_upload"`
	ConfigSchema              []ConfigFieldResponse `json:"config_schema"`
}

type WriterInfoResponse struct {
	Name        string `json:"name" example:"postgres"`
	Description string `json:"description" example:"Store transactions in PostgreSQL"`
}

type ReaderGuideResponse map[string]any

type ReaderStatusResponse struct {
	Ready  bool     `json:"ready"`
	Reader string   `json:"reader" example:"thunderbird"`
	Missing []string `json:"missing,omitempty"`
}

type ReaderCredentialsStatusResponse struct {
	Exists bool `json:"exists"`
}

type ReaderAuthStatusResponse struct {
	Authenticated bool       `json:"authenticated"`
	AuthType       string     `json:"auth_type,omitempty" example:"config"`
	Expiry         *time.Time `json:"expiry,omitempty"`
}

type ReaderConfigResponse map[string]any

type DashboardDataResponse map[string]any
type ChartDataResponse map[string]any
type HeatmapDataResponse map[string]any

type AnnualHeatmapResponse struct {
	Year    int           `json:"year" example:"2026"`
	Buckets []interface{} `json:"buckets"`
}

type MonthlyBreakdownResponse map[string]any
```

If Swaggo cannot render `map[string]any` for any response, replace that response with a minimal concrete struct that matches the actual JSON keys returned by the store type.

- [ ] **Step 2: Add Swaggo annotations for deterministic plugin/reader GET routes**

Annotate these methods in `backend/internal/api/handlers_readers.go`:

```go
// @Summary List reader plugins
// @Tags Readers
// @Produce json
// @Success 200 {array} ReaderInfoResponse
// @Router /plugins/readers [get]
```

```go
// @Summary List writer plugins
// @Tags Readers
// @Produce json
// @Success 200 {array} WriterInfoResponse
// @Router /plugins/writers [get]
```

```go
// @Summary Get reader setup guide
// @Tags Readers
// @Produce json
// @Param name path string true "Reader name" example(thunderbird)
// @Success 200 {object} ReaderGuideResponse
// @Failure 404 {object} ErrorResponse
// @Router /readers/{name}/guide [get]
```

Do not add allowlist entries for the excluded live-state reader routes in this task.

- [ ] **Step 3: Add Swaggo annotations for stats GET routes**

Annotate methods in `backend/internal/api/handlers_stats.go`:

```go
// @Summary Get dashboard data
// @Tags Stats
// @Produce json
// @Success 200 {object} DashboardDataResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /stats/dashboard [get]
```

```go
// @Summary Get chart data
// @Tags Stats
// @Produce json
// @Success 200 {object} ChartDataResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /stats/charts [get]
```

```go
// @Summary Get label monthly spend
// @Tags Stats
// @Produce json
// @Param dimension query string false "Breakdown dimension" Enums(label,category,bucket) default(label)
// @Param months query int false "Number of months" minimum(1) maximum(24) default(6)
// @Success 200 {object} MonthlyBreakdownResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /stats/labels/monthly [get]
```

```go
// @Summary Get spending heatmap
// @Tags Stats
// @Produce json
// @Param from query string false "RFC3339 start timestamp" example(2026-05-01T00:00:00Z)
// @Param to query string false "RFC3339 end timestamp" example(2026-05-31T23:59:59Z)
// @Success 200 {object} HeatmapDataResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /stats/heatmap [get]
```

```go
// @Summary Get annual spending heatmap
// @Tags Stats
// @Produce json
// @Param year query int false "Calendar year" example(2026)
// @Success 200 {object} AnnualHeatmapResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /stats/heatmap/annual [get]
```

- [ ] **Step 4: Expand the contract allowlist for deterministic GET routes**

Append these entries to `tests/contract/allowlist.tsv`:

```tsv
GET	/plugins/readers	reader plugin metadata
GET	/plugins/writers	writer plugin metadata
GET	/readers/{name}/guide	reader setup guide
GET	/stats/dashboard	dashboard stats data
GET	/stats/charts	chart stats data
GET	/stats/labels/monthly	label monthly stats data
GET	/stats/heatmap	spending heatmap data
GET	/stats/heatmap/annual	annual heatmap data
```

- [ ] **Step 5: Generate OpenAPI and verify the focused tests**

Run:

```bash
task openapi:generate
task test:be -- ./internal/api
```

Expected: coverage test still fails for routes not yet documented or allowlisted, but no longer lists the routes added in this task.

- [ ] **Step 6: Commit this phase**

```bash
git add backend/internal/api/openapi_types.go backend/internal/api/handlers_readers.go backend/internal/api/handlers_stats.go api/openapi/expensor.openapi.yaml tests/contract/allowlist.tsv
git commit --no-gpg-sign -m "docs: cover reader metadata and stats api contract"
```

## Task 3: Document Rules Import, Export, and CRUD Routes

**Files:**

- Modify: `backend/internal/api/openapi_types.go`
- Modify: `backend/internal/api/handlers_rules.go`
- Modify: `tests/contract/allowlist.tsv`
- Modify: `api/openapi/expensor.openapi.yaml`

- [ ] **Step 1: Add rule DTOs to OpenAPI types**

Add:

```go
type RuleSourceResponse struct {
	Type  string `json:"type" example:"Email"`
	Label string `json:"label" example:"Contract"`
	Bank  string `json:"bank" example:"Contract Bank"`
}

type RuleResponse struct {
	ID                string             `json:"id,omitempty" example:"11111111-1111-1111-1111-111111111111"`
	Name              string             `json:"name" example:"Contract Rule"`
	SenderEmail       string             `json:"sender_email,omitempty" example:"contract@example.com"`
	SenderEmails      []string           `json:"sender_emails"`
	SubjectContains   string             `json:"subject_contains" example:"Contract transaction"`
	AmountRegex       string             `json:"amount_regex" example:"INR\\s+([0-9,.]+)"`
	MerchantRegex     string             `json:"merchant_regex" example:"at\\s+(.+)$"`
	CurrencyRegex     string             `json:"currency_regex" example:"(INR)"`
	TransactionSource string             `json:"transaction_source,omitempty" example:"Email - Contract Bank"`
	SourceType        string             `json:"source_type,omitempty" example:"Email"`
	SourceLabel       string             `json:"source_label,omitempty" example:"Contract"`
	Bank              string             `json:"bank,omitempty" example:"Contract Bank"`
	Source            RuleSourceResponse `json:"source"`
	Predefined        bool               `json:"predefined"`
	CreatedAt         time.Time          `json:"created_at,omitempty"`
	UpdatedAt         time.Time          `json:"updated_at,omitempty"`
}

type RuleMutationRequest struct {
	Name            string             `json:"name" example:"Contract Rule"`
	SenderEmails    []string           `json:"sender_emails"`
	SubjectContains string             `json:"subject_contains" example:"Contract transaction"`
	AmountRegex     string             `json:"amount_regex" example:"INR\\s+([0-9,.]+)"`
	MerchantRegex   string             `json:"merchant_regex" example:"at\\s+(.+)$"`
	CurrencyRegex   string             `json:"currency_regex" example:"(INR)"`
	Source          RuleSourceResponse `json:"source"`
}

type RuleDocumentResponse struct {
	Version int            `json:"version" example:"2"`
	Presets map[string]any `json:"presets"`
	Rules   []RuleResponse `json:"rules"`
}

type RuleImportResponse struct {
	Imported int `json:"imported"`
}
```

- [ ] **Step 2: Add annotations for rule routes**

Annotate `ListRules`, `ExportRules`, `ImportRules`, `CreateRule`, `UpdateRule`, and `DeleteRule` in `backend/internal/api/handlers_rules.go` with:

```go
// @Tags Rules
// @Accept json
// @Produce json
```

Use these route-specific response references:

- `GET /rules`: `@Success 200 {array} RuleResponse`
- `GET /rules/export`: `@Success 200 {object} RuleDocumentResponse`
- `POST /rules/import`: `@Param request body RuleDocumentResponse true "Rules document"` and `@Success 200 {object} RuleImportResponse`
- `POST /rules`: `@Param request body RuleMutationRequest true "Rule payload"` and `@Success 201 {object} RuleResponse`
- `PUT /rules/{id}`: `@Param id path string true "Rule ID"` and `@Success 200 {object} RuleResponse`
- `DELETE /rules/{id}`: `@Param id path string true "Rule ID"` and `@Success 200 {object} StatusOnlyResponse`

Add `@Failure 422 {object} ErrorResponse` to mutation/import routes and `@Failure 409 {object} ErrorResponse` where the handler maps rule conflicts.

- [ ] **Step 3: Add deterministic rule contract decisions**

Add `GET /rules` and `GET /rules/export` to `tests/contract/allowlist.tsv`.

Do not add rule mutations to the allowlist until Task 7, where mutation examples are verified together. The coverage guard will continue failing until then.

- [ ] **Step 4: Generate OpenAPI and run focused tests**

Run:

```bash
task openapi:generate
task test:be -- ./internal/api
```

Expected: OpenAPI coverage no longer reports rules routes. Contract decision coverage still reports rule mutation routes until Task 7.

- [ ] **Step 5: Commit this phase**

```bash
git add backend/internal/api/openapi_types.go backend/internal/api/handlers_rules.go api/openapi/expensor.openapi.yaml tests/contract/allowlist.tsv
git commit --no-gpg-sign -m "docs: cover rules api contract"
```

## Task 4: Document Taxonomy Export, Mapping, Apply, and Remove Routes

**Files:**

- Modify: `backend/internal/api/openapi_types.go`
- Modify: `backend/internal/api/handlers_taxonomy.go`
- Modify: `tests/contract/allowlist.tsv`
- Modify: `api/openapi/expensor.openapi.yaml`

- [ ] **Step 1: Add shared taxonomy DTOs**

Add:

```go
type TaxonomyExportRowResponse struct {
	Name      string   `json:"name" example:"Food & Dining"`
	Merchants []string `json:"merchants,omitempty"`
}

type TaxonomyMappingsResponse map[string][]string

type TaxonomyMerchantRequest struct {
	MerchantPattern string `json:"merchant_pattern" example:"Swiggy"`
}

type RemovedCountResponse struct {
	Removed int64 `json:"removed"`
}
```

- [ ] **Step 2: Add missing annotations for taxonomy GET/export/mapping routes**

Annotate:

- `ExportLabels`: `GET /config/labels/export`, success `{array} TaxonomyExportRowResponse`
- `ExportCategories`: `GET /config/categories/export`, success `{array} TaxonomyExportRowResponse`
- `GetCategoryMappings`: `GET /config/categories/mappings`, success `{object} TaxonomyMappingsResponse`
- `ExportBuckets`: `GET /config/buckets/export`, success `{array} TaxonomyExportRowResponse`
- `GetBucketMappings`: `GET /config/buckets/mappings`, success `{object} TaxonomyMappingsResponse`

- [ ] **Step 3: Add missing annotations for taxonomy merchant apply/remove routes**

Annotate:

- `RemoveLabelByMerchant`: `DELETE /config/labels/{name}/merchant`, body `TaxonomyMerchantRequest`, success `{object} RemovedCountResponse`
- `ApplyCategoryByMerchant`: `POST /config/categories/{name}/apply`, body `TaxonomyMerchantRequest`, success `{object} AppliedCountResponse`
- `RemoveCategoryByMerchant`: `DELETE /config/categories/{name}/merchant`, body `TaxonomyMerchantRequest`, success `{object} RemovedCountResponse`
- `ApplyBucketByMerchant`: `POST /config/buckets/{name}/apply`, body `TaxonomyMerchantRequest`, success `{object} AppliedCountResponse`
- `RemoveBucketByMerchant`: `DELETE /config/buckets/{name}/merchant`, body `TaxonomyMerchantRequest`, success `{object} RemovedCountResponse`

For each `{name}` path parameter, use examples from Deterministic Examples.

- [ ] **Step 4: Add deterministic taxonomy routes to the allowlist**

Append:

```tsv
GET	/config/labels/export	label taxonomy export
GET	/config/categories/export	category taxonomy export
GET	/config/categories/mappings	category merchant mappings
GET	/config/buckets/export	bucket taxonomy export
GET	/config/buckets/mappings	bucket merchant mappings
POST	/config/categories/{name}/apply	category merchant apply
DELETE	/config/categories/{name}/merchant	category merchant removal
POST	/config/buckets/{name}/apply	bucket merchant apply
DELETE	/config/buckets/{name}/merchant	bucket merchant removal
DELETE	/config/labels/{name}/merchant	label merchant removal
```

- [ ] **Step 5: Generate OpenAPI and run focused tests**

Run:

```bash
task openapi:generate
task test:be -- ./internal/api
```

Expected: OpenAPI coverage no longer reports taxonomy routes. Contract decision coverage still reports remaining mutation routes.

- [ ] **Step 6: Commit this phase**

```bash
git add backend/internal/api/openapi_types.go backend/internal/api/handlers_taxonomy.go api/openapi/expensor.openapi.yaml tests/contract/allowlist.tsv
git commit --no-gpg-sign -m "docs: cover taxonomy api contract"
```

## Task 5: Document Muted Merchant and Merchant Categorization Routes

**Files:**

- Modify: `backend/internal/api/openapi_types.go`
- Modify: `backend/internal/api/handlers_transactions.go`
- Modify: `tests/contract/allowlist.tsv`
- Modify: `api/openapi/expensor.openapi.yaml`

- [ ] **Step 1: Add muted merchant DTOs**

Add:

```go
type MutedMerchantResponse struct {
	ID             string    `json:"id" example:"11111111-1111-1111-1111-111111111111"`
	MerchantPattern string  `json:"merchant_pattern" example:"Swiggy"`
	Reason         string   `json:"reason,omitempty" example:"contract check"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	TransactionCount int     `json:"transaction_count,omitempty"`
}

type MuteMerchantRequest struct {
	MerchantPattern string `json:"merchant_pattern" example:"Swiggy"`
	Reason          string `json:"reason,omitempty" example:"contract check"`
}

type MerchantReasonRequest struct {
	Reason string `json:"reason" example:"contract check"`
}

type CategorizeMerchantRequest struct {
	Merchant string `json:"merchant" example:"Swiggy"`
	Category string `json:"category" example:"Food & Dining"`
	Bucket   string `json:"bucket" example:"Wants"`
}

type CategorizeMerchantResponse struct {
	Updated int `json:"updated"`
}
```

- [ ] **Step 2: Add annotations for muted merchant routes**

Annotate:

- `ListMutedMerchants`: `GET /muted-merchants`, success `{array} MutedMerchantResponse`
- `MuteByMerchant`: `POST /muted-merchants`, body `MuteMerchantRequest`, success `{object} StatusOnlyResponse`
- `UpdateMerchantReason`: `PUT /muted-merchants/{id}/reason`, body `MerchantReasonRequest`, success `{object} StatusOnlyResponse`
- `DeleteMutedMerchant`: `DELETE /muted-merchants/{id}`, success `{object} StatusOnlyResponse`

- [ ] **Step 3: Add annotations for merchant categorization**

Annotate `CategorizeMerchant`:

```go
// @Summary Categorize all matching merchant transactions
// @Tags Transactions
// @Accept json
// @Produce json
// @Param request body CategorizeMerchantRequest true "Merchant categorization payload"
// @Success 200 {object} CategorizeMerchantResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /merchants/categorize [post]
```

- [ ] **Step 4: Add deterministic merchant routes to contract decisions**

Add `GET /muted-merchants`, `POST /muted-merchants`, and `POST /merchants/categorize` to `allowlist.tsv`.

Keep `PUT /muted-merchants/{id}/reason` and `DELETE /muted-merchants/{id}` out of the allowlist until Task 7 unless the seed has a stable muted merchant ID. If no stable muted merchant exists, Task 7 must add one to `tests/component/fixtures/seed.sql`.

- [ ] **Step 5: Generate OpenAPI and run focused tests**

Run:

```bash
task openapi:generate
task test:be -- ./internal/api
```

Expected: OpenAPI coverage no longer reports muted merchant or merchant categorization routes. Contract decision coverage may still report muted merchant ID mutation routes until Task 7.

- [ ] **Step 6: Commit this phase**

```bash
git add backend/internal/api/openapi_types.go backend/internal/api/handlers_transactions.go api/openapi/expensor.openapi.yaml tests/contract/allowlist.tsv
git commit --no-gpg-sign -m "docs: cover merchant management api contract"
```

## Task 6: Document Remaining Reader, Auth, and Config Routes

**Files:**

- Modify: `backend/internal/api/openapi_types.go`
- Modify: `backend/internal/api/handlers_readers.go`
- Modify: `backend/internal/api/handlers_config.go`
- Modify: `tests/contract/allowlist.tsv`
- Modify: `tests/contract/exclusions.tsv`
- Modify: `api/openapi/expensor.openapi.yaml`

- [ ] **Step 1: Add remaining reader/auth/config DTOs**

Add:

```go
type UploadCredentialsResponse struct {
	Path string `json:"path" example:"db://reader_runtime/gmail/client_secret"`
}

type AuthStartResponse struct {
	URL         string `json:"url" example:"https://accounts.google.com/o/oauth2/auth"`
	RedirectURI string `json:"redirect_uri" example:"http://localhost:8080/api/auth/callback"`
}

type AuthExchangeRequest struct {
	URL string `json:"url" example:"http://localhost:8080/api/auth/callback?state=state&code=code"`
}

type ReaderDisconnectResponse struct {
	Status       string   `json:"status" example:"disconnected"`
	FilesRemoved []string `json:"files_removed"`
}

type ReaderConfigRequest map[string]any
```

- [ ] **Step 2: Annotate excluded-but-documented reader/auth routes**

Add OpenAPI annotations for every route listed in `tests/contract/exclusions.tsv`. These routes must be documented even though they are excluded from contract execution.

Use `@Param name path string true "Reader name" example(thunderbird)` for config-only examples and `example(gmail)` for OAuth credential/token examples.

- [ ] **Step 3: Verify config routes remain documented and deterministic**

Review existing annotations in `backend/internal/api/handlers_config.go` for:

- `GET /config/setup-status`
- `GET /config/readers/{name}/checkpoint`
- `DELETE /config/readers/{name}/checkpoint`

Make sure path examples use `thunderbird` and response DTOs match the handlers.

- [ ] **Step 4: Add deterministic config routes to allowlist**

Add:

```tsv
GET	/config/setup-status	first-run setup status
GET	/config/readers/{name}/checkpoint	reader checkpoint config
DELETE	/config/readers/{name}/checkpoint	clear reader checkpoint
```

Do not add reader/auth routes listed in `exclusions.tsv`.

- [ ] **Step 5: Generate OpenAPI and run focused tests**

Run:

```bash
task openapi:generate
task test:be -- ./internal/api
```

Expected: OpenAPI coverage should pass if all registered routes are now documented. Contract decision coverage should fail only for deterministic mutation routes scheduled for Task 7.

- [ ] **Step 6: Commit this phase**

```bash
git add backend/internal/api/openapi_types.go backend/internal/api/handlers_readers.go backend/internal/api/handlers_config.go api/openapi/expensor.openapi.yaml tests/contract/allowlist.tsv tests/contract/exclusions.tsv
git commit --no-gpg-sign -m "docs: cover reader auth and config api contract"
```

## Task 7: Add Deterministic Mutation Contract Coverage

**Files:**

- Modify: `backend/internal/api/openapi_types.go`
- Modify: handler files whose examples are insufficient
- Modify: `tests/component/fixtures/seed.sql` if stable records are needed
- Modify: `tests/contract/allowlist.tsv`
- Modify: `api/openapi/expensor.openapi.yaml`

- [ ] **Step 1: Identify remaining non-excluded contract decisions**

Run:

```bash
task test:be -- ./internal/api
```

Expected: FAIL lists only routes that are documented in OpenAPI but not in `allowlist.tsv` or `exclusions.tsv`.

- [ ] **Step 2: Add stable seed records if ID-based mutations need them**

If `PUT /muted-merchants/{id}/reason` or `DELETE /muted-merchants/{id}` needs a stable row, add this to `tests/component/fixtures/seed.sql` after the existing seed cleanup sections:

```sql
INSERT INTO muted_merchants (id, merchant_pattern, reason)
VALUES ('00000000-0000-0000-0000-00000000c001', 'Contract Muted Merchant', 'contract seed')
ON CONFLICT (id) DO UPDATE
SET merchant_pattern = EXCLUDED.merchant_pattern,
    reason = EXCLUDED.reason,
    updated_at = NOW();
```

Use the actual column names from the current muted merchant migration/store implementation. If the table uses a different primary key or timestamp column, adapt this insert to that schema and keep the UUID stable.

- [ ] **Step 3: Ensure mutation annotations contain deterministic examples**

Review and fix examples for these routes:

```tsv
POST	/daemon/start
POST	/daemon/rescan
PUT	/config/base-currency
PUT	/config/scan-interval
PUT	/config/lookback-days
PUT	/config/timezone
PUT	/config/time-format
POST	/config/labels
PUT	/config/labels/{name}
DELETE	/config/labels/{name}
POST	/config/labels/{name}/apply
POST	/config/categories
DELETE	/config/categories/{name}
POST	/config/buckets
DELETE	/config/buckets/{name}
POST	/rules
POST	/rules/import
PUT	/rules/{id}
DELETE	/rules/{id}
PUT	/transactions/{id}
POST	/transactions/{id}/labels
DELETE	/transactions/{id}/labels/{label}
PUT	/transactions/{id}/mute
PUT	/transactions/{id}/mute-reason
PUT	/extraction-diagnostics/{id}/status
PUT	/muted-merchants/{id}/reason
DELETE	/muted-merchants/{id}
```

Use the Deterministic Examples section for path/body examples. If Schemathesis still generates invalid positive-mode examples for a route, tighten the OpenAPI schema with enum/min/max/required fields that match existing handler validation.

- [ ] **Step 4: Add remaining deterministic mutation routes to the allowlist**

Append every route from Step 3 that does not require the exclusions in Scope Boundary.

If a route cannot be made deterministic without external OAuth/filesystem/live reader state, move it to `exclusions.tsv` with a reason matching the allowed exclusion categories and explain that decision in `tests/contract/README.md`.

- [ ] **Step 5: Generate OpenAPI and verify coverage tests pass**

Run:

```bash
task openapi:generate
task test:be -- ./internal/api
```

Expected: PASS. Both coverage guard tests must pass.

- [ ] **Step 6: Run the contract suite**

Run:

```bash
task test:be:contract
```

Expected: PASS. If a route fails because Schemathesis generated semantically invalid positive input, fix the OpenAPI schema/example first. Only exclude a route if it falls within the Scope Boundary exclusion categories.

- [ ] **Step 7: Commit deterministic mutation coverage**

```bash
git add backend/internal/api api/openapi/expensor.openapi.yaml tests/component/fixtures/seed.sql tests/contract/allowlist.tsv tests/contract/exclusions.tsv tests/contract/README.md
git commit --no-gpg-sign -m "test: expand deterministic api contract coverage"
```

## Task 8: Final Verification and PR Readiness

**Files:**

- Modify only files required by verification fixes.

- [ ] **Step 1: Run OpenAPI drift check**

Run:

```bash
task openapi:check
```

Expected: PASS with no diff in `api/openapi/expensor.openapi.yaml`.

- [ ] **Step 2: Run backend unit tests**

Run:

```bash
task test:be
```

Expected: PASS.

- [ ] **Step 3: Run backend contract tests**

Run:

```bash
task test:be:contract
```

Expected: PASS.

- [ ] **Step 4: Run strict backend lint**

Run:

```bash
task lint:be:prod
```

Expected: PASS with `0 issues`.

- [ ] **Step 5: Inspect route coverage counts**

Run:

```bash
rg 'mux\.HandleFunc\("[A-Z]+ /api' backend/internal/api/server.go | wc -l
rg '@Router ' backend/internal/api -n | wc -l
wc -l tests/contract/allowlist.tsv tests/contract/exclusions.tsv
```

Expected:

- `@Router` count equals registered route count.
- `allowlist.tsv + exclusions.tsv` non-comment entries equal registered route count.
- Exclusions are limited to the Scope Boundary categories.

- [ ] **Step 6: Review final diff**

Run:

```bash
git diff --stat origin/main...HEAD
git diff --check origin/main...HEAD
```

Expected: no whitespace errors and a diff concentrated in API annotations/types, OpenAPI artifact, contract files, and optional deterministic seed additions.

## Plan Self-Review

- Spec coverage: covers complete OpenAPI documentation, contract allowlist expansion, explicit exclusions, route inventory counts, and required verification commands.
- Placeholder scan: no implementation placeholders remain; each task has concrete files, code snippets, commands, and expected outcomes.
- Type consistency: DTO names are unique and referenced by task annotations; route paths match `server.go` registrations without the `/api` base path.
