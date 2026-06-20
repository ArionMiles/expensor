package state

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

type fakeProcessedMessageStore struct {
	processed map[string]time.Time
	lastCtx   context.Context
	checkErr  error
	markErr   error
}

func (f *fakeProcessedMessageStore) IsMessageProcessed(ctx context.Context, _ store.Tenant, key string) (bool, error) {
	f.lastCtx = ctx
	if f.checkErr != nil {
		return false, f.checkErr
	}
	_, ok := f.processed[key]
	return ok, nil
}

func (f *fakeProcessedMessageStore) MarkMessageProcessed(ctx context.Context, _ store.Tenant, key string, at time.Time) error {
	f.lastCtx = ctx
	if f.markErr != nil {
		return f.markErr
	}
	f.processed[key] = at
	return nil
}

func TestDBManagerMarksProcessedMessages(t *testing.T) {
	processedStore := &fakeProcessedMessageStore{processed: map[string]time.Time{}}
	m := NewDBManager(processedStore, store.Tenant{}, testLogger())

	if m.IsProcessed(context.Background(), "msg-1") {
		t.Fatal("message should not start processed")
	}
	if err := m.MarkProcessed(context.Background(), "msg-1"); err != nil {
		t.Fatalf("MarkProcessed: %v", err)
	}
	if !m.IsProcessed(context.Background(), "msg-1") {
		t.Fatal("message should be processed")
	}
}

func TestDBManagerUsesCallerContext(t *testing.T) {
	processedStore := &fakeProcessedMessageStore{processed: map[string]time.Time{}}
	m := NewDBManager(processedStore, store.Tenant{}, testLogger())
	ctx := context.WithValue(context.Background(), testContextKey{}, "caller")

	if m.IsProcessed(ctx, "msg-1") {
		t.Fatal("message should not start processed")
	}
	assertStoreContext(processedStore.lastCtx, t)

	if err := m.MarkProcessed(ctx, "msg-1"); err != nil {
		t.Fatalf("MarkProcessed: %v", err)
	}
	assertStoreContext(processedStore.lastCtx, t)
}

func TestDBManagerCheckErrorDoesNotSkipMessages(t *testing.T) {
	processedStore := &fakeProcessedMessageStore{
		processed: map[string]time.Time{},
		checkErr:  errors.New("db unavailable"),
	}
	m := NewDBManager(processedStore, store.Tenant{}, testLogger())

	if m.IsProcessed(context.Background(), "msg-1") {
		t.Fatal("check errors should not mark messages as already processed")
	}
}

func TestDBManagerMarkErrorIsReturned(t *testing.T) {
	processedStore := &fakeProcessedMessageStore{
		processed: map[string]time.Time{},
		markErr:   errors.New("db unavailable"),
	}
	m := NewDBManager(processedStore, store.Tenant{}, testLogger())

	if err := m.MarkProcessed(context.Background(), "msg-1"); err == nil {
		t.Fatal("expected mark error")
	}
}

func TestDBManagerNilStoreIsConservative(t *testing.T) {
	m := NewDBManager(nil, store.Tenant{}, testLogger())

	if m.IsProcessed(context.Background(), "msg-1") {
		t.Fatal("nil store should not mark messages as already processed")
	}
	if err := m.MarkProcessed(context.Background(), "msg-1"); err == nil {
		t.Fatal("expected nil store error")
	}
}

type testContextKey struct{}

func assertStoreContext(ctx context.Context, t *testing.T) {
	t.Helper()
	if ctx == nil {
		t.Fatal("store context was nil")
	}
	if got := ctx.Value(testContextKey{}); got != "caller" {
		t.Fatalf("store context marker = %v, want caller context", got)
	}
}

func TestGenerateKey(t *testing.T) {
	tests := []struct {
		source    string
		messageID string
		date      string
	}{
		{"gmail", "msg123", "2024-01-15"},
		{"thunderbird", "abc456", "2024-01-16"},
	}

	for _, tc := range tests {
		key := GenerateKey(tc.source, tc.messageID, tc.date)

		if len(key) != 64 {
			t.Errorf("expected 64 char key, got %d", len(key))
		}

		key2 := GenerateKey(tc.source, tc.messageID, tc.date)
		if key != key2 {
			t.Error("same inputs should produce same key")
		}

		key3 := GenerateKey(tc.source+"x", tc.messageID, tc.date)
		if key == key3 {
			t.Error("different inputs should produce different keys")
		}
	}
}
