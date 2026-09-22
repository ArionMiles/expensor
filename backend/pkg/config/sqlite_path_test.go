package config

import (
	"path/filepath"
	"testing"
)

func TestResolveSQLitePath(t *testing.T) {
	tests := []struct {
		name, goos, home, xdg, explicit, want string
	}{
		{name: "linux xdg", goos: "linux", home: "/home/user", xdg: "/data", want: "/data/expensor/expensor.db"},
		{name: "linux home", goos: "linux", home: "/home/user", want: "/home/user/.local/share/expensor/expensor.db"},
		{name: "macOS", goos: "darwin", home: "/Users/user", want: "/Users/user/Library/Application Support/Expensor/expensor.db"},
		{name: "explicit absolute", goos: "linux", explicit: "/srv/expensor.db", want: "/srv/expensor.db"},
		{name: "explicit relative", goos: "linux", explicit: "data/expensor.db", want: "data/expensor.db"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolveSQLitePath(test.goos, test.home, test.xdg, test.explicit)
			if err != nil || got != filepath.Clean(test.want) {
				t.Fatalf("resolveSQLitePath() = %q, %v; want %q", got, err, test.want)
			}
		})
	}
}

func TestResolveSQLitePathRequiresHome(t *testing.T) {
	if _, err := resolveSQLitePath("linux", "", "", ""); err == nil {
		t.Fatal("resolveSQLitePath() succeeded without a home or XDG data directory")
	}
}
