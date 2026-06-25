# Multi-Tenant Phase 1 Follow-Ups

These notes capture deferred review findings from PR #42, `Add backend auth session API`.
Resolve them before treating issue #24 Phase 1 as complete.

## Deferred PR #42 Review Notes

### Preserve tenant identity through OAuth callbacks

`GET /api/auth/callback` must remain public for provider redirects, but it cannot rely on
`requestTenant(r)` because public routes do not receive an authenticated principal from the
auth middleware. OAuth state created by `AuthStart` should bind the initiating user and tenant,
and callback exchange/token persistence should use that stored tenant. Without this, tenant-scoped
credentials uploaded before redirect can be missed, or OAuth tokens can be saved to the legacy
tenantless runtime row.

Acceptance checks:

- OAuth start stores user and tenant identity in pending state.
- OAuth callback and manual exchange save tokens under the initiating tenant.
- A tenant-scoped Gmail OAuth flow remains connected after the normal provider redirect.

### Adopt tenantless legacy data during first-admin bootstrap

Upgraded single-user installs can already have transactions, preferences, rules, diagnostics,
reader runtime, processed messages, and taxonomy rows with `tenant_id IS NULL`. After bootstrap,
authenticated requests use the first admin's non-empty tenant ID, so tenant-scoped queries no
longer see those legacy rows. First-admin bootstrap or the removable legacy migration slice must
claim tenantless rows for the initial tenant before the installation is considered migrated.

Acceptance checks:

- Bootstrapping the first admin on an upgraded install adopts tenantless private data.
- Existing preferences, active reader/runtime state, rules, transactions, labels, categories,
  buckets, muted merchants, diagnostics, and processed-message checkpoints remain visible.
- New authenticated writes use the first admin tenant and do not create additional tenantless rows.

### Serialize first-admin bootstrap creation

The public bootstrap endpoint must make first-admin creation atomic under concurrent requests.
A count-then-insert check under default transaction isolation can allow two requests to observe
zero users and create multiple admins. Use a database-backed guard such as an advisory lock,
serializable transaction, or singleton bootstrap record so only one initial admin can be created.

Acceptance checks:

- Concurrent `POST /api/bootstrap` requests on a fresh database produce exactly one admin.
- The losing request receives a conflict response.
- The repository-level bootstrap test covers the race or the selected locking primitive.
