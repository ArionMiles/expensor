# Multi-Tenant Accounts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Phase 1 of issue #24: secure multi-tenant accounts with browser sessions, programmatic tokens, user-as-tenant data scoping, encrypted per-user reader runtime, admin-created users, bundled SVG avatars, and removable single-user migration.

**Architecture:** Add a dedicated `backend/internal/auth` package for principals, password/session/token helpers, and request context helpers. Add auth/runtime repositories under `backend/internal/store`, then require explicit tenant/principal arguments across tenant-owned store capabilities. In Phase 1 each user is the tenant, so `tenant_id` values are `users.id`; no separate tenant hierarchy is introduced. The frontend adds an auth gate and account/profile/admin surfaces before entering the existing app shell.

**Tech Stack:** Go 1.26, net/http ServeMux, pgx/PostgreSQL, golang-migrate, envconfig, React, TanStack Query, React Router, Vitest, Playwright, OpenAPI/Schemathesis.

---

## Command Note

Use `task` targets for formatting, broad tests, linting, OpenAPI, component, contract, and browser verification. The current `Taskfile.yml` does not expose package/test-name argument passthrough for narrow red-green loops, so task steps use direct `go test` or frontend test-runner commands only where a focused failing/passing check is required.

---

## File Structure

Create:

- `backend/internal/auth/principal.go`: principal and role types.
- `backend/internal/auth/context.go`: request context helpers.
- `backend/internal/auth/password.go`: bcrypt password hashing and verification.
- `backend/internal/auth/tokens.go`: opaque token generation and SHA-256 token hashing.
- `backend/internal/auth/crypto.go`: authenticated encryption for reader secrets.
- `backend/internal/auth/*_test.go`: unit tests for auth primitives.
- `backend/internal/httpapi/handlers_auth.go`: bootstrap, session, profile, token, account setup, and admin user handlers.
- `backend/internal/httpapi/auth_middleware.go`: public route allowlist and authenticated principal injection.
- `backend/internal/store/auth_repository.go`: users, sessions, access tokens, and setup tokens.
- `backend/internal/store/secretbox.go`: repository helper for encrypting and decrypting reader runtime secrets.
- `backend/internal/bootstrap/legacy.go`: isolated legacy single-user claim/import flow.
- `backend/internal/bootstrap/legacy_test.go`: isolated legacy migration tests.
- `backend/migrations/004_multi_tenant_accounts.up.sql`: permanent auth schema.
- `backend/migrations/004_multi_tenant_accounts.down.sql`: rollback for migration tests.
- `docs/deployment/secrets.md`: deployment guidance for `EXPENSOR_SECRET_KEY` and `EXPENSOR_SECRET_KEY_FILE`.
- `scripts/secrets/generate-key.sh`: helper that prints a base64-encoded 32-byte key.
- `content/avatars/catalog.json`: bundled avatar metadata.
- `content/avatars/default.svg`: default avatar.
- `content/avatars/ledger.svg`: additional built-in avatar.
- `content/avatars/wallet.svg`: additional built-in avatar.
- `frontend/src/assets/avatars.ts`: frontend avatar catalog loader and key helpers.
- `frontend/src/pages/Login.tsx`: login page.
- `frontend/src/pages/BootstrapAdmin.tsx`: first-admin setup page.
- `frontend/src/pages/AccountSetup.tsx`: setup-token password page.
- `frontend/src/pages/settings/ProfileSettings.tsx`: profile and avatar settings.
- `frontend/src/pages/settings/TokenSettings.tsx`: programmatic token management.
- `frontend/src/pages/settings/AdminUsersSettings.tsx`: admin user management.
- `frontend/src/components/AuthGate.tsx`: bootstrap/session gate for routes.
- `frontend/src/contexts/AuthContext.tsx`: current session context.

Modify:

- `backend/go.mod`, `backend/go.sum`: add `golang.org/x/crypto` for bcrypt.
- `backend/pkg/config/config.go`: add auth/session/encryption config.
- `Taskfile.yml`: add `secrets:generate` helper target.
- `backend/cmd/server/main.go`: pass auth/encryption dependencies into handlers and store/runtime wiring.
- `backend/internal/httpapi/server.go`: register RESTful auth/profile/token/admin/account routes and wrap private routes.
- `backend/internal/httpapi/store.go`: include auth store capability.
- `backend/internal/httpapi/store_capabilities.go`: add tenant/principal arguments to tenant-owned capabilities.
- `backend/internal/httpapi/handlers_*.go`: read principal from context and pass tenant explicitly.
- `backend/internal/httpapi/handlers_test.go`: extend `mockStore` with auth methods and tenant argument assertions.
- `backend/internal/store/models.go`: add tenant/auth DTOs and `TenantID` fields where returned internally.
- `backend/internal/store/runtime_repository.go`: tenant-scope app preferences, reader runtime, and processed messages.
- `backend/internal/store/*_repository.go`: tenant-scope transactions, read models, taxonomy, rules, diagnostics, muted merchants, and user-owned merchant overrides.
- `backend/internal/store/instrumented.go`: mirror new tenant-aware method signatures without adding high-cardinality attributes.
- `backend/internal/daemon/runner.go`, `backend/cmd/server/daemon.go`: carry tenant identity for reader operations.
- `api/openapi/expensor.openapi.yaml`: document new auth/security routes and protected endpoint security.
- `tests/component/helpers/client.go`: add authenticated request helpers.
- `tests/component/*.go`: create/bootstrap/login users before private API calls.
- `tests/contract/allowlist.tsv`, `tests/contract/exclusions.tsv`: update only for documented protected routes.
- `frontend/vite.config.ts`: add an alias to repository-root `content/avatars` and allow it in dev server filesystem access.
- `frontend/src/api/types.ts`: add auth/profile/token/admin types.
- `frontend/src/api/client.ts`: add session/profile/token/admin/account setup clients and preserve cookie credentials.
- `frontend/src/api/queries.ts`: add auth/profile/token/admin query hooks.
- `frontend/src/App.tsx`: replace first-run-only gate with auth gate and public auth routes.
- `frontend/src/pages/Settings.tsx`: add profile, tokens, and admin users settings tabs with URL state.
- `frontend/src/i18n/messages.ts`: add user-facing strings for new auth/account UI.
- `frontend/src/mocks/handlers.ts`: add MSW handlers for auth flows.
- `frontend/playwright/*`: add browser coverage for bootstrap, login/logout, and account setup.

---

### Task 1: Auth Primitives

**Files:**
- Create: `backend/internal/auth/principal.go`
- Create: `backend/internal/auth/context.go`
- Create: `backend/internal/auth/password.go`
- Create: `backend/internal/auth/tokens.go`
- Create: `backend/internal/auth/crypto.go`
- Create: `backend/internal/auth/password_test.go`
- Create: `backend/internal/auth/tokens_test.go`
- Create: `backend/internal/auth/context_test.go`
- Create: `backend/internal/auth/crypto_test.go`
- Modify: `backend/go.mod`
- Modify: `backend/go.sum`

- [ ] **Step 1: Write failing auth primitive tests**

Add tests that prove the security behavior before implementation:

```go
func TestPasswordHashVerify(t *testing.T) {
	hash, err := auth.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash == "correct horse battery staple" || !strings.HasPrefix(hash, "$2") {
		t.Fatalf("hash %q does not look like bcrypt", hash)
	}
	if err := auth.VerifyPassword(hash, "correct horse battery staple"); err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if err := auth.VerifyPassword(hash, "wrong"); err == nil {
		t.Fatal("VerifyPassword() succeeded for wrong password")
	}
}

func TestTokenHashDoesNotExposeRawToken(t *testing.T) {
	raw, hash, err := auth.NewOpaqueToken("expensor_pat")
	if err != nil {
		t.Fatalf("NewOpaqueToken() error = %v", err)
	}
	if !strings.HasPrefix(raw, "expensor_pat_") {
		t.Fatalf("raw token prefix = %q", raw)
	}
	if strings.Contains(hash, raw) {
		t.Fatalf("hash contains raw token")
	}
	if got := auth.HashOpaqueToken(raw); got != hash {
		t.Fatalf("HashOpaqueToken() = %q, want %q", got, hash)
	}
}

func TestSealOpenBindsAssociatedData(t *testing.T) {
	key := bytes.Repeat([]byte{7}, auth.SecretKeySize)
	box, err := auth.NewSecretBox(key)
	if err != nil {
		t.Fatalf("NewSecretBox() error = %v", err)
	}
	associated := auth.SecretAssociatedData{TenantID: "tenant-a", Reader: "gmail", Kind: "oauth_token"}
	ciphertext, err := box.Seal([]byte(`{"access_token":"secret"}`), associated)
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	if bytes.Contains(ciphertext, []byte("access_token")) {
		t.Fatal("ciphertext contains plaintext")
	}
	if _, err := box.Open(ciphertext, auth.SecretAssociatedData{TenantID: "tenant-b", Reader: "gmail", Kind: "oauth_token"}); err == nil {
		t.Fatal("Open() succeeded with wrong tenant associated data")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/auth -run 'TestPasswordHashVerify|TestTokenHashDoesNotExposeRawToken|TestSealOpenBindsAssociatedData' -count=1`

Expected: FAIL because `backend/internal/auth` does not exist.

- [ ] **Step 3: Implement auth primitive types**

Add these shapes:

```go
package auth

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

type Principal struct {
	UserID     string
	TenantID   string
	Role       Role
	AuthMethod string
}

type SecretAssociatedData struct {
	TenantID string
	Reader   string
	Kind     string
}
```

Use:

- `golang.org/x/crypto/bcrypt` in `password.go`.
- `crypto/rand` plus URL-safe base64 in `tokens.go`.
- SHA-256 hex hashes for session IDs, setup tokens, and programmatic tokens.
- `crypto/aes` with `cipher.NewGCM` in `crypto.go`.
- an unexported context key in `context.go` with `WithPrincipal(ctx, p)` and `PrincipalFromContext(ctx)`.

- [ ] **Step 4: Add bcrypt dependency and tidy modules**

Run: `go get golang.org/x/crypto/bcrypt` from `backend/`, then `go mod tidy`.

Expected: `backend/go.mod` includes `golang.org/x/crypto`.

- [ ] **Step 5: Run auth primitive tests**

Run: `cd backend && go test ./internal/auth -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/go.mod backend/go.sum backend/internal/auth
git commit --no-gpg-sign -m "feat: add auth security primitives"
```

---

### Task 2: Tenant/Auth Schema And Store Repository

**Files:**
- Create: `backend/migrations/004_multi_tenant_accounts.up.sql`
- Create: `backend/migrations/004_multi_tenant_accounts.down.sql`
- Create: `backend/internal/store/auth_repository.go`
- Create: `backend/internal/store/auth_repository_test.go`
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/store/models.go`
- Modify: `backend/internal/store/instrumented.go`
- Modify: `backend/internal/httpapi/store_capabilities.go`
- Modify: `backend/internal/httpapi/store.go`

- [ ] **Step 1: Write failing store tests**

Add tests in `backend/internal/store/auth_repository_test.go`:

```go
func TestAuthRepositoryBootstrapAndSessionLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := newTestStore(t)

	required, err := st.BootstrapRequired(ctx)
	if err != nil {
		t.Fatalf("BootstrapRequired() error = %v", err)
	}
	if !required {
		t.Fatal("BootstrapRequired() = false before users exist")
	}

	admin, err := st.CreateBootstrapAdmin(ctx, store.CreateBootstrapAdminInput{
		Email: "admin@example.com", DisplayName: "Admin", PasswordHash: "$2a$10$hash", AvatarKey: "default",
	})
	if err != nil {
		t.Fatalf("CreateBootstrapAdmin() error = %v", err)
	}
	if admin.TenantID != admin.ID || admin.Role != store.UserRoleAdmin {
		t.Fatalf("admin = %#v", admin)
	}

	_, err = st.CreateBootstrapAdmin(ctx, store.CreateBootstrapAdminInput{
		Email: "other@example.com", DisplayName: "Other", PasswordHash: "$2a$10$hash", AvatarKey: "default",
	})
	if !errors.Is(err, store.ErrBootstrapUnavailable) {
		t.Fatalf("second bootstrap error = %v, want ErrBootstrapUnavailable", err)
	}
}

func TestAuthRepositoryStoresOnlyTokenHashes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := newTestStore(t)
	admin := createTestUserWithTenant(t, st, "admin@example.com")

	token, err := st.CreateAccessToken(ctx, store.CreateAccessTokenInput{
		UserID: admin.ID, Name: "cli", TokenHash: "sha256:abc123", ExpiresAt: nil,
	})
	if err != nil {
		t.Fatalf("CreateAccessToken() error = %v", err)
	}
	if token.ID == "" || token.Name != "cli" {
		t.Fatalf("token = %#v", token)
	}

	found, err := st.FindAccessTokenByHash(ctx, "sha256:abc123")
	if err != nil {
		t.Fatalf("FindAccessTokenByHash() error = %v", err)
	}
	if found == nil || found.UserID != admin.ID {
		t.Fatalf("found = %#v", found)
	}
}
```

- [ ] **Step 2: Run store tests to verify they fail**

Run: `cd backend && go test ./internal/store -run 'TestAuthRepositoryBootstrapAndSessionLifecycle|TestAuthRepositoryStoresOnlyTokenHashes' -count=1`

Expected: FAIL because auth tables and methods do not exist.

- [ ] **Step 3: Add migration**

Create permanent schema with these tables and constraints. There is no `tenants` table in Phase 1; the user is the tenant. The authenticated principal exposes `tenant_id`, but it is equal to `users.id`.

```sql
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL,
    password_hash TEXT,
    display_name TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'user')),
    avatar_key TEXT NOT NULL DEFAULT 'default',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    disabled_at TIMESTAMPTZ,
    UNIQUE (email)
);

CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS access_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    UNIQUE (user_id, name)
);

CREATE TABLE IF NOT EXISTS account_setup_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ
);
```

The down migration drops these tables in dependency order.

- [ ] **Step 4: Add auth models and repository methods**

Add store models:

```go
type UserRole string

const (
	UserRoleAdmin UserRole = "admin"
	UserRoleUser  UserRole = "user"
)

type Tenant struct {
	ID string
}

type User struct {
	ID           string
	TenantID     string // equal to ID in Phase 1
	Email        string
	PasswordHash string
	DisplayName  string
	Role         UserRole
	AvatarKey    string
	DisabledAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
```

Add repository methods for bootstrap admin, user lookup by email/id, sessions, access tokens, account setup tokens, and admin-created users. Wrap duplicate email/name errors with existing store error conventions or add exported sentinel errors in `backend/internal/store/store.go`.

- [ ] **Step 5: Wire repository into Store and InstrumentedStore**

Add `auth *pgAuthRepository` to `Store`, initialize it in `initRepositories`, and forward methods on both `Store` and `InstrumentedStore`. Keep delegated calls visible in instrumented methods:

```go
func (s *InstrumentedStore) CreateSession(ctx context.Context, input CreateSessionInput) (*Session, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.create_session")
	defer span.End()

	session, err := s.next.CreateSession(ctx, input)
	s.recordOperation(ctx, "auth.create_session", err)
	return session, err
}
```

- [ ] **Step 6: Run migration and store tests**

Run: `cd backend && go test ./migrations ./internal/store -run 'TestAuthRepository|TestMigrations' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/migrations backend/internal/store backend/internal/httpapi/store.go backend/internal/httpapi/store_capabilities.go
git commit --no-gpg-sign -m "feat: add tenant auth storage"
```

---

### Task 3: Config And Encrypted Reader Runtime Foundation

**Files:**
- Modify: `backend/pkg/config/config.go`
- Modify: `backend/pkg/config/config_test.go`
- Create: `backend/internal/store/secretbox.go`
- Create: `backend/internal/store/secretbox_test.go`
- Create: `docs/deployment/secrets.md`
- Create: `scripts/secrets/generate-key.sh`
- Modify: `Taskfile.yml`
- Modify: `backend/internal/store/runtime_repository.go`
- Modify: `backend/internal/store/store_repositories_test.go`

- [ ] **Step 1: Write failing config and encryption tests**

Add config tests for explicit key loading:

```go
func TestLoadAuthEncryptionKey(t *testing.T) {
	t.Setenv("EXPENSOR_SECRET_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32)))
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Security.SecretKey) != 32 {
		t.Fatalf("SecretKey length = %d, want 32", len(cfg.Security.SecretKey))
	}
}

func TestLoadAuthEncryptionKeyFile(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "expensor_secret_key")
	if err := os.WriteFile(keyPath, []byte(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{8}, 32))+"\n"), 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	t.Setenv("EXPENSOR_SECRET_KEY_FILE", keyPath)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Security.SecretKey) != 32 {
		t.Fatalf("SecretKey length = %d, want 32", len(cfg.Security.SecretKey))
	}
}

func TestLoadRejectsBothSecretKeyInputs(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "expensor_secret_key")
	if err := os.WriteFile(keyPath, []byte(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{8}, 32))), 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	t.Setenv("EXPENSOR_SECRET_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32)))
	t.Setenv("EXPENSOR_SECRET_KEY_FILE", keyPath)
	if _, err := config.Load(); err == nil {
		t.Fatal("Load() succeeded with both EXPENSOR_SECRET_KEY and EXPENSOR_SECRET_KEY_FILE")
	}
}
```

Add runtime encryption tests:

```go
func TestReaderRuntimeEncryptsTokenPerTenant(t *testing.T) {
	ctx := context.Background()
	st := newTestStoreWithSecretKey(t, bytes.Repeat([]byte{4}, 32))
	tenantA := createTestUserWithTenant(t, st, "a@example.com").TenantID
	tenantB := createTestUserWithTenant(t, st, "b@example.com").TenantID

	if err := st.SetReaderToken(ctx, store.Tenant{ID: tenantA}, "gmail", []byte(`{"access_token":"a"}`)); err != nil {
		t.Fatalf("SetReaderToken() error = %v", err)
	}
	if _, found, err := st.GetReaderToken(ctx, store.Tenant{ID: tenantB}, "gmail"); err != nil || found {
		t.Fatalf("tenant B token = found %v err %v, want not found", found, err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./pkg/config ./internal/store -run 'TestLoadAuthEncryptionKey|TestLoadRejectsBothSecretKeyInputs|TestReaderRuntimeEncryptsTokenPerTenant' -count=1`

Expected: FAIL because config/security and tenant runtime signatures do not exist.

- [ ] **Step 3: Add security config**

Add:

```go
type Security struct {
	SecretKey []byte `envconfig:"EXPENSOR_SECRET_KEY"`
	SecretKeyFile string `envconfig:"EXPENSOR_SECRET_KEY_FILE"`
	SessionTTL time.Duration `envconfig:"EXPENSOR_SESSION_TTL" default:"168h"`
	SetupTokenTTL time.Duration `envconfig:"EXPENSOR_SETUP_TOKEN_TTL" default:"24h"`
}
```

Decode the secret from exactly one source in `Load()`:

- `EXPENSOR_SECRET_KEY`: base64-encoded 32-byte key.
- `EXPENSOR_SECRET_KEY_FILE`: file containing the base64-encoded 32-byte key, with surrounding whitespace trimmed.

If both are set, return a clear config error. If either source is set but does not decode to exactly 32 bytes, return a clear config error. Enforce startup failure in `cmd/server/main.go` once encrypted runtime is wired and neither source is set; tests may inject explicit keys. Never log the key or file contents.

- [ ] **Step 4: Add key-generation helper and deployment docs**

Create `scripts/secrets/generate-key.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail
openssl rand -base64 32
```

Add `secrets:generate` to `Taskfile.yml`:

```yaml
  secrets:generate:
    summary: Generate a base64-encoded 32-byte secret key for credential encryption.
    cmd: scripts/secrets/generate-key.sh
```

Create `docs/deployment/secrets.md` documenting:

- encryption is mandatory for reader client secrets and OAuth tokens
- `task secrets:generate`
- `.env` usage with `EXPENSOR_SECRET_KEY`
- Docker Compose secret-file usage with `EXPENSOR_SECRET_KEY_FILE=/run/secrets/expensor_secret_key`
- the key must be backed up, because losing it prevents decrypting stored reader credentials
- host compromise can still expose local secret files; this protects against accidental exposure and database-only compromise
- external secret managers can feed Expensor by exporting `EXPENSOR_SECRET_KEY` or rendering a file consumed by `EXPENSOR_SECRET_KEY_FILE`

- [ ] **Step 5: Tenant-scope reader runtime schema and repository**

Extend the migration from Task 2 or add `005_tenant_runtime.up.sql` if Task 2 is already merged:

```sql
ALTER TABLE reader_runtime ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES users(id);
ALTER TABLE processed_messages ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES users(id);
CREATE UNIQUE INDEX IF NOT EXISTS reader_runtime_tenant_reader_key ON reader_runtime (tenant_id, reader);
CREATE UNIQUE INDEX IF NOT EXISTS processed_messages_tenant_key ON processed_messages (tenant_id, message_key);
```

Change runtime methods to accept `store.Tenant` and use encrypted byte columns for secret/token data. Keep reader config JSON unencrypted unless it contains secrets.

- [ ] **Step 6: Run focused tests**

Run: `cd backend && go test ./pkg/config ./internal/store -run 'TestLoadAuthEncryptionKey|TestLoadRejectsBothSecretKeyInputs|TestReaderRuntime' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add Taskfile.yml scripts/secrets docs/deployment backend/pkg/config backend/internal/store backend/migrations
git commit --no-gpg-sign -m "feat: encrypt tenant reader runtime"
```

---

### Task 4: Auth HTTP Routes And Middleware

**Files:**
- Create: `backend/internal/httpapi/handlers_auth.go`
- Create: `backend/internal/httpapi/auth_middleware.go`
- Create: `backend/internal/httpapi/handlers_auth_test.go`
- Create: `backend/internal/httpapi/auth_middleware_test.go`
- Modify: `backend/internal/httpapi/server.go`
- Modify: `backend/internal/httpapi/handlers.go`
- Modify: `backend/internal/httpapi/http_helpers.go`
- Modify: `backend/internal/httpapi/store_capabilities.go`
- Modify: `backend/internal/httpapi/handlers_test.go`

- [ ] **Step 1: Write failing handler and middleware tests**

Add tests:

```go
func TestAuthMiddlewareAllowsOnlyPublicRoutes(t *testing.T) {
	h := NewHandlers(testHandlersConfig(t, &mockStore{}))
	mux := http.NewServeMux()
	registerRoutes(mux, h)
	server := authMiddleware(h, mux)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/transactions", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestBootstrapCreatesAdminAndSessionCookie(t *testing.T) {
	ms := &mockStore{bootstrapRequired: true}
	h := NewHandlers(testHandlersConfig(t, ms))
	body := strings.NewReader(`{"email":"admin@example.com","password":"passphrase with length","display_name":"Admin","avatar_key":"default"}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/bootstrap", body)
	rec := httptest.NewRecorder()

	h.Bootstrap(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if len(rec.Result().Cookies()) == 0 {
		t.Fatal("expected session cookie")
	}
	if ms.createdBootstrapAdmin.Email != "admin@example.com" {
		t.Fatalf("created admin = %#v", ms.createdBootstrapAdmin)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/httpapi -run 'TestAuthMiddlewareAllowsOnlyPublicRoutes|TestBootstrapCreatesAdminAndSessionCookie' -count=1`

Expected: FAIL because handlers and middleware do not exist.

- [ ] **Step 3: Implement RESTful auth routes**

Register:

```go
mux.HandleFunc("GET /api/bootstrap", h.GetBootstrap)
mux.HandleFunc("POST /api/bootstrap", h.Bootstrap)
mux.HandleFunc("POST /api/session", h.Login)
mux.HandleFunc("GET /api/session", h.GetSession)
mux.HandleFunc("DELETE /api/session", h.Logout)
mux.HandleFunc("GET /api/profile", h.GetProfile)
mux.HandleFunc("PATCH /api/profile", h.PatchProfile)
mux.HandleFunc("GET /api/tokens", h.ListTokens)
mux.HandleFunc("POST /api/tokens", h.CreateToken)
mux.HandleFunc("DELETE /api/tokens/{id}", h.RevokeToken)
mux.HandleFunc("GET /api/admin/users", h.ListUsers)
mux.HandleFunc("POST /api/admin/users", h.CreateUser)
mux.HandleFunc("PATCH /api/admin/users/{id}", h.UpdateUser)
mux.HandleFunc("POST /api/admin/users/{id}/setup-tokens", h.CreateSetupToken)
mux.HandleFunc("POST /api/account-setup", h.CompleteAccountSetup)
```

Use request-specific DTO validation in each handler. Return `400` for malformed JSON and `422` for semantic validation failures.

- [ ] **Step 4: Implement auth middleware**

The middleware checks session cookie first, then bearer token. On success it attaches `auth.Principal` to request context. On failure it returns `401` for protected routes. It allows only the public exceptions from the design.

Keep sensitive data out of logs. Do not log raw tokens or password validation errors beyond generic messages.

- [ ] **Step 5: Run focused HTTP tests**

Run: `cd backend && go test ./internal/httpapi -run 'TestAuth|TestBootstrap|TestSession|TestToken|TestAccountSetup|TestAdminUsers' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/httpapi
git commit --no-gpg-sign -m "feat: add account auth API"
```

---

### Task 5: Avatar Catalog And Frontend Bundling

**Files:**
- Create: `content/avatars/catalog.json`
- Create: `content/avatars/default.svg`
- Create: `content/avatars/ledger.svg`
- Create: `content/avatars/wallet.svg`
- Create: `frontend/src/assets/avatars.ts`
- Modify: `frontend/vite.config.ts`
- Modify: `frontend/src/vite-env.d.ts`
- Modify: `backend/internal/httpapi/handlers_auth_test.go`
- Modify: `backend/internal/httpapi/handlers_auth.go`

- [ ] **Step 1: Write failing avatar key validation tests**

Add:

```go
func TestAvatarCatalogRejectsUnknownKey(t *testing.T) {
	if !httpapi.ValidAvatarKey("default") {
		t.Fatal("default avatar not valid")
	}
	if httpapi.ValidAvatarKey("remote-url") {
		t.Fatal("unknown avatar key accepted")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/httpapi -run TestAvatarCatalogRejectsUnknownKey -count=1`

Expected: FAIL because avatar key validation does not exist.

- [ ] **Step 3: Add pure SVG avatars and catalog**

Use only SVG text assets under repository-root `content/avatars/`. Catalog shape:

```json
[
  {"key":"default","label":"Default","file":"default.svg"},
  {"key":"ledger","label":"Ledger","file":"ledger.svg"},
  {"key":"wallet","label":"Wallet","file":"wallet.svg"}
]
```

Each SVG must start with `<svg` and contain no scripts, external references, images, or event attributes.

- [ ] **Step 4: Bundle avatars from root content into the frontend**

Do not add `//go:embed` for avatar SVGs and do not add `GET /api/avatars`.

Update `frontend/vite.config.ts` to resolve a repository-root avatar alias. Preserve the existing `/api` proxy settings while adding `server.fs.allow`:

```ts
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const repoRoot = path.resolve(__dirname, '..')

export default defineConfig({
  resolve: {
    alias: {
      '@': '/src',
      '@avatar-content': path.resolve(repoRoot, 'content/avatars'),
    },
  },
  server: {
    fs: { allow: [repoRoot] },
  },
})
```

Create `frontend/src/assets/avatars.ts` that imports the root catalog plus SVG text assets using the alias:

```ts
import catalog from '@avatar-content/catalog.json'
import defaultSvg from '@avatar-content/default.svg?raw'
import ledgerSvg from '@avatar-content/ledger.svg?raw'
import walletSvg from '@avatar-content/wallet.svg?raw'

const svgByFile = {
  'default.svg': defaultSvg,
  'ledger.svg': ledgerSvg,
  'wallet.svg': walletSvg,
} as const

export const avatars = catalog.map((entry) => ({
  ...entry,
  svg: svgByFile[entry.file as keyof typeof svgByFile],
}))
```

Backend profile/account handlers validate only the submitted `avatar_key` against a small server-side allowlist. The allowlist must include `default` and match the catalog keys.

- [ ] **Step 5: Run focused avatar tests**

Run: `cd backend && go test ./internal/httpapi -run TestAvatar -count=1 && cd ../frontend && npm run test -- --run ProfileSettings`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add content/avatars frontend/vite.config.ts frontend/src/assets/avatars.ts frontend/src/vite-env.d.ts backend/internal/httpapi
git commit --no-gpg-sign -m "feat: add bundled account avatars"
```

---

### Task 6: Tenant-Scope Existing Store Capabilities

**Files:**
- Modify: `backend/internal/httpapi/store_capabilities.go`
- Modify: `backend/internal/httpapi/handlers_transactions.go`
- Modify: `backend/internal/httpapi/handlers_stats.go`
- Modify: `backend/internal/httpapi/handlers_taxonomy.go`
- Modify: `backend/internal/httpapi/handlers_rules.go`
- Modify: `backend/internal/httpapi/handlers_config.go`
- Modify: `backend/internal/httpapi/handlers_readers.go`
- Modify: `backend/internal/httpapi/handlers_diagnostics.go`
- Modify: `backend/internal/httpapi/handlers_daemon.go`
- Modify: `backend/internal/store/transactions_repository.go`
- Modify: `backend/internal/store/read_model_repository.go`
- Modify: `backend/internal/store/taxonomy_repository.go`
- Modify: `backend/internal/store/rules_repository.go`
- Modify: `backend/internal/store/runtime_repository.go`
- Modify: `backend/internal/store/diagnostics_repository.go`
- Modify: `backend/internal/store/community_repository.go` only for user-owned overrides, not global sync.
- Modify: `backend/internal/store/instrumented.go`
- Modify: `backend/internal/httpapi/handlers_test.go`
- Modify: `backend/internal/store/store_test.go`
- Modify: `backend/internal/store/ingestion_test.go`

- [ ] **Step 1: Write failing cross-tenant tests**

Add store integration tests for representative repositories first:

```go
func TestTransactionsAreTenantIsolated(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := newTestStore(t)
	a := createTestUserWithTenant(t, st, "a@example.com")
	b := createTestUserWithTenant(t, st, "b@example.com")

	txID := insertTestTransaction(t, st, store.Tenant{ID: a.TenantID}, "message-1")

	if _, err := st.GetTransaction(ctx, store.Tenant{ID: b.TenantID}, txID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("tenant B GetTransaction() err = %v, want ErrNotFound", err)
	}
}

func TestLabelsAreTenantIsolated(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := newTestStore(t)
	a := createTestUserWithTenant(t, st, "a@example.com")
	b := createTestUserWithTenant(t, st, "b@example.com")

	if err := st.CreateLabel(ctx, store.Tenant{ID: a.TenantID}, "travel", "#2563eb"); err != nil {
		t.Fatalf("tenant A CreateLabel() error = %v", err)
	}
	if err := st.CreateLabel(ctx, store.Tenant{ID: b.TenantID}, "travel", "#16a34a"); err != nil {
		t.Fatalf("tenant B CreateLabel() error = %v", err)
	}
	labels, err := st.ListLabels(ctx, store.Tenant{ID: b.TenantID})
	if err != nil {
		t.Fatalf("ListLabels() error = %v", err)
	}
	if len(labels) != 1 || labels[0].Color != "#16a34a" {
		t.Fatalf("tenant B labels = %#v", labels)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/store -run 'TestTransactionsAreTenantIsolated|TestLabelsAreTenantIsolated' -count=1`

Expected: FAIL because methods are not tenant-aware.

- [ ] **Step 3: Update capability signatures**

Change tenant-owned store interfaces to require a typed tenant:

```go
type transactionStore interface {
	ListTransactions(ctx context.Context, tenant store.Tenant, f store.ListFilter) ([]store.Transaction, store.TransactionListResult, error)
	GetTransaction(ctx context.Context, tenant store.Tenant, id string) (*store.Transaction, error)
	UpdateTransaction(ctx context.Context, tenant store.Tenant, id string, u store.TransactionUpdate) error
}
```

Apply the same pattern to taxonomy, rules, runtime, diagnostics, muted merchant, read-model, and ingestion-facing operations. Keep global content sync methods tenant-free.

- [ ] **Step 4: Update handlers**

Every private handler starts with:

```go
principal, ok := auth.PrincipalFromContext(r.Context())
if !ok {
	writeError(w, http.StatusUnauthorized, "authentication required")
	return
}
tenant := store.Tenant{ID: principal.TenantID}
```

Then pass `tenant` into the store method. Do not read tenant identity from the request body, path, or query string.

- [ ] **Step 5: Update SQL**

Every tenant-owned query includes `tenant_id = $n`, and every insert sets `tenant_id` from the method argument. For object lookup/update/delete, use tenant-filtered predicates:

```sql
UPDATE transactions
SET description = $3
WHERE tenant_id = $1 AND id = $2
```

- [ ] **Step 6: Run focused tenant tests**

Run: `cd backend && go test ./internal/httpapi ./internal/store -run 'Test.*Tenant|Test.*Auth|Test.*Transaction|Test.*Label|Test.*Rule|Test.*Runtime' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/httpapi backend/internal/store
git commit --no-gpg-sign -m "feat: enforce tenant-scoped stores"
```

---

### Task 7: Tenant-Aware Daemon And Ingestion

**Files:**
- Modify: `backend/internal/daemon/runner.go`
- Modify: `backend/internal/daemon/runner_test.go`
- Modify: `backend/internal/store/ingestion.go`
- Modify: `backend/internal/store/ingestion_test.go`
- Modify: `backend/cmd/server/daemon.go`
- Modify: `backend/cmd/server/daemon_test.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/internal/httpapi/handlers_daemon.go`

- [ ] **Step 1: Write failing daemon tenant tests**

Add tests:

```go
func TestDaemonStartUsesAuthenticatedTenant(t *testing.T) {
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	ms := &mockStore{}
	h := NewHandlers(testHandlersConfig(t, ms))
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/daemon/start", strings.NewReader(`{"reader":"gmail"}`))
	rec := httptest.NewRecorder()

	h.StartDaemon(rec, req)

	if ms.startedTenant.ID != "tenant-a" {
		t.Fatalf("started tenant = %q, want tenant-a", ms.startedTenant.ID)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./cmd/server ./internal/daemon ./internal/httpapi -run 'TestDaemon.*Tenant|Test.*Ingest.*Tenant' -count=1`

Expected: FAIL because daemon state is global.

- [ ] **Step 3: Add tenant to daemon contracts**

Change daemon coordinator callbacks from `func(reader string)` to typed inputs:

```go
type ReaderRunRequest struct {
	Tenant store.Tenant
	Reader string
}
```

Thread this through start, rescan, restart, runtime reads, processed-message checks, and transaction ingestion.

- [ ] **Step 4: Update ingestion**

`NewTransactionIngestor` receives a tenant or has `Ingest(ctx, tenant, transactions)` so inserted transactions always set `tenant_id`. Message uniqueness is `(tenant_id, message_id)`.

- [ ] **Step 5: Run daemon and ingestion tests**

Run: `cd backend && go test ./cmd/server ./internal/daemon ./internal/store ./internal/httpapi -run 'TestDaemon|TestIngestion|TestReader' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/cmd/server backend/internal/daemon backend/internal/httpapi backend/internal/store
git commit --no-gpg-sign -m "feat: make reader runtime tenant-aware"
```

---

### Task 8: Removable Legacy Single-User Migration

**Files:**
- Create: `backend/internal/bootstrap/legacy.go`
- Create: `backend/internal/bootstrap/legacy_test.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/internal/httpapi/handlers_auth.go`
- Modify: `backend/internal/store/auth_repository.go`
- Modify: `backend/internal/store/runtime_repository.go`

- [ ] **Step 1: Write failing legacy migration tests**

Add tests:

```go
func TestLegacyClaimPreviewCountsRows(t *testing.T) {
	ctx := context.Background()
	st := newTestStoreWithLegacyRows(t)
	preview, err := bootstrap.PreviewLegacyClaim(ctx, st)
	if err != nil {
		t.Fatalf("PreviewLegacyClaim() error = %v", err)
	}
	if preview.Transactions == 0 || preview.ReaderRuntime == 0 {
		t.Fatalf("preview = %#v", preview)
	}
}

func TestLegacyClaimAssignsRowsToFirstAdminTenantID(t *testing.T) {
	ctx := context.Background()
	st := newTestStoreWithLegacyRows(t)
	admin := createTestUserWithTenant(t, st, "admin@example.com")

	if err := bootstrap.ClaimLegacyData(ctx, st, store.Tenant{ID: admin.TenantID}); err != nil {
		t.Fatalf("ClaimLegacyData() error = %v", err)
	}
	assertNoNullTenantUserRows(t, st)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/bootstrap -count=1`

Expected: FAIL because package does not exist.

- [ ] **Step 3: Implement isolated legacy package**

Implement only these package-level operations:

```go
type LegacyPreview struct {
	Transactions int64 `json:"transactions"`
	Labels int64 `json:"labels"`
	Rules int64 `json:"rules"`
	ReaderRuntime int64 `json:"reader_runtime"`
	ProcessedMessages int64 `json:"processed_messages"`
	BlockingReasons []string `json:"blocking_reasons"`
}
```

`PreviewLegacyClaim` counts rows with `tenant_id IS NULL`. `ClaimLegacyData` assigns those rows to the first admin's tenant inside a transaction and verifies no tenant-owned legacy rows remain null.

- [ ] **Step 4: Wire into bootstrap**

When no users exist and legacy rows are present, `GET /api/bootstrap` returns preview counts. `POST /api/bootstrap` creates the admin and claims legacy data in one flow. Keep steady-state repositories free of legacy branches.

- [ ] **Step 5: Run bootstrap tests**

Run: `cd backend && go test ./internal/bootstrap ./internal/httpapi -run 'TestLegacy|TestBootstrap' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/bootstrap backend/cmd/server backend/internal/httpapi backend/internal/store
git commit --no-gpg-sign -m "feat: add removable legacy tenant migration"
```

---

### Task 9: Frontend Auth Gate, Profile, Tokens, And Admin Users

**Files:**
- Create: `frontend/src/contexts/AuthContext.tsx`
- Create: `frontend/src/components/AuthGate.tsx`
- Create: `frontend/src/pages/Login.tsx`
- Create: `frontend/src/pages/BootstrapAdmin.tsx`
- Create: `frontend/src/pages/AccountSetup.tsx`
- Create: `frontend/src/pages/settings/ProfileSettings.tsx`
- Create: `frontend/src/pages/settings/TokenSettings.tsx`
- Create: `frontend/src/pages/settings/AdminUsersSettings.tsx`
- Create: `frontend/src/pages/Login.test.tsx`
- Create: `frontend/src/components/AuthGate.test.tsx`
- Create: `frontend/src/pages/settings/ProfileSettings.test.tsx`
- Create: `frontend/src/pages/settings/TokenSettings.test.tsx`
- Create: `frontend/src/pages/settings/AdminUsersSettings.test.tsx`
- Modify: `frontend/src/api/types.ts`
- Modify: `frontend/src/api/client.ts`
- Modify: `frontend/src/api/queries.ts`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/pages/Settings.tsx`
- Modify: `frontend/src/i18n/messages.ts`
- Modify: `frontend/src/mocks/handlers.ts`

- [ ] **Step 1: Write failing frontend tests**

Add tests:

```tsx
it('redirects unauthenticated users to login', async () => {
  renderWithProviders(<App />, { route: '/transactions' })
  expect(await screen.findByRole('heading', { name: /sign in/i })).toBeInTheDocument()
})

it('shows raw programmatic token only once after creation', async () => {
  renderWithProviders(<TokenSettings />)
  await userEvent.click(await screen.findByRole('button', { name: /create token/i }))
  expect(await screen.findByText(/^expensor_pat_/)).toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: /close/i }))
  expect(screen.queryByText(/^expensor_pat_/)).not.toBeInTheDocument()
})
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd frontend && npm run test -- --run AuthGate Login ProfileSettings TokenSettings AdminUsersSettings`

Expected: FAIL because components and hooks do not exist.

- [ ] **Step 3: Add frontend API types and clients**

Add types:

```ts
export interface Principal {
  user_id: string
  tenant_id: string
  email: string
  display_name: string
  role: 'admin' | 'user'
  avatar_key: string
}

export interface AccessToken {
  id: string
  name: string
  created_at: string
  expires_at?: string | null
  last_used_at?: string | null
  revoked_at?: string | null
}

export interface CreatedAccessToken extends AccessToken {
  token: string
}
```

Set Axios `withCredentials: true` so session cookies work:

```ts
const apiClient = axios.create({
  baseURL: '/api',
  withCredentials: true,
  headers: {'Content-Type': 'application/json'},
})
```

- [ ] **Step 4: Add auth gate and pages**

`AuthGate` queries `/api/bootstrap` and `/api/session`. It routes:

- bootstrap required -> `/bootstrap`
- unauthenticated private route -> `/login`
- authenticated user on `/login` -> `/`
- setup token route -> `/account-setup`

Add login, bootstrap, and account setup pages using existing dark design language and i18n strings. Do not use browser-native alerts, confirms, prompts, selects, or title attributes.

- [ ] **Step 5: Add settings tabs**

Update `Settings.tsx` to persist tab state in URL with `useSearchParams`. Add:

- profile tab for display name and avatar key selection
- tokens tab for creating/revoking programmatic tokens
- admin users tab visible only for admins

The raw token returned by `POST /api/tokens` is displayed only in local component state after creation and disappears when dismissed or navigated away.

- [ ] **Step 6: Run frontend focused tests**

Run: `cd frontend && npm run test -- --run AuthGate Login ProfileSettings TokenSettings AdminUsersSettings`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add frontend/src
git commit --no-gpg-sign -m "feat: add account auth frontend"
```

---

### Task 10: OpenAPI, Component, Contract, And Browser Coverage

**Files:**
- Modify: `api/openapi/expensor.openapi.yaml`
- Modify: `tests/component/helpers/client.go`
- Create: `tests/component/auth_test.go`
- Modify: `tests/component/settings_test.go`
- Modify: `tests/component/transactions_test.go`
- Modify: `tests/contract/allowlist.tsv`
- Modify: `tests/contract/exclusions.tsv`
- Create: `frontend/playwright/auth.spec.ts`

- [ ] **Step 1: Write component tests for auth and isolation**

Add component tests that bootstrap an admin, log in, and prove protected endpoints require auth:

```go
func TestProtectedEndpointsRequireAuthentication(t *testing.T) {
	c := helpers.NewClient(t)
	resp := c.Get(t, "/api/transactions")
	defer resp.Body.Close()
	helpers.AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestBootstrapLoginAndTenantIsolation(t *testing.T) {
	c := helpers.NewClient(t)
	adminCookie := c.BootstrapAdmin(t, "admin@example.com", "Admin")
	userSetup := c.AdminCreateUser(t, adminCookie, "user@example.com", "User")
	userCookie := c.CompleteAccountSetup(t, userSetup.Token, "user passphrase")

	adminTx := c.CreateTransactionFixture(t, adminCookie, "admin-message")
	resp := c.AuthGet(t, userCookie, "/api/transactions/"+adminTx.ID)
	defer resp.Body.Close()
	helpers.AssertStatus(t, resp, http.StatusNotFound)
}
```

- [ ] **Step 2: Run component tests to verify they fail before final wiring**

Run: `task test:be:component`

Expected before final fixes: FAIL on auth/tenant behavior if any Phase 1 wiring is incomplete.

- [ ] **Step 3: Update OpenAPI**

Document:

- cookie session auth
- bearer token auth
- `/bootstrap`
- `/session`
- `/profile`
- `/tokens`
- `/admin/users`
- `/admin/users/{id}/setup-tokens`
- `/account-setup`

Mark private existing routes with security requirements.

- [ ] **Step 4: Add Playwright auth flow**

Add browser tests for:

- unauthenticated redirect to login
- first-admin bootstrap
- login/logout
- admin-created user account setup

- [ ] **Step 5: Run generated-contract checks**

Run:

```bash
task openapi:check
task test:be:contract
```

Expected: PASS.

- [ ] **Step 6: Run broad verification**

Run:

```bash
task fmt
task lint:be:prod
task test
task test:fe:e2e
```

Expected:

- `task fmt` completes with no remaining formatting diff.
- `task lint:be:prod` reports 0 issues.
- `task test` passes backend and frontend suites.
- `task test:fe:e2e` passes mocked browser flows.

- [ ] **Step 7: Commit**

```bash
git add api/openapi tests frontend/playwright frontend/src backend
git commit --no-gpg-sign -m "test: cover multi-tenant account flows"
```

---

## Final Verification

Before opening a PR for Phase 1, run:

```bash
task fmt
task lint:be:prod
task openapi:check
task test
task test:be:component
task test:be:contract
task test:fe:e2e
```

Expected:

- formatting leaves no unstaged source changes except generated artifacts intended for commit
- strict backend lint reports 0 issues
- OpenAPI check reports no committed artifact drift
- default backend and frontend tests pass
- backend component tests pass
- contract tests pass
- mocked Playwright E2E tests pass

## Plan Self-Review

- Spec coverage: Phase 1 auth, tokens, admin-created users, avatars, user-as-tenant scoping, encrypted reader runtime, removable legacy migration, frontend, OpenAPI, and tests are covered. Cross-user grouping and sharing features remain intentionally excluded from implementation tasks.
- Placeholder scan: no unspecified implementation placeholders are required to execute the tasks; each task names files, commands, and expected outcomes.
- Type consistency: the plan uses `tenant_id`, `store.Tenant`, `auth.Principal`, and `/api/tokens` consistently.
