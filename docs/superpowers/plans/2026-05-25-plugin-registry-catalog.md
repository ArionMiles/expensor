# Plugin Registry Catalog Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `internal/plugins.Registry` a catalog-only type while moving reader/writer assembly and persisted reader config handling to daemon wiring.

**Architecture:** Plugins expose required metadata through explicit metadata structs and construct readers/writers from input structs instead of long flattened argument lists. The registry only registers, retrieves, lists, and combines plugin scope metadata; daemon runtime code resolves plugins and calls constructors directly. API handlers keep the same response shape but read all reader metadata, including setup guides, from `ReaderMetadata`.

**Tech Stack:** Go 1.26, `log/slog`, `encoding/json`, existing `task` backend test targets, existing Gmail/Thunderbird/Postgres plugin packages.

---

## Scope Check

This plan implements implementation-order item 5 from `docs/superpowers/specs/2026-05-24-backend-observability-and-code-health.md`: plugin metadata and construction boundary cleanup. It intentionally does not split `internal/api/handlers.go`, change processed-message state contexts, move Gmail query construction out of `pkg/api`, or add new observability spans. The result should be independently testable with backend unit tests and the prod linter.

## File Structure

- `backend/internal/plugins/registry.go`: keep plugin catalog and shared plugin-facing types; add metadata/input structs; remove optional `GuideProvider`, optional `ConfigApplier`, and `Registry.CreateReader/CreateWriter`.
- `backend/internal/plugins/registry_test.go`: change tests from factory behavior to catalog-only behavior, metadata exposure, and input struct constructor behavior through test plugins.
- `backend/internal/daemon/runner.go`: own daemon assembly by retrieving plugins from the registry, loading persisted reader config, and calling `plugin.NewReader(plugins.ReaderInput{...})` / `plugin.NewWriter(plugins.WriterInput{...})`.
- `backend/internal/daemon/runner_test.go`: update mock plugins to metadata/input structs and assert persisted reader config is passed to reader construction without optional config mutation.
- `backend/internal/api/handlers.go`: read reader/writer listing, auth, readiness, credentials, and guide behavior from metadata.
- `backend/internal/api/handlers_test.go`: update test plugins; add guide success coverage now that guides are required metadata rather than an optional interface.
- `backend/pkg/plugins/readers/gmail/plugin.go`: return `ReaderMetadata`; accept `ReaderInput`.
- `backend/pkg/plugins/readers/gmail/plugin_test.go`: update constructor tests to `ReaderInput`.
- `backend/pkg/plugins/readers/thunderbird/plugin.go`: return `ReaderMetadata`; accept `ReaderInput`; decode persisted `ReaderConfig` directly into Thunderbird reader config.
- `backend/pkg/plugins/readers/thunderbird/plugin_test.go`: add persisted config decode tests and update constructor tests.
- `backend/pkg/plugins/writers/postgres/plugin.go`: return `WriterMetadata`; accept `WriterInput`.
- `backend/pkg/plugins/writers/postgres/plugin_test.go`: update constructor tests.
- `backend/cmd/server/main.go`: no behavior change expected; compile-time updates only for metadata/constructor API if needed.
- `docs/superpowers/specs/2026-05-24-backend-observability-and-code-health.md`: mark plugin registry cleanup complete after implementation and verification.

---

### Task 1: Introduce Metadata And Input Struct API

**Files:**
- Modify: `backend/internal/plugins/registry.go`
- Modify: `backend/internal/plugins/registry_test.go`

- [ ] **Step 1: Write failing tests for catalog-only registry behavior**

Update the test mock types in `backend/internal/plugins/registry_test.go` so readers and writers expose metadata and input-struct constructors:

```go
type mockReaderPlugin struct {
	name        string
	description string
	scopes      []string
	guide       []byte
	reader      api.Reader
	err         error
	input       ReaderInput
}

func (m *mockReaderPlugin) Metadata() ReaderMetadata {
	return ReaderMetadata{
		Name:        m.name,
		Description: m.description,
		Auth: AuthSpec{
			Type:                      AuthTypeOAuth,
			RequiredScopes:            m.scopes,
			RequiresCredentialsUpload: false,
		},
		ConfigSchema: nil,
		SetupGuide:   m.guide,
	}
}

func (m *mockReaderPlugin) NewReader(input ReaderInput) (api.Reader, error) {
	m.input = input
	if m.err != nil {
		return nil, m.err
	}
	return m.reader, nil
}

type mockWriterPlugin struct {
	name        string
	description string
	scopes      []string
	writer      api.Writer
	err         error
	input       WriterInput
}

func (m *mockWriterPlugin) Metadata() WriterMetadata {
	return WriterMetadata{Name: m.name, Description: m.description, RequiredScopes: m.scopes}
}

func (m *mockWriterPlugin) NewWriter(input WriterInput) (api.Writer, error) {
	m.input = input
	if m.err != nil {
		return nil, m.err
	}
	return m.writer, nil
}
```

Replace assertions that call `Name()`, `Description()`, and `RequiredScopes()` with `Metadata()` assertions. Add a test that registry no longer exposes factory helpers:

```go
func TestRegistryIsCatalogOnly(t *testing.T) {
	registry := NewRegistry()
	reader := &mockReaderPlugin{name: "test-reader", reader: &mockReader{}}
	writer := &mockWriterPlugin{name: "test-writer", writer: &mockWriter{}}

	if err := registry.RegisterReader(reader); err != nil {
		t.Fatalf("RegisterReader() error = %v", err)
	}
	if err := registry.RegisterWriter(writer); err != nil {
		t.Fatalf("RegisterWriter() error = %v", err)
	}

	gotReader, err := registry.GetReader("test-reader")
	if err != nil {
		t.Fatalf("GetReader() error = %v", err)
	}
	if gotReader.Metadata().Name != "test-reader" {
		t.Fatalf("reader name = %q, want test-reader", gotReader.Metadata().Name)
	}

	gotWriter, err := registry.GetWriter("test-writer")
	if err != nil {
		t.Fatalf("GetWriter() error = %v", err)
	}
	if gotWriter.Metadata().Name != "test-writer" {
		t.Fatalf("writer name = %q, want test-writer", gotWriter.Metadata().Name)
	}
}
```

Delete `TestCreateReader`, `TestCreateReader_NotFound`, `TestCreateWriter`, and `TestCreateWriter_NotFound`; those behaviors will move to daemon tests.

- [ ] **Step 2: Run registry tests to verify red**

Run:

```bash
task test:be -- ./internal/plugins
```

Expected: FAIL because `ReaderMetadata`, `WriterMetadata`, `ReaderInput`, `WriterInput`, and `Metadata()` do not exist yet.

- [ ] **Step 3: Implement metadata and input structs**

Replace the plugin interfaces and remove optional interfaces in `backend/internal/plugins/registry.go`:

```go
type AuthSpec struct {
	Type                      AuthType `json:"type"`
	RequiredScopes            []string `json:"required_scopes"`
	RequiresCredentialsUpload bool     `json:"requires_credentials_upload"`
}

type ReaderMetadata struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Auth         AuthSpec        `json:"auth"`
	ConfigSchema []ConfigField   `json:"config_schema"`
	SetupGuide   json.RawMessage `json:"setup_guide,omitempty"`
}

type WriterMetadata struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	RequiredScopes []string `json:"required_scopes"`
}

type ReaderInput struct {
	HTTPClient     *http.Client
	AppConfig      *config.Config
	ReaderConfig   json.RawMessage
	Rules          []api.Rule
	Resolver       api.CategoryResolver
	StateManager   *state.Manager
	DiagnosticSink api.DiagnosticSink
	Logger         *slog.Logger
}

type WriterInput struct {
	HTTPClient *http.Client
	AppConfig  *config.Config
	Logger     *slog.Logger
}

type ReaderPlugin interface {
	Metadata() ReaderMetadata
	NewReader(input ReaderInput) (api.Reader, error)
}

type WriterPlugin interface {
	Metadata() WriterMetadata
	NewWriter(input WriterInput) (api.Writer, error)
}
```

Update registry methods to use `plugin.Metadata().Name`, update `GetAllScopes` to read `reader.Metadata().Auth.RequiredScopes` and `writer.Metadata().RequiredScopes`, and remove `CreateReader` / `CreateWriter`.

- [ ] **Step 4: Run registry tests to verify green**

Run:

```bash
task test:be -- ./internal/plugins
```

Expected: PASS.

- [ ] **Step 5: Commit registry API change**

Run:

```bash
git add backend/internal/plugins/registry.go backend/internal/plugins/registry_test.go
git commit --no-gpg-sign -m "Refactor plugin registry as a catalog"
```

---

### Task 2: Move Reader And Writer Assembly Into Daemon Runner

**Files:**
- Modify: `backend/internal/daemon/runner.go`
- Modify: `backend/internal/daemon/runner_test.go`

- [ ] **Step 1: Write failing daemon tests for direct plugin construction**

Update daemon mock plugins to implement `Metadata()` and input-struct constructors. Add a test that persisted reader config is passed into `ReaderInput.ReaderConfig`:

```go
func TestRun_PassesPersistedReaderConfigToPlugin(t *testing.T) {
	reader := &mockReader{done: make(chan struct{})}
	writer := &mockWriter{done: make(chan struct{})}
	readerPlugin := &mockReaderPlugin{name: "test-reader", reader: reader}
	writerPlugin := &mockWriterPlugin{name: "test-writer", writer: writer}
	registry := plugins.NewRegistry()
	if err := registry.RegisterReader(readerPlugin); err != nil {
		t.Fatalf("RegisterReader() error = %v", err)
	}
	if err := registry.RegisterWriter(writerPlugin); err != nil {
		t.Fatalf("RegisterWriter() error = %v", err)
	}

	runtimeStore := &mockRuntimeStore{
		readerConfig: json.RawMessage(`{"config":{"profilePath":"/tmp/profile","mailboxes":"Inbox"}}`),
		hasConfig:    true,
	}
	runner := New(registry, &http.Client{}, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	reader.onRead = func() { cancel() }
	err := runner.Run(ctx, RunConfig{
		ReaderName:   "test-reader",
		WriterName:   "test-writer",
		Config:       &config.Config{},
		RuntimeStore: runtimeStore,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if string(readerPlugin.input.ReaderConfig) != string(runtimeStore.readerConfig) {
		t.Fatalf("ReaderConfig = %s, want %s", readerPlugin.input.ReaderConfig, runtimeStore.readerConfig)
	}
}
```

Update `TestRun_CreateReaderError` and `TestRun_CreateWriterError` to expect errors from direct `plugin.NewReader` and `plugin.NewWriter` calls rather than `Registry.CreateReader/CreateWriter`.

- [ ] **Step 2: Run daemon tests to verify red**

Run:

```bash
task test:be -- ./internal/daemon
```

Expected: FAIL because daemon still calls removed registry factory helpers and still uses `ConfigApplier`.

- [ ] **Step 3: Implement daemon-owned assembly**

In `backend/internal/daemon/runner.go`, remove `persistedReaderConfigInput` and `applyPersistedReaderConfig`. Add:

```go
func (r *Runner) loadReaderConfig(ctx context.Context, readerName string, store ReaderRuntimeStore) json.RawMessage {
	if store == nil {
		return nil
	}
	data, ok, err := store.GetReaderConfig(ctx, readerName)
	if err != nil {
		r.logger.Warn("failed to read persisted reader config", "reader", readerName, "error", err)
		return nil
	}
	if !ok {
		return nil
	}
	return data
}
```

Then replace registry factory calls in `Run`:

```go
readerPlugin, err := r.registry.GetReader(runCfg.ReaderName)
if err != nil {
	return fmt.Errorf("creating reader: %w", err)
}
reader, err := readerPlugin.NewReader(plugins.ReaderInput{
	HTTPClient:     r.httpClient,
	AppConfig:      cfg,
	ReaderConfig:   r.loadReaderConfig(ctx, runCfg.ReaderName, runCfg.RuntimeStore),
	Rules:          runCfg.Rules,
	Resolver:       runCfg.Resolver,
	StateManager:   runCfg.StateManager,
	DiagnosticSink: runCfg.DiagnosticSink,
	Logger:         r.logger.With("component", "reader", "plugin", runCfg.ReaderName),
})
if err != nil {
	return fmt.Errorf("creating reader: %w", err)
}

writerPlugin, err := r.registry.GetWriter(runCfg.WriterName)
if err != nil {
	return fmt.Errorf("creating writer: %w", err)
}
writer, err := writerPlugin.NewWriter(plugins.WriterInput{
	HTTPClient: r.httpClient,
	AppConfig:  cfg,
	Logger:     r.logger.With("component", "writer", "plugin", runCfg.WriterName),
})
if err != nil {
	return fmt.Errorf("creating writer: %w", err)
}
```

- [ ] **Step 4: Run daemon tests to verify green**

Run:

```bash
task test:be -- ./internal/daemon
```

Expected: PASS.

- [ ] **Step 5: Commit daemon assembly change**

Run:

```bash
git add backend/internal/daemon/runner.go backend/internal/daemon/runner_test.go
git commit --no-gpg-sign -m "Move plugin assembly into daemon runner"
```

---

### Task 3: Update API Handlers To Use Required Metadata

**Files:**
- Modify: `backend/internal/api/handlers.go`
- Modify: `backend/internal/api/handlers_test.go`

- [ ] **Step 1: Write failing API metadata tests**

Update `testReaderPlugin` and `testWriterPlugin` in `backend/internal/api/handlers_test.go` to implement metadata and input-struct constructors. Add a successful guide test:

```go
func TestHandleGetReaderGuide_ReturnsMetadataGuide(t *testing.T) {
	h := newTestHandlers()
	registry := plugins.NewRegistry()
	guide := json.RawMessage(`{"sections":[{"title":"Setup","steps":[{"text":"Do the setup"}]}]}`)
	if err := registry.RegisterReader(&testReaderPlugin{name: "guided", authType: plugins.AuthTypeConfig, guide: guide}); err != nil {
		t.Fatalf("RegisterReader() error = %v", err)
	}
	h.registry = registry

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/readers/guided/guide", nil)
	req.SetPathValue("name", "guided")
	rr := httptest.NewRecorder()

	h.HandleGetReaderGuide(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Do the setup") {
		t.Fatalf("body = %s, want setup guide", rr.Body.String())
	}
}
```

Keep `TestHandleGetReaderGuide_NoGuide_Returns404`, but make it use a reader with empty `Metadata().SetupGuide`.

- [ ] **Step 2: Run API tests to verify red**

Run:

```bash
task test:be -- ./internal/api
```

Expected: FAIL because handlers still call methods removed from plugin interfaces and still type-assert `GuideProvider`.

- [ ] **Step 3: Update handlers to read metadata**

In `backend/internal/api/handlers.go`, replace direct method calls as follows:

```go
meta := p.Metadata()
ReaderInfo{
	Name:                      meta.Name,
	Description:               meta.Description,
	AuthType:                  meta.Auth.Type,
	RequiresCredentialsUpload: meta.Auth.RequiresCredentialsUpload,
	ConfigSchema:              meta.ConfigSchema,
}
```

Use `plugin.Metadata().Auth.Type`, `plugin.Metadata().Auth.RequiredScopes`, `plugin.Metadata().Auth.RequiresCredentialsUpload`, and `plugin.Metadata().ConfigSchema` in credentials, OAuth, auth status, reader status, and setup guide handlers. Replace the guide handler body with:

```go
guideData := plugin.Metadata().SetupGuide
if len(guideData) == 0 {
	writeError(w, http.StatusNotFound, "no setup guide available for this reader")
	return
}
var guide plugins.ReaderGuide
if err := json.Unmarshal(guideData, &guide); err != nil {
	h.logger.Error("parsing reader guide", "reader", name, "error", err)
	writeError(w, http.StatusInternalServerError, "failed to parse reader guide")
	return
}
writeJSON(w, http.StatusOK, guide)
```

- [ ] **Step 4: Run API tests to verify green**

Run:

```bash
task test:be -- ./internal/api
```

Expected: PASS.

- [ ] **Step 5: Commit API metadata usage**

Run:

```bash
git add backend/internal/api/handlers.go backend/internal/api/handlers_test.go
git commit --no-gpg-sign -m "Read plugin metadata in API handlers"
```

---

### Task 4: Update Concrete Reader And Writer Plugins

**Files:**
- Modify: `backend/pkg/plugins/readers/gmail/plugin.go`
- Modify: `backend/pkg/plugins/readers/gmail/plugin_test.go`
- Modify: `backend/pkg/plugins/readers/thunderbird/plugin.go`
- Modify: `backend/pkg/plugins/readers/thunderbird/plugin_test.go`
- Modify: `backend/pkg/plugins/writers/postgres/plugin.go`
- Modify: `backend/pkg/plugins/writers/postgres/plugin_test.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Write failing concrete plugin tests**

Update Gmail tests to assert metadata:

```go
func TestPlugin_Metadata(t *testing.T) {
	p := &gmailplugin.Plugin{}
	p.SetGuideData([]byte(`{"sections":[]}`))

	meta := p.Metadata()

	if meta.Name != "gmail" {
		t.Fatalf("Name = %q, want gmail", meta.Name)
	}
	if meta.Auth.Type != plugins.AuthTypeOAuth {
		t.Fatalf("Auth.Type = %q, want oauth", meta.Auth.Type)
	}
	if !meta.Auth.RequiresCredentialsUpload {
		t.Fatal("RequiresCredentialsUpload = false, want true")
	}
	if len(meta.Auth.RequiredScopes) == 0 {
		t.Fatal("RequiredScopes is empty")
	}
	if len(meta.SetupGuide) == 0 {
		t.Fatal("SetupGuide is empty")
	}
}
```

Update Thunderbird tests to assert persisted reader config is decoded by `NewReader`:

```go
func TestPlugin_NewReader_UsesPersistedReaderConfig(t *testing.T) {
	plugin := &thunderbirdplugin.Plugin{}
	logger := testLogger()
	stateManager, err := state.New(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatalf("state.New() error = %v", err)
	}

	reader, err := plugin.NewReader(plugins.ReaderInput{
		AppConfig:    &config.Config{ScanInterval: 30},
		ReaderConfig: json.RawMessage(`{"config":{"profilePath":"/tmp/profile","mailboxes":"Inbox,Sent"}}`),
		StateManager: stateManager,
		Logger:       logger,
	})
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	if reader == nil {
		t.Fatal("NewReader() returned nil reader")
	}
}
```

Update Postgres tests to use `WriterInput`.

- [ ] **Step 2: Run plugin package tests to verify red**

Run:

```bash
task test:be -- ./pkg/plugins/readers/gmail ./pkg/plugins/readers/thunderbird ./pkg/plugins/writers/postgres ./cmd/server
```

Expected: FAIL because concrete plugins still expose old methods and constructors.

- [ ] **Step 3: Implement concrete metadata and input constructors**

For Gmail, implement:

```go
func (p *Plugin) Metadata() plugins.ReaderMetadata {
	return plugins.ReaderMetadata{
		Name:        "gmail",
		Description: "Read expense transactions from Gmail messages",
		Auth: plugins.AuthSpec{
			Type:                      plugins.AuthTypeOAuth,
			RequiredScopes:            []string{gmailapi.GmailReadonlyScope},
			RequiresCredentialsUpload: true,
		},
		ConfigSchema: []plugins.ConfigField{},
		SetupGuide:   p.guideData,
	}
}

func (p *Plugin) NewReader(input plugins.ReaderInput) (api.Reader, error) {
	cfg := input.AppConfig
	interval := time.Duration(cfg.ScanInterval) * time.Second
	if interval == 0 {
		interval = 60 * time.Second
	}
	readerCfg := gmailreader.Config{
		Rules:          input.Rules,
		Resolver:       input.Resolver,
		Interval:       interval,
		State:          input.StateManager,
		LookbackDays:   cfg.LookbackDays,
		LastScanAt:     cfg.LastScanAt,
		ForceFullScan:  cfg.ForceFullScan,
		OnCheckpoint:   cfg.OnCheckpoint,
		DiagnosticSink: input.DiagnosticSink,
	}
	return gmailreader.New(input.HTTPClient, readerCfg, input.Logger)
}
```

For Thunderbird, create a small private decoder:

```go
type persistedReaderConfig struct {
	Config struct {
		ProfilePath string `json:"profilePath"`
		Mailboxes   string `json:"mailboxes"`
	} `json:"config"`
}

func applyReaderConfig(cfg *config.Config, raw json.RawMessage) {
	if len(raw) == 0 {
		return
	}
	var persisted persistedReaderConfig
	if err := json.Unmarshal(raw, &persisted); err != nil {
		return
	}
	if persisted.Config.ProfilePath != "" {
		cfg.Thunderbird.ProfilePath = persisted.Config.ProfilePath
	}
	if persisted.Config.Mailboxes != "" {
		cfg.Thunderbird.Mailboxes = persisted.Config.Mailboxes
	}
}
```

Call `applyReaderConfig(cfg, input.ReaderConfig)` before building `tbreader.Config`.

For Postgres, implement `Metadata() plugins.WriterMetadata` and change `NewWriter(input plugins.WriterInput)`.

Update `registerPlugins` to log/register using `p.Metadata().Name`.

- [ ] **Step 4: Run plugin package tests to verify green**

Run:

```bash
task test:be -- ./pkg/plugins/readers/gmail ./pkg/plugins/readers/thunderbird ./pkg/plugins/writers/postgres ./cmd/server
```

Expected: PASS.

- [ ] **Step 5: Commit concrete plugin migration**

Run:

```bash
git add backend/pkg/plugins/readers/gmail backend/pkg/plugins/readers/thunderbird backend/pkg/plugins/writers/postgres backend/cmd/server/main.go
git commit --no-gpg-sign -m "Migrate concrete plugins to metadata inputs"
```

---

### Task 5: Remove Old Plugin Surface And Update Spec Status

**Files:**
- Modify: `backend/internal/plugins/registry.go`
- Modify: `docs/superpowers/specs/2026-05-24-backend-observability-and-code-health.md`

- [ ] **Step 1: Search for removed optional/factory APIs**

Run:

```bash
rg -n "GuideProvider|ConfigApplier|CreateReader|CreateWriter|RequiredScopes\\(|RequiresCredentialsUpload\\(|ConfigSchema\\(|AuthType\\(|Description\\(|Name\\(\\)" backend
```

Expected: FAIL as a cleanup check if any old plugin-surface references remain outside comments that are being edited in this task.

- [ ] **Step 2: Remove stale comments and update the spec status**

In `backend/internal/plugins/registry.go`, ensure no comments mention optional guide/config interfaces or registry factory helpers.

In `docs/superpowers/specs/2026-05-24-backend-observability-and-code-health.md`, update the program status table row:

```markdown
| Plugin registry cleanup | Complete | Registry is now catalog-only; reader/writer metadata is explicit; daemon wiring owns construction. |
```

Update the implementation order item:

```markdown
5. Refactor plugin metadata and construction boundary. **Complete.**
```

- [ ] **Step 3: Run cleanup search again**

Run:

```bash
rg -n "GuideProvider|ConfigApplier|CreateReader|CreateWriter" backend docs/superpowers/specs/2026-05-24-backend-observability-and-code-health.md
```

Expected: no output.

- [ ] **Step 4: Commit cleanup and status update**

Run:

```bash
git add backend/internal/plugins/registry.go docs/superpowers/specs/2026-05-24-backend-observability-and-code-health.md
git commit --no-gpg-sign -m "Document plugin registry cleanup completion"
```

---

### Task 6: Format, Verify, And Final Review

**Files:**
- Modify: any files changed by formatting.

- [ ] **Step 1: Format backend code**

Run:

```bash
task fmt:be
```

Expected: command exits 0.

- [ ] **Step 2: Run focused backend tests**

Run:

```bash
task test:be -- ./internal/plugins ./internal/daemon ./internal/api ./pkg/plugins/readers/gmail ./pkg/plugins/readers/thunderbird ./pkg/plugins/writers/postgres ./cmd/server
```

Expected: PASS.

- [ ] **Step 3: Run full backend tests**

Run:

```bash
task test:be
```

Expected: PASS.

- [ ] **Step 4: Run strict backend lint**

Run:

```bash
task lint:be:prod
```

Expected: PASS with `0 issues`.

- [ ] **Step 5: Run OpenAPI drift check**

Run:

```bash
task openapi:check
```

Expected: PASS. This should not change generated OpenAPI output because API behavior and annotations are unchanged.

- [ ] **Step 6: Inspect final diff**

Run:

```bash
git diff --stat main...HEAD
git diff --check
git status --short
```

Expected: no whitespace errors; only intended backend and spec files changed.

---

## Self-Review

- Spec coverage: This plan covers plugin registry cleanup from the backend observability/code-health spec, including catalog-only registry, required setup guide metadata, constructor input structs, and removal of optional metadata/config interfaces. It intentionally leaves API handler decomposition, store ownership cleanup, processed-message state context cleanup, and Gmail query domain cleanup for later plans.
- Placeholder scan: No `TBD`, `TODO`, or deferred implementation placeholders are present.
- Type consistency: `ReaderMetadata`, `WriterMetadata`, `AuthSpec`, `ReaderInput`, and `WriterInput` are introduced in Task 1 and used consistently by daemon, API, and concrete plugin tasks.
