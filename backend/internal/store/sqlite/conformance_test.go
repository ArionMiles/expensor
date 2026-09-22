package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/auth"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/internal/store/storetest"
	"github.com/ArionMiles/expensor/backend/pkg/config"
)

var _ store.Backend = (*Store)(nil)

func TestStoreConformance(t *testing.T) {
	storetest.Run(t, func(t *testing.T) storetest.Subject[store.Backend] {
		t.Helper()
		directory := filepath.Join(t.TempDir(), "private")
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatalf("create conformance directory: %v", err)
		}
		options := Options{
			Config:   config.SQLite{Path: filepath.Join(directory, "conformance.db"), BusyTimeout: 750 * time.Millisecond},
			Security: config.Security{SecretKey: make([]byte, auth.SecretKeySize)},
		}
		current := openConformanceStore(t, options)
		return storetest.Subject[store.Backend]{
			Store: current,
			Close: func() { current.Close() },
			Restart: func(t *testing.T) store.Backend {
				t.Helper()
				current.Close()
				current = openConformanceStore(t, options)
				return current
			},
			RejectIngestionMessage: func(t *testing.T, messageID string) {
				t.Helper()
				if _, err := current.db.ExecContext(context.Background(), `CREATE TRIGGER reject_conformance_ingestion
					BEFORE INSERT ON transactions WHEN NEW.message_id='`+messageID+`'
					BEGIN SELECT RAISE(ABORT,'rejected ingestion'); END`); err != nil {
					t.Fatalf("install ingestion rejection trigger: %v", err)
				}
			},
			Now: storetest.AnalyticsNow(),
		}
	})
}

func openConformanceStore(t *testing.T, options Options) *Store {
	t.Helper()
	result, err := New(context.Background(), options)
	if err != nil {
		t.Fatalf("open conformance store: %v", err)
	}
	result.repositories.now = storetest.AnalyticsNow
	return result
}
