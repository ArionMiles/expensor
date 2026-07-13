package postgres

import "testing"

func TestParsePoolConfigSetsSearchPath(t *testing.T) {
	cfg, err := ParsePoolConfig("host=localhost port=5432 user=expensor dbname=expensor sslmode=disable")
	if err != nil {
		t.Fatalf("ParsePoolConfig: %v", err)
	}

	if got := cfg.ConnConfig.RuntimeParams["search_path"]; got != SearchPath {
		t.Fatalf("search_path = %q, want %s", got, SearchPath)
	}
}
