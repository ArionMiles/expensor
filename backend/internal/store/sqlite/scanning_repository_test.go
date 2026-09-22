package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/internal/store/storetest"
)

func TestScanningStoreConformance(t *testing.T) {
	storetest.RunScanning(t, func(t *testing.T) storetest.Subject[storetest.ScanningSubject] {
		t.Helper()
		st, err := New(context.Background(), testStoreOptions(filepath.Join(privateTestDir(t), "scanning.db")))
		if err != nil {
			t.Fatalf("open scanning test store: %v", err)
		}
		return storetest.Subject[storetest.ScanningSubject]{Store: st, Close: st.Close}
	})
}

func TestScanningWritesFixedUTCTimestamps(t *testing.T) {
	st, err := New(context.Background(), testStoreOptions(filepath.Join(privateTestDir(t), "scanning-time.db")))
	if err != nil {
		t.Fatalf("open scanning test store: %v", err)
	}
	t.Cleanup(st.Close)

	ctx := context.Background()
	tenant := store.Tenant{ID: "scanning-time"}
	if _, err := st.db.ExecContext(ctx, `
		INSERT INTO users (id, email, display_name, role) VALUES (?, 'scanning-time@example.test', 'Scanning Time', 'user')
	`, tenant.ID); err != nil {
		t.Fatalf("insert scanning test user: %v", err)
	}
	want := time.Date(2026, time.September, 19, 12, 34, 56, 123456000, time.FixedZone("test", 5*60*60+30*60))
	st.repositories.now = func() time.Time { return want }
	if err := st.SetActiveScanningReader(ctx, tenant, "reader"); err != nil {
		t.Fatalf("set active scanning reader: %v", err)
	}

	var stored string
	if err := st.db.QueryRowContext(ctx, `SELECT updated_at FROM tenant_scanning_state WHERE tenant_id = ?`, tenant.ID).Scan(&stored); err != nil {
		t.Fatalf("read scanning timestamp: %v", err)
	}
	if stored != timeToText(want) {
		t.Fatalf("stored timestamp = %q, want %q", stored, timeToText(want))
	}
	state, err := st.GetScanningState(ctx, tenant)
	if err != nil {
		t.Fatalf("get scanning state: %v", err)
	}
	if !state.UpdatedAt.Equal(want) || state.UpdatedAt.Location() != time.UTC {
		t.Fatalf("updated timestamp = %v in %v, want %v in UTC", state.UpdatedAt, state.UpdatedAt.Location(), want)
	}
}
