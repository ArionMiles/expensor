# Testing Phase 3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Go stdlib functional/component tests under `tests/component`, runnable via Docker Compose against a live backend and seeded Postgres.

**Architecture:** Phase 3 is a black-box HTTP harness. A Compose stack starts Postgres, a coverage-instrumented backend binary, a one-shot SQL seeding step, and a Go test runner that exercises the live API over HTTP. Runtime backend coverage is collected from the running binary with `GOCOVERDIR` and post-processed on the host.

**Tech Stack:** Go stdlib `testing`, Docker Compose, PostgreSQL, `go build -cover`, `GOCOVERDIR`, `go tool covdata`, Task, GitHub Actions

---

## Status

- Status: Complete
- Last updated: 2026-05-19
- Owner: Unassigned
- Execution note: Added tests/component Compose harness, anonymized seed fixture, helper package, and build-tagged table-driven suites for health/settings/taxonomy/transactions. Added `task test:be:component`, backend-component CI job, failure-only container log printing, and runtime coverage export at `tests/component/artifacts/backend-component.coverage.out`. Execution uses `docker compose up --quiet-pull -d` + `run --rm` for seed/runner to avoid premature shutdown from one-shot seed exits. The Go containers use pinned `golang:1.26.2-alpine` images with non-login `sh -c` commands so Alpine login-shell PATH resets do not hide the Go toolchain.

## Files

- Create: `tests/component/docker-compose.yml`
- Create: `tests/component/README.md`
- Create: `tests/component/go.mod`
- Create: `tests/component/fixtures/seed.sql`
- Create: `tests/component/helpers/client.go`
- Create: `tests/component/helpers/assertions.go`
- Create: `tests/component/helpers/waits.go`
- Create: `tests/component/health_test.go`
- Create: `tests/component/settings_test.go`
- Create: `tests/component/taxonomy_test.go`
- Create: `tests/component/transactions_test.go`
- Modify: `Taskfile.yml`
- Modify: `.github/workflows/ci.yml`
- Modify: `docs/superpowers/specs/2026-05-19-frontend-testing.md`
- Modify: `docs/superpowers/plans/2026-05-19-testing-program.md`
- Modify: `docs/superpowers/plans/2026-05-19-testing-phase-3-backend-component.md`

## Planned File Responsibilities

- `tests/component/docker-compose.yml`
  Purpose: define the `postgres`, `backend`, `seed`, and `runner` services for the local/CI component harness.
- `tests/component/README.md`
  Purpose: document the harness, seed data, artifacts, and the expected local workflow.
- `tests/component/go.mod`
  Purpose: keep the component suite isolated from the backend module while still using Go stdlib `testing`.
- `tests/component/fixtures/seed.sql`
  Purpose: insert a deterministic, reviewable dataset for settings, taxonomy, and transaction behavior, initially derived from anonymized realistic local dev data.
- `tests/component/helpers/client.go`
  Purpose: provide a minimal HTTP client and JSON request helpers for black-box tests.
- `tests/component/helpers/assertions.go`
  Purpose: keep status-code and JSON/body assertions consistent without hiding the business assertions.
- `tests/component/helpers/waits.go`
  Purpose: block until the live backend is healthy before tests start.
- `tests/component/*_test.go`
  Purpose: encode the first high-value seeded behavior checks by user-facing domain.
- `Taskfile.yml`
  Purpose: add `test:be:component` and coverage post-processing.
- `.github/workflows/ci.yml`
  Purpose: add the backend component-test job and upload its coverage artifacts.

---

### Task 1: Mark Phase 3 Ready and Define the Harness Layout

**Files:**
- Modify: `docs/superpowers/specs/2026-05-19-frontend-testing.md`
- Modify: `docs/superpowers/plans/2026-05-19-testing-program.md`
- Modify: `docs/superpowers/plans/2026-05-19-testing-phase-3-backend-component.md`
- Create: `tests/component/go.mod`
- Create: `tests/component/README.md`

- [x] **Step 1: Mark the workstream `In Progress` when execution starts**

Update the tracking docs at execution time:

- spec `Program Status` row for Phase 3 → `In Progress`
- program index Phase 3 line → `Status: In Progress`
- this plan status block → `In Progress`

- [x] **Step 2: Add the component module file**

Create `tests/component/go.mod`:

```go
module github.com/ArionMiles/expensor/tests/component

go 1.26.0
```

- [x] **Step 3: Add the component-suite README**

Create `tests/component/README.md` with these sections:

```md
# Backend component test harness

This suite runs black-box backend functional tests against a live Expensor backend and PostgreSQL instance started with Docker Compose.

## What this harness covers

- live HTTP behavior through the public API
- deterministic seeded data scenarios
- runtime backend coverage from the running binary

## What this harness does not cover

- OpenAPI contract conformance
- frontend/browser behavior
- reader OAuth and daemon happy-path scanning flows

## Local workflow

- `task test:be:component`

Artifacts are written under `tests/component/artifacts/`:

- `backend-coverage/` raw `GOCOVERDIR` files
- `backend-component.coverage.out` text coverage profile

## Seed data

The suite loads `fixtures/seed.sql` after migrations complete. Tests should assume the seeded rows described there instead of creating large ad hoc datasets inside each test.

## Seed source and privacy rule

The first version of `fixtures/seed.sql` should be derived from the realistic data already present in the local `expensor-dev-postgres` container rather than invented from scratch.

Before any row is copied into the fixture:

- replace message IDs, UUIDs, merchant fragments, descriptions, reasons, and free-text fields that could reveal personal information with anonymized placeholders
- normalize timestamps where exact real-world times are not required for the asserted behavior
- keep the business shape that matters for tests: currencies, categories, buckets, muted state, label relationships, and config values
- do not copy OAuth tokens, credentials, or any reader-specific secrets into fixtures

The committed fixture must be safe to publish.

## Test organization rule

All backend component tests in this phase must use table-driven test structure.

For each endpoint-oriented test function:

- define a `[]struct{...}` table of cases
- include the case name, request path/body, expected status, and the key assertion for the response
- iterate with `for _, tc := range cases { t.Run(tc.name, func(t *testing.T) { ... }) }`

Different behaviors for the same endpoint should be added as table entries, not as separate one-off test functions unless the setup shape is genuinely different.
```

- [x] **Step 4: Verify the scaffolding exists**

Run:

```bash
test -f tests/component/go.mod && test -f tests/component/README.md
```

Expected: command exits 0.

---

### Task 2: Add the Compose Harness and Seed Dataset

**Files:**
- Create: `tests/component/docker-compose.yml`
- Create: `tests/component/fixtures/seed.sql`

- [x] **Step 1: Create the Compose stack**

Create `tests/component/docker-compose.yml`:

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: expensor_component
      POSTGRES_USER: expensor
      POSTGRES_PASSWORD: expensor
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U expensor -d expensor_component"]
      interval: 5s
      timeout: 3s
      retries: 20

  backend:
    image: golang:1.26.2-alpine
    working_dir: /workspace/backend
    volumes:
      - ../..:/workspace
      - ./artifacts/backend-coverage:/coverage/backend
    environment:
      PORT: "8080"
      BASE_URL: http://backend:8080
      FRONTEND_URL: http://backend:8080
      POSTGRES_HOST: postgres
      POSTGRES_PORT: "5432"
      POSTGRES_DB: expensor_component
      POSTGRES_USER: expensor
      POSTGRES_PASSWORD: expensor
      POSTGRES_SSLMODE: disable
      EXPENSOR_DATA_DIR: /tmp/expensor-component-data
      EXPENSOR_STATE_FILE: /tmp/expensor-component-data/state.json
      GOCOVERDIR: /coverage/backend
    command:
      - sh
      - -lc
      - |
        apk add --no-cache git
        mkdir -p /tmp/expensor-component-data /coverage/backend
        go build -cover -o /tmp/expensor ./cmd/server
        /tmp/expensor
    depends_on:
      postgres:
        condition: service_healthy
    healthcheck:
      test: ["CMD-SHELL", "wget -qO- http://localhost:8080/api/health >/dev/null 2>&1"]
      interval: 5s
      timeout: 3s
      retries: 30

  seed:
    image: postgres:16-alpine
    volumes:
      - ./fixtures/seed.sql:/seed.sql:ro
    environment:
      PGPASSWORD: expensor
    command:
      - sh
      - -lc
      - |
        until pg_isready -h postgres -U expensor -d expensor_component; do sleep 1; done
        psql -h postgres -U expensor -d expensor_component -f /seed.sql
    depends_on:
      backend:
        condition: service_healthy

  runner:
    image: golang:1.26.2-alpine
    working_dir: /workspace/tests/component
    volumes:
      - ../..:/workspace
    environment:
      COMPONENT_BASE_URL: http://backend:8080
    command:
      - sh
      - -lc
      - |
        go test -count=1 -tags=component ./...
    depends_on:
      seed:
        condition: service_completed_successfully
```

- [x] **Step 2: Add the deterministic SQL fixture from anonymized dev data**

Create `tests/component/fixtures/seed.sql`:

Before writing the final SQL file:

- create a scratch extraction note locally at `tests/component/fixtures/seed.extraction.md` during execution, but do not commit it
- inspect the local `expensor-dev-postgres` container and export a small representative slice of realistic rows for `app_config`, `labels`, `categories`, `buckets`, `transactions`, `transaction_labels`, and `transaction_label_sources`
- anonymize all PII and account-specific text before it enters the committed fixture
- keep only the rows needed for the initial component suites

Use this extraction process:

```bash
docker exec -it expensor-dev-postgres psql -U expensor -d expensor
```

Inside `psql`, inspect only the needed columns first:

```sql
SELECT key, value
FROM app_config
WHERE key IN (
  'base_currency',
  'scan_interval',
  'lookback_days',
  'app.timezone',
  'app.time_format'
)
ORDER BY key;

SELECT name, color
FROM labels
ORDER BY name
LIMIT 10;

SELECT name, description, is_default
FROM categories
ORDER BY name
LIMIT 10;

SELECT name, description, is_default
FROM buckets
ORDER BY name
LIMIT 10;

SELECT id, message_id, amount, currency, timestamp, merchant_info, category, bucket, source, description, muted, muted_by_merchant, mute_reason
FROM transactions
ORDER BY timestamp DESC
LIMIT 20;

SELECT transaction_id, label
FROM transaction_labels
LIMIT 20;

SELECT transaction_id, label, source_type, merchant_pattern
FROM transaction_label_sources
LIMIT 20;
```

Use this anonymization checklist before writing `seed.sql`:

- replace every UUID with a deterministic placeholder UUID
- replace every `message_id` with `seed-msg-N`
- replace merchant names with generic but behavior-preserving names like `Merchant A`, `Subscription Service`, `Internal Transfer`
- replace free-text descriptions with generic descriptions like `Seeded purchase`, `Seeded subscription`, `Seeded transfer`
- replace mute reasons with generic reasons like `seeded muted case`
- if an exact timestamp is not part of the assertion, normalize it to a simple fixed RFC3339 value
- preserve only fields that matter to the tested behavior: amount, currency, category, bucket, label relationships, muted flags, and config values

Before committing the fixture, manually review `tests/component/fixtures/seed.sql` and verify:

- no email addresses remain
- no real merchant strings remain
- no real message IDs remain
- no real UUIDs copied from dev remain
- no credential/token/config secret values appear
- the row set is the minimum needed for the planned tests

The committed `seed.sql` should look like this shape after anonymization:

```sql
INSERT INTO app_config (key, value) VALUES
  ('base_currency', 'USD'),
  ('scan_interval', '120'),
  ('lookback_days', '365'),
  ('app.timezone', 'Asia/Kolkata'),
  ('app.time_format', 'HH:mm:ss'),
  ('reader.gmail.last_scan_at', '2026-05-19T00:00:00Z')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

INSERT INTO labels (name, color) VALUES
  ('Bills', '#ef4444'),
  ('Recurring', '#6366f1')
ON CONFLICT (name) DO UPDATE SET color = EXCLUDED.color;

INSERT INTO categories (name, description, is_default) VALUES
  ('Income', 'Incoming transfers and salary', false)
ON CONFLICT (name) DO UPDATE SET description = EXCLUDED.description;

INSERT INTO buckets (name, description, is_default) VALUES
  ('Transfers', 'Internal transfers and account movements', false)
ON CONFLICT (name) DO UPDATE SET description = EXCLUDED.description;

INSERT INTO transactions (
  id, message_id, amount, currency, timestamp, merchant_info, category, bucket, source, description, muted, muted_by_merchant, mute_reason
) VALUES
  ('11111111-1111-1111-1111-111111111111', 'seed-msg-1', 249.50, 'INR', '2026-05-19T12:30:00Z', 'Swiggy', 'Food & Dining', 'Wants', 'gmail', 'Lunch order', false, false, NULL),
  ('22222222-2222-2222-2222-222222222222', 'seed-msg-2', 520.00, 'INR', '2026-05-19T08:15:00Z', 'Uber', 'Transport', 'Needs', 'gmail', 'Airport ride', false, false, NULL),
  ('33333333-3333-3333-3333-333333333333', 'seed-msg-3', 15.99, 'USD', '2026-05-19T18:45:00Z', 'Netflix', 'Entertainment', 'Wants', 'gmail', 'Monthly subscription', false, false, NULL),
  ('44444444-4444-4444-4444-444444444444', 'seed-msg-4', 1000.00, 'USD', '2026-05-19T05:00:00Z', 'Internal Transfer', 'Income', 'Transfers', 'thunderbird', 'Savings move', true, false, 'internal transfer')
ON CONFLICT (id) DO NOTHING;

INSERT INTO transaction_labels (transaction_id, label) VALUES
  ('33333333-3333-3333-3333-333333333333', 'Recurring')
ON CONFLICT (transaction_id, label) DO NOTHING;

INSERT INTO transaction_label_sources (transaction_id, label, source_type, merchant_pattern) VALUES
  ('33333333-3333-3333-3333-333333333333', 'Recurring', 'manual', '')
ON CONFLICT (transaction_id, label, source_type, merchant_pattern) DO NOTHING;
```

- [x] **Step 3: Smoke-test the stack without tests**

Run:

```bash
docker compose -f tests/component/docker-compose.yml up --build --no-start
docker compose -f tests/component/docker-compose.yml down -v --remove-orphans
```

Expected: services resolve and Compose exits 0.

---

### Task 3: Add Shared HTTP Test Helpers

**Files:**
- Create: `tests/component/helpers/client.go`
- Create: `tests/component/helpers/assertions.go`
- Create: `tests/component/helpers/waits.go`

- [x] **Step 1: Add the client helper**

Create `tests/component/helpers/client.go`:

```go
package helpers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func NewClient(t *testing.T) *Client {
	t.Helper()

	baseURL := strings.TrimRight(os.Getenv("COMPONENT_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = "http://backend:8080"
	}

	return &Client{
		BaseURL: baseURL,
		HTTP: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) Get(t *testing.T, path string) *http.Response {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		t.Fatalf("new GET request: %v", err)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		t.Fatalf("do GET %s: %v", path, err)
	}
	return resp
}

func (c *Client) JSON(t *testing.T, method, path string, body any) *http.Response {
	t.Helper()

	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}

	req, err := http.NewRequestWithContext(t.Context(), method, c.BaseURL+path, &buf)
	if err != nil {
		t.Fatalf("new %s request: %v", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		t.Fatalf("do %s %s: %v", method, path, err)
	}
	return resp
}
```

- [x] **Step 2: Add the assertion helper**

Create `tests/component/helpers/assertions.go`:

```go
package helpers

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func RequireStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("status=%d want=%d body=%s", resp.StatusCode, want, string(body))
	}
}

func DecodeJSON[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()

	var out T
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response JSON: %v", err)
	}
	return out
}
```

- [x] **Step 3: Add the wait helper**

Create `tests/component/helpers/waits.go`:

```go
package helpers

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func WaitForHealthy(t *testing.T) string {
	t.Helper()

	baseURL := strings.TrimRight(os.Getenv("COMPONENT_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = "http://backend:8080"
	}

	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(30 * time.Second)

	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, baseURL+"/api/health", nil)
		if err == nil {
			resp, doErr := client.Do(req)
			if doErr == nil {
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return baseURL
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Fatalf("backend did not become healthy before timeout")
	return ""
}
```

- [x] **Step 4: Verify the helpers compile**

Run:

```bash
cd tests/component && go test ./helpers
```

Expected: `? github.com/ArionMiles/expensor/tests/component/helpers [no test files]`

---

### Task 4: Add the First Health and Settings Suites

**Files:**
- Create: `tests/component/health_test.go`
- Create: `tests/component/settings_test.go`

- [x] **Step 1: Add the build-tagged health suite**

Create `tests/component/health_test.go` as a table-driven suite:

```go
//go:build component

package component_test

import (
	"testing"

	"github.com/ArionMiles/expensor/tests/component/helpers"
)

func TestHealthVersionAndActiveReader(t *testing.T) {
	helpers.WaitForHealthy(t)
	client := helpers.NewClient(t)

	cases := []struct {
		name   string
		path   string
		assert func(t *testing.T, body map[string]string)
	}{
		{
			name: "health ok",
			path: "/api/health",
			assert: func(t *testing.T, body map[string]string) {
				t.Helper()
				if body["status"] != "ok" {
					t.Fatalf("unexpected health payload: %#v", body)
				}
			},
		},
		{
			name: "version present",
			path: "/api/version",
			assert: func(t *testing.T, body map[string]string) {
				t.Helper()
				if body["version"] == "" {
					t.Fatalf("expected non-empty version payload, got %#v", body)
				}
			},
		},
		{
			name: "active reader empty",
			path: "/api/config/active-reader",
			assert: func(t *testing.T, body map[string]string) {
				t.Helper()
				if body["reader"] != "" {
					t.Fatalf("expected no active reader, got %#v", body)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := client.Get(t, tc.path)
			helpers.RequireStatus(t, resp, 200)
			body := helpers.DecodeJSON[map[string]string](t, resp)
			tc.assert(t, body)
		})
	}
}
```

- [x] **Step 2: Add the build-tagged settings suite**

Create `tests/component/settings_test.go` as a table-driven suite:

```go
//go:build component

package component_test

import (
	"net/http"
	"testing"

	"github.com/ArionMiles/expensor/tests/component/helpers"
)

func TestSettingsRoundTripAndCheckpointClear(t *testing.T) {
	helpers.WaitForHealthy(t)
	client := helpers.NewClient(t)

	readCases := []struct {
		name string
		path string
		key  string
		want string
	}{
		{name: "base currency", path: "/api/config/base-currency", key: "base_currency", want: "USD"},
		{name: "scan interval", path: "/api/config/scan-interval", key: "scan_interval", want: "120"},
		{name: "timezone", path: "/api/config/timezone", key: "timezone", want: "Asia/Kolkata"},
	}

	for _, tc := range readCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := client.Get(t, tc.path)
			helpers.RequireStatus(t, resp, http.StatusOK)
			body := helpers.DecodeJSON[map[string]string](t, resp)
			if body[tc.key] != tc.want {
				t.Fatalf("unexpected payload for %s: %#v", tc.name, body)
			}
		})
	}

	t.Run("update base currency", func(t *testing.T) {
		updateCurrency := client.JSON(t, http.MethodPut, "/api/config/base-currency", map[string]string{"base_currency": "INR"})
		helpers.RequireStatus(t, updateCurrency, http.StatusOK)
	})

	t.Run("checkpoint exists then clears", func(t *testing.T) {
		checkpoint := client.Get(t, "/api/config/readers/gmail/checkpoint")
		helpers.RequireStatus(t, checkpoint, http.StatusOK)
		checkpointBody := helpers.DecodeJSON[map[string]any](t, checkpoint)
		if checkpointBody["last_scan_at"] == nil {
			t.Fatalf("expected seeded checkpoint, got %#v", checkpointBody)
		}

		clearCheckpoint := client.JSON(t, http.MethodDelete, "/api/config/readers/gmail/checkpoint", nil)
		helpers.RequireStatus(t, clearCheckpoint, http.StatusNoContent)

		checkpointAfter := client.Get(t, "/api/config/readers/gmail/checkpoint")
		helpers.RequireStatus(t, checkpointAfter, http.StatusOK)
		checkpointAfterBody := helpers.DecodeJSON[map[string]any](t, checkpointAfter)
		if checkpointAfterBody["last_scan_at"] != nil {
			t.Fatalf("expected null checkpoint after clear, got %#v", checkpointAfterBody)
		}
	})
}
```

- [x] **Step 3: Run the first two suites**

Run:

```bash
docker compose -f tests/component/docker-compose.yml up --build --abort-on-container-exit --exit-code-from runner
docker compose -f tests/component/docker-compose.yml down -v --remove-orphans
```

Expected: runner exits 0 with the two component test files passing.

---

### Task 5: Add the First Taxonomy and Transaction Suites

**Files:**
- Create: `tests/component/taxonomy_test.go`
- Create: `tests/component/transactions_test.go`

- [x] **Step 1: Add the taxonomy suite**

Create `tests/component/taxonomy_test.go` as a table-driven suite:

```go
//go:build component

package component_test

import (
	"net/http"
	"testing"

	"github.com/ArionMiles/expensor/tests/component/helpers"
)

func TestTaxonomyListAndLabelMutationFlow(t *testing.T) {
	helpers.WaitForHealthy(t)
	client := helpers.NewClient(t)

	listCases := []struct {
		name string
		path string
	}{
		{name: "labels list", path: "/api/config/labels"},
		{name: "categories list", path: "/api/config/categories"},
		{name: "buckets list", path: "/api/config/buckets"},
	}

	for _, tc := range listCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := client.Get(t, tc.path)
			helpers.RequireStatus(t, resp, http.StatusOK)
			rows := helpers.DecodeJSON[[]map[string]any](t, resp)
			if len(rows) == 0 {
				t.Fatalf("expected seeded rows for %s", tc.name)
			}
		})
	}

	t.Run("create and apply label", func(t *testing.T) {
		createLabel := client.JSON(t, http.MethodPost, "/api/config/labels", map[string]string{
			"name":  "ComponentLabel",
			"color": "#22c55e",
		})
		helpers.RequireStatus(t, createLabel, http.StatusCreated)

		applyLabel := client.JSON(t, http.MethodPost, "/api/config/labels/ComponentLabel/apply", map[string]string{
			"merchant_pattern": "Swiggy",
		})
		helpers.RequireStatus(t, applyLabel, http.StatusOK)
		applyBody := helpers.DecodeJSON[map[string]any](t, applyLabel)
		if applyBody["applied"] == nil {
			t.Fatalf("expected applied count, got %#v", applyBody)
		}
	})
}
```

- [x] **Step 2: Add the transactions suite**

Create `tests/component/transactions_test.go` as a table-driven suite:

```go
//go:build component

package component_test

import (
	"net/http"
	"testing"

	"github.com/ArionMiles/expensor/tests/component/helpers"
)

func TestTransactionsSeededFiltersAndMutations(t *testing.T) {
	helpers.WaitForHealthy(t)
	client := helpers.NewClient(t)

	readCases := []struct {
		name   string
		path   string
		assert func(t *testing.T, body map[string]any)
	}{
		{
			name: "category filter",
			path: "/api/transactions?page=1&page_size=10&category=Food%20%26%20Dining",
			assert: func(t *testing.T, body map[string]any) {
				t.Helper()
				if int(body["total"].(float64)) != 1 {
					t.Fatalf("expected one Food & Dining transaction, got %#v", body)
				}
			},
		},
		{
			name: "facets available",
			path: "/api/transactions/facets",
			assert: func(t *testing.T, body map[string]any) {
				t.Helper()
				if len(body["categories"].([]any)) == 0 {
					t.Fatalf("expected seeded categories in facets, got %#v", body)
				}
			},
		},
	}

	for _, tc := range readCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := client.Get(t, tc.path)
			helpers.RequireStatus(t, resp, http.StatusOK)
			body := helpers.DecodeJSON[map[string]any](t, resp)
			tc.assert(t, body)
		})
	}

	mutationCases := []struct {
		name   string
		method string
		path   string
		body   any
		want   int
	}{
		{
			name:   "update transaction",
			method: http.MethodPut,
			path:   "/api/transactions/11111111-1111-1111-1111-111111111111",
			body: map[string]string{
				"description": "Updated lunch order",
				"category":    "Food & Dining",
				"bucket":      "Needs",
			},
			want: http.StatusOK,
		},
		{
			name:   "add label",
			method: http.MethodPost,
			path:   "/api/transactions/11111111-1111-1111-1111-111111111111/labels",
			body: map[string][]string{
				"labels": []string{"Bills"},
			},
			want: http.StatusOK,
		},
		{
			name:   "mute transaction",
			method: http.MethodPut,
			path:   "/api/transactions/11111111-1111-1111-1111-111111111111/mute",
			body: map[string]any{
				"muted":  true,
				"reason": "component test mute",
			},
			want: http.StatusOK,
		},
		{
			name:   "update mute reason",
			method: http.MethodPut,
			path:   "/api/transactions/11111111-1111-1111-1111-111111111111/mute-reason",
			body: map[string]string{
				"reason": "component test reason",
			},
			want: http.StatusOK,
		},
	}

	for _, tc := range mutationCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := client.JSON(t, tc.method, tc.path, tc.body)
			helpers.RequireStatus(t, resp, tc.want)
		})
	}
}
```

- [x] **Step 3: Run the full component suite**

Run:

```bash
docker compose -f tests/component/docker-compose.yml up --build --abort-on-container-exit --exit-code-from runner
docker compose -f tests/component/docker-compose.yml down -v --remove-orphans
```

Expected: all four component test files pass.

---

### Task 6: Add the Taskfile Target and Coverage Post-Processing

**Files:**
- Modify: `Taskfile.yml`

- [x] **Step 1: Add the Task target**

Add this target to `Taskfile.yml`:

```yaml
  test:be:component:
    summary: Run Docker Compose-backed backend component tests.
    desc: >-
      Starts the component-test Compose stack under tests/component, seeds a
      deterministic PostgreSQL dataset, runs the Go stdlib black-box API tests,
      then post-processes runtime backend coverage from GOCOVERDIR into a
      human-readable summary and text coverage profile.
    cmds:
      - rm -rf tests/component/artifacts
      - mkdir -p tests/component/artifacts/backend-coverage
      - docker compose -f tests/component/docker-compose.yml down -v --remove-orphans
      - docker compose -f tests/component/docker-compose.yml up --build --abort-on-container-exit --exit-code-from runner
      - docker compose -f tests/component/docker-compose.yml down -v --remove-orphans
      - cd backend && go tool covdata percent -i=../tests/component/artifacts/backend-coverage
      - cd backend && go tool covdata textfmt -i=../tests/component/artifacts/backend-coverage -o=../tests/component/artifacts/backend-component.coverage.out
```

- [x] **Step 2: Verify the target is discoverable**

Run:

```bash
task --list | rg "test:be:component"
```

Expected: `test:be:component` appears with the descriptive text.

- [x] **Step 3: Verify the task produces component coverage**

Run:

```bash
task test:be:component
test -f tests/component/artifacts/backend-component.coverage.out
```

Expected: task exits 0 and the text coverage file exists.

---

### Task 7: Add the CI Job

**Files:**
- Modify: `.github/workflows/ci.yml`

- [x] **Step 1: Add the backend component-test job**

Add a CI job with this shape:

```yaml
  test-backend-component:
    name: Test (backend component)
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6

      - name: Set up Go
        uses: actions/setup-go@v6
        with:
          go-version-file: 'backend/go.mod'
          cache-dependency-path: 'backend/go.sum'

      - name: Install Task
        uses: go-task/setup-task@v2
        with:
          version: 3.x

      - name: Run backend component tests
        run: task test:be:component

      - name: Upload backend component coverage artifact
        uses: actions/upload-artifact@v4
        with:
          name: backend-component-coverage
          path: tests/component/artifacts
```

- [x] **Step 2: Verify the workflow diff**

Run:

```bash
git diff -- .github/workflows/ci.yml
```

Expected: a dedicated backend component-test job is visible and readable.

---

### Task 8: Final Verification and Status Reflection

**Files:**
- Modify: `docs/superpowers/specs/2026-05-19-frontend-testing.md`
- Modify: `docs/superpowers/plans/2026-05-19-testing-program.md`
- Modify: `docs/superpowers/plans/2026-05-19-testing-phase-3-backend-component.md`

- [x] **Step 1: Run the final verification commands**

Run:

```bash
cd tests/component && go test ./helpers
cd backend && go test ./internal/api ./cmd/server
task test:be:component
```

Expected:

- helper package compiles
- backend unit packages still pass
- component suite passes
- `tests/component/artifacts/backend-component.coverage.out` exists
- endpoint cases are organized as table entries, not scattered one-off tests

- [x] **Step 2: Mark the workstream complete**

Update:

- spec Phase 3 row → `Complete`
- program index Phase 3 checkbox → checked and `Status: Complete`
- this plan status block → `Complete`

- [x] **Step 3: Record the execution note**

Add an execution note to this plan summarizing:

- the Compose stack files created
- the initial test files landed
- the Task target added
- the CI job added
- the location of the component coverage artifact

---

## Exit Criteria

- `tests/component` exists with Compose, seed SQL, helpers, and build-tagged Go tests
- `task test:be:component` runs locally
- CI has a dedicated backend component-test job
- component coverage is collected from the running backend binary via `GOCOVERDIR`
- initial seeded suites cover health, settings, taxonomy, and transactions
