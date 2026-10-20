package sqlite

import "testing"

func TestFTSQueryQuotesTokens(t *testing.T) {
	t.Parallel()

	if got, want := ftsQuery(`cafe "prime"`), `"cafe" AND """prime"""`; got != want {
		t.Fatalf("ftsQuery() = %q, want %q", got, want)
	}
}
