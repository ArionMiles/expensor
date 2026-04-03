# Expensor Transformation Plan: Web UI + Plugin System

**Status**: In Progress (Phase 2 Complete)
**Started**: 2026-01-11
**Last Updated**: 2026-01-11

---

## Overview

Transform Expensor from a CLI-only Go binary into a modern web application with backend/frontend separation, web-based OAuth onboarding, and an extensible plugin system for readers/writers.

## User Requirements

1. **Deployment**: Single Docker container with minimal configuration - just run with reader/writer env vars
2. **UI-First Onboarding**: All setup through web UI - upload credentials, configure readers/writers
3. **No Manual File Mounting**: Users upload service account JSON and configs via web forms
4. **Backend/Frontend Separation**: Go backend in `backend/`, TypeScript+Tailwind frontend in `frontend/`
5. **Plugin System**: Configurable reader/writer selection via UI dropdowns or env vars
6. **Single Docker Image**: Combined image with both backend and frontend
7. **Frontend Sub-Agent**: Create `frontend-expert` agent for TypeScript/Tailwind expertise
8. **Data Persistence**: Expose `/app/data` volume for uploaded credentials and token storage

## Architecture Decisions

- **Frontend**: React + Vite + TypeScript + Tailwind CSS
- **OAuth Flow**: Web-based (backend serves OAuth callback endpoint at `:8080/api/auth/callback`)
- **Credentials Upload**: Users upload `client_secret.json` via web UI, stored in `/app/data/`
- **Plugin Configuration**: UI-first approach with file uploads and form inputs
  - Gmail: Upload `client_secret.json` via UI
  - Thunderbird: Provide MBOX path via form input
  - Sheets/CSV/JSON/PostgreSQL: Configure via web forms
- **Plugin System**: Registry-based with factory pattern, designed for future extensibility
- **Data Persistence**: `/app/data` volume contains uploaded credentials, tokens, and state files
- **No Backward Compatibility**: Clean-slate rewrite, removed CLI commands and legacy env vars

---

## Progress Tracking

| Phase | Status | Started | Completed | Notes |
|-------|--------|---------|-----------|-------|
| Phase 1: Frontend Sub-Agent | ✅ Complete | 2026-01-11 | 2026-01-11 | Created `.claude/agents/frontend-expert.md` |
| Phase 2: Restructure & Plugin System | ✅ Complete | 2026-01-11 | 2026-01-11 | Backend restructured, plugin system implemented |
| Phase 3: Thunderbird Reader | ⏳ Pending | - | - | Next to implement |
| Phase 4: Database Writer Plugin | ⏳ Pending | - | - | PostgreSQL writer with multi-currency support |
| Phase 5: Move Config Files | ✅ Complete | 2026-01-11 | 2026-01-11 | Moved to `backend/cmd/server/content/` |
| Phase 6: Web Server & API | ⏳ Pending | - | - | Enhanced with DB query endpoints |
| Phase 7: Frontend | ⏳ Pending | - | - | Transaction list, labels, search, notes |
| Phase 8: Integration & Docker | ⏳ Pending | - | - | Multi-container with PostgreSQL |
| Phase 9: Configuration UI & Polish | ⏳ Pending | - | - | Label automation, advanced search |
| Phase 10: Webhook System | ⏳ Pending | - | - | Event-driven extensibility for integrations |

---

## Implementation Phases

### Phase 1: Frontend Sub-Agent (Tooling) ✅
**Effort**: 30 minutes | **Priority**: HIGHEST | **Status**: Complete

#### Goal
Create `frontend-expert` sub-agent before starting frontend work to guide all React/TypeScript development.

#### Task
Created `.claude/agents/frontend-expert.md` with expertise in:
- React 18+ with hooks and functional components
- TypeScript for type safety
- Vite for fast development
- Tailwind CSS for utility-first styling
- React Router for navigation
- 2025-26 modern standards and best practices

#### Validation ✅
- [x] Agent definition created at `.claude/agents/frontend-expert.md`
- [x] Can invoke with Task tool
- [x] Agent has access to correct tools

---

### Phase 2: Restructure & Plugin System (Foundation) ✅
**Effort**: 2-3 days | **Priority**: HIGHEST | **Status**: Complete

#### Goals
- Reorganize codebase into `backend/` directory structure
- Implement plugin registry system
- Remove CLI commands and legacy code

#### Tasks Completed

1. **Created Directory Structure** ✅
   ```
   backend/
   ├── cmd/server/              # New HTTP server entry point
   ├── internal/
   │   ├── plugins/            # Plugin registry and loader
   │   ├── daemon/             # Refactored run.go
   │   └── api/                # HTTP handlers (Phase 4)
   └── pkg/                     # Moved from root pkg/
       ├── api/                # Core interfaces
       ├── client/             # OAuth client
       ├── config/             # Configuration
       ├── logging/            # Logging utilities
       ├── reader/             # Readers (gmail, thunderbird)
       ├── writer/             # Writers (sheets, csv, json)
       └── plugins/            # Plugin wrappers
           ├── readers/
           │   └── gmail/       # Gmail plugin wrapper
           └── writers/         # Sheets, CSV, JSON wrappers
   ```

2. **Moved and Updated Code** ✅
   - Moved `cmd/expensor/` → `backend/cmd/server/`
   - Moved `pkg/` → `backend/pkg/`
   - Updated all import paths: `github.com/ArionMiles/expensor/pkg` → `github.com/ArionMiles/expensor/backend/pkg`
   - **DELETED** old CLI commands (`setup.go`, `status.go`)
   - **DELETED** local OAuth callback code in `pkg/client/client.go`

3. **Implemented Plugin System** ✅
   - Created `backend/internal/plugins/registry.go`:
     - `Registry` struct with reader/writer maps
     - `RegisterReader()`, `RegisterWriter()` methods
     - `GetReader()`, `GetWriter()` factory methods
     - `ListReaders()`, `ListWriters()` for API
   - Created plugin wrappers:
     - Gmail reader: `backend/pkg/plugins/readers/gmail/plugin.go`
     - Sheets writer: `backend/pkg/plugins/writers/sheets/plugin.go`
     - CSV writer: `backend/pkg/plugins/writers/csv/plugin.go`
     - JSON writer: `backend/pkg/plugins/writers/json/plugin.go`

4. **Updated Configuration** ✅
   - Simplified `backend/pkg/config/config.go` for plugin-based config
   - Added env vars: `EXPENSOR_READER`, `EXPENSOR_WRITER`, `EXPENSOR_READER_CONFIG`, `EXPENSOR_WRITER_CONFIG`
   - Maintained backward compatibility with legacy `GSHEETS_*` env vars

5. **Refactored Run Logic** ✅
   - Created `backend/internal/daemon/runner.go` (refactored from `run.go`)
   - Uses plugin registry instead of hardcoded reader/writer
   - Accepts context parameter for cancellation (needed for web server)

6. **Created OAuth Client** ✅
   - Created `backend/pkg/client/oauth.go` with web-only OAuth flow
   - Removed local callback server code

#### Validation Checklist ✅
- [x] Plugin registry initializes and registers Gmail reader
- [x] Plugin registry registers Sheets, CSV, JSON writers
- [x] Can select reader/writer via `EXPENSOR_READER`/`EXPENSOR_WRITER` env vars
- [x] Code compiles without errors
- [x] Binary builds successfully: `backend/bin/expensor-server`

#### Critical Files
- `backend/internal/plugins/registry.go` - Plugin registry implementation
- `backend/pkg/config/config.go` - Configuration with plugin support
- `backend/internal/daemon/runner.go` - Refactored run logic
- `backend/pkg/plugins/readers/gmail/plugin.go` - Gmail plugin wrapper
- `backend/pkg/plugins/writers/sheets/plugin.go` - Sheets plugin wrapper
- `backend/pkg/client/oauth.go` - Web-only OAuth flow

#### Documentation Created
- `backend/README.md` - Backend architecture guide
- `backend/QUICKSTART.md` - Quick start guide
- `PHASE2_MIGRATION.md` - Detailed migration guide
- `PHASE2_SUMMARY.md` - Feature summary
- `PHASE2_STATUS.txt` - Status report

---

### Phase 3: Thunderbird Reader Plugin (Extension)
**Effort**: 2-3 days | **Priority**: HIGH | **Status**: In Progress

#### Goal
Implement Thunderbird reader as a plugin following the existing [docs/thunderbird-reader-plan.md](docs/thunderbird-reader-plan.md) design.

#### Tasks

1. **Implement Thunderbird Reader** (following docs/thunderbird-reader-plan.md)
   - Create `backend/pkg/reader/thunderbird/` package:
     - `thunderbird.go` - Main reader with MBOX scanning
     - `profile.go` - Thunderbird profile discovery
     - `state.go` - Processed message tracking (file-based)
     - `mime.go` - MIME body extraction helpers
     - `thunderbird_test.go` - Unit tests
   - Add dependency: `go get github.com/emersion/go-mbox`
   - Reuse `ExtractTransactionDetails` from Gmail reader

2. **Create Thunderbird Plugin Wrapper**
   - Create `backend/pkg/plugins/readers/thunderbird/plugin.go`:
     - Implement `ReaderPlugin` interface
     - Metadata: name="thunderbird", description, required scopes (none)
     - Config schema for profile path, mailboxes, state file
     - Factory function for registry

3. **Register in Plugin Registry**
   - Update `backend/internal/plugins/registry.go`:
     - Add `r.RegisterReader("thunderbird", NewThunderbirdReader)` in `registerBuiltins()`

4. **Update Rule Format**
   - Extend `backend/cmd/server/config/rules.json` to support Thunderbird rules:
     - Add `type: "thunderbird"` field
     - Add `senderEmail` field
     - Add `subjectContains` field
   - Support both Gmail and Thunderbird rule types in same file

5. **Configuration Support**
   - Add Thunderbird config to env vars:
     ```bash
     EXPENSOR_READER=thunderbird
     EXPENSOR_READER_CONFIG='{"profilePath":"","mailboxes":["INBOX"],"stateFile":"data/thunderbird_state.json"}'
     ```

#### Validation Checklist
- [ ] Thunderbird reader discovers profiles correctly
- [ ] MBOX files parsed successfully
- [ ] Rules match sender email and subject
- [ ] Transactions extracted from email bodies
- [ ] State tracking prevents duplicate processing
- [ ] Plugin registered and selectable via env var
- [ ] Unit tests pass
- [ ] Integration test with sample MBOX file

#### Critical Files
- `backend/pkg/reader/thunderbird/thunderbird.go` - Main reader
- `backend/pkg/reader/thunderbird/profile.go` - Profile discovery
- `backend/pkg/reader/thunderbird/state.go` - State tracking
- `backend/pkg/plugins/readers/thunderbird/plugin.go` - Plugin wrapper
- `backend/internal/plugins/registry.go` - Updated registration

---

### Phase 4: Database Writer Plugin (PostgreSQL)
**Effort**: 2-3 days | **Priority**: HIGH | **Status**: Pending

#### Goal
Implement PostgreSQL writer plugin with multi-currency support, allowing transaction storage, notes, labels, and full-text search.

**Note for Phase 10**: When implementing this phase, design the writer with event emission hooks in mind. The PostgreSQL writer should have placeholder/optional callback functions for emitting events when transactions are created or updated. These hooks can remain no-ops initially and be wired up to the event bus in Phase 10.

#### Database Choice: PostgreSQL

**Why PostgreSQL?**
- Native JSON/JSONB support for flexible metadata
- Built-in full-text search for descriptions and labels
- `NUMERIC` type for precise currency handling
- Excellent Go support via `pgx` driver
- ACID compliance for financial data
- Docker-friendly with official images

#### Tasks

1. **Database Schema Design**
   - Create migrations in `backend/migrations/`:
     - `001_create_transactions.sql`:
       ```sql
       CREATE TABLE transactions (
           id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
           message_id VARCHAR(255) UNIQUE NOT NULL,
           amount NUMERIC(19,4) NOT NULL,
           currency VARCHAR(3) NOT NULL DEFAULT 'INR',
           original_amount NUMERIC(19,4),
           original_currency VARCHAR(3),
           exchange_rate NUMERIC(10,6),
           timestamp TIMESTAMPTZ NOT NULL,
           merchant_info TEXT NOT NULL,
           category VARCHAR(100),
           bucket VARCHAR(50),
           source VARCHAR(100) NOT NULL,
           description TEXT,
           metadata JSONB DEFAULT '{}',
           created_at TIMESTAMPTZ DEFAULT NOW(),
           updated_at TIMESTAMPTZ DEFAULT NOW()
       );

       CREATE TABLE transaction_labels (
           id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
           transaction_id UUID REFERENCES transactions(id) ON DELETE CASCADE,
           label VARCHAR(100) NOT NULL,
           created_at TIMESTAMPTZ DEFAULT NOW(),
           UNIQUE(transaction_id, label)
       );

       CREATE INDEX idx_transactions_timestamp ON transactions(timestamp DESC);
       CREATE INDEX idx_transactions_merchant ON transactions USING gin(to_tsvector('english', merchant_info));
       CREATE INDEX idx_transactions_description ON transactions USING gin(to_tsvector('english', description));
       CREATE INDEX idx_transaction_labels_label ON transaction_labels(label);
       CREATE INDEX idx_transactions_currency ON transactions(currency);
       ```

2. **Implement Database Writer**
   - Create `backend/pkg/writer/postgres/postgres.go`:
     - Connection pooling with `pgx`
     - Batch inserts for performance
     - Upsert logic (INSERT ON CONFLICT)
     - Multi-currency transaction storage
     - Transaction with rollback on errors
   - Add migration runner utility

3. **Create PostgreSQL Plugin Wrapper**
   - Create `backend/pkg/plugins/writers/postgres/plugin.go`:
     - Implement `WriterPlugin` interface
     - Metadata: name="postgres", description
     - Config schema: connection string, pool size, batch size
     - Factory function for registry

4. **Update Configuration**
   - Add PostgreSQL config to `backend/pkg/config/config.go`:
     ```go
     type PostgresConfig struct {
         Host     string `koanf:"POSTGRES_HOST"`
         Port     int    `koanf:"POSTGRES_PORT"`
         Database string `koanf:"POSTGRES_DB"`
         User     string `koanf:"POSTGRES_USER"`
         Password string `koanf:"POSTGRES_PASSWORD"`
         SSLMode  string `koanf:"POSTGRES_SSLMODE"`
     }
     ```

5. **Multi-Currency Support**
   - Update `backend/pkg/api/api.go` `TransactionDetails` struct:
     ```go
     type TransactionDetails struct {
         Amount           float64
         Currency         string  // e.g., "INR", "USD", "EUR"
         OriginalAmount   *float64 // If converted
         OriginalCurrency *string  // Original currency if converted
         ExchangeRate     *float64 // Conversion rate if applicable
         Timestamp        time.Time
         MerchantInfo     string
         Category         string
         Bucket           string
         Source           string
         MessageID        string
         Description      string  // User-added description
         Labels           []string // User-added labels
     }
     ```

6. **Register Plugin**
   - Update `backend/internal/plugins/registry.go`:
     - Add `r.RegisterWriter("postgres", NewPostgresWriter)` in `registerBuiltins()`

#### Validation Checklist
- [ ] PostgreSQL writer connects to database successfully
- [ ] Migrations run and create correct schema
- [ ] Batch inserts work correctly
- [ ] Upserts prevent duplicate transactions (by message_id)
- [ ] Multi-currency transactions stored correctly
- [ ] Plugin registered and selectable via env var
- [ ] Unit tests pass with test database
- [ ] Integration test with real PostgreSQL container

#### Critical Files
- `backend/pkg/writer/postgres/postgres.go` - PostgreSQL writer
- `backend/pkg/plugins/writers/postgres/plugin.go` - Plugin wrapper
- `backend/migrations/001_create_transactions.sql` - Database schema
- `backend/internal/plugins/registry.go` - Updated registration

#### Environment Variables
```bash
EXPENSOR_WRITER=postgres
EXPENSOR_WRITER_CONFIG='{
  "host": "postgres",
  "port": 5432,
  "database": "expensor",
  "user": "expensor",
  "password": "expensor_password",
  "sslmode": "disable"
}'
```

---

### Phase 5: Move Configuration Files ✅
**Effort**: 1 hour | **Priority**: MEDIUM | **Status**: Complete

#### Goal
Move `rules.json` and `labels.json` from `backend/cmd/server/config/` to `backend/cmd/server/content/` directory for better organization while maintaining embed compatibility.

#### Tasks

1. **Create Content Directory**
   - Create `content/` directory at project root
   - Move `backend/cmd/server/config/rules.json` → `content/rules.json`
   - Move `backend/cmd/server/config/labels.json` → `content/labels.json`

2. **Update Embed Directives**
   - Update `backend/cmd/server/main.go` (or wherever embedded):
     ```go
     //go:embed ../../content/rules.json
     var rulesJSON string

     //go:embed ../../content/labels.json
     var labelsJSON string
     ```

3. **Update Documentation**
   - Update README and docs to reference `content/` directory
   - Add example rules and labels to documentation

4. **Update Build Process**
   - Ensure Docker build copies `content/` directory correctly
   - Update `.dockerignore` if needed

#### Validation Checklist ✅
- [x] `content/` directory created with rules.json and labels.json
- [x] Backend reads embedded files correctly
- [x] No broken imports or build errors
- [ ] Docker image includes content files (will be verified in Phase 8)

**Implementation Notes:**
- Files placed in `backend/cmd/server/content/` instead of project root `content/`
- Go's `embed` directive requires files within the module, cannot use `..` paths
- Removed old `backend/cmd/server/config/` directory
- Build tested and verified successful

#### Critical Files
- `backend/cmd/server/content/rules.json` - Transaction extraction rules
- `backend/cmd/server/content/labels.json` - Merchant-to-category mappings
- `backend/cmd/server/main.go` - Updated embed directives

---

### Phase 6: Web Server & API (Backend)
**Effort**: 2-3 days | **Priority**: HIGH | **Status**: Pending

#### Goals
- Add HTTP server that serves API endpoints
- Run expensor daemon in background goroutine
- Implement web-based OAuth flow (only - no CLI support)
- Provide status/health endpoints

**Note for Phase 10**: When implementing transaction API handlers (PUT/POST/DELETE for descriptions and labels), design them with optional event callback parameters. This allows easy integration with the event bus in Phase 10 without requiring major refactoring.

#### Tasks

1. **Create HTTP Server**
   - Create `backend/cmd/server/main.go`:
     - HTTP server listening on port 8080 (configurable via `PORT` env var)
     - Serve API routes at `/api/*`
     - Run daemon in background goroutine
     - Graceful shutdown on SIGINT/SIGTERM
     - **ONLY** command: `serve` (no CLI commands)

2. **Implement API Handlers** (`backend/internal/api/handlers.go`)
   - **File Upload endpoints**:
     - `POST /api/upload/credentials` - Upload `client_secret.json`, save to `/app/data/`
     - `GET /api/upload/status` - Check if credentials file exists
     - File validation: JSON format, required OAuth fields
     - Security: sanitize filenames, limit file size (5MB), verify JSON structure
   - **OAuth endpoints**:
     - `POST /api/auth/start` - Generate OAuth URL (requires credentials uploaded first)
     - `GET /api/auth/callback` - Handle OAuth callback, exchange code for token
     - `GET /api/auth/status` - Check if authenticated and token validity
     - `DELETE /api/auth/token` - Revoke and delete OAuth token
   - **Plugin endpoints**:
     - `GET /api/plugins/readers` - List available readers with metadata
     - `POST /api/plugins/readers/:name/config` - Save reader config (e.g., Thunderbird MBOX path)
     - `GET /api/plugins/writers` - List available writers with metadata
     - `POST /api/plugins/writers/:name/config` - Save writer config (e.g., Sheets ID)
   - **Status endpoints**:
     - `GET /api/health` - Simple health check
     - `GET /api/status` - Daemon status (running, last sync, transaction count)
   - **Configuration endpoints**:
     - `GET /api/config` - Get current configuration (reader/writer selection, configs)
     - `PUT /api/config` - Update configuration

3. **Implement OAuth Client for Web Flow**
   - Already created in Phase 2: `backend/pkg/client/oauth.go`
   - Integrate with API handlers

4. **Add Middleware** (`backend/internal/api/middleware.go`)
   - Request logging (method, path, duration)
   - CORS headers (for development with Vite dev server)
   - Error recovery
   - JSON response helpers

5. **Environment Variables**
   - Add `PORT` (default: 8080)
   - Add `BASE_URL` (default: "http://localhost:8080") - for OAuth callback URL

#### Validation Checklist
- [ ] Server starts and listens on port 8080
- [ ] `GET /api/health` returns 200 OK
- [ ] **`POST /api/upload/credentials` accepts JSON file upload and saves to data/**
- [ ] **`GET /api/upload/status` returns true after credentials uploaded**
- [ ] **File upload validates JSON structure and rejects invalid files**
- [ ] **File upload rejects files > 5MB**
- [ ] `POST /api/auth/start` returns OAuth URL (requires credentials uploaded first)
- [ ] OAuth callback flow completes and saves token
- [ ] `GET /api/auth/status` shows correct authentication status
- [ ] `DELETE /api/auth/token` revokes and deletes token successfully
- [ ] Daemon runs in background while server is running
- [ ] `GET /api/plugins/readers` lists Gmail and Thunderbird readers
- [ ] **`POST /api/plugins/readers/:name/config` saves reader configuration**
- [ ] `GET /api/plugins/writers` lists Sheets, CSV, JSON, PostgreSQL writers
- [ ] **`POST /api/plugins/writers/:name/config` saves writer configuration**
- [ ] `GET /api/transactions` returns paginated transaction list
- [ ] `PUT /api/transactions/:id` updates description successfully
- [ ] `POST /api/transactions/:id/labels` adds labels correctly
- [ ] `GET /api/transactions/search` performs full-text search
- [ ] Multi-currency transactions display correctly in API responses

#### Critical Files
- `backend/cmd/server/main.go` - HTTP server entry point
- `backend/internal/api/handlers.go` - API endpoint handlers
- `backend/internal/api/middleware.go` - HTTP middleware
- `backend/pkg/client/oauth.go` - Web-only OAuth flow (already created)

---

### Phase 7: Frontend (UI)
**Effort**: 4-5 days | **Priority**: HIGH | **Status**: Pending

#### Goals
- Build React frontend with OAuth onboarding
- Create status dashboard
- Responsive design with Tailwind CSS
- Use `frontend-expert` sub-agent for all development

#### Tasks

1. **Initialize Frontend Project**
   ```bash
   mkdir frontend && cd frontend
   npm create vite@latest . -- --template react-ts
   npm install -D tailwindcss postcss autoprefixer
   npx tailwindcss init -p
   npm install axios react-router-dom
   ```

2. **Configure Tailwind CSS**
   - Update `tailwind.config.js` with content paths
   - Add Tailwind directives to `src/index.css`
   - Configure responsive breakpoints and theme

3. **Create Utility Helpers**
   - `src/lib/cn.ts` - Class name utility for conditional Tailwind classes
   - `src/lib/api.ts` - Axios client with TypeScript types

4. **Create Components**
   - **Onboarding Components (NEW)**:
     - `src/components/FileUpload.tsx` - Drag-and-drop file upload with validation
     - `src/components/CredentialsUpload.tsx` - Upload `client_secret.json` with instructions
     - `src/components/ReaderConfigForm.tsx` - Dynamic form based on selected reader
     - `src/components/WriterConfigForm.tsx` - Dynamic form based on selected writer
     - `src/components/SetupStepper.tsx` - Multi-step onboarding wizard
   - **Core Components**:
     - `src/components/OAuthButton.tsx` - "Connect Gmail" button (disabled until credentials uploaded)
     - `src/components/StatusCard.tsx` - Display connection status
     - `src/components/LoadingSpinner.tsx` - Loading indicator
     - `src/components/ErrorMessage.tsx` - Error display with retry
     - `src/components/Layout.tsx` - Common layout wrapper
     - `src/components/PluginSelector.tsx` - Reader/writer dropdown selection

5. **Create Pages**
   - `src/pages/Home.tsx` - Landing page with "Get Started" flow
   - **`src/pages/Onboarding.tsx` - Multi-step onboarding wizard (NEW)**:
     - Step 1: Select reader plugin (Gmail/Thunderbird)
     - Step 2: Upload credentials or provide config (e.g., MBOX path)
     - Step 3: Complete OAuth flow (if applicable)
     - Step 4: Select and configure writer plugin
     - Step 5: Review and start daemon
   - `src/pages/Dashboard.tsx` - Status dashboard
   - `src/pages/Settings.tsx` - Configuration page (reader/writer switching)

6. **Create Custom Hooks**
   - **Onboarding Hooks (NEW)**:
     - `src/hooks/useFileUpload.ts` - Handle file uploads with progress and validation
     - `src/hooks/useOnboarding.ts` - Manage onboarding wizard state and navigation
   - **Core Hooks**:
     - `src/hooks/useOAuth.ts` - OAuth flow management
     - `src/hooks/useStatus.ts` - Daemon status polling
     - `src/hooks/usePlugins.ts` - Plugin listing and configuration
   - **Transaction Hooks**:
     - `src/hooks/useTransactions.ts` - Fetch, paginate, filter transactions
     - `src/hooks/useTransactionSearch.ts` - Full-text search with debouncing
     - `src/hooks/useLabels.ts` - Fetch and manage labels
     - `src/hooks/useTransactionUpdate.ts` - Update description and labels
     - `src/hooks/useCurrencyFormat.ts` - Format amounts with currency symbols

7. **Setup Routing**
   - Configure React Router
   - Routes:
     - `/` - Home page (redirects to /onboarding if not configured)
     - **`/onboarding` - Multi-step onboarding wizard (NEW)**
     - `/dashboard` - Status dashboard
     - `/transactions` - Transaction list
     - `/transactions/:id` - Transaction detail
     - `/settings` - Configuration (reader/writer switching, re-upload credentials)

8. **API Client** (`src/api/client.ts`)
   - Type-safe API client with Axios
   - **File upload support with multipart/form-data**
   - Error handling and interceptors
   - **API methods (NEW)**:
     ```typescript
     upload: {
       credentials: (file: File) => Promise<void>,
       status: () => Promise<{exists: boolean}>
     },
     plugins: {
       readers: {
         list: () => Promise<PluginInfo[]>,
         configure: (name: string, config: any) => Promise<void>
       },
       writers: {
         list: () => Promise<PluginInfo[]>,
         configure: (name: string, config: any) => Promise<void>
       }
     }
     ```

9. **Development Setup**
   - Configure Vite to proxy `/api/*` to `http://localhost:8080`

#### Validation Checklist
- [ ] Frontend builds successfully
- [ ] Dev server runs
- [ ] **Onboarding wizard completes all 5 steps successfully**
- [ ] **File upload works with drag-and-drop and file picker**
- [ ] **Upload shows progress indicator**
- [ ] **Invalid files rejected with clear error messages**
- [ ] **Reader selection shows Gmail and Thunderbird options**
- [ ] **Gmail requires credentials upload, Thunderbird requires MBOX path**
- [ ] **Writer selection shows Sheets, CSV, JSON, PostgreSQL options**
- [ ] **Each writer shows appropriate configuration form**
- [ ] OAuth flow completes end-to-end (after credentials uploaded)
- [ ] Status dashboard shows authentication status
- [ ] **Transaction table displays all transactions with pagination**
- [ ] **Search bar filters transactions in real-time**
- [ ] **Can edit transaction descriptions inline**
- [ ] **Can add/remove labels to transactions**
- [ ] **Multi-currency amounts display correctly with symbols**
- [ ] **Filters work (by date, category, labels, currency)**
- [ ] **Transaction statistics show on dashboard**
- [ ] Error states display properly
- [ ] Responsive on mobile, tablet, desktop
- [ ] No console errors or warnings
- [ ] Loading states work correctly

#### Critical Files
- **Onboarding (NEW)**:
  - `frontend/src/pages/Onboarding.tsx` - Multi-step onboarding wizard
  - `frontend/src/components/FileUpload.tsx` - Drag-and-drop file upload
  - `frontend/src/components/CredentialsUpload.tsx` - Credentials upload with docs links
  - `frontend/src/components/ReaderConfigForm.tsx` - Dynamic reader config form
  - `frontend/src/components/WriterConfigForm.tsx` - Dynamic writer config form
  - `frontend/src/components/SetupStepper.tsx` - Stepper UI component
  - `frontend/src/hooks/useFileUpload.ts` - File upload state management
  - `frontend/src/hooks/useOnboarding.ts` - Onboarding wizard state
- **Core**:
  - `frontend/src/hooks/useOAuth.ts` - OAuth flow management
  - `frontend/src/components/OAuthButton.tsx` - Connect button (requires credentials)
  - `frontend/src/pages/Dashboard.tsx` - Status dashboard
- **Transactions**:
  - `frontend/src/pages/Transactions.tsx` - Transaction list page
  - `frontend/src/components/TransactionTable.tsx` - Main transaction table
  - `frontend/src/components/DescriptionEditor.tsx` - Description editing
  - `frontend/src/components/LabelSelector.tsx` - Label management
  - `frontend/src/hooks/useTransactions.ts` - Transaction data hook
  - `frontend/src/hooks/useTransactionSearch.ts` - Search functionality
- **Infrastructure**:
  - `frontend/src/api/client.ts` - API client with file upload support
  - `frontend/vite.config.ts` - Vite configuration with proxy

---

### Phase 8: Integration & Docker (Deployment)
**Effort**: 2-3 days | **Priority**: HIGH | **Status**: Pending

#### Goals
- Embed frontend in backend binary
- Create multi-stage Docker build
- Single production-ready image
- Update CI/CD pipeline

#### Tasks

1. **Embed Frontend in Backend**
   - Build frontend: `npm run build` → `frontend/dist/`
   - Copy built files to `backend/cmd/server/static/`
   - Add embed directive to `backend/cmd/server/main.go`
   - Serve static files for all non-API routes

2. **Create Multi-Stage Dockerfile** (`docker/Dockerfile`)
   - Stage 1: Build frontend (Node.js)
   - Stage 2: Build backend with embedded frontend (Go)
   - Stage 3: Runtime (Alpine)

3. **Update Docker Compose** (`docker-compose.yml`)

4. **Update Taskfile.yml**
   - Add frontend build tasks
   - Update Docker build

5. **Update GitHub Actions** (`.github/workflows/release.yml`)
   - Build frontend before Docker image
   - Multi-platform builds

6. **Test Docker Build**

#### Validation Checklist
- [ ] Docker images build successfully (expensor + postgres)
- [ ] Expensor image size < 50MB (excluding postgres)
- [ ] Containers start and connect to each other
- [ ] PostgreSQL healthcheck passes
- [ ] Expensor waits for PostgreSQL to be ready
- [ ] Web UI accessible at http://localhost:8080
- [ ] OAuth flow works in container
- [ ] Token persists in volume
- [ ] Daemon runs and processes transactions
- [ ] Transactions stored in PostgreSQL
- [ ] Data persists across container restarts
- [ ] Multi-container orchestration works smoothly

#### Critical Files
- `docker/Dockerfile` - Multi-stage build
- `docker-compose.yml` - Docker Compose configuration
- `backend/cmd/server/main.go` - Updated with static file serving
- `Taskfile.yml` - Updated build tasks
- `.github/workflows/release.yml` - Updated CI/CD

---

### Phase 9: Configuration UI & Polish (Enhancement)
**Effort**: 3-4 days | **Priority**: MEDIUM | **Status**: Pending

#### Goals
- Add configuration panel in web UI
- Real-time status updates
- Improved error handling
- Testing and optimization

#### Tasks

1. **Configuration Panel**
   - `src/pages/Settings.tsx`
   - `src/components/PluginSelector.tsx`
   - Backend: Implement `PUT /api/config` handler

2. **Real-Time Updates**
   - Add Server-Sent Events (SSE) endpoint
   - Update `useStatus` hook for SSE

3. **Error Handling Improvements**
   - Better error messages
   - Retry logic
   - Toast notifications

4. **Frontend Testing**
   - Install testing libraries
   - Unit tests for components
   - Integration tests for API calls

5. **Performance Optimization**
   - Code splitting
   - Lazy loading
   - Asset optimization

6. **Accessibility**
   - ARIA labels
   - Keyboard navigation
   - Screen reader testing

7. **Documentation**
   - Update `README.md`
   - Create `docs/ARCHITECTURE.md`
   - Create `docs/PLUGIN_SYSTEM.md`

#### Validation Checklist
- [ ] Configuration panel works correctly
- [ ] Can switch between readers/writers
- [ ] **Automatic label rules can be created and managed**
- [ ] **New transactions automatically labeled based on rules**
- [ ] **Rule priorities work correctly**
- [ ] Real-time updates display correctly
- [ ] Error handling is robust
- [ ] Tests pass (backend + frontend)
- [ ] Lighthouse score > 90
- [ ] Accessible (WCAG AA compliant)
- [ ] Documentation complete

#### Critical Files
- `frontend/src/pages/Settings.tsx` - Configuration UI
- `frontend/src/components/LabelRuleEditor.tsx` - Automatic label rule management
- `backend/internal/api/handlers.go` - Config update handler + label rule endpoints
- `backend/internal/daemon/labeler.go` - Automatic label application logic
- `frontend/src/hooks/useStatus.ts` - SSE integration
- Documentation files

---

### Phase 10: Webhook System (Extensibility)
**Effort**: 3-4 days | **Priority**: LOW (Future Enhancement) | **Status**: Pending

#### Goal
Implement an event-driven webhook system that allows users to extend Expensor functionality by integrating external tools and services. This enables custom notifications, automation workflows, and third-party integrations without modifying Expensor's core code.

#### Use Cases
1. **Notification Systems**: Send transaction alerts to WhatsApp, Telegram, Slack, Discord
2. **Two-Way Integrations**: Reply to notifications to update transaction descriptions/labels
3. **Custom Analytics**: Stream transactions to external analytics platforms
4. **Budget Alerts**: Trigger webhooks when spending exceeds thresholds
5. **Automation**: Integration with Zapier, IFTTT, n8n for complex workflows
6. **Custom Workflows**: Trigger approval flows, expense reporting, or reconciliation

#### Architecture Design

**Event System**:
- Event bus pattern with pub/sub model
- Events emitted at key lifecycle points
- Webhook delivery with retry logic and exponential backoff
- Signature verification (HMAC) for security

**Event Types**:
```go
type EventType string

const (
    EventTransactionCreated  EventType = "transaction.created"
    EventTransactionUpdated  EventType = "transaction.updated"
    EventTransactionDeleted  EventType = "transaction.deleted"
    EventLabelAdded         EventType = "transaction.label_added"
    EventLabelRemoved       EventType = "transaction.label_removed"
    EventDescriptionUpdated EventType = "transaction.description_updated"
    EventReaderStarted      EventType = "reader.started"
    EventReaderStopped      EventType = "reader.stopped"
    EventReaderError        EventType = "reader.error"
)
```

#### Tasks

1. **Event Bus Implementation**
   - Create `backend/internal/events/bus.go`:
     - Event bus with concurrent-safe pub/sub
     - Event payload with transaction data
     - Async event processing with goroutines
     - Event queue with configurable buffer size

2. **Webhook Manager**
   - Create `backend/internal/webhooks/manager.go`:
     - Webhook registration and storage (database)
     - HTTP delivery with configurable timeout
     - Retry logic with exponential backoff (3 retries: 5s, 15s, 45s)
     - Dead letter queue for failed deliveries
     - Webhook signature generation (HMAC-SHA256)

3. **Database Schema**
   - Create `backend/migrations/002_create_webhooks.sql`:
     ```sql
     CREATE TABLE webhooks (
         id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
         name VARCHAR(255) NOT NULL,
         url TEXT NOT NULL,
         events TEXT[] NOT NULL, -- Array of event types
         secret VARCHAR(255) NOT NULL, -- For HMAC signature
         enabled BOOLEAN DEFAULT true,
         created_at TIMESTAMPTZ DEFAULT NOW(),
         updated_at TIMESTAMPTZ DEFAULT NOW()
     );

     CREATE TABLE webhook_deliveries (
         id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
         webhook_id UUID REFERENCES webhooks(id) ON DELETE CASCADE,
         event_type VARCHAR(100) NOT NULL,
         payload JSONB NOT NULL,
         response_status INT,
         response_body TEXT,
         error TEXT,
         delivered_at TIMESTAMPTZ,
         attempts INT DEFAULT 0,
         created_at TIMESTAMPTZ DEFAULT NOW()
     );

     CREATE INDEX idx_webhook_deliveries_webhook_id ON webhook_deliveries(webhook_id);
     CREATE INDEX idx_webhook_deliveries_created_at ON webhook_deliveries(created_at DESC);
     ```

4. **Integrate Event Emission**
   - Update `backend/pkg/writer/postgres/postgres.go`:
     - Emit `transaction.created` after INSERT
     - Emit `transaction.updated` after UPDATE
   - Update transaction API handlers:
     - Emit `transaction.label_added` when labels added
     - Emit `transaction.description_updated` when description changed
   - Update daemon runner:
     - Emit `reader.started`, `reader.stopped`, `reader.error`

5. **Webhook API Endpoints**
   - Add to `backend/internal/api/handlers.go`:
     - `POST /api/webhooks` - Create webhook
     - `GET /api/webhooks` - List all webhooks
     - `GET /api/webhooks/:id` - Get webhook details
     - `PUT /api/webhooks/:id` - Update webhook (URL, events, enabled)
     - `DELETE /api/webhooks/:id` - Delete webhook
     - `GET /api/webhooks/:id/deliveries` - List delivery history
     - `POST /api/webhooks/:id/test` - Test webhook with sample payload

6. **Webhook UI Components**
   - Create `frontend/src/pages/Webhooks.tsx`:
     - List all configured webhooks
     - Create/edit webhook form
     - Test webhook button
     - Enable/disable toggle
   - Create `frontend/src/components/WebhookForm.tsx`:
     - URL input with validation
     - Event type multi-select checkboxes
     - Secret generation button
     - Name and description fields
   - Create `frontend/src/components/WebhookDeliveryLog.tsx`:
     - Delivery history table
     - Success/failure status
     - Retry information
     - Response body viewer

7. **Security Considerations**
   - HMAC signature in `X-Expensor-Signature` header
   - Webhook secret stored encrypted in database
   - Rate limiting on webhook endpoints
   - Webhook URL validation (no localhost/internal IPs in production)
   - Timeout for HTTP requests (10 seconds)

8. **Documentation**
   - Create `docs/WEBHOOKS.md`:
     - Webhook setup guide
     - Event payload schemas
     - Signature verification examples (Go, Python, Node.js)
     - Example integrations (WhatsApp, Slack, Telegram)
   - Add webhook examples to `docs/examples/`:
     - `whatsapp-notifications/` - WhatsApp Business API integration
     - `telegram-bot/` - Telegram bot for notifications and updates
     - `slack-webhook/` - Slack incoming webhook integration

#### Event Payload Schema

```json
{
  "event_type": "transaction.created",
  "timestamp": "2026-01-11T12:34:56Z",
  "webhook_id": "uuid",
  "data": {
    "transaction": {
      "id": "uuid",
      "message_id": "msg123",
      "amount": 1234.56,
      "currency": "INR",
      "timestamp": "2026-01-11T10:00:00Z",
      "merchant_info": "SWIGGY",
      "category": "Food",
      "bucket": "Wants",
      "source": "Credit Card - ICICI",
      "description": "",
      "labels": ["food", "delivery"],
      "created_at": "2026-01-11T12:34:56Z"
    }
  }
}
```

#### Validation Checklist
- [ ] Event bus emits events correctly
- [ ] Webhooks registered and stored in database
- [ ] HTTP delivery works with retry logic
- [ ] HMAC signature generated and verified correctly
- [ ] Failed deliveries logged and retried
- [ ] Dead letter queue captures permanently failed deliveries
- [ ] Webhook UI allows create/edit/delete operations
- [ ] Test webhook button sends sample payload
- [ ] Delivery log shows success/failure history
- [ ] Documentation includes integration examples
- [ ] Security: signatures verified, rate limiting works
- [ ] No localhost webhooks allowed in production mode

#### Critical Files
- `backend/internal/events/bus.go` - Event bus implementation
- `backend/internal/webhooks/manager.go` - Webhook delivery manager
- `backend/migrations/002_create_webhooks.sql` - Database schema
- `backend/internal/api/handlers.go` - Webhook API endpoints
- `frontend/src/pages/Webhooks.tsx` - Webhook management UI
- `frontend/src/components/WebhookForm.tsx` - Create/edit form
- `docs/WEBHOOKS.md` - Webhook documentation
- `docs/examples/whatsapp-notifications/` - WhatsApp integration example

#### Future Enhancements
- Webhook templates for common services (Slack, Discord, Telegram)
- Webhook payload filtering (only send transactions > $100)
- Webhook transformation (custom payload formats)
- Bi-directional webhooks (receive updates from external services)
- Webhook metrics and monitoring dashboard
- Webhook marketplace (community-contributed integrations)

---

## Environment Variables Reference

### Plugin System
- `EXPENSOR_READER` - Reader plugin name (default: "gmail")
- `EXPENSOR_WRITER` - Writer plugin name (default: "sheets")
- `EXPENSOR_READER_CONFIG` - Reader config as JSON string (optional)
- `EXPENSOR_WRITER_CONFIG` - Writer config as JSON string (optional)

### Server
- `PORT` - HTTP server port (default: 8080)
- `BASE_URL` - Base URL for OAuth callbacks (default: "http://localhost:8080")
- `LOG_LEVEL` - Log level

### Example Configurations

**Gmail + PostgreSQL (Recommended):**
```bash
EXPENSOR_READER=gmail
EXPENSOR_WRITER=postgres
POSTGRES_HOST=postgres
POSTGRES_PORT=5432
POSTGRES_DB=expensor
POSTGRES_USER=expensor
POSTGRES_PASSWORD=expensor_password
POSTGRES_SSLMODE=disable
```

**Gmail + Sheets (Legacy):**
```bash
EXPENSOR_READER=gmail
EXPENSOR_WRITER=sheets
EXPENSOR_WRITER_CONFIG='{"sheet_id":"abc123","sheet_name":"Sheet1"}'
```

**Thunderbird + CSV:**
```bash
EXPENSOR_READER=thunderbird
EXPENSOR_READER_CONFIG='{"profilePath":"","mailboxes":["INBOX"],"stateFile":"data/thunderbird_state.json"}'
EXPENSOR_WRITER=csv
EXPENSOR_WRITER_CONFIG='{"output_path":"data/transactions.csv"}'
```

**Multi-Writer Setup (PostgreSQL + Sheets):**
```bash
EXPENSOR_READER=gmail
EXPENSOR_WRITER=postgres,sheets
POSTGRES_HOST=postgres
# ... postgres config
EXPENSOR_WRITER_CONFIG='{"sheets":{"sheet_id":"abc123","sheet_name":"Sheet1"}}'
```

---

## Critical Files Summary

### Backend
1. `backend/cmd/server/main.go` - HTTP server, daemon orchestration, static file serving
2. `backend/internal/plugins/registry.go` - Plugin registry system
3. `backend/internal/api/handlers.go` - API endpoint handlers (OAuth, transactions, labels)
4. `backend/pkg/client/oauth.go` - Web-only OAuth flow
5. `backend/internal/daemon/runner.go` - Refactored daemon logic
6. `backend/internal/daemon/labeler.go` - Automatic label application
7. `backend/pkg/config/config.go` - Configuration with plugin support
8. `backend/pkg/reader/thunderbird/thunderbird.go` - Thunderbird reader
9. `backend/pkg/plugins/readers/thunderbird/plugin.go` - Thunderbird plugin wrapper
10. **`backend/pkg/writer/postgres/postgres.go`** - PostgreSQL writer with multi-currency
11. **`backend/pkg/plugins/writers/postgres/plugin.go`** - PostgreSQL plugin wrapper
12. **`backend/migrations/001_create_transactions.sql`** - Database schema

### Frontend
13. `frontend/src/hooks/useOAuth.ts` - OAuth flow management
14. `frontend/src/components/OAuthButton.tsx` - Main CTA
15. `frontend/src/pages/Dashboard.tsx` - Status dashboard
16. **`frontend/src/pages/Transactions.tsx`** - Transaction list page
17. **`frontend/src/components/TransactionTable.tsx`** - Transaction table with filters
18. **`frontend/src/components/LabelSelector.tsx`** - Label management
19. **`frontend/src/hooks/useTransactions.ts`** - Transaction data management
20. `frontend/src/api/client.ts` - API client with types

### Infrastructure
21. `docker/Dockerfile` - Multi-stage build
22. `docker-compose.yml` - Multi-container with PostgreSQL
23. `.claude/agents/frontend-expert.md` - Frontend sub-agent

### Configuration
24. **`content/rules.json`** - Transaction extraction rules (moved from backend/cmd/server/config/)
25. **`content/labels.json`** - Merchant-to-category mappings (moved from backend/cmd/server/config/)

### Webhooks (Phase 10)
26. **`backend/internal/events/bus.go`** - Event bus for pub/sub
27. **`backend/internal/webhooks/manager.go`** - Webhook delivery with retry logic
28. **`backend/migrations/002_create_webhooks.sql`** - Webhook database schema
29. **`frontend/src/pages/Webhooks.tsx`** - Webhook management UI
30. **`docs/WEBHOOKS.md`** - Webhook integration guide

---

## Breaking Changes (No Backward Compatibility)

This is a clean-slate rewrite. Old CLI commands and legacy configuration are **removed**:

### Removed
- ❌ CLI commands (`expensor setup`, `expensor run`, `expensor status`)
- ❌ Local OAuth callback server (port 8085)
- ❌ Manual file mounting of credentials (no more `-v ./client_secret.json:/app/data/client_secret.json`)
- ❌ Legacy env vars (`GSHEETS_*`)
- ❌ Standalone binary mode

### New Approach
- ✅ Web-only interface with UI-first onboarding
- ✅ Upload credentials via web UI (no manual file mounting)
- ✅ Single `serve` command
- ✅ Plugin selection and configuration through web forms
- ✅ OAuth through web UI only
- ✅ All setup completed in browser - minimal Docker command

**Rationale**: Removing technical debt early allows faster development and cleaner codebase. Focus on web UI as the primary interface.

### Quick Start Example

**Before (Old Approach)**:
```bash
# Download client_secret.json from Google Cloud Console
# Create docker-compose.yml with volume mounts
# docker-compose up
# Navigate to http://localhost:8080
# Click setup button and follow OAuth flow
```

**After (New UI-First Approach)**:
```bash
# Just run the container with data volume
docker run -d \
  -p 8080:8080 \
  -v expensor-data:/app/data \
  expensor:latest

# Navigate to http://localhost:8080
# Complete 5-step onboarding wizard:
#   1. Select Gmail reader
#   2. Upload client_secret.json (downloaded from Google)
#   3. Click "Connect Gmail" and complete OAuth
#   4. Select PostgreSQL writer and configure
#   5. Start tracking!
```

**Even Simpler with Docker Compose**:
```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: expensor
      POSTGRES_USER: expensor
      POSTGRES_PASSWORD: expensor_password
    volumes:
      - postgres_data:/var/lib/postgresql/data

  expensor:
    image: expensor:latest
    ports:
      - "8080:8080"
    volumes:
      - expensor-data:/app/data
    depends_on:
      - postgres
    environment:
      - POSTGRES_HOST=postgres

volumes:
  postgres_data:
  expensor-data:
```

Then visit `http://localhost:8080` and complete onboarding!

---

## Success Criteria

### Functional
- [ ] **UI-first onboarding: users can complete full setup in browser without touching config files**
- [ ] **File upload works: upload client_secret.json via web UI**
- [ ] **Onboarding wizard guides through all 5 steps seamlessly**
- [ ] Web UI OAuth flow completes successfully (after file upload)
- [ ] Multi-container Docker setup deploys successfully (expensor + postgres)
- [ ] Plugin system supports Gmail and Thunderbird readers
- [ ] Plugin system supports Sheets, CSV, JSON, **PostgreSQL** writers
- [ ] **Multi-currency transactions stored and displayed correctly**
- [ ] **Transaction list shows all transactions with pagination**
- [ ] **Users can add descriptions and labels to transactions**
- [ ] **Full-text search works across merchant info and descriptions**
- [ ] **Automatic label rules apply to new transactions**
- [ ] **Filters work (date, category, labels, currency)**
- [ ] Configuration via environment variables works
- [ ] **Webhooks fire correctly on transaction events (Phase 10)**
- [ ] **Webhook delivery retry logic works (Phase 10)**
- [ ] **HMAC signatures verify correctly (Phase 10)**

### Non-Functional
- [ ] Expensor Docker image size < 50MB
- [ ] Frontend loads in < 2 seconds
- [ ] API response times < 200ms (with database queries)
- [ ] Transaction table handles 10,000+ transactions smoothly
- [ ] Full-text search returns results in < 500ms
- [ ] PostgreSQL queries optimized with proper indexes
- [ ] All tests pass (backend + frontend + integration)
- [ ] Documentation complete

### User Experience
- [ ] **Onboarding wizard is clear and easy to follow (5 steps)**
- [ ] **File upload is intuitive with drag-and-drop support**
- [ ] **Each plugin shows helpful documentation links (Gmail, Thunderbird setup guides)**
- [ ] **Configuration forms are self-explanatory (clear labels, placeholders, examples)**
- [ ] OAuth flow is intuitive (disabled until credentials uploaded)
- [ ] Status dashboard is informative with quick stats
- [ ] **Transaction table is easy to navigate and filter**
- [ ] **Adding descriptions and labels is seamless**
- [ ] **Search results are relevant and fast**
- [ ] **Multi-currency amounts are clear and well-formatted**
- [ ] **Automatic label rules reduce manual work**
- [ ] Error messages are helpful and actionable
- [ ] Responsive on mobile/desktop
- [ ] Accessible (WCAG AA)

---

## Execution Order

1. **Phase 1** - Frontend Sub-Agent ✅ (30 min, complete)
2. **Phase 2** - Restructure & Plugin System ✅ (MUST complete before others, complete)
3. **Phase 3** - Thunderbird Reader Plugin 🔄 (extends plugin system, in progress)
4. **Phase 4** - Database Writer Plugin (PostgreSQL with multi-currency support)
5. **Phase 5** - Move Configuration Files ✅ (quick organizational task, complete)
6. **Phase 6** - Web Server & API (Backend infrastructure with transaction endpoints)
7. **Phase 7** - Frontend (Transaction list, search, labels, descriptions)
8. **Phase 8** - Integration & Docker (Multi-container with PostgreSQL)
9. **Phase 9** - Configuration UI & Polish (Label automation, advanced features)
10. **Phase 10** - Webhook System (Event-driven extensibility for integrations)

### Parallel Work Opportunities
- After Phase 2: Phase 3 (Thunderbird) + Phase 4 (PostgreSQL) can run in parallel
- Phase 5 (Move Config) ✅ completed
- After Phase 6: Phase 7 (Frontend) + Phase 8 (Docker setup) can overlap
- Phase 9 can be done incrementally while using the app
- Phase 10 can be done after Phase 4 (requires database) and Phase 6 (requires API)

---

## Notes

- **Prioritize Phases 1-7** for MVP (web UI with OAuth onboarding + transaction management)
- Phase 8 (Docker) and Phase 9 (Polish) can be done incrementally
- **Phase 10 (Webhooks)** is a future enhancement - design earlier phases with event hooks in mind
- Plugin system designed for future extensibility (Outlook, Notion, etc.)
- Frontend sub-agent can assist with all React/TypeScript development
- **PostgreSQL chosen for database** due to JSON support, full-text search, and multi-currency handling
- **Multi-currency support** is built into the data model from the start
- **Automatic label rules** reduce manual work for recurring merchants
- **Webhook system** enables extensibility without modifying core code (notifications, automation, integrations)
- This plan is designed to be continued across multiple conversations

## Key Features Added

1. **Database Storage (PostgreSQL)**:
   - Persistent transaction storage
   - Multi-currency support (INR, USD, EUR, etc.)
   - Exchange rate tracking
   - Full-text search on merchant info and descriptions

2. **Transaction Management UI**:
   - Sortable, filterable transaction table
   - Add descriptions to identify transactions
   - Manual label management per transaction
   - Full-text search across all fields

3. **Multi-Currency Support**:
   - Store original and converted amounts
   - Track exchange rates
   - Display with proper currency symbols
   - Filter by currency

4. **Label System**:
   - Manual labels per transaction
   - Automatic label rules ("If merchant contains X, add label Y")
   - Search by labels
   - Label management UI

5. **Configuration Files Moved**:
   - `content/rules.json` - Transaction extraction rules
   - `content/labels.json` - Merchant-to-category mappings
   - Better organization at project root

6. **Webhook System (Phase 10 - Future)**:
   - Event-driven architecture for extensibility
   - Custom notifications (WhatsApp, Telegram, Slack, Discord)
   - Two-way integrations (reply to update transaction notes)
   - Automation workflows (Zapier, IFTTT, n8n integration)
   - Budget alerts and custom triggers
   - No code changes needed for new integrations

---

## Session Notes

### Session 1 (2026-01-11)
- ✅ Created plan in plan mode
- ✅ Completed Phase 1: Created frontend-expert sub-agent
- ✅ Completed Phase 2: Restructured backend and implemented plugin system
  - Used golang-expert agent for implementation
  - Binary compiles successfully
  - Documentation created
- 🔄 Started Phase 3: Thunderbird Reader Plugin
- 📝 Updated plan with new features:
  - PostgreSQL database writer plugin (Phase 4)
  - Multi-currency transaction support
  - Transaction management UI with labels and descriptions
  - Automatic label rules
  - Move configuration files to `content/` directory
  - Updated Docker Compose for multi-container setup

**Next Steps**:
- Option A: Continue with Phase 3 (Thunderbird Reader)
- Option B: Start Phase 4 (PostgreSQL Writer) in parallel
- Option C: Quick win - Phase 5 (Move config files to content/)
