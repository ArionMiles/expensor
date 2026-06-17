# Multi-Tenant Accounts Design

## Context

GitHub issue #24 asks Expensor to support authenticated multi-tenant accounts. Today Expensor assumes one user per installation: transactions, reader configuration, credentials, rules, runtime state, and preferences are effectively global.

This design focuses strictly on secure multi-tenancy. Phase 1 does not model any cross-user grouping, sharing, or ownership hierarchy.

## Goals

- Add authenticated user accounts without public registration.
- Protect all private APIs by default.
- Scope every user-owned resource by tenant.
- Keep tenant identity server-derived; never accept tenant identity from client payloads.
- Store browser sessions and programmatic access tokens securely.
- Store reader credentials and OAuth tokens encrypted at rest.
- Migrate existing single-user installations into the first administrator's tenant without data loss.
- Keep the legacy migration path isolated so it can be removed cleanly in a later release.
- Preserve stable existing private resource routes where possible, changing their authorization and tenant behavior rather than renaming them.

## Non-Goals

- No public signup page.
- No invite system.
- No open-registration configuration.
- No custom avatar uploads, linked image URLs, binary media storage, crop, zoom, or image transforms.
- No token scopes in Phase 1; programmatic tokens are full user-equivalent tokens.
- No key rotation in Phase 1.
- No transaction co-ownership.
- No backup/restore implementation in this spec.
- No sharing or group-membership data model in Phase 1.

## Phase 1 Scope

Phase 1 adds secure multi-tenancy:

- User accounts with email, password hash, display name, instance role, and `avatar_key`.
- First-account bootstrap as instance administrator.
- Admin-controlled user creation after bootstrap.
- One-time account setup tokens for admin-created users to set their own password.
- Session auth with secure HTTP-only cookies for browser use.
- User-generated programmatic access tokens using bearer authentication.
- Auth middleware protecting all private `/api/*` routes, with only explicit public exceptions.
- Authenticated principal propagated through request context.
- Tenant-scoped store and repository APIs for all tenant-owned operations.
- Per-tenant reader runtime, credentials, active reader, app preferences, processed-message state, diagnostics, and private transaction data.
- Application-layer authenticated encryption for reader secrets and OAuth tokens.
- Existing single-user installation migration into the first administrator tenant.

## Identity Model

Phase 1 introduces `users`. There is no separate `tenants` table in Phase 1.

Each user is their own tenant in Phase 1. Tenant-owned tables use a `tenant_id` column, and that value is the owning user's `users.id`. The authenticated principal still exposes both `user_id` and `tenant_id`; in Phase 1 they are equal.

Future collaboration features may introduce their own hierarchy or group model when those requirements are designed. Phase 1 must not add tables or abstractions solely to anticipate that future work.

User profile fields:

- `id`
- `email`
- `password_hash`
- `display_name`
- `role`
- `avatar_key`
- `created_at`
- `updated_at`
- `disabled_at`

The first user created through bootstrap receives the instance administrator role. Subsequent users are created only by an administrator.

## Avatars

Bundled avatars live under `content/avatars/` and are pure SVG assets. A catalog defines the allowed avatar keys and display labels.

The user table stores only `avatar_key`. The API rejects unknown avatar keys and applies a stable default when no avatar is selected. Adding a new avatar should require adding a new SVG asset and catalog metadata, not changing profile storage.

The frontend must not provide any upload, external URL, crop, zoom, or transform UI.

## Account Provisioning

Bootstrap:

- `GET /api/bootstrap` returns whether first-admin bootstrap is available or required.
- `POST /api/bootstrap` creates the first admin account.
- Bootstrap is disabled once any user exists.

Admin user management:

- Admins create additional users from the authenticated app.
- Admin-created users do not receive generated passwords.
- Admins generate one-time setup links for users.
- Setup tokens are stored hashed, expire, and are single-use.
- The raw setup token is returned only when created.
- Setup tokens, generated links, passwords, and hashes must not be logged.

Account setup:

- `POST /api/account-setup` accepts the setup token in the request body plus the new password.
- Setup tokens are not placed in URL paths.
- After successful setup, the token is consumed.

## Authentication

### Browser Sessions

Browser auth uses opaque session IDs stored in cookies with these attributes:

- `HttpOnly`
- `Secure` when served over HTTPS or configured production mode
- `SameSite=Lax`
- explicit expiry

Sessions are stored hashed in Postgres and include:

- `id`
- `user_id`
- `token_hash`
- `created_at`
- `expires_at`
- `last_used_at`
- `revoked_at`

Logout revokes the current session server-side and clears the cookie.

### Programmatic Access Tokens

Users can create named tokens for programmatic access:

- `GET /api/tokens` lists token metadata for the current user.
- `POST /api/tokens` creates a token and returns the raw token once.
- `DELETE /api/tokens/{id}` revokes a token owned by the current user.

Tokens are stored only as hashes. Metadata includes:

- `id`
- `user_id`
- `name`
- `created_at`
- `last_used_at`
- `expires_at`
- `revoked_at`

API requests authenticate with `Authorization: Bearer <token>`. Tokens authenticate as the owning user and do not bypass tenant scoping.

Phase 1 tokens are full user-equivalent tokens. Scoped tokens are a future enhancement.

## Authorization And Request Context

Authentication middleware resolves either a valid session cookie or bearer token into an authenticated principal:

- `user_id`
- `tenant_id` (equal to `user_id` in Phase 1)
- `role`
- authentication method

Handlers read this principal from request context. Clients must never submit `tenant_id` or `user_id` to access tenant-owned resources. If a request includes such fields for private resources, handlers ignore them or reject them according to the endpoint DTO.

Private routes are protected by default. Public exceptions are limited to:

- `GET /api/health`
- `GET /api/version`
- `GET /api/bootstrap`
- `POST /api/bootstrap`
- `POST /api/session`
- `POST /api/account-setup`
- OAuth callback routes that must remain reachable for provider redirects
- static frontend assets

Cross-tenant private object reads should generally return not found to avoid disclosing existence. Admin user-management routes remain role-protected and must not permit administrators to read tenant-private financial data unless a later feature explicitly designs that capability.

OAuth state records must bind provider callbacks to the authenticated user and tenant that initiated the flow. The callback route may be public for provider redirects, but it must not be able to attach credentials or tokens to the wrong tenant.

## Tenant-Owned Data

Add `tenant_id` to tenant-owned tables:

- `transactions`
- `transaction_labels`
- `transaction_label_sources`
- `labels`
- `label_merchants`
- user-created `rules`
- `muted_merchants`
- user-owned merchant category/bucket overrides
- `extraction_diagnostics`
- `reader_runtime`
- `processed_messages`
- user preferences, reader selections, and user-scoped app configuration rows

System-owned content stays global:

- predefined bundled rules
- MCC codes
- bundled category and bucket defaults
- community content
- banks
- reader plugin catalog
- avatar catalog
- system-wide operational configuration, such as community content sync settings

Uniqueness constraints become tenant-scoped where relevant:

- labels by `(tenant_id, name)`
- user rules by `(tenant_id, name)` while predefined rules remain global
- transactions by `(tenant_id, message_id)`
- muted merchant patterns by `(tenant_id, pattern)`
- reader runtime by `(tenant_id, reader)`
- processed messages by `(tenant_id, message_key)`

Tenant-aware indexes must support common list, search, and filter paths.

## Repository And Store Boundaries

Tenant identity is enforced at the repository/store boundary. The preferred implementation shape is explicit tenant parameters rather than hidden context lookups, because explicit arguments are easier to audit.

For tenant-owned operations:

- Store/repository methods accept a typed tenant or principal value.
- Every query includes `tenant_id = $n`.
- Every write sets `tenant_id` from the authenticated principal.
- No tenant-owned write accepts `tenant_id` from request payloads.
- Handler unit tests use mock stores that require tenant arguments.
- Store integration tests include cross-tenant reads and writes for every affected repository.

`internal/plugins.Registry` remains a catalog. Runtime wiring stays outside the registry.

## Reader Runtime, Daemon State, And Secrets

`reader_runtime` becomes tenant-scoped and stores encrypted secret/token fields. Active reader, reader config, OAuth token, OAuth client secret, checkpoints, and processed-message state are scoped by `tenant_id`.

Secrets are encrypted in application code using authenticated encryption and a deployment-provided secret. Encryption is not optional for reader client secrets or OAuth tokens.

The deployment secret can be provided in either form:

- `EXPENSOR_SECRET_KEY`: a base64-encoded 32-byte key.
- `EXPENSOR_SECRET_KEY_FILE`: a path to a file containing the base64-encoded 32-byte key, suitable for Docker Compose secrets, local secret files, or external secret-manager wrapper scripts.

If both are set, startup fails with a clear configuration error. If neither is set, startup fails once encrypted reader credential storage is part of the release. The application must never silently generate a production encryption key. Setup documentation should include a simple `openssl rand -base64 32` path, a Docker Compose secret-file example, and guidance that the key must be backed up.

Associated data binds ciphertext to:

- `tenant_id`
- reader name
- credential type

Development and tests may use explicit test keys.

Never include these values in logs, API responses, traces, metrics, span events, or error messages:

- passwords
- password hashes
- session IDs
- bearer tokens
- setup tokens
- OAuth tokens
- OAuth client secrets
- encrypted secret plaintext

The daemon can no longer assume a single global active reader loop. Reader operations are per authenticated tenant. Daemon coordination must include tenant identity when starting, rescanning, checkpointing, reading runtime state, and persisting processed messages.

If continuous background scanning for many users is too large for Phase 1, Phase 1 may limit daemon behavior to secure per-user manual starts and rescans. It must not keep global reader state that can mix users.

## Existing Installation Migration

The migration must protect existing users without becoming permanent architecture.

Permanent schema changes live in normal SQL migrations under `backend/migrations/`.

Existing rows are assigned to the first admin user's tenant ID during migration:

- transactions
- transaction labels and label sources
- labels
- label merchant mappings
- user-created rules
- muted merchants
- user-owned merchant category and bucket overrides
- extraction diagnostics
- reader runtime
- active reader
- processed messages
- app preferences

Bundled/system data remains global.

On existing installations with data but no users, startup enters a restricted bootstrap-required state. The first admin creation flow claims or migrates existing single-user data to the first admin user's tenant ID.

The migration should provide a dry-run or preview result before commit when feasible. The preview reports row counts that will be assigned to the first admin user's tenant ID and any blocking validation errors.

The committed migration is transactional where possible. Legacy credential/runtime file import, if still relevant at implementation time, lives in one isolated importer. Legacy files are never deleted until DB migration/import commits and verification passes.

Failure behavior:

- If migration cannot safely assign all existing user-owned rows to the first admin user's tenant ID, startup remains in bootstrap-required mode.
- Partial migration must not leave mixed global and tenant-owned private data.
- Legacy secret import failures must not silently drop credentials. The flow should either block or clearly mark the affected reader setup incomplete.

## Removable Legacy Path

The legacy single-user migration path must be easy to delete after current users have migrated.

Isolation rules:

- Put legacy migration/import code in a narrow package such as `backend/internal/bootstrap` or `backend/internal/legacyimport`.
- Keep steady-state repositories tenant-native with no "if legacy" branches.
- Keep migration tests separate from normal tenant tests.
- Do not let legacy file formats or global-data assumptions leak into handler DTOs or repository APIs.

Future deletion checklist:

- Remove the legacy importer package.
- Remove bootstrap migration tests.
- Remove startup detection for unmigrated single-user installs.
- Remove preview logic for claiming legacy rows.
- Keep normal first-admin bootstrap for empty installations.
- Keep all tenant-scoped schema, constraints, repositories, auth, and UI.

## API Design

Phase 1 routes:

- `GET /api/bootstrap`
- `POST /api/bootstrap`
- `POST /api/session`
- `GET /api/session`
- `DELETE /api/session`
- `GET /api/profile`
- `PATCH /api/profile`
- `GET /api/avatars`
- `GET /api/tokens`
- `POST /api/tokens`
- `DELETE /api/tokens/{id}`
- `GET /api/admin/users`
- `POST /api/admin/users`
- `PATCH /api/admin/users/{id}`
- `POST /api/admin/users/{id}/setup-tokens`
- `POST /api/account-setup`

Existing private resource routes remain stable where possible and become authenticated and tenant-scoped.

Cross-user grouping or sharing endpoints are not implemented or modeled in Phase 1.

## Frontend Design

The frontend adds an auth gate before the app shell:

- If bootstrap is required, show first-admin setup.
- If unauthenticated, show login.
- If authenticated, show the existing app layout.

New authenticated UI:

- Profile settings for display name and bundled SVG avatar selection.
- Token management for creating, viewing metadata for, and revoking programmatic access tokens.
- Admin-only user management for creating users and generating one-time setup links.
- Account setup flow for users arriving with a setup token.

The existing setup wizard remains reader/onboarding focused but runs inside an authenticated tenant context.

Frontend URL state rules still apply. Any tabs, filters, or navigation state introduced by user management, profile settings, or token management must be persisted with `useSearchParams`.

## OpenAPI

OpenAPI must document:

- cookie session authentication
- bearer token authentication
- protected endpoint security requirements
- bootstrap schemas
- session schemas
- profile schemas
- avatar catalog schemas
- token schemas
- admin user schemas
- account setup schemas
- tenant-scoped behavior for existing private routes where relevant

Do not leave undocumented routes. Update contract allowlists or exclusions only when the contract suite needs it.

## Testing Strategy

Backend unit tests:

- auth handlers
- auth middleware public/protected route behavior
- password hashing and verification
- session creation, expiry, revocation, and logout
- token creation, hash-only storage, last-used update, expiry, and revocation
- setup token expiry and single-use behavior
- avatar key validation
- role checks for admin routes

Backend store and integration tests:

- tenant-scoped uniqueness constraints
- cross-tenant reads return not found for private resources
- cross-tenant writes cannot affect another tenant
- transactions, labels, label mappings, categories, buckets, rules, muted merchants, diagnostics, preferences, reader runtime, and processed messages all include tenant coverage
- encrypted reader runtime cannot be decrypted when associated data uses the wrong tenant, reader, or credential type

Component tests:

- bootstrap-required behavior
- single-user migration preview and commit behavior
- encrypted reader runtime persistence
- per-tenant daemon start/rescan state

Contract tests:

- auth requirements for protected endpoints
- schemas for new auth, profile, token, admin user, account setup, and avatar routes

Frontend tests:

- auth gate
- first-admin bootstrap
- login and logout
- profile avatar selection
- token creation and raw-token one-time display
- token revocation
- admin user creation
- setup-token account completion

Playwright coverage:

- unauthenticated redirect
- first-admin bootstrap
- login/logout
- admin-created user setup

Existing tests must be updated to create an authenticated tenant/principal rather than relying on global state.

## Security Review Checklist

Before Phase 1 ships:

- No handler accepts `tenant_id` from clients for private resource access.
- Every tenant-owned query includes tenant filtering.
- API tokens and sessions are stored only as hashes.
- Passwords are hashed with a modern password hashing function.
- Reader secrets and OAuth tokens are encrypted before persistence.
- Encryption associated data binds ciphertext to tenant, reader, and credential type.
- Production startup fails without required encryption key material.
- Sensitive values are absent from logs, traces, metrics, span events, API responses, and error messages.
- Cross-tenant access tests exist for every tenant-owned repository.
- Admin routes cannot read private tenant financial data unless explicitly designed.

## Follow-Ups

- Future collaboration features, with their own requirements and data model.
- Multi-tenant background scheduler if Phase 1 limits daemon behavior to per-tenant manual starts/rescans.
- Token scopes.
- Key rotation.
- Backup/restore format that is tenant-aware and excludes OAuth secrets.
- Transaction co-ownership.
- Removal of the legacy single-user migration path after current installations have migrated.
