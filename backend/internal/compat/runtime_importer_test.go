package compat_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/compat"
)

type fakeRuntimeStore struct {
	activeReader string
	secrets      map[string][]byte
	tokens       map[string][]byte
	configs      map[string]json.RawMessage
	processed    map[string]time.Time
}

func newFakeRuntimeStore() *fakeRuntimeStore {
	return &fakeRuntimeStore{
		secrets:   make(map[string][]byte),
		tokens:    make(map[string][]byte),
		configs:   make(map[string]json.RawMessage),
		processed: make(map[string]time.Time),
	}
}

func (f *fakeRuntimeStore) SetActiveReader(_ context.Context, reader string) error {
	f.activeReader = reader
	return nil
}

func (f *fakeRuntimeStore) SetReaderSecret(_ context.Context, reader string, secret []byte) error {
	f.secrets[reader] = append([]byte(nil), secret...)
	return nil
}

func (f *fakeRuntimeStore) SetReaderToken(_ context.Context, reader string, token []byte) error {
	f.tokens[reader] = append([]byte(nil), token...)
	return nil
}

func (f *fakeRuntimeStore) SetReaderConfig(_ context.Context, reader string, config json.RawMessage) error {
	f.configs[reader] = append(json.RawMessage(nil), config...)
	return nil
}

func (f *fakeRuntimeStore) MarkMessageProcessed(_ context.Context, key string, at time.Time) error {
	f.processed[key] = at
	return nil
}

func TestRuntimeImporterImportsLegacyFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "active_reader"), "gmail")
	writeFile(t, filepath.Join(dir, "client_secret_gmail.json"), `{"installed":{}}`)
	writeFile(t, filepath.Join(dir, "token_gmail.json"), `{"access_token":"a","token_type":"Bearer"}`)
	writeFile(t, filepath.Join(dir, "config_thunderbird.json"), `{"config":{"mailboxes":"Inbox"}}`)
	writeFile(t, filepath.Join(dir, "state.json"), `{"processed_messages":{"msg-1":"2026-04-28T00:00:00Z"}}`)

	store := newFakeRuntimeStore()
	importer := compat.NewRuntimeImporter(dir, store, slog.Default())
	result, err := importer.Import(context.Background())
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.ImportedFiles != 5 {
		t.Fatalf("imported files = %d", result.ImportedFiles)
	}
	if store.activeReader != "gmail" {
		t.Fatalf("active reader = %q", store.activeReader)
	}
	if string(store.secrets["gmail"]) != `{"installed":{}}` {
		t.Fatalf("secret = %s", store.secrets["gmail"])
	}
	if string(store.tokens["gmail"]) != `{"access_token":"a","token_type":"Bearer"}` {
		t.Fatalf("token = %s", store.tokens["gmail"])
	}
	if string(store.configs["thunderbird"]) != `{"config":{"mailboxes":"Inbox"}}` {
		t.Fatalf("config = %s", store.configs["thunderbird"])
	}
	if _, ok := store.processed["msg-1"]; !ok {
		t.Fatal("processed message was not imported")
	}
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
