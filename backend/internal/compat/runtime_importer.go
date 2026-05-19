package compat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RuntimeStore is the narrow store surface needed by the temporary legacy
// runtime importer. Keep this interface local so the importer is easy to delete.
type RuntimeStore interface {
	SetActiveReader(ctx context.Context, reader string) error
	SetReaderSecret(ctx context.Context, reader string, secret []byte) error
	SetReaderToken(ctx context.Context, reader string, token []byte) error
	SetReaderConfig(ctx context.Context, reader string, config json.RawMessage) error
	MarkMessageProcessed(ctx context.Context, key string, at time.Time) error
}

// RuntimeImporter imports legacy file-backed runtime state into PostgreSQL once
// at startup. It is intentionally isolated from normal runtime reads/writes and
// should be removed after existing installs have upgraded.
type RuntimeImporter struct {
	dataDir string
	store   RuntimeStore
	logger  *slog.Logger
}

type ImportResult struct {
	ImportedFiles int
}

func NewRuntimeImporter(dataDir string, store RuntimeStore, logger *slog.Logger) *RuntimeImporter {
	if logger == nil {
		logger = slog.Default()
	}
	return &RuntimeImporter{dataDir: dataDir, store: store, logger: logger}
}

func (i *RuntimeImporter) Import(ctx context.Context) (ImportResult, error) {
	if i.dataDir == "" {
		return ImportResult{}, nil
	}
	if i.store == nil {
		return ImportResult{}, errors.New("runtime importer store is nil")
	}

	var result ImportResult
	if err := i.importActiveReader(ctx, &result); err != nil {
		return result, err
	}
	if err := i.importReaderFiles(ctx, &result, "client_secret_", ".json", i.store.SetReaderSecret); err != nil {
		return result, err
	}
	if err := i.importReaderFiles(ctx, &result, "token_", ".json", i.store.SetReaderToken); err != nil {
		return result, err
	}
	if err := i.importConfigFiles(ctx, &result); err != nil {
		return result, err
	}
	if err := i.importProcessedMessages(ctx, &result); err != nil {
		return result, err
	}

	return result, nil
}

func (i *RuntimeImporter) importActiveReader(ctx context.Context, result *ImportResult) error {
	path := filepath.Join(i.dataDir, "active_reader")
	data, ok, err := readOptionalFile(path)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	reader := strings.TrimSpace(string(data))
	if reader == "" {
		return nil
	}
	if err := i.store.SetActiveReader(ctx, reader); err != nil {
		return fmt.Errorf("import active reader: %w", err)
	}
	result.ImportedFiles++
	return nil
}

func (i *RuntimeImporter) importReaderFiles(
	ctx context.Context,
	result *ImportResult,
	prefix string,
	suffix string,
	save func(context.Context, string, []byte) error,
) error {
	matches, err := filepath.Glob(filepath.Join(i.dataDir, prefix+"*"+suffix))
	if err != nil {
		return fmt.Errorf("glob %s files: %w", prefix, err)
	}
	for _, path := range matches {
		reader := readerFromFilename(path, prefix, suffix)
		if reader == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if !json.Valid(data) {
			return fmt.Errorf("import %s for %s: invalid JSON", prefix, reader)
		}
		if err := save(ctx, reader, data); err != nil {
			return fmt.Errorf("import %s for %s: %w", prefix, reader, err)
		}
		result.ImportedFiles++
	}
	return nil
}

func (i *RuntimeImporter) importConfigFiles(ctx context.Context, result *ImportResult) error {
	matches, err := filepath.Glob(filepath.Join(i.dataDir, "config_*.json"))
	if err != nil {
		return fmt.Errorf("glob config files: %w", err)
	}
	for _, path := range matches {
		reader := readerFromFilename(path, "config_", ".json")
		if reader == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if !json.Valid(data) {
			return fmt.Errorf("import config for %s: invalid JSON", reader)
		}
		if err := i.store.SetReaderConfig(ctx, reader, json.RawMessage(data)); err != nil {
			return fmt.Errorf("import config for %s: %w", reader, err)
		}
		result.ImportedFiles++
	}
	return nil
}

func (i *RuntimeImporter) importProcessedMessages(ctx context.Context, result *ImportResult) error {
	path := filepath.Join(i.dataDir, "state.json")
	data, ok, err := readOptionalFile(path)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	var state struct {
		ProcessedMessages map[string]time.Time `json:"processed_messages"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	for key, processedAt := range state.ProcessedMessages {
		if err := i.store.MarkMessageProcessed(ctx, key, processedAt); err != nil {
			return fmt.Errorf("import processed message %s: %w", key, err)
		}
	}
	result.ImportedFiles++
	return nil
}

func readOptionalFile(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return data, true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	return nil, false, fmt.Errorf("read %s: %w", path, err)
}

func readerFromFilename(path, prefix, suffix string) string {
	name := filepath.Base(path)
	reader := strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix)
	if reader == name {
		return ""
	}
	return strings.TrimSpace(reader)
}
