// Package state provides unified state management for tracking processed messages.
package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

// Manager tracks processed messages to avoid reprocessing.
type Manager struct {
	mu                sync.RWMutex
	ProcessedMessages map[string]time.Time `json:"processed_messages"`
	filePath          string
	logger            *slog.Logger
}

// New creates a new state manager, loading from the specified file if it exists.
func New(filePath string, logger *slog.Logger) (*Manager, error) {
	if filePath == "" {
		return nil, fmt.Errorf("state file path is empty")
	}

	m := &Manager{
		ProcessedMessages: make(map[string]time.Time),
		filePath:          filePath,
		logger:            logger,
	}

	// Load existing state if file exists
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info("creating new state file", "path", filePath)
			return m, nil
		}
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	if err := json.Unmarshal(data, m); err != nil {
		return nil, fmt.Errorf("unmarshaling state: %w", err)
	}

	if m.ProcessedMessages == nil {
		m.ProcessedMessages = make(map[string]time.Time)
	}

	logger.Info("loaded state", "path", filePath, "messages", len(m.ProcessedMessages))
	return m, nil
}

// GenerateKey creates a unique key for a message using SHA256 hash.
func GenerateKey(source, messageID, date string) string {
	h := sha256.New()
	h.Write([]byte(source))
	h.Write([]byte(messageID))
	h.Write([]byte(date))
	return hex.EncodeToString(h.Sum(nil))
}

// IsProcessed checks if a message has been processed.
func (m *Manager) IsProcessed(msgKey string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.ProcessedMessages[msgKey]
	return exists
}

// MarkProcessed marks a message as processed and persists to disk.
func (m *Manager) MarkProcessed(msgKey string) error {
	m.mu.Lock()
	m.ProcessedMessages[msgKey] = time.Now()
	m.mu.Unlock()

	return m.save()
}

// save persists the state to the file.
func (m *Manager) save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}

	// Write atomically by writing to a temp file and renaming
	tmpPath := m.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("writing state file: %w", err)
	}

	if err := os.Rename(tmpPath, m.filePath); err != nil {
		os.Remove(tmpPath) // Clean up temp file on error
		return fmt.Errorf("renaming state file: %w", err)
	}

	return nil
}

// Prune removes old entries from the state (older than the specified duration).
func (m *Manager) Prune(olderThan time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	removed := 0

	for key, timestamp := range m.ProcessedMessages {
		if timestamp.Before(cutoff) {
			delete(m.ProcessedMessages, key)
			removed++
		}
	}

	if removed > 0 {
		m.logger.Info("pruned old state entries", "removed", removed)
	}

	return removed
}

// Count returns the number of processed messages.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.ProcessedMessages)
}

// FilePath returns the path to the state file.
func (m *Manager) FilePath() string {
	return m.filePath
}
