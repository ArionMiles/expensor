package api

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type routeOperation struct {
	Method string
	Path   string
}

func (op routeOperation) key() string {
	return op.Method + " " + op.Path
}

func TestRegisteredRoutesAreDocumentedInOpenAPI(t *testing.T) {
	registered := registeredRouteOperations(t)
	documented := openAPIOperations(t)

	var missing []string
	for _, op := range registered {
		if !documented[op.key()] {
			missing = append(missing, op.key())
		}
	}
	if len(missing) > 0 {
		t.Fatalf("registered routes missing from OpenAPI:\n%s", strings.Join(missing, "\n"))
	}
}

func TestRegisteredRoutesHaveContractDecision(t *testing.T) {
	registered := registeredRouteOperations(t)
	allowlisted := routeSetFromTSV(t, "../../../tests/contract/allowlist.tsv")
	excluded := routeSetFromTSV(t, "../../../tests/contract/exclusions.tsv")

	var missing []string
	for _, op := range registered {
		key := op.key()
		if !allowlisted[key] && !excluded[key] {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("registered routes missing contract allowlist/exclusion decision:\n%s", strings.Join(missing, "\n"))
	}

	if extras := unregisteredDecisionRoutes(registered, allowlisted); len(extras) > 0 {
		t.Fatalf("contract allowlist entries are not registered routes:\n%s", strings.Join(extras, "\n"))
	}
	if extras := unregisteredDecisionRoutes(registered, excluded); len(extras) > 0 {
		t.Fatalf("contract exclusion entries are not registered routes:\n%s", strings.Join(extras, "\n"))
	}

	for key := range allowlisted {
		if excluded[key] {
			t.Fatalf("route %s is both allowlisted and excluded from contract coverage", key)
		}
	}
}

func TestContractDecisionRoutesMustBeRegistered(t *testing.T) {
	registered := []routeOperation{
		{Method: "GET", Path: "/health"},
		{Method: "POST", Path: "/daemon/start"},
	}
	decisionRoutes := map[string]bool{
		"GET /health":  true,
		"GET /made-up": true,
	}

	extras := unregisteredDecisionRoutes(registered, decisionRoutes)

	if len(extras) != 1 || extras[0] != "GET /made-up" {
		t.Fatalf("unregisteredDecisionRoutes() = %v, want [GET /made-up]", extras)
	}
}

func TestParseRouteSetFromTSVRejectsMalformedRows(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "too few fields",
			content: "GET\t/health\n",
			wantErr: "line 1: must have exactly 3 tab-separated fields",
		},
		{
			name:    "too many fields",
			content: "GET\t/health\treason\textra\n",
			wantErr: "line 1: must have exactly 3 tab-separated fields",
		},
		{
			name:    "blank line",
			content: "GET\t/health\treason\n\n",
			wantErr: "line 2: blank lines are not supported",
		},
		{
			name:    "comment line",
			content: "GET\t/health\treason\n# comment\n",
			wantErr: "line 2: comment lines are not supported",
		},
		{
			name:    "empty method",
			content: "\t/health\treason\n",
			wantErr: "line 1: method must not be empty",
		},
		{
			name:    "empty path",
			content: "GET\t\treason\n",
			wantErr: "line 1: path must not be empty",
		},
		{
			name:    "empty reason",
			content: "GET\t/health\t\n",
			wantErr: "line 1: reason must not be empty",
		},
		{
			name:    "unsupported method",
			content: "PATCH\t/health\treason\n",
			wantErr: "line 1: method PATCH must be one of DELETE, GET, POST, PUT",
		},
		{
			name:    "path without slash",
			content: "GET\thealth\treason\n",
			wantErr: "line 1: path must start with /",
		},
		{
			name:    "duplicate route",
			content: "GET\t/health\tfirst\nGET\t/health\tsecond\n",
			wantErr: "line 2: duplicate route GET /health",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTempTSV(t, tt.content)

			_, err := parseRouteSetFromTSV(path)
			if err == nil {
				t.Fatalf("parseRouteSetFromTSV(%q) succeeded, want error containing %q", path, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("parseRouteSetFromTSV(%q) error = %q, want containing %q", path, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestParseRouteSetFromTSVAcceptsValidRows(t *testing.T) {
	path := writeTempTSV(t, "get\t/health\treadiness\nDELETE\t/readers/{name}\tlive reader state\n")

	routes, err := parseRouteSetFromTSV(path)
	if err != nil {
		t.Fatalf("parseRouteSetFromTSV(%q): %v", path, err)
	}

	want := map[string]bool{
		"GET /health":            true,
		"DELETE /readers/{name}": true,
	}
	for key := range want {
		if !routes[key] {
			t.Fatalf("parseRouteSetFromTSV(%q) missing route %s", path, key)
		}
	}
	if len(routes) != len(want) {
		t.Fatalf("parseRouteSetFromTSV(%q) returned %d routes, want %d", path, len(routes), len(want))
	}
}

func registeredRouteOperations(t *testing.T) []routeOperation {
	t.Helper()
	data, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatalf("read server.go: %v", err)
	}
	// This guard intentionally tracks literal mux.HandleFunc("METHOD /api...") route registrations.
	re := regexp.MustCompile(`mux\.HandleFunc\("([A-Z]+) /api([^"]+)"`)
	matches := re.FindAllStringSubmatch(string(data), -1)
	ops := make([]routeOperation, 0, len(matches))
	for _, match := range matches {
		ops = append(ops, routeOperation{Method: match[1], Path: match[2]})
	}
	sortRouteOperations(ops)
	return ops
}

func openAPIOperations(t *testing.T) map[string]bool {
	t.Helper()
	file, err := os.Open(filepath.Clean("../../../api/openapi/expensor.openapi.yaml"))
	if err != nil {
		t.Fatalf("open OpenAPI artifact: %v", err)
	}
	defer file.Close()

	ops := map[string]bool{}
	var currentPath string
	pathRE := regexp.MustCompile(`^  (/[^:]+):$`)
	methodRE := regexp.MustCompile(`^    (get|post|put|delete):$`)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if match := pathRE.FindStringSubmatch(line); match != nil {
			currentPath = match[1]
			continue
		}
		if currentPath == "" {
			continue
		}
		if match := methodRE.FindStringSubmatch(line); match != nil {
			ops[strings.ToUpper(match[1])+" "+currentPath] = true
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan OpenAPI artifact: %v", err)
	}
	return ops
}

func routeSetFromTSV(t *testing.T, path string) map[string]bool {
	t.Helper()
	routes, err := parseRouteSetFromTSV(path)
	if err != nil {
		t.Fatal(err)
	}
	return routes
}

func parseRouteSetFromTSV(path string) (map[string]bool, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	routes := map[string]bool{}
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
		if line == "" {
			return nil, fmt.Errorf("%s line %d: blank lines are not supported", path, lineNumber)
		}
		if strings.HasPrefix(line, "#") {
			return nil, fmt.Errorf("%s line %d: comment lines are not supported", path, lineNumber)
		}
		parts := strings.Split(rawLine, "\t")
		if len(parts) != 3 {
			return nil, fmt.Errorf("%s line %d: must have exactly 3 tab-separated fields", path, lineNumber)
		}
		method := strings.ToUpper(strings.TrimSpace(parts[0]))
		routePath := strings.TrimSpace(parts[1])
		reason := strings.TrimSpace(parts[2])
		if method == "" {
			return nil, fmt.Errorf("%s line %d: method must not be empty", path, lineNumber)
		}
		if routePath == "" {
			return nil, fmt.Errorf("%s line %d: path must not be empty", path, lineNumber)
		}
		if reason == "" {
			return nil, fmt.Errorf("%s line %d: reason must not be empty", path, lineNumber)
		}
		if !isContractRouteMethod(method) {
			return nil, fmt.Errorf("%s line %d: method %s must be one of DELETE, GET, POST, PUT", path, lineNumber, method)
		}
		if !strings.HasPrefix(routePath, "/") {
			return nil, fmt.Errorf("%s line %d: path must start with /", path, lineNumber)
		}
		key := method + " " + routePath
		if routes[key] {
			return nil, fmt.Errorf("%s line %d: duplicate route %s", path, lineNumber, key)
		}
		routes[key] = true
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}
	return routes, nil
}

func sortRouteOperations(ops []routeOperation) {
	sort.Slice(ops, func(i, j int) bool {
		return ops[i].key() < ops[j].key()
	})
}

func unregisteredDecisionRoutes(registered []routeOperation, decisionRoutes map[string]bool) []string {
	registeredRoutes := map[string]bool{}
	for _, op := range registered {
		registeredRoutes[op.key()] = true
	}

	var extras []string
	for key := range decisionRoutes {
		if !registeredRoutes[key] {
			extras = append(extras, key)
		}
	}
	sort.Strings(extras)
	return extras
}

func isContractRouteMethod(method string) bool {
	switch method {
	case "DELETE", "GET", "POST", "PUT":
		return true
	default:
		return false
	}
}

func writeTempTSV(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "routes.tsv")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp TSV: %v", err)
	}
	return path
}
