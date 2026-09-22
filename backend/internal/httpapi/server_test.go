package httpapi

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSpaHandler_ServesExistingFile(t *testing.T) {
	files := fstest.MapFS{
		"index.html":    {Data: []byte("<html>app</html>")},
		"assets/app.js": {Data: []byte("// js")},
	}

	h := spaHandler(files)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/assets/app.js", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for existing file, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestSpaHandler_FallsBackToIndexHTML(t *testing.T) {
	files := fstest.MapFS{
		"index.html": {Data: []byte("<html>spa</html>")},
	}

	h := spaHandler(files)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/some/spa/route", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for SPA fallback, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "<html>spa</html>") {
		t.Errorf("expected index.html content in SPA fallback, got: %s", rr.Body.String())
	}
}

func TestSpaHandler_ReturnsNotFoundForMissingAsset(t *testing.T) {
	files := fstest.MapFS{
		"index.html": {Data: []byte("<html>spa</html>")},
	}
	h := spaHandler(files)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/assets/missing.js", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing asset, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestUIFileSystem_DiskDirectoryOverridesEmbeddedAssets(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("disk"), 0o600); err != nil {
		t.Fatal(err)
	}
	embedded := fstest.MapFS{
		"index.html": {Data: []byte("embedded")},
	}

	files := uiFileSystem(dir, embedded)
	contents, err := fs.ReadFile(files, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "disk" {
		t.Errorf("expected disk override, got %q", contents)
	}
}
