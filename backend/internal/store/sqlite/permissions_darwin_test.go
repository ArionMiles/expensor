//go:build darwin

package sqlite

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewAllowsRootOwnedMacOSVarAlias(t *testing.T) {
	info, err := os.Lstat("/var")
	if err != nil {
		t.Fatalf("inspect /var alias: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 || !trustedSystemSymlink(info) {
		t.Fatal("/var is not a root-owned symbolic link")
	}

	parent, err := os.MkdirTemp("/var/tmp", "expensor-sqlite-")
	if err != nil {
		t.Fatalf("create database parent through /var alias: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(parent); err != nil {
			t.Errorf("remove database parent: %v", err)
		}
	})
	if err := os.Chmod(parent, 0o700); err != nil {
		t.Fatalf("secure database parent: %v", err)
	}

	store := openTestStore(t, filepath.Join(parent, "expensor.db"))
	store.Close()
}
