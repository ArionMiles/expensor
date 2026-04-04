# Thunderbird Discovery & Onboarding Guide — Design Spec

**Date:** 2026-04-04

---

## Goals

1. Thunderbird onboarding config step shows discovered profiles and mailboxes in dropdowns instead of blank text inputs. Works on macOS, Linux, Windows, and Docker. Free-text fallback always available.
2. Each reader shows a contextual setup guide (structured sections, steps, links, notes) when selected in the wizard. Guide content lives in embedded JSON files per reader — not hardcoded in TSX.

---

## Part 1: Thunderbird Profile & Mailbox Discovery

### 1.1 New backend endpoints

**`GET /api/readers/thunderbird/discover/profiles`**

Returns a deduplicated list of profile paths found by:
1. Running the existing `FindProfiles()` (platform-aware: `~/Library/Thunderbird`, `~/.thunderbird`, `%APPDATA%/Thunderbird/Profiles`)
2. Checking `/thunderbird-profile` (the standard Docker mount point in `docker-compose.yml`)
3. Checking the value of `THUNDERBIRD_DATA_DIR` env var if set

Returns `200 {"profiles": [...]}` — empty array when nothing is found (not an error). Returns `500` only on unexpected filesystem errors.

**`GET /api/readers/thunderbird/discover/mailboxes?profile=<path>`**

Returns the list of available mailbox names in the given profile directory:
- Walks `<profile>/Mail/Local Folders/`, `<profile>/Mail/<account>/`, `<profile>/ImapMail/<account>/`
- Returns file entries that are MBOX files (no extension, not `.msf`)
- Returns `200 {"mailboxes": [...]}`, `400` if `profile` param missing, `404` if path doesn't exist

New helper: `ListMailboxes(profilePath string) ([]string, error)` in `backend/pkg/reader/thunderbird/profile.go`.

### 1.2 ConfigField extensions

**Go (`backend/internal/plugins/registry.go`)** — add `DependsOn string` to `ConfigField`:

```go
type ConfigField struct {
    Key       string `json:"key"`
    Label     string `json:"label"`
    Type      string `json:"type"` // "text", "password", "path", "thunderbird-profile", "thunderbird-mailboxes"
    Required  bool   `json:"required"`
    Help      string `json:"help,omitempty"`
    DependsOn string `json:"depends_on,omitempty"` // key of another field this field's options depend on
}
```

**TypeScript (`frontend/src/api/types.ts`)** — add the two missing fields (additive, no renames):

```ts
export interface ConfigField {
  name: string       // unchanged — matches JSON "key" from Go
  label: string
  type: string       // adds "thunderbird-profile" | "thunderbird-mailboxes"
  required: boolean
  help?: string      // NEW — was in Go but missing from TS interface
  depends_on?: string // NEW
  placeholder?: string
}
```

> Note: The existing frontend `ConfigField` uses `name` (from the API) but the Go struct field is `Key` (serialised as `"key"`). **This spec does not rename the field** — `name` stays as `name` in both Go JSON tags and TypeScript to avoid a cascade of breakage. The `DependsOn` and `Help` additions are purely additive. The `"key"` reference above is a naming slip — keep `"name"` throughout.

### 1.3 Thunderbird plugin ConfigSchema update

`backend/pkg/plugins/readers/thunderbird/plugin.go` — update `ConfigSchema()`:

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

### 1.4 Frontend ConfigureStep enhancements

`frontend/src/pages/setup/steps/ConfigureStep.tsx` — handle the new types:

- `type === "thunderbird-profile"` → render `ThunderbirdProfileField`: combobox that calls `GET /api/readers/thunderbird/discover/profiles` on mount; shows results in a dropdown; allows free-text entry; shows a spinner while loading
- `type === "thunderbird-mailboxes"` → render `ThunderbirdMailboxesField`: combobox that calls `GET /api/readers/thunderbird/discover/mailboxes?profile=<dependsOn field value>` when the dependent field has a value; multi-select chip style with free-text entry; stores as comma-separated string

Both components: if the discovery API returns an empty list, show a "No profiles/mailboxes found" hint and fall back to a plain text input.

New API hooks in `frontend/src/api/queries.ts`:
- `useThunderbirdProfiles()` — queries `/api/readers/thunderbird/discover/profiles`
- `useThunderbirdMailboxes(profilePath: string)` — queries `/api/readers/thunderbird/discover/mailboxes?profile=<profilePath>`, disabled when `profilePath` is empty

---

## Part 2: Onboarding Reader Guide

### 2.1 Guide JSON format

Each reader that wants a guide provides an embedded `guide.json` file. Parsed as:

```go
type ReaderGuide struct {
    Sections []GuideSection `json:"sections"`
    Notes    []GuideNote    `json:"notes,omitempty"`
}

type GuideSection struct {
    Title string      `json:"title"`
    Steps []string    `json:"steps"`
    Link  *GuideLink  `json:"link,omitempty"`
}

type GuideLink struct {
    Label string `json:"label"`
    URL   string `json:"url"`
}

type GuideNote struct {
    Type string `json:"type"` // "info" | "warning" | "tip" | "docker"
    Text string `json:"text"`
}
```

`GuideNote.Type` maps to visual treatment:
| Type | Color | Icon |
|------|-------|------|
| `info` | Blue | ℹ |
| `warning` | Amber | ⚠ |
| `tip` | Green | ✓ |
| `docker` | Purple | 🐳 |

### 2.2 Optional plugin interface

Rather than modifying `ReaderPlugin` (which would require updating all plugin stubs in tests), expose an optional interface:

```go
// GuideProvider is an optional interface for reader plugins that provide setup guides.
type GuideProvider interface {
    SetupGuide() []byte // returns embedded guide.json bytes, or nil if none
}
```

The handler type-asserts the plugin:
```go
gp, ok := plugin.(plugins.GuideProvider)
if !ok || gp.SetupGuide() == nil {
    writeError(w, http.StatusNotFound, "no setup guide available for this reader")
    return
}
```

### 2.3 Guide files

**`backend/pkg/plugins/readers/gmail/guide.json`** (embedded):
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

**`backend/pkg/plugins/readers/thunderbird/guide.json`** (embedded):
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

Each plugin embeds its guide file:
```go
//go:embed guide.json
var guideData []byte

func (p *Plugin) SetupGuide() []byte { return guideData }
```

### 2.4 New API endpoint

**`GET /api/readers/{name}/guide`** — registered in `server.go` alongside the existing reader routes. Returns `200` with the parsed `ReaderGuide` struct as JSON, or `404` if the reader has no guide.

### 2.5 Frontend guide rendering

**`useReaderGuide(name: string)`** — TanStack Query hook, `staleTime: Infinity` (guides don't change at runtime). Disabled when `name` is empty.

In **`SelectReader.tsx`** — when a reader is selected, a guide panel slides in below the reader list. The panel is collapsible ("Setup guide ▾ / ▴"). Structure:
- Numbered sections, each with a title and step list
- Optional `link` button per section
- Notes bar at the bottom: colour-coded left-border callouts per note type

---

## Files Created / Modified

| File | Change |
|------|--------|
| `backend/pkg/reader/thunderbird/profile.go` | Add `ListMailboxes(profilePath string) ([]string, error)` |
| `backend/pkg/reader/thunderbird/profile_test.go` | Tests for `ListMailboxes` |
| `backend/internal/plugins/registry.go` | Add `DependsOn string` to `ConfigField`; add `GuideProvider` optional interface |
| `backend/pkg/plugins/readers/thunderbird/plugin.go` | Update `ConfigSchema()` to use new field types; implement `SetupGuide()` |
| `backend/pkg/plugins/readers/thunderbird/guide.json` | New embedded guide |
| `backend/pkg/plugins/readers/gmail/plugin.go` | Implement `SetupGuide()` |
| `backend/pkg/plugins/readers/gmail/guide.json` | New embedded guide |
| `backend/internal/api/handlers.go` | Add `HandleGetReaderGuide`, `HandleDiscoverProfiles`, `HandleDiscoverMailboxes` |
| `backend/internal/api/handlers_test.go` | Tests for the three new handlers |
| `backend/internal/api/server.go` | Register three new routes |
| `frontend/src/api/types.ts` | Update `ConfigField` (rename `name→key`, add `help`, `depends_on`) |
| `frontend/src/api/client.ts` | Add discovery + guide API methods |
| `frontend/src/api/queries.ts` | Add `useThunderbirdProfiles`, `useThunderbirdMailboxes`, `useReaderGuide` |
| `frontend/src/pages/setup/steps/ConfigureStep.tsx` | Handle `thunderbird-profile` and `thunderbird-mailboxes` field types |
| `frontend/src/pages/setup/steps/SelectReader.tsx` | Add guide panel |
