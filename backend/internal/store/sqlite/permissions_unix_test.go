//go:build unix

package sqlite

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

func TestNewCreatesExactSecureModesWithRestrictiveUmask(t *testing.T) {
	tempDir := t.TempDir()
	oldUmask := syscall.Umask(0o777)
	defer syscall.Umask(oldUmask)

	private := filepath.Join(tempDir, "private")
	parent := filepath.Join(private, "data")
	path := filepath.Join(parent, "expensor.db")
	store := openTestStore(t, path)

	assertMode(t, private, 0o700)
	assertMode(t, parent, 0o700)
	assertMode(t, path, 0o600)
	if got := store.db.Stats().MaxOpenConnections; got != connectionPoolSize {
		t.Fatalf("maximum open connections = %d, want %d", got, connectionPoolSize)
	}
}

func TestConcurrentNewWaitsForSecurePathCreation(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*testing.T) (string, string)
		setBlocker func(*pathCreationHooks, func())
	}{
		{
			name: "nested directory",
			setup: func(t *testing.T) (string, string) {
				t.Helper()
				private := filepath.Join(t.TempDir(), "private")
				if err := os.Mkdir(private, 0o700); err != nil {
					t.Fatalf("create existing parent: %v", err)
				}
				parent := filepath.Join(private, "data")
				return filepath.Join(parent, "expensor.db"), parent
			},
			setBlocker: func(hooks *pathCreationHooks, block func()) {
				hooks.afterDirectoryCreate = block
			},
		},
		{
			name: "database file",
			setup: func(t *testing.T) (string, string) {
				t.Helper()
				parent := filepath.Join(t.TempDir(), "private")
				if err := os.Mkdir(parent, 0o700); err != nil {
					t.Fatalf("create existing parent: %v", err)
				}
				path := filepath.Join(parent, "expensor.db")
				return path, path
			},
			setBlocker: func(hooks *pathCreationHooks, block func()) {
				hooks.afterFileCreate = block
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path, securedPath := test.setup(t)
			oldUmask := syscall.Umask(0o777)
			defer syscall.Umask(oldUmask)

			created := make(chan struct{})
			release := make(chan struct{})
			t.Cleanup(func() {
				select {
				case <-release:
				default:
					close(release)
				}
			})
			firstHooks := defaultLifecycleHooks()
			test.setBlocker(&firstHooks.pathCreation, func() {
				close(created)
				<-release
			})
			first := openStoreAsync(testStoreOptions(path), firstHooks)
			waitForSignal(t, created, "first creator")
			temporaryMode := os.FileMode(0)
			if test.name == "nested directory" {
				temporaryMode = 0o500
			}
			assertMode(t, securedPath, temporaryMode)

			retrying := make(chan struct{}, 1)
			secondHooks := defaultLifecycleHooks()
			secondHooks.pathCreation.beforeRetry = func() {
				select {
				case retrying <- struct{}{}:
				default:
				}
			}
			second := openStoreAsync(testStoreOptions(path), secondHooks)
			waitForSignal(t, retrying, "second creator retry")
			close(release)

			firstStore := waitForStore(t, first, "first creator")
			defer firstStore.Close()
			secondStore := waitForStore(t, second, "second creator")
			defer secondStore.Close()

			if test.name == "nested directory" {
				assertMode(t, securedPath, 0o700)
			}
			assertMode(t, path, 0o600)
		})
	}
}

func TestNewDirectoryReplacementDoesNotChmodTarget(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "unrelated")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("create unrelated target: %v", err)
	}
	if err := os.Chmod(target, 0o755); err != nil {
		t.Fatalf("set unrelated target mode: %v", err)
	}

	created := filepath.Join(root, "private")
	displaced := filepath.Join(root, "displaced")
	var hookErr error
	hooks := defaultLifecycleHooks()
	hooks.pathCreation.afterDirectoryCreate = func() {
		if hookErr != nil {
			return
		}
		if err := os.Rename(created, displaced); err != nil {
			hookErr = err
			return
		}
		hookErr = os.Symlink(target, created)
	}

	store, err := newWithLifecycle(
		context.Background(),
		testStoreOptions(filepath.Join(created, "data", "expensor.db")),
		hooks,
	)
	if store != nil {
		store.Close()
		t.Fatal("New returned a store after directory replacement")
	}
	if hookErr != nil {
		t.Fatalf("replace created directory: %v", hookErr)
	}
	if got := errors.WhatKind(err); got != errors.InvalidArgument && got != errors.PermissionDenied {
		t.Fatalf("error kind = %v, want InvalidArgument or PermissionDenied: %v", got, err)
	}
	assertMode(t, target, 0o755)
}

func TestNewRejectsSymlinkPaths(t *testing.T) {
	t.Run("ancestor component", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, "target")
		if err := os.Mkdir(target, 0o700); err != nil {
			t.Fatalf("create target directory: %v", err)
		}
		link := filepath.Join(root, "linked-parent")
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("create parent symlink: %v", err)
		}

		assertNewErrorKind(t, filepath.Join(link, "data", "expensor.db"), errors.InvalidArgument)
	})

	t.Run("immediate parent", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, "target")
		if err := os.Mkdir(target, 0o700); err != nil {
			t.Fatalf("create target directory: %v", err)
		}
		link := filepath.Join(root, "linked-parent")
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("create immediate parent symlink: %v", err)
		}

		assertNewErrorKind(t, filepath.Join(link, "expensor.db"), errors.InvalidArgument)
	})

	t.Run("database file", func(t *testing.T) {
		parent := filepath.Join(t.TempDir(), "private")
		if err := os.Mkdir(parent, 0o700); err != nil {
			t.Fatalf("create parent: %v", err)
		}
		target := filepath.Join(parent, "target.db")
		if err := os.WriteFile(target, []byte("not a database"), 0o600); err != nil {
			t.Fatalf("create target file: %v", err)
		}
		link := filepath.Join(parent, "expensor.db")
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("create database symlink: %v", err)
		}

		assertNewErrorKind(t, link, errors.InvalidArgument)
	})
}

func TestAllowsOnlyRootOwnedAncestorSymlinks(t *testing.T) {
	databasePath := filepath.Join("root", "system-alias", "private", "expensor.db")
	userID := uint32(os.Geteuid())
	if userID == 0 {
		userID = 1
	}
	rootOwnedLink := stubFileInfo{
		mode: os.ModeSymlink,
		sys:  &syscall.Stat_t{Uid: 0},
	}
	userOwnedLink := stubFileInfo{
		mode: os.ModeSymlink,
		sys:  &syscall.Stat_t{Uid: userID},
	}

	tests := []struct {
		name    string
		path    string
		info    os.FileInfo
		allowed bool
	}{
		{name: "root-owned ancestor", path: filepath.Join("root", "system-alias"), info: rootOwnedLink, allowed: true},
		{name: "user-owned ancestor", path: filepath.Join("root", "system-alias"), info: userOwnedLink},
		{name: "root-owned immediate parent", path: filepath.Dir(databasePath), info: rootOwnedLink},
		{name: "root-owned database file", path: databasePath, info: rootOwnedLink},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := allowsSystemAncestorSymlink(test.info, test.path, databasePath); got != test.allowed {
				t.Fatalf("allowsSystemAncestorSymlink() = %t, want %t", got, test.allowed)
			}
		})
	}
}

type stubFileInfo struct {
	mode os.FileMode
	sys  any
}

func (info stubFileInfo) Name() string       { return "system-alias" }
func (info stubFileInfo) Size() int64        { return 0 }
func (info stubFileInfo) Mode() os.FileMode  { return info.mode }
func (info stubFileInfo) ModTime() time.Time { return time.Time{} }
func (info stubFileInfo) IsDir() bool        { return false }
func (info stubFileInfo) Sys() any           { return info.sys }

func TestNewRejectsInsecureExistingPathModesWithoutChangingThem(t *testing.T) {
	tests := []struct {
		name       string
		mode       os.FileMode
		changePath func(parent, database string) string
	}{
		{
			name: "parent 0755",
			mode: 0o755,
			changePath: func(parent, _ string) string {
				return parent
			},
		},
		{
			name: "database 0400",
			mode: 0o400,
			changePath: func(_, database string) string {
				return database
			},
		},
		{
			name: "database 0644",
			mode: 0o644,
			changePath: func(_, database string) string {
				return database
			},
		},
		{
			name: "database 0666",
			mode: 0o666,
			changePath: func(_, database string) string {
				return database
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parent := filepath.Join(t.TempDir(), "private")
			database := filepath.Join(parent, "expensor.db")
			store := openTestStore(t, database)
			store.Close()

			changedPath := test.changePath(parent, database)
			if err := os.Chmod(changedPath, test.mode); err != nil {
				t.Fatalf("set insecure existing mode: %v", err)
			}

			reopened, err := New(context.Background(), testStoreOptions(database))
			if reopened != nil {
				reopened.Close()
				t.Fatal("New returned a store for an insecure existing path")
			}
			if got := errors.WhatKind(err); got != errors.PermissionDenied {
				t.Fatalf("error kind = %v, want %v: %v", got, errors.PermissionDenied, err)
			}
			assertMode(t, changedPath, test.mode)
		})
	}
}

func TestNewRedactsFilesystemPathsAndRetainsCause(t *testing.T) {
	secret := strings.Repeat("private-marker-", 30)
	path := filepath.Join(t.TempDir(), secret, "expensor.db")
	store, err := New(context.Background(), testStoreOptions(path))
	if store != nil {
		store.Close()
		t.Fatal("New returned a store for an overlong path")
	}
	if got := errors.WhatKind(err); got != errors.InvalidArgument {
		t.Fatalf("error kind = %v, want %v: %v", got, errors.InvalidArgument, err)
	}
	if !errors.Is(err, syscall.ENAMETOOLONG) {
		t.Fatalf("error = %v, want ENAMETOOLONG cause", err)
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), path) {
		t.Fatalf("error exposed configured path: %v", err)
	}
}

func TestNewCancelsConcurrentPathWait(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	if err := os.Chmod(parent, 0); err != nil {
		t.Fatalf("make parent temporarily inaccessible: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(parent, 0o700); err != nil {
			t.Errorf("restore parent mode: %v", err)
		}
	})

	retrying := make(chan struct{})
	hooks := defaultLifecycleHooks()
	hooks.pathCreation.beforeRetry = func() { close(retrying) }
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan storeOpenResult, 1)
	go func() {
		store, err := newWithLifecycle(ctx, testStoreOptions(filepath.Join(parent, "expensor.db")), hooks)
		result <- storeOpenResult{store: store, err: err}
	}()

	waitForSignal(t, retrying, "path retry")
	cancel()
	select {
	case opened := <-result:
		if opened.store != nil {
			opened.store.Close()
			t.Fatal("New returned a store after cancellation")
		}
		if !errors.Is(opened.err, context.Canceled) {
			t.Fatalf("New error = %v, want context.Canceled", opened.err)
		}
		if got := errors.WhatKind(opened.err); got != errors.Canceled {
			t.Fatalf("error kind = %v, want %v", got, errors.Canceled)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("New did not return after path wait cancellation")
	}
}

func TestNewRejectsDatabaseReplacementAfterOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private data", "expensor.db")
	dsn := (&url.URL{Scheme: "file", Path: path}).String()
	store := openTestStore(t, dsn)
	store.Close()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read initialized database: %v", err)
	}

	backup := path + ".original"
	hooks := defaultLifecycleHooks()
	hooks.afterDatabaseOpen = func() {
		if err := os.Rename(path, backup); err != nil {
			t.Fatalf("move validated database: %v", err)
		}
		if err := os.WriteFile(path, contents, 0o600); err != nil {
			t.Fatalf("replace validated database: %v", err)
		}
	}

	reopened, err := newWithLifecycle(context.Background(), testStoreOptions(dsn), hooks)
	if reopened != nil {
		reopened.Close()
		t.Fatal("New returned a store after database replacement")
	}
	if got := errors.WhatKind(err); got != errors.PermissionDenied {
		t.Fatalf("error kind = %v, want %v: %v", got, errors.PermissionDenied, err)
	}
}

func TestPreparedDatabasePathRejectsDifferentLiveDatabase(t *testing.T) {
	root := privateTestDir(t)
	path := filepath.Join(root, "expensor.db")
	replacement := filepath.Join(root, "replacement.db")
	for _, databasePath := range []string{path, replacement} {
		store := openTestStore(t, databasePath)
		store.Close()
	}

	prepared, err := prepareDatabasePath(context.Background(), path, pathCreationHooks{})
	if err != nil {
		t.Fatalf("prepare original database: %v", err)
	}
	t.Cleanup(func() { _ = prepared.close() })
	original := path + ".original"
	if err := os.Rename(path, original); err != nil {
		t.Fatalf("move original database: %v", err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatalf("replace database with B: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Remove(path + "-shm")
		_ = os.Remove(path + "-wal")
		_ = os.Rename(path, replacement)
		_ = os.Rename(original, path)
	})

	db, err := openDatabase(context.Background(), path, time.Second)
	if err != nil {
		t.Fatalf("open replacement database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	err = prepared.verifyLiveIdentity(context.Background(), db)
	if got := errors.WhatKind(err); got != errors.PermissionDenied {
		t.Fatalf("error kind = %v, want %v: %v", got, errors.PermissionDenied, err)
	}
	if strings.Contains(err.Error(), path) || strings.Contains(err.Error(), replacement) {
		t.Fatalf("identity error exposed a database path: %v", err)
	}
}

func assertNewErrorKind(t *testing.T, path string, want errors.Kind) {
	t.Helper()
	store, err := New(context.Background(), testStoreOptions(path))
	if store != nil {
		store.Close()
		t.Fatal("New returned a store for an invalid path")
	}
	if got := errors.WhatKind(err); got != want {
		t.Fatalf("error kind = %v, want %v: %v", got, want, err)
	}
}

type storeOpenResult struct {
	store *Store
	err   error
}

func openStoreAsync(opts Options, hooks lifecycleHooks) <-chan storeOpenResult {
	result := make(chan storeOpenResult, 1)
	go func() {
		store, err := newWithLifecycle(context.Background(), opts, hooks)
		result <- storeOpenResult{store: store, err: err}
	}()
	return result
}

func waitForSignal(t *testing.T, signal <-chan struct{}, operation string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", operation)
	}
}

func waitForStore(t *testing.T, result <-chan storeOpenResult, operation string) *Store {
	t.Helper()
	select {
	case opened := <-result:
		if opened.err != nil {
			t.Fatalf("%s: %v", operation, opened.err)
		}
		if opened.store == nil {
			t.Fatalf("%s returned no store", operation)
		}
		return opened.store
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", operation)
		return nil
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat path: %v", err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("path mode = %#o, want %#o", got, want)
	}
}
