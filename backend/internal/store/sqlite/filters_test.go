package sqlite

import "testing"

func TestEscapeLikePattern(t *testing.T) {
	t.Parallel()

	if got, want := escapeLikePattern(`100%_\\done`), `%100\%\_\\\\done%`; got != want {
		t.Fatalf("escapeLikePattern() = %q, want %q", got, want)
	}
}

func TestFTSQueryQuotesTokens(t *testing.T) {
	t.Parallel()

	if got, want := ftsQuery(`cafe "prime"`), `"cafe" AND """prime"""`; got != want {
		t.Fatalf("ftsQuery() = %q, want %q", got, want)
	}
}
