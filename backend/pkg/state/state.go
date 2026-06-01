// Package state provides unified state management for tracking processed messages.
package state

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"
)

// Manager tracks processed messages to avoid reprocessing.
type Manager struct {
	store  ProcessedMessageStore
	logger *slog.Logger
}

// ProcessedMessageStore is the DB persistence surface used by DB-backed state managers.
type ProcessedMessageStore interface {
	IsMessageProcessed(ctx context.Context, key string) (bool, error)
	MarkMessageProcessed(ctx context.Context, key string, at time.Time) error
}

// NewDBManager creates a state manager backed by the runtime database.
func NewDBManager(store ProcessedMessageStore, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		store:  store,
		logger: logger,
	}
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
func (m *Manager) IsProcessed(ctx context.Context, msgKey string) bool {
	if m.store == nil {
		m.logger.Warn("processed message state store is nil", "key", msgKey)
		return false
	}
	processed, err := m.store.IsMessageProcessed(ctx, msgKey)
	if err != nil {
		m.logger.Warn("failed to check processed message state", "key", msgKey, "error", err)
		return false
	}
	return processed
}

// MarkProcessed marks a message as processed.
func (m *Manager) MarkProcessed(ctx context.Context, msgKey string) error {
	if m.store == nil {
		return fmt.Errorf("processed message state store is nil")
	}
	if err := m.store.MarkMessageProcessed(ctx, msgKey, time.Now()); err != nil {
		return fmt.Errorf("marking message processed in DB: %w", err)
	}
	return nil
}
