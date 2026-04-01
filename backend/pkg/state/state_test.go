package state

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNew_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "nonexistent.json")

	m, err := New(stateFile, testLogger())
	if err != nil {
		t.Errorf("New should not error on non-existent file: %v", err)
	}

	if m == nil {
		t.Fatal("manager should not be nil")
	}

	if len(m.ProcessedMessages) != 0 {
		t.Error("processed messages should be empty for new state")
	}
}

func TestNew_EmptyPath(t *testing.T) {
	_, err := New("", testLogger())
	if err == nil {
		t.Error("expected error for empty path")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected error about empty path, got: %v", err)
	}
}

func TestNew_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "invalid.json")

	// Write invalid JSON to file
	if err := os.WriteFile(stateFile, []byte("invalid json"), 0o600); err != nil {
		t.Fatalf("failed to write invalid json: %v", err)
	}

	// Try to create manager with invalid file
	_, err := New(stateFile, testLogger())
	if err == nil {
		t.Error("expected error loading invalid JSON")
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

		// Key should be 64 hex characters (SHA256)
		if len(key) != 64 {
			t.Errorf("expected 64 char key, got %d", len(key))
		}

		// Same inputs should produce same key
		key2 := GenerateKey(tc.source, tc.messageID, tc.date)
		if key != key2 {
			t.Error("same inputs should produce same key")
		}

		// Different inputs should produce different keys
		key3 := GenerateKey(tc.source+"x", tc.messageID, tc.date)
		if key == key3 {
			t.Error("different inputs should produce different keys")
		}
	}
}

func TestIsProcessed(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.json")

	m, err := New(stateFile, testLogger())
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Manually add entries for testing
	m.ProcessedMessages["msg1"] = time.Now()
	m.ProcessedMessages["msg2"] = time.Now()

	tests := []struct {
		name   string
		msgKey string
		want   bool
	}{
		{"existing message", "msg1", true},
		{"non-existing message", "msg3", false},
		{"empty key", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.IsProcessed(tt.msgKey)
			if got != tt.want {
				t.Errorf("IsProcessed(%q) = %v, want %v", tt.msgKey, got, tt.want)
			}
		})
	}
}

func TestMarkProcessed(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.json")

	m, err := New(stateFile, testLogger())
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	msgKey := "test-message"
	before := time.Now()

	if err := m.MarkProcessed(msgKey); err != nil {
		t.Fatalf("MarkProcessed failed: %v", err)
	}

	if !m.IsProcessed(msgKey) {
		t.Error("message should be marked as processed")
	}

	timestamp, exists := m.ProcessedMessages[msgKey]
	if !exists {
		t.Fatal("message key not found in processed messages")
	}

	if timestamp.Before(before) {
		t.Error("timestamp should be after the test started")
	}

	// Verify file was written
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Error("state file should exist after MarkProcessed")
	}
}

func TestSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.json")

	// Create and save state
	m1, err := New(stateFile, testLogger())
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	m1.ProcessedMessages["msg1"] = time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	m1.ProcessedMessages["msg2"] = time.Date(2024, 1, 2, 11, 0, 0, 0, time.UTC)

	if err := m1.MarkProcessed("msg3"); err != nil {
		t.Fatalf("MarkProcessed failed: %v", err)
	}

	// Load state in new manager
	m2, err := New(stateFile, testLogger())
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	// Verify loaded state has all messages
	if m2.Count() != 3 {
		t.Errorf("expected 3 processed messages, got %d", m2.Count())
	}

	for _, key := range []string{"msg1", "msg2", "msg3"} {
		if !m2.IsProcessed(key) {
			t.Errorf("message %q should be processed", key)
		}
	}
}

func TestPrune(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.json")

	m, err := New(stateFile, testLogger())
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	now := time.Now()
	m.ProcessedMessages["old1"] = now.Add(-48 * time.Hour)
	m.ProcessedMessages["old2"] = now.Add(-36 * time.Hour)
	m.ProcessedMessages["recent"] = now.Add(-1 * time.Hour)

	// Prune entries older than 24 hours
	removed := m.Prune(24 * time.Hour)

	if removed != 2 {
		t.Errorf("expected 2 removed, got %d", removed)
	}

	if m.Count() != 1 {
		t.Errorf("expected 1 remaining message, got %d", m.Count())
	}

	if !m.IsProcessed("recent") {
		t.Error("recent message should still be processed")
	}

	if m.IsProcessed("old1") || m.IsProcessed("old2") {
		t.Error("old messages should be pruned")
	}
}

func TestCount(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.json")

	m, err := New(stateFile, testLogger())
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	if m.Count() != 0 {
		t.Errorf("expected 0, got %d", m.Count())
	}

	m.ProcessedMessages["msg1"] = time.Now()
	m.ProcessedMessages["msg2"] = time.Now()
	m.ProcessedMessages["msg3"] = time.Now()

	if m.Count() != 3 {
		t.Errorf("expected 3, got %d", m.Count())
	}
}

func TestConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.json")

	m, err := New(stateFile, testLogger())
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	const goroutines = 50
	const opsPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	// Concurrent writes
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				msgKey := fmt.Sprintf("msg-%d-%d", id, j)
				_ = m.MarkProcessed(msgKey)
			}
		}(i)
	}

	wg.Wait()

	// Verify all messages were recorded
	expectedCount := goroutines * opsPerGoroutine
	actualCount := m.Count()

	if actualCount != expectedCount {
		t.Errorf("expected %d processed messages, got %d", expectedCount, actualCount)
	}

	// Concurrent reads and writes
	wg.Add(goroutines * 2)

	for i := 0; i < goroutines; i++ {
		// Readers
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				msgKey := fmt.Sprintf("msg-%d-%d", id, j)
				m.IsProcessed(msgKey)
			}
		}(i)

		// Writers
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				msgKey := fmt.Sprintf("new-msg-%d-%d", id, j)
				_ = m.MarkProcessed(msgKey)
			}
		}(i)
	}

	wg.Wait()

	// Should have both old and new messages
	finalCount := m.Count()
	if finalCount != expectedCount*2 {
		t.Errorf("expected %d total messages, got %d", expectedCount*2, finalCount)
	}
}

func TestAtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.json")

	m, err := New(stateFile, testLogger())
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Mark processed (which triggers save)
	if err := m.MarkProcessed("msg1"); err != nil {
		t.Fatalf("MarkProcessed failed: %v", err)
	}

	// Verify temp file is cleaned up
	tmpFile := stateFile + ".tmp"
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Error("temporary file should be cleaned up after save")
	}

	// Verify state file exists
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Error("state file should exist after save")
	}
}
