package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	storepkg "github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/internal/store/storetest"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

func TestAuthStoreConformance(t *testing.T) {
	storetest.RunAuth(t, func(t *testing.T) storetest.Subject[storepkg.AuthStore] {
		t.Helper()
		store := openAuthTestStore(t, "auth.db")
		return storetest.Subject[storepkg.AuthStore]{
			Store: store,
			Close: store.Close,
		}
	})
}

func TestCreateBootstrapAdminIsAtomic(t *testing.T) {
	store := openAuthTestStore(t, "bootstrap.db")
	ctx := context.Background()
	start := make(chan struct{})
	errs := make(chan error, 2)

	var ready sync.WaitGroup
	ready.Add(2)
	for _, email := range []string{"first@example.test", "second@example.test"} {
		go func() {
			ready.Done()
			<-start
			_, err := store.CreateBootstrapAdmin(ctx, storepkg.CreateBootstrapAdminInput{
				Email:        email,
				DisplayName:  email,
				PasswordHash: "hash",
			})
			errs <- err
		}()
	}
	ready.Wait()
	close(start)

	assertConcurrentResults(t, errs, errors.Conflict)
	users, err := store.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("ListUsers count = %d, want 1", len(users))
	}
}

func TestCompleteAccountSetupIsOneUse(t *testing.T) {
	store := openAuthTestStore(t, "setup.db")
	ctx := context.Background()
	user, err := store.CreateUser(ctx, storepkg.CreateUserInput{
		Email:       "setup@example.test",
		DisplayName: "Pending",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	_, err = store.CreateAccountSetupToken(ctx, storepkg.CreateAccountSetupTokenInput{
		UserID:    user.ID,
		TokenHash: "setup-hash",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateAccountSetupToken: %v", err)
	}

	start := make(chan struct{})
	errs := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for range 2 {
		go func() {
			ready.Done()
			<-start
			_, err := store.CompleteAccountSetup(ctx, storepkg.CompleteAccountSetupInput{
				TokenHash:    "setup-hash",
				PasswordHash: "completed-hash",
				DisplayName:  "Complete",
				AvatarKey:    "wallet",
			})
			errs <- err
		}()
	}
	ready.Wait()
	close(start)

	assertConcurrentResults(t, errs, errors.NotFound)
}

func assertConcurrentResults(t *testing.T, results <-chan error, losingKind errors.Kind) {
	t.Helper()
	successes := 0
	losses := 0
	for range 2 {
		err := <-results
		if err == nil {
			successes++
			continue
		}
		if errors.WhatKind(err) != losingKind {
			t.Fatalf("concurrent call error = %v, want %v kind", err, losingKind)
		}
		losses++
	}
	if successes != 1 || losses != 1 {
		t.Fatalf("concurrent results = %d successes and %d losses, want 1 and 1", successes, losses)
	}
}

func openAuthTestStore(t *testing.T, name string) *Store {
	t.Helper()
	directory := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatalf("create auth test directory: %v", err)
	}
	store, err := New(context.Background(), Options{
		Config: config.SQLite{
			Path:        filepath.Join(directory, name),
			BusyTimeout: 750 * time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("open auth test store: %v", err)
	}
	t.Cleanup(store.Close)
	return store
}
