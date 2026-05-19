# Remove Runtime Compatibility Importer Implementation Plan

**Status:** Pending. Do not execute until the two known existing users have updated Docker images that include DB-backed runtime state.

**Goal:** Delete the temporary file-to-DB runtime importer after existing users have updated Docker images.

## Delete

- `backend/internal/compat/runtime_importer.go`
- `backend/internal/compat/runtime_importer_test.go`
- The startup call to `compat.NewRuntimeImporter(...).Import(...)` in `backend/cmd/server/main.go`
- Any comments that mention temporary legacy runtime file import.

## Verify

- `rg -n "RuntimeImporter|legacy runtime|runtime_importer|client_secret_.*json|token_.*json|config_.*json|active_reader|state.json" backend`
- `task test:be`
- `task lint:be:prod`
