# Thunderbird Discovery & Onboarding Guide Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show discovered Thunderbird profiles and mailboxes in dropdowns during onboarding, and display a collapsible setup guide for each reader when it is selected in the wizard.

**Architecture:** A `ListMailboxes` helper is added to the existing `profile.go` to enumerate available MBOX files. Two new discovery endpoints and one guide endpoint are added to the API. The Go `ConfigField` struct gains a `DependsOn` field and its JSON tag is fixed to `"name"` (aligning with the TypeScript type). Both reader plugins embed a `guide.json` file and expose it via an optional `GuideProvider` interface. The frontend `ConfigureStep` renders discovery-aware combobox components for the new field types, and `SelectReader` gains a collapsible guide panel.

**Tech Stack:** Go 1.23+, `//go:embed`, React 18, TypeScript, TanStack Query, Tailwind CSS.

---

## File Map

| File | Change |
|------|--------|
| `backend/pkg/reader/thunderbird/profile.go` | Add `ListMailboxes(profilePath string) ([]string, error)` |
| `backend/pkg/reader/thunderbird/profile_test.go` | Tests for `ListMailboxes` |
| `backend/internal/plugins/registry.go` | Fix `ConfigField.Key json:"name"`; add `DependsOn`; add `GuideProvider` interface + `ReaderGuide` types |
| `backend/pkg/plugins/readers/thunderbird/plugin.go` | Update `ConfigSchema()`; embed `guide.json`; implement `SetupGuide()` |
| `backend/pkg/plugins/readers/thunderbird/guide.json` | New |
| `backend/pkg/plugins/readers/gmail/plugin.go` | Embed `guide.json`; implement `SetupGuide()` |
| `backend/pkg/plugins/readers/gmail/guide.json` | New |
| `backend/internal/api/handlers.go` | Add `HandleDiscoverProfiles`, `HandleDiscoverMailboxes`, `HandleGetReaderGuide` |
| `backend/internal/api/handlers_test.go` | 4 new handler tests |
| `backend/internal/api/server.go` | Register 3 new routes |
| `frontend/src/api/types.ts` | Update `ConfigField` (add `help`, `depends_on`); add `ReaderGuide` types |
| `frontend/src/api/client.ts` | Add `api.readers.guide()`, `api.thunderbird.discoverProfiles()`, `api.thunderbird.discoverMailboxes()` |
| `frontend/src/api/queries.ts` | Add `useReaderGuide`, `useThunderbirdProfiles`, `useThunderbirdMailboxes` |
| `frontend/src/pages/setup/steps/ConfigureStep.tsx` | Handle `thunderbird-profile` and `thunderbird-mailboxes` types |
| `frontend/src/pages/setup/steps/SelectReader.tsx` | Add collapsible guide panel |

---

## Task 1: Backend — `ListMailboxes` + unit tests

**Files:**
- Modify: `backend/pkg/reader/thunderbird/profile.go`
- Modify: `backend/pkg/reader/thunderbird/profile_test.go`

### Background

`collectMailDirs` already walks `Mail/` and `ImapMail/` subdirectories. `ListMailboxes` reuses it to enumerate MBOX files — files with no extension (`.msf` index files are skipped, as are `.sbd` subdirectories). Results are deduplicated and sorted.

- [ ] **Step 1: Write the failing unit test**

Add to `backend/pkg/reader/thunderbird/profile_test.go`:

```go
func TestListMailboxes_ReturnsMailboxNames(t *testing.T) {
	dir := t.TempDir()

	// Create a realistic Mail directory structure
	localFolders := filepath.Join(dir, "Mail", "Local Folders")
	if err := os.MkdirAll(localFolders, 0o700); err != nil {
		t.Fatal(err)
	}
	// MBOX files (no extension)
	for _, name := range []string{"INBOX", "Sent", "Trash"} {
		if err := os.WriteFile(filepath.Join(localFolders, name), []byte("From "), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// .msf index files — should NOT appear in results
	if err := os.WriteFile(filepath.Join(localFolders, "INBOX.msf"), []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}

	mailboxes, err := ListMailboxes(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mailboxes) != 3 {
		t.Errorf("expected 3 mailboxes, got %d: %v", len(mailboxes), mailboxes)
	}
	// Results must be sorted
	expected := []string{"INBOX", "Sent", "Trash"}
	for i, want := range expected {
		if mailboxes[i] != want {
			t.Errorf("mailboxes[%d] = %q, want %q", i, mailboxes[i], want)
		}
	}
}

func TestListMailboxes_EmptyProfile_ReturnsEmptySlice(t *testing.T) {
	dir := t.TempDir()
	mailboxes, err := ListMailboxes(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mailboxes) != 0 {
		t.Errorf("expected 0 mailboxes, got %d", len(mailboxes))
	}
}

func TestListMailboxes_NonexistentPath_ReturnsError(t *testing.T) {
	_, err := ListMailboxes("/nonexistent/thunderbird/profile")
	if err == nil {
		t.Error("expected error for nonexistent path, got nil")
	}
}

func TestListMailboxes_EmptyPath_ReturnsError(t *testing.T) {
	_, err := ListMailboxes("")
	if err == nil {
		t.Error("expected error for empty path, got nil")
	}
}
```

- [ ] **Step 2: Confirm compile error**

```bash
cd /Users/ksingh/code/expensor/backend && go test ./pkg/reader/thunderbird/... -short -run "TestListMailboxes" 2>&1 | head -10
```
Expected: compile error — `ListMailboxes` undefined.

- [ ] **Step 3: Implement ListMailboxes in profile.go**

Add after `pathExists` in `backend/pkg/reader/thunderbird/profile.go`:

```go
// ListMailboxes returns the names of all available mailboxes in a Thunderbird profile.
// It walks Mail/Local Folders, Mail/<account>/, and ImapMail/<account>/ directories,
// returning file names that are MBOX files (no file extension — .msf index files are excluded).
// Results are deduplicated and sorted alphabetically.
func ListMailboxes(profilePath string) ([]string, error) {
	if profilePath == "" {
		return nil, fmt.Errorf("profile path is empty")
	}
	if _, err := os.Stat(profilePath); os.IsNotExist(err) { //nolint:gosec // profilePath comes from API query param, validated by handler
		return nil, fmt.Errorf("profile path does not exist: %s", profilePath)
	}

	dirs := collectMailDirs(profilePath)
	seen := make(map[string]struct{})
	var mailboxes []string

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir) //nolint:gosec // dir is derived from validated profilePath
		if err != nil {
			continue // directory may not exist — skip silently
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue // .sbd subdirectories are not MBOX files
			}
			name := entry.Name()
			if filepath.Ext(name) != "" {
				continue // skip .msf index files and any other extension
			}
			if _, exists := seen[name]; !exists {
				seen[name] = struct{}{}
				mailboxes = append(mailboxes, name)
			}
		}
	}

	sort.Strings(mailboxes)
	return mailboxes, nil
}
```

Add `"sort"` to the imports in `profile.go`.

- [ ] **Step 4: Run tests**

```bash
cd /Users/ksingh/code/expensor/backend && go test ./pkg/reader/thunderbird/... -v -run "TestListMailboxes" 2>&1
```
Expected: all 4 tests pass.

- [ ] **Step 5: Run prod linter**

```bash
cd /Users/ksingh/code/expensor && task lint:be:prod 2>&1 | tail -5
```
Expected: 0 issues.

- [ ] **Step 6: Commit**

```bash
cd /Users/ksingh/code/expensor && git add backend/pkg/reader/thunderbird/profile.go backend/pkg/reader/thunderbird/profile_test.go
git commit --no-gpg-sign -m "feat: add ListMailboxes to Thunderbird profile discovery

ListMailboxes enumerates all MBOX files in a profile's Mail/ and
ImapMail/ directories. .msf index files and subdirectories are excluded.
Results are deduplicated and sorted alphabetically."
```

---

## Task 2: Backend — registry types, guide infrastructure, handlers, routes

**Files:**
- Modify: `backend/internal/plugins/registry.go`
- Modify: `backend/pkg/plugins/readers/thunderbird/plugin.go`
- Create: `backend/pkg/plugins/readers/thunderbird/guide.json`
- Modify: `backend/pkg/plugins/readers/gmail/plugin.go`
- Create: `backend/pkg/plugins/readers/gmail/guide.json`
- Modify: `backend/internal/api/handlers.go`
- Modify: `backend/internal/api/handlers_test.go`
- Modify: `backend/internal/api/server.go`

### Background

**ConfigField JSON tag fix:** The Go struct has `Key string json:"key"` but the TypeScript type uses `name: string`. The handler tests and `testReaderPlugin` use the field directly as a struct, so changing the json tag from `"key"` to `"name"` is backwards-incompatible only for API consumers. Since the frontend already uses `field.name` (it would have silently received `undefined` before), this fix is correct and necessary.

**TDD order:** Write handler tests first (compile error), then add the types and handlers, then the routes.

- [ ] **Step 1: Fix ConfigField and add GuideProvider in registry.go**

Replace the `ConfigField` struct and add the new types at the end of `backend/internal/plugins/registry.go`:

```go
// ConfigField describes a single user-provided configuration field for a plugin.
type ConfigField struct {
	Key       string `json:"name"`           // serialised as "name" for frontend compatibility
	Label     string `json:"label"`
	Type      string `json:"type"`           // "text", "password", "path", "thunderbird-profile", "thunderbird-mailboxes"
	Required  bool   `json:"required"`
	Help      string `json:"help,omitempty"`
	DependsOn string `json:"depends_on,omitempty"` // key of another field whose value this field depends on
}

// GuideProvider is an optional interface for reader plugins that provide setup guides.
// Plugins that implement this will have their guide served at GET /api/readers/{name}/guide.
type GuideProvider interface {
	SetupGuide() []byte // returns embedded guide.json bytes; nil means no guide
}

// ReaderGuide is the structured setup guide for a reader plugin.
type ReaderGuide struct {
	Sections []GuideSection `json:"sections"`
	Notes    []GuideNote    `json:"notes,omitempty"`
}

// GuideSection is a titled group of steps in the setup guide.
type GuideSection struct {
	Title string     `json:"title"`
	Steps []string   `json:"steps"`
	Link  *GuideLink `json:"link,omitempty"`
}

// GuideLink is an optional external link attached to a guide section.
type GuideLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// GuideNote is a colour-coded callout displayed below the guide sections.
// Type: "info" (blue), "warning" (amber), "tip" (green), "docker" (purple).
type GuideNote struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
```

- [ ] **Step 2: Create guide.json files**

Create `backend/pkg/plugins/readers/gmail/guide.json`:

```json
{
  "sections": [
    {
      "title": "Enable the Gmail API",
      "steps": [
        "Go to console.cloud.google.com and open or create a project.",
        "Search for 'Gmail API' and click Enable."
      ],
      "link": { "label": "Google Cloud Console", "url": "https://console.cloud.google.com" }
    },
    {
      "title": "Create OAuth 2.0 credentials",
      "steps": [
        "Navigate to APIs & Services → Credentials.",
        "Click Create Credentials → OAuth client ID.",
        "Set Application type to Desktop app.",
        "Under Authorized redirect URIs add: http://localhost:8080/api/auth/callback",
        "Click Create, then Download JSON — this is your client_secret.json."
      ]
    },
    {
      "title": "Configure the OAuth consent screen",
      "steps": [
        "Set User Type to External.",
        "Add your email address as a test user.",
        "Add scope: https://www.googleapis.com/auth/gmail.readonly"
      ]
    }
  ],
  "notes": [
    {
      "type": "warning",
      "text": "The redirect URI must be exactly: http://localhost:8080/api/auth/callback (or your deployed base URL + /api/auth/callback)."
    },
    {
      "type": "info",
      "text": "Only gmail.readonly is required. Expensor never modifies your inbox."
    }
  ]
}
```

Create `backend/pkg/plugins/readers/thunderbird/guide.json`:

```json
{
  "sections": [
    {
      "title": "Find your Thunderbird profile",
      "steps": [
        "In Thunderbird, open Help → Troubleshooting Information.",
        "Click Open Folder next to Profile Directory — this is the path to enter below.",
        "On macOS the path is usually ~/Library/Thunderbird/Profiles/<name>.",
        "On Linux: ~/.thunderbird/<name>. On Windows: %APPDATA%/Thunderbird/Profiles/<name>."
      ]
    },
    {
      "title": "Select mailboxes to scan",
      "steps": [
        "Pick the mailboxes (folders) Expensor should scan for transaction emails.",
        "INBOX is the most common choice; add others like Promotions or bank-specific folders if needed."
      ]
    }
  ],
  "notes": [
    {
      "type": "docker",
      "text": "Running Expensor in Docker? Mount your profile directory as a read-only volume in docker-compose.yml: - /path/to/your/profile:/thunderbird-profile:ro. The profile selector will then show it automatically."
    },
    {
      "type": "info",
      "text": "If no profiles appear in the dropdown, the profile path may not be accessible. Enter it manually or check the Docker volume mount."
    }
  ]
}
```

- [ ] **Step 3: Update Gmail plugin to embed and serve guide**

In `backend/pkg/plugins/readers/gmail/plugin.go`, add after the package declaration and imports:

```go
import _ "embed"

//go:embed guide.json
var guideData []byte

// SetupGuide returns the embedded setup guide for Gmail.
func (p *Plugin) SetupGuide() []byte { return guideData }
```

- [ ] **Step 4: Update Thunderbird plugin — guide + ConfigSchema**

In `backend/pkg/plugins/readers/thunderbird/plugin.go`, add:

```go
import _ "embed"

//go:embed guide.json
var guideData []byte

// SetupGuide returns the embedded setup guide for Thunderbird.
func (p *Plugin) SetupGuide() []byte { return guideData }
```

Update `ConfigSchema()`:

```go
func (p *Plugin) ConfigSchema() []plugins.ConfigField {
	return []plugins.ConfigField{
		{
			Key:      "profilePath",
			Label:    "Thunderbird Profile Directory",
			Type:     "thunderbird-profile",
			Required: true,
			Help:     "Path to your Thunderbird profile directory (contains Mail/ and ImapMail/).",
		},
		{
			Key:       "mailboxes",
			Label:     "Mailboxes to scan",
			Type:      "thunderbird-mailboxes",
			Required:  true,
			DependsOn: "profilePath",
			Help:      "Select mailboxes to scan. Comma-separated if entering manually (e.g. INBOX,Sent).",
		},
	}
}
```

- [ ] **Step 5: Write the four failing handler tests**

Add to `backend/internal/api/handlers_test.go`:

```go
// --- thunderbird discovery + guide ---

func TestHandleDiscoverProfiles_Returns200WithProfilesKey(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/readers/thunderbird/discover/profiles", nil)
	rr := httptest.NewRecorder()
	h.HandleDiscoverProfiles(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string][]string
	decodeJSON(t, rr.Body.String(), &resp)
	if _, ok := resp["profiles"]; !ok {
		t.Error("expected 'profiles' key in response")
	}
}

func TestHandleDiscoverMailboxes_MissingParam_Returns400(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/readers/thunderbird/discover/mailboxes", nil)
	rr := httptest.NewRecorder()
	h.HandleDiscoverMailboxes(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleDiscoverMailboxes_NonexistentProfile_Returns404(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/readers/thunderbird/discover/mailboxes?profile=/nonexistent/thunderbird/profile", nil)
	rr := httptest.NewRecorder()
	h.HandleDiscoverMailboxes(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestHandleGetReaderGuide_NoGuide_Returns404(t *testing.T) {
	// testReaderPlugin does not implement GuideProvider, so the handler returns 404.
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/readers/gmail/guide", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.HandleGetReaderGuide(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}
```

- [ ] **Step 6: Confirm compile error**

```bash
cd /Users/ksingh/code/expensor/backend && go test ./internal/api/... -short -run "TestHandleDiscover|TestHandleGetReaderGuide" 2>&1 | head -10
```
Expected: compile error — handlers undefined.

- [ ] **Step 7: Add the three handlers to handlers.go**

First ensure the import for the thunderbird reader package is present. In the import block of `backend/internal/api/handlers.go`, add:

```go
tbreader "github.com/ArionMiles/expensor/backend/pkg/reader/thunderbird"
```

Add these three handlers after `HandleGetChartData` (keeping the rules/stats section together):

```go
// HandleDiscoverProfiles handles GET /api/readers/thunderbird/discover/profiles.
// Returns discovered Thunderbird profile directories from platform-specific paths,
// the Docker mount point /thunderbird-profile, and THUNDERBIRD_DATA_DIR env var.
func (h *Handlers) HandleDiscoverProfiles(w http.ResponseWriter, r *http.Request) {
	var paths []string
	seen := make(map[string]struct{})

	addIfExists := func(p string) {
		if p == "" {
			return
		}
		if _, err := os.Stat(p); err == nil { //nolint:gosec // p is from OS path or trusted env var
			if _, exists := seen[p]; !exists {
				seen[p] = struct{}{}
				paths = append(paths, p)
			}
		}
	}

	if discovered, err := tbreader.FindProfiles(); err == nil {
		for _, p := range discovered {
			addIfExists(p)
		}
	}
	addIfExists("/thunderbird-profile")
	addIfExists(os.Getenv("THUNDERBIRD_DATA_DIR"))

	if paths == nil {
		paths = []string{}
	}
	writeJSON(w, http.StatusOK, map[string][]string{"profiles": paths})
}

// HandleDiscoverMailboxes handles GET /api/readers/thunderbird/discover/mailboxes?profile=<path>.
// Returns available MBOX mailbox names within the given Thunderbird profile directory.
func (h *Handlers) HandleDiscoverMailboxes(w http.ResponseWriter, r *http.Request) {
	profile := r.URL.Query().Get("profile")
	if profile == "" {
		writeError(w, http.StatusBadRequest, "profile query parameter is required")
		return
	}
	if _, err := os.Stat(profile); os.IsNotExist(err) { //nolint:gosec // profile from query param, existence checked
		writeError(w, http.StatusNotFound, "profile directory not found")
		return
	}
	mailboxes, err := tbreader.ListMailboxes(profile)
	if err != nil {
		h.logger.Error("discovering mailboxes", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to discover mailboxes")
		return
	}
	if mailboxes == nil {
		mailboxes = []string{}
	}
	writeJSON(w, http.StatusOK, map[string][]string{"mailboxes": mailboxes})
}

// HandleGetReaderGuide handles GET /api/readers/{name}/guide.
// Returns the structured setup guide for a reader if it implements plugins.GuideProvider.
func (h *Handlers) HandleGetReaderGuide(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	plugin, err := h.registry.GetReader(name)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("reader %q not found", name))
		return
	}
	gp, ok := plugin.(plugins.GuideProvider)
	if !ok || gp.SetupGuide() == nil {
		writeError(w, http.StatusNotFound, "no setup guide available for this reader")
		return
	}
	var guide plugins.ReaderGuide
	if err := json.Unmarshal(gp.SetupGuide(), &guide); err != nil {
		h.logger.Error("parsing reader guide", "reader", name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to parse reader guide")
		return
	}
	writeJSON(w, http.StatusOK, guide)
}
```

- [ ] **Step 8: Register the three routes in server.go**

In `backend/internal/api/server.go`, add after the existing reader routes:

```go
	// Thunderbird profile/mailbox discovery
	mux.HandleFunc("GET /api/readers/thunderbird/discover/profiles", h.HandleDiscoverProfiles)
	mux.HandleFunc("GET /api/readers/thunderbird/discover/mailboxes", h.HandleDiscoverMailboxes)

	// Reader setup guide
	mux.HandleFunc("GET /api/readers/{name}/guide", h.HandleGetReaderGuide)
```

Note: register `/api/readers/thunderbird/discover/profiles` and `/api/readers/thunderbird/discover/mailboxes` **before** `GET /api/readers/{name}/guide` and any other `{name}` wildcard routes to prevent the wildcard from capturing `thunderbird`.

- [ ] **Step 9: Run the four new handler tests**

```bash
cd /Users/ksingh/code/expensor/backend && go test ./internal/api/... -v -run "TestHandleDiscover|TestHandleGetReaderGuide" 2>&1
```
Expected: all 4 pass.

- [ ] **Step 10: Run full backend tests + prod linter**

```bash
cd /Users/ksingh/code/expensor/backend && go test -short ./... 2>&1 | tail -5
cd /Users/ksingh/code/expensor && task lint:be:prod 2>&1 | tail -5
```
Expected: all pass, 0 issues.

- [ ] **Step 11: Commit**

```bash
cd /Users/ksingh/code/expensor && git add \
  backend/internal/plugins/registry.go \
  backend/pkg/plugins/readers/thunderbird/plugin.go \
  backend/pkg/plugins/readers/thunderbird/guide.json \
  backend/pkg/plugins/readers/gmail/plugin.go \
  backend/pkg/plugins/readers/gmail/guide.json \
  backend/internal/api/handlers.go \
  backend/internal/api/handlers_test.go \
  backend/internal/api/server.go
git commit --no-gpg-sign -m "feat: Thunderbird discovery endpoints, reader guide API, and guide files

Adds HandleDiscoverProfiles, HandleDiscoverMailboxes, HandleGetReaderGuide.
Discovery checks platform paths, /thunderbird-profile (Docker mount),
and THUNDERBIRD_DATA_DIR env var. GuideProvider optional interface lets
plugins embed guide.json; both gmail and thunderbird plugins now implement
it. ConfigField gains DependsOn; json tag fixed to name for TS alignment.
Thunderbird ConfigSchema updated to use thunderbird-profile and
thunderbird-mailboxes field types."
```

---

## Task 3: Frontend — API types, client methods, query hooks

**Files:**
- Modify: `frontend/src/api/types.ts`
- Modify: `frontend/src/api/client.ts`
- Modify: `frontend/src/api/queries.ts`

- [ ] **Step 1: Update ConfigField and add ReaderGuide types in types.ts**

In `frontend/src/api/types.ts`, replace `ConfigField` and add the guide types:

```ts
export interface ConfigField {
  name: string
  type: string
  label: string
  required: boolean
  help?: string        // NEW
  depends_on?: string  // NEW
  placeholder?: string
}

export interface GuideLink {
  label: string
  url: string
}

export interface GuideSection {
  title: string
  steps: string[]
  link?: GuideLink
}

export interface GuideNote {
  type: 'info' | 'warning' | 'tip' | 'docker'
  text: string
}

export interface ReaderGuide {
  sections: GuideSection[]
  notes?: GuideNote[]
}
```

- [ ] **Step 2: Add client methods in client.ts**

In `frontend/src/api/client.ts`, import `ReaderGuide`:

```ts
import type {
  ...
  ReaderGuide,
  ...
} from './types'
```

Add `guide` to the existing `api.readers` object:

```ts
  readers: {
    ...existing methods...
    guide: (name: string) => apiClient.get<ReaderGuide>(`/readers/${name}/guide`),
  },
```

Add a new `api.thunderbird` object after the `readers` block:

```ts
  thunderbird: {
    discoverProfiles: () =>
      apiClient.get<{ profiles: string[] }>('/readers/thunderbird/discover/profiles'),
    discoverMailboxes: (profile: string) =>
      apiClient.get<{ mailboxes: string[] }>(
        `/readers/thunderbird/discover/mailboxes?profile=${encodeURIComponent(profile)}`,
      ),
  },
```

- [ ] **Step 3: Add hooks in queries.ts**

Add at the end of `frontend/src/api/queries.ts`:

```ts
export function useReaderGuide(name: string) {
  return useQuery({
    queryKey: ['readers', name, 'guide'] as const,
    queryFn: () => api.readers.guide(name).then((r) => r.data),
    staleTime: Infinity,
    enabled: name.length > 0,
  })
}

export function useThunderbirdProfiles() {
  return useQuery({
    queryKey: ['thunderbird', 'profiles'] as const,
    queryFn: () => api.thunderbird.discoverProfiles().then((r) => r.data.profiles),
    staleTime: 60_000,
  })
}

export function useThunderbirdMailboxes(profilePath: string) {
  return useQuery({
    queryKey: ['thunderbird', 'mailboxes', profilePath] as const,
    queryFn: () =>
      api.thunderbird.discoverMailboxes(profilePath).then((r) => r.data.mailboxes),
    staleTime: 60_000,
    enabled: profilePath.length > 0,
  })
}
```

- [ ] **Step 4: TypeScript check**

```bash
cd /Users/ksingh/code/expensor && task lint:fe 2>&1 | tail -5
```
Expected: 0 errors.

- [ ] **Step 5: Commit**

```bash
cd /Users/ksingh/code/expensor && git add frontend/src/api/types.ts frontend/src/api/client.ts frontend/src/api/queries.ts
git commit --no-gpg-sign -m "feat(frontend): add ReaderGuide types, discovery client methods, and hooks"
```

---

## Task 4: Frontend — ConfigureStep with discovery-aware field components

**Files:**
- Modify: `frontend/src/pages/setup/steps/ConfigureStep.tsx`

### Background

`ConfigureStep` receives `configSchema: ConfigField[]`. For `type === "thunderbird-profile"`, render `ThunderbirdProfileField` — a combobox populated by `useThunderbirdProfiles()`. For `type === "thunderbird-mailboxes"`, render `ThunderbirdMailboxesField` — a multi-value chip input populated by `useThunderbirdMailboxes(profilePath)` where `profilePath` is read from the sibling field identified by `field.depends_on`.

Both components follow the project's custom combobox pattern (no `<datalist>`, no `<select>`). If discovery returns an empty list, show a hint and fall back to a plain text input.

The `ThunderbirdMailboxesField` stores its value as a comma-separated string (matching what the Thunderbird reader config expects). Chips are parsed by splitting on commas.

- [ ] **Step 1: Read ConfigureStep.tsx**

Read the full file to understand the current field rendering before making changes:

```bash
cat /Users/ksingh/code/expensor/frontend/src/pages/setup/steps/ConfigureStep.tsx
```

- [ ] **Step 2: Replace ConfigureStep.tsx**

Write the complete new version of `frontend/src/pages/setup/steps/ConfigureStep.tsx`:

```tsx
import { useRef, useState } from 'react'
import {
  useThunderbirdMailboxes,
  useThunderbirdProfiles,
} from '@/api/queries'
import type { ConfigField } from '@/api/types'
import { useSaveReaderConfig } from '@/api/queries'
import { cn } from '@/lib/utils'

// ─── Thunderbird profile combobox ─────────────────────────────────────────────

function ThunderbirdProfileField({
  value,
  onChange,
  disabled,
}: {
  value: string
  onChange: (v: string) => void
  disabled?: boolean
}) {
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)
  const { data: profiles = [], isLoading } = useThunderbirdProfiles()

  const filtered = profiles.filter((p) => p.toLowerCase().includes(value.toLowerCase()))

  return (
    <div ref={containerRef} className="relative">
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onFocus={() => setOpen(true)}
        onBlur={() => setTimeout(() => setOpen(false), 150)}
        disabled={disabled}
        placeholder={
          isLoading ? 'Scanning for profiles…' : 'e.g. /home/user/.thunderbird/abc.default'
        }
        className="w-full rounded-md border border-border bg-secondary px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50"
      />
      {open && filtered.length > 0 && (
        <ul className="absolute left-0 top-full z-50 mt-0.5 max-h-48 w-full overflow-y-auto rounded-md border border-border bg-card shadow-lg">
          {filtered.map((p) => (
            <li
              key={p}
              onMouseDown={() => { onChange(p); setOpen(false) }}
              className="cursor-pointer truncate px-3 py-1.5 text-xs text-foreground hover:bg-accent"
            >
              {p}
            </li>
          ))}
        </ul>
      )}
      {!isLoading && profiles.length === 0 && (
        <p className="mt-0.5 text-xs text-muted-foreground">
          No profiles found automatically — enter the path manually.
        </p>
      )}
    </div>
  )
}

// ─── Thunderbird mailboxes multi-select ───────────────────────────────────────

function ThunderbirdMailboxesField({
  value,
  onChange,
  profilePath,
  disabled,
}: {
  value: string
  onChange: (v: string) => void
  profilePath: string
  disabled?: boolean
}) {
  const [input, setInput] = useState('')
  const [open, setOpen] = useState(false)
  const { data: available = [], isLoading } = useThunderbirdMailboxes(profilePath)

  const selected = value
    ? value.split(',').map((s) => s.trim()).filter(Boolean)
    : []

  const addMailbox = (name: string) => {
    const trimmed = name.trim()
    if (!trimmed || selected.includes(trimmed)) return
    onChange([...selected, trimmed].join(','))
    setInput('')
    setOpen(false)
  }

  const removeMailbox = (name: string) => {
    onChange(selected.filter((s) => s !== name).join(','))
  }

  const filtered = available.filter(
    (m) => !selected.includes(m) && m.toLowerCase().includes(input.toLowerCase()),
  )

  return (
    <div className="space-y-1.5">
      {/* Selected chips */}
      {selected.length > 0 && (
        <div className="flex flex-wrap gap-1">
          {selected.map((s) => (
            <span
              key={s}
              className="inline-flex items-center gap-1 rounded-sm border border-border bg-secondary px-1.5 py-0.5 text-xs text-foreground"
            >
              {s}
              <button
                type="button"
                onClick={() => removeMailbox(s)}
                className="text-muted-foreground hover:text-foreground"
                aria-label={`Remove ${s}`}
              >
                ✕
              </button>
            </span>
          ))}
        </div>
      )}

      {/* Input + dropdown */}
      <div className="relative">
        <input
          value={input}
          onChange={(e) => { setInput(e.target.value); setOpen(true) }}
          onFocus={() => setOpen(true)}
          onBlur={() => setTimeout(() => setOpen(false), 150)}
          onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); addMailbox(input) } }}
          disabled={disabled || !profilePath}
          placeholder={
            !profilePath
              ? 'Select a profile first'
              : isLoading
              ? 'Loading mailboxes…'
              : 'Add mailbox (e.g. INBOX)'
          }
          className="w-full rounded-md border border-border bg-secondary px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50"
        />
        {open && filtered.length > 0 && (
          <ul className="absolute left-0 top-full z-50 mt-0.5 max-h-40 w-full overflow-y-auto rounded-md border border-border bg-card shadow-lg">
            {filtered.map((m) => (
              <li
                key={m}
                onMouseDown={() => addMailbox(m)}
                className="cursor-pointer px-3 py-1.5 text-xs text-foreground hover:bg-accent"
              >
                {m}
              </li>
            ))}
          </ul>
        )}
      </div>

      {profilePath && !isLoading && available.length === 0 && (
        <p className="text-xs text-muted-foreground">
          No mailboxes found — type names manually and press Enter.
        </p>
      )}
    </div>
  )
}

// ─── ConfigureStep ────────────────────────────────────────────────────────────

interface ConfigureStepProps {
  readerName: string
  configSchema: ConfigField[]
  onNext: () => void
  onBack: () => void
}

export function ConfigureStep({ readerName, configSchema, onNext, onBack }: ConfigureStepProps) {
  const [values, setValues] = useState<Record<string, string>>(() => {
    const init: Record<string, string> = {}
    configSchema.forEach((field) => {
      init[field.name] = ''
    })
    return init
  })
  const [validationError, setValidationError] = useState<string | null>(null)
  const { mutate: saveConfig, isPending, error } = useSaveReaderConfig()

  const handleChange = (name: string, value: string) => {
    setValues((prev) => ({ ...prev, [name]: value }))
    setValidationError(null)
  }

  const handleSubmit = () => {
    for (const field of configSchema) {
      if (field.required && !values[field.name]?.trim()) {
        setValidationError(`"${field.label}" is required`)
        return
      }
    }
    saveConfig({ readerName, config: values }, { onSuccess: () => onNext() })
  }

  const inputClass = cn(
    'w-full px-3 py-2 text-sm rounded-md',
    'bg-secondary border border-border text-foreground placeholder:text-muted-foreground',
    'focus:outline-none focus:ring-1 focus:ring-ring focus:border-primary',
  )

  if (configSchema.length === 0) {
    return (
      <div className="space-y-6">
        <div>
          <h2 className="mb-1 text-base font-semibold text-foreground">Configure reader</h2>
          <p className="text-sm text-muted-foreground">No configuration required for this reader.</p>
        </div>
        <div className="flex items-center justify-between">
          <button onClick={onBack} className="px-4 py-2 text-sm text-muted-foreground transition-colors hover:text-foreground">← Back</button>
          <button onClick={onNext} className="rounded-md bg-primary px-4 py-2 text-sm text-primary-foreground transition-colors hover:bg-primary/90">Next →</button>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="mb-1 text-base font-semibold text-foreground">Configure reader</h2>
        <p className="text-sm text-muted-foreground">
          Set the required options for the{' '}
          <span className="font-mono text-primary">{readerName}</span> reader.
        </p>
      </div>

      <div className="space-y-4">
        {configSchema.map((field) => (
          <div key={field.name} className="space-y-1.5">
            <label
              htmlFor={`config-${field.name}`}
              className="block text-xs font-medium uppercase tracking-wider text-muted-foreground"
            >
              {field.label}
              {field.required && <span className="ml-1 text-destructive">*</span>}
            </label>

            {field.type === 'thunderbird-profile' ? (
              <ThunderbirdProfileField
                value={values[field.name] ?? ''}
                onChange={(v) => handleChange(field.name, v)}
              />
            ) : field.type === 'thunderbird-mailboxes' ? (
              <ThunderbirdMailboxesField
                value={values[field.name] ?? ''}
                onChange={(v) => handleChange(field.name, v)}
                profilePath={field.depends_on ? (values[field.depends_on] ?? '') : ''}
              />
            ) : field.type === 'textarea' ? (
              <textarea
                id={`config-${field.name}`}
                value={values[field.name] ?? ''}
                onChange={(e) => handleChange(field.name, e.target.value)}
                placeholder={field.placeholder}
                rows={4}
                className={cn(inputClass, 'resize-y')}
              />
            ) : (
              <input
                id={`config-${field.name}`}
                type={field.type === 'password' ? 'password' : 'text'}
                value={values[field.name] ?? ''}
                onChange={(e) => handleChange(field.name, e.target.value)}
                placeholder={field.placeholder}
                className={inputClass}
              />
            )}

            {field.help && (
              <p className="text-xs text-muted-foreground">{field.help}</p>
            )}
          </div>
        ))}
      </div>

      {(validationError || error) && (
        <p className="text-xs text-destructive" role="alert">
          {validationError ?? (error instanceof Error ? error.message : 'Save failed')}
        </p>
      )}

      <div className="flex items-center justify-between">
        <button onClick={onBack} className="px-4 py-2 text-sm text-muted-foreground transition-colors hover:text-foreground">← Back</button>
        <button
          onClick={handleSubmit}
          disabled={isPending}
          className={cn(
            'rounded-md px-4 py-2 text-sm transition-colors',
            isPending
              ? 'cursor-not-allowed bg-secondary text-muted-foreground opacity-50'
              : 'bg-primary text-primary-foreground hover:bg-primary/90',
          )}
        >
          {isPending ? 'Saving...' : 'Save & continue →'}
        </button>
      </div>
    </div>
  )
}
```

- [ ] **Step 3: TypeScript check + Prettier**

```bash
cd /Users/ksingh/code/expensor && task lint:fe 2>&1 | tail -5
task fmt:fe:check 2>&1 | tail -5
```
If Prettier violations: `task fmt:fe` then re-check.

- [ ] **Step 4: Commit**

```bash
cd /Users/ksingh/code/expensor && git add frontend/src/pages/setup/steps/ConfigureStep.tsx
git commit --no-gpg-sign -m "feat(frontend): discovery-aware comboboxes for Thunderbird profile and mailboxes

ConfigureStep handles thunderbird-profile (combobox from /discover/profiles)
and thunderbird-mailboxes (multi-value chip input from /discover/mailboxes).
Both fall back to free-text entry when discovery returns no results.
Help text from ConfigField.help is rendered below each field."
```

---

## Task 5: Frontend — SelectReader guide panel + full CI

**Files:**
- Modify: `frontend/src/pages/setup/steps/SelectReader.tsx`

### Background

When a reader is selected, a collapsible "Setup guide" panel appears below the reader list. It is open by default (experienced users can collapse it). The guide panel renders numbered sections with steps, optional section-level links, and colour-coded note callouts. `useReaderGuide` returns `null`/`undefined` when no guide exists — the panel renders nothing in that case.

Note type → colour mapping:
- `info` → `border-blue-500/40 bg-blue-500/5 text-blue-400`
- `warning` → `border-warning/40 bg-warning/5 text-warning`
- `tip` → `border-green-500/40 bg-green-500/5 text-green-500`
- `docker` → `border-purple-500/40 bg-purple-500/5 text-purple-400`

- [ ] **Step 1: Add guide imports and panel to SelectReader.tsx**

Read the full current file first:
```bash
cat /Users/ksingh/code/expensor/frontend/src/pages/setup/steps/SelectReader.tsx
```

Then add the guide panel. At the top, add to imports:
```tsx
import { useState } from 'react'
import { useReaderGuide } from '@/api/queries'
import type { ReaderGuide } from '@/api/types'
```

Add the `ReaderGuidePanel` component above `SelectReader`:

```tsx
function noteStyle(type: string) {
  switch (type) {
    case 'warning': return 'border-l-2 border-warning/60 bg-warning/5 px-3 py-2 text-xs text-warning'
    case 'tip': return 'border-l-2 border-green-500/60 bg-green-500/5 px-3 py-2 text-xs text-green-500'
    case 'docker': return 'border-l-2 border-purple-500/60 bg-purple-500/5 px-3 py-2 text-xs text-purple-400'
    default: return 'border-l-2 border-blue-500/60 bg-blue-500/5 px-3 py-2 text-xs text-blue-400'
  }
}

function noteIcon(type: string) {
  switch (type) {
    case 'warning': return '⚠'
    case 'tip': return '✓'
    case 'docker': return '🐳'
    default: return 'ℹ'
  }
}

function ReaderGuidePanel({ guide }: { guide: ReaderGuide }) {
  const [open, setOpen] = useState(true)

  return (
    <div className="mt-4 rounded-lg border border-border bg-card">
      <button
        onClick={() => setOpen((o) => !o)}
        className="flex w-full items-center justify-between px-4 py-2.5 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground hover:text-foreground"
      >
        <span>Setup guide</span>
        <span>{open ? '▴' : '▾'}</span>
      </button>

      {open && (
        <div className="space-y-4 border-t border-border px-4 pb-4 pt-3">
          {guide.sections.map((section, i) => (
            <div key={i} className="space-y-1.5">
              <p className="text-xs font-semibold text-foreground">{section.title}</p>
              <ol className="space-y-1 pl-4">
                {section.steps.map((step, j) => (
                  <li key={j} className="list-decimal text-xs text-muted-foreground">
                    {step}
                  </li>
                ))}
              </ol>
              {section.link && (
                <a
                  href={section.link.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-block text-xs text-primary hover:underline"
                >
                  {section.link.label} ↗
                </a>
              )}
            </div>
          ))}

          {guide.notes && guide.notes.length > 0 && (
            <div className="space-y-2 pt-1">
              {guide.notes.map((note, i) => (
                <div key={i} className={noteStyle(note.type)}>
                  <span className="mr-1.5">{noteIcon(note.type)}</span>
                  {note.text}
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
```

In `SelectReader`, add `useReaderGuide` call and render the panel below the reader list. Find the return block and after the readers grid add:

```tsx
{selected && <ReaderGuideWrapper name={selected.name} />}
```

Add `ReaderGuideWrapper` above `SelectReader`:
```tsx
function ReaderGuideWrapper({ name }: { name: string }) {
  const { data: guide } = useReaderGuide(name)
  if (!guide) return null
  return <ReaderGuidePanel guide={guide} />
}
```

- [ ] **Step 2: TypeScript check + Prettier**

```bash
cd /Users/ksingh/code/expensor && task lint:fe 2>&1 | tail -5
task fmt:fe:check 2>&1 | tail -5
```

- [ ] **Step 3: Run full CI gate**

```bash
cd /Users/ksingh/code/expensor && task ci 2>&1 | tail -10
```
Expected: Go lint 0 issues, all tests pass, TypeScript 0 errors, npm audit clean.

- [ ] **Step 4: Commit**

```bash
cd /Users/ksingh/code/expensor && git add frontend/src/pages/setup/steps/SelectReader.tsx
git commit --no-gpg-sign -m "feat(frontend): add collapsible setup guide panel to SelectReader

When a reader is selected, a guide panel slides in below the reader
list. It renders numbered sections with steps and optional links,
followed by colour-coded callout notes (info/warning/tip/docker).
Guide content is fetched from GET /api/readers/{name}/guide.
The panel is open by default and collapsible."
```
