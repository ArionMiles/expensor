package dbconn

import "testing"

func TestParseConfigSetsSearchPath(t *testing.T) {
	cfg, err := ParseConfig("host=localhost port=5432 user=expensor dbname=expensor sslmode=disable")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	if got := cfg.ConnConfig.RuntimeParams["search_path"]; got != "expensor,public" {
		t.Fatalf("search_path = %q, want expensor,public", got)
	}
}
