# Remove Runtime Compatibility Importer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Status:** Complete. The two known existing users have updated Docker images that include DB-backed runtime state, and the compatibility importer has been removed.

**Goal:** Delete the temporary file-to-DB runtime importer now that normal runtime credentials, reader config, active reader selection, OAuth tokens, and processed-message state are stored in PostgreSQL.

**Architecture:** Remove the isolated compatibility package and the single startup call in `backend/cmd/server/main.go`. Keep DB-backed runtime store paths intact, then search for any remaining normal runtime file references and delete only dead helpers that are no longer referenced outside their own tests.

**Tech Stack:** Go, PostgreSQL runtime store, existing Taskfile targets, ripgrep.

---

### Task 1: Remove Startup Import Wiring

**Files:**
- Modify: `backend/cmd/server/main.go`
- Test: `backend/cmd/server/main_test.go`

- [x] **Step 1: Write the failing test**

Add or update a startup-oriented test so `backend/cmd/server/main.go` no longer imports `backend/internal/compat` and no longer calls `compat.NewRuntimeImporter(...).Import(...)`.

```go
func TestMainDoesNotImportLegacyRuntimeFiles(t *testing.T) {
	data, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	body := string(data)
	for _, disallowed := range []string{
		"backend/internal/compat",
		"NewRuntimeImporter",
		"legacy runtime",
		"runtime_importer",
	} {
		if strings.Contains(body, disallowed) {
			t.Fatalf("main.go still contains %q", disallowed)
		}
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `task test:be -- ./cmd/server -run TestMainDoesNotImportLegacyRuntimeFiles`

Expected: FAIL because `backend/cmd/server/main.go` still imports and calls the compatibility importer.

- [x] **Step 3: Remove startup importer call**

Delete the `backend/internal/compat` import from `backend/cmd/server/main.go` and remove:

```go
if result, err := compat.NewRuntimeImporter(cfg.DataDir, pgStore, logger.With("component", "runtime_importer")).Import(ctx); err != nil {
	logger.Warn("legacy runtime import failed", "error", err)
} else if result.ImportedFiles > 0 {
	logger.Info("legacy runtime files imported", "files", result.ImportedFiles)
}
```

- [x] **Step 4: Run test to verify it passes**

Run: `task test:be -- ./cmd/server -run TestMainDoesNotImportLegacyRuntimeFiles`

Expected: PASS.

### Task 2: Delete Compatibility Importer Package

**Files:**
- Delete: `backend/internal/compat/runtime_importer.go`
- Delete: `backend/internal/compat/runtime_importer_test.go`

- [x] **Step 1: Delete importer package files**

Remove both compatibility importer files. No replacement package is needed because normal runtime code already uses PostgreSQL store methods.

- [x] **Step 2: Verify package references are gone**

Run: `rg -n "RuntimeImporter|runtime_importer|backend/internal/compat|legacy runtime" backend`

Expected: no matches outside the guard test that lists disallowed strings.

- [x] **Step 3: Run backend package tests**

Run: `task test:be -- ./cmd/server ./internal/... ./pkg/client ./pkg/state`

Expected: PASS.

### Task 3: Remove Proven-Dead File Credential Helpers

**Files:**
- Modify if dead: `backend/pkg/client/oauth.go`
- Modify if dead: `backend/pkg/client/oauth_test.go`
- Modify if dead: `backend/pkg/state/state.go`
- Modify if dead: `backend/pkg/state/state_test.go`

- [x] **Step 1: Search for remaining file runtime references**

Run:

```bash
rg -n "TokenFromFile|SaveToken|client\\.New\\(|NewFromJSON\\(|state\\.New\\(|client_secret_.*json|token_.*json|config_.*json|active_reader|state.json" backend
```

Expected: production runtime paths should not reference legacy credential/state files. Test-only references are acceptable if they exercise still-supported helper APIs.

- [x] **Step 2: Remove helpers only if they are self-contained dead code**

If `TokenFromFile`, `SaveToken`, `client.New`, or file-backed `state.New` are referenced only by their own tests and not by production/plugin construction, remove the helper and its tests in the same change. If a helper is still used by production code or broader reader/plugin tests, leave it in place and record it as a separate follow-up.

- [x] **Step 3: Run narrow tests**

Run: `task test:be -- ./pkg/client ./pkg/state ./pkg/plugins/readers/... ./pkg/reader/...`

Expected: PASS.

### Task 4: Final Verification

**Files:**
- Modify only if verification exposes stale references.

- [x] **Step 1: Run final runtime-file search**

Run:

```bash
rg -n "RuntimeImporter|legacy runtime|runtime_importer|client_secret_.*json|token_.*json|config_.*json|active_reader|state.json" backend
```

Expected: matches are limited to tests, user-facing descriptions of accepted upload file names, Thunderbird profile fixtures, config defaults, or intentionally retained helper APIs. No normal runtime startup, handler, daemon, OAuth token refresh, or processed-message path should read/write legacy runtime files.

- [x] **Step 2: Format backend**

Run: `task fmt:be`

Expected: PASS.

- [x] **Step 3: Run backend tests**

Run: `task test:be`

Expected: PASS.

- [x] **Step 4: Run strict backend lint**

Run: `task lint:be:prod`

Expected: PASS with `0 issues`.
