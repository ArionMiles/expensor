package sqlite

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/auth"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/internal/store/storetest"
	"github.com/ArionMiles/expensor/backend/pkg/config"
)

func TestRuntimeStoreConformance(t *testing.T) {
	storetest.RunRuntime(t, func(t *testing.T) storetest.Subject[storetest.RuntimeSubject] {
		t.Helper()
		st := openRuntimeTestStore(t, true)
		return storetest.Subject[storetest.RuntimeSubject]{Store: st, Close: st.Close}
	})
}

func TestRuntimeStoreWithoutSecret(t *testing.T) {
	storetest.RunRuntimeWithoutSecret(t, func(t *testing.T) storetest.Subject[storetest.RuntimeSubject] {
		t.Helper()
		st := openRuntimeTestStore(t, false)
		return storetest.Subject[storetest.RuntimeSubject]{Store: st, Close: st.Close}
	})
}

func TestReaderSecretCiphertextIsBoundToAssociatedData(t *testing.T) {
	st := openRuntimeTestStore(t, true)
	ctx := context.Background()
	tenant := insertRuntimeTestUser(t, st, "runtime-associated-data")
	secret := []byte(`{"client_secret":"private-marker"}`)
	if err := st.SetReaderSecret(ctx, tenant, "gmail", secret); err != nil {
		t.Fatalf("set reader secret: %v", err)
	}

	var ciphertext []byte
	if err := st.db.QueryRowContext(ctx, `
		SELECT client_secret_ciphertext FROM reader_runtime WHERE tenant_id = ? AND reader = 'gmail'
	`, tenant.ID).Scan(&ciphertext); err != nil {
		t.Fatalf("read stored ciphertext: %v", err)
	}
	if strings.Contains(string(ciphertext), "private-marker") {
		t.Fatal("stored reader secret contains plaintext")
	}
	if _, err := st.db.ExecContext(ctx, `
		INSERT INTO reader_runtime (tenant_id, reader, client_secret_ciphertext) VALUES (?, 'other', ?)
	`, tenant.ID, ciphertext); err != nil {
		t.Fatalf("copy ciphertext to another reader: %v", err)
	}
	if _, _, err := st.GetReaderSecret(ctx, tenant, "other"); err == nil {
		t.Fatal("reader ciphertext decrypted with different associated data")
	}
}

func TestSetActiveLLMProviderRollsBackClearWhenSetFails(t *testing.T) {
	st := openRuntimeTestStore(t, true)
	ctx := context.Background()
	tenant := insertRuntimeTestUser(t, st, "runtime-activation")
	if err := st.SetActiveLLMProvider(ctx, tenant, "existing"); err != nil {
		t.Fatalf("set initial active provider: %v", err)
	}
	if _, err := st.db.ExecContext(ctx, `
		CREATE TRIGGER reject_runtime_provider
		BEFORE INSERT ON llm_provider_runtime
		WHEN NEW.provider = 'rejected'
		BEGIN
			SELECT RAISE(ABORT, 'rejected provider');
		END
	`); err != nil {
		t.Fatalf("create rejection trigger: %v", err)
	}
	if err := st.SetActiveLLMProvider(ctx, tenant, "rejected"); err == nil {
		t.Fatal("set rejected active provider returned nil error")
	}
	runtime, found, err := st.GetActiveLLMProviderRuntime(ctx, tenant)
	if err != nil || !found || runtime.Provider != "existing" {
		t.Fatalf("active provider after rollback = %#v, found=%v, err=%v", runtime, found, err)
	}
}

func openRuntimeTestStore(t *testing.T, withSecret bool) *Store {
	t.Helper()

	opts := testStoreOptions(filepath.Join(privateTestDir(t), "runtime.db"))
	if withSecret {
		opts.Security = config.Security{SecretKey: make([]byte, auth.SecretKeySize)}
	}
	st, err := New(context.Background(), opts)
	if err != nil {
		t.Fatalf("open runtime test store: %v", err)
	}
	return st
}

func insertRuntimeTestUser(t *testing.T, st *Store, name string) store.Tenant {
	t.Helper()

	tenant := store.Tenant{ID: name}
	if _, err := st.db.ExecContext(context.Background(), `
		INSERT INTO users (id, email, display_name, role) VALUES (?, ?, ?, 'user')
	`, tenant.ID, name+"@example.test", name); err != nil {
		t.Fatalf("insert runtime test user: %v", err)
	}
	return tenant
}
