package app

import (
	"context"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/internal/store/postgres"
	"github.com/ArionMiles/expensor/backend/internal/store/sqlite"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

type backendStub struct{ store.Backend }

func TestOpenStoreBackendSelection(t *testing.T) {
	for _, backend := range []config.DatabaseBackend{"", config.DatabaseBackendSQLite} {
		t.Run(string(backend), func(t *testing.T) {
			sqliteCalls := 0
			got, err := openStoreBackendWith(context.Background(), StoreOptions{Database: config.Database{Backend: backend}}, storeConstructors{
				openSQLite: func(context.Context, sqlite.Options) (store.Backend, error) {
					sqliteCalls++
					return &backendStub{}, nil
				},
				openPostgres: func(context.Context, postgres.Options) (store.Backend, error) {
					t.Fatal("PostgreSQL constructor called")
					return nil, nil
				},
			})
			if err != nil || got == nil || sqliteCalls != 1 {
				t.Fatalf("openStoreBackendWith() = %v, %v; SQLite calls=%d", got, err, sqliteCalls)
			}
		})
	}
}

func TestOpenStoreBackendSelectsPostgresAndPreservesErrors(t *testing.T) {
	want := errors.B.Op("test.open").KindUnavailable().Build()
	_, err := openStoreBackendWith(context.Background(), StoreOptions{Database: config.Database{Backend: config.DatabaseBackendPostgres}}, storeConstructors{
		openSQLite:   func(context.Context, sqlite.Options) (store.Backend, error) { return nil, nil },
		openPostgres: func(context.Context, postgres.Options) (store.Backend, error) { return nil, want },
	})
	if !errors.Is(err, want) {
		t.Fatalf("openStoreBackendWith() error = %v, want preserved constructor error", err)
	}
}
