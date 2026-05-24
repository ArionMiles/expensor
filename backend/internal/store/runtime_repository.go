package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	readerRuntimeClientSecret = "client_secret"
	readerRuntimeOAuthToken   = "oauth_token"
	readerRuntimeConfig       = "config"
)

type RuntimeRepository interface {
	InitAppConfig(ctx context.Context) error
	GetAppConfig(ctx context.Context, key string) (string, error)
	SetAppConfig(ctx context.Context, key, value string) error
	SetActiveReader(ctx context.Context, reader string) error
	GetActiveReader(ctx context.Context) (string, error)
	SetReaderSecret(ctx context.Context, reader string, secret []byte) error
	GetReaderSecret(ctx context.Context, reader string) ([]byte, bool, error)
	SetReaderToken(ctx context.Context, reader string, token []byte) error
	GetReaderToken(ctx context.Context, reader string) ([]byte, bool, error)
	DeleteReaderToken(ctx context.Context, reader string) error
	SetReaderConfig(ctx context.Context, reader string, readerConfig json.RawMessage) error
	GetReaderConfig(ctx context.Context, reader string) (json.RawMessage, bool, error)
	DeleteReaderRuntime(ctx context.Context, reader string) error
	IsMessageProcessed(ctx context.Context, key string) (bool, error)
	MarkMessageProcessed(ctx context.Context, key string, at time.Time) error
	GetSyncStatus(ctx context.Context) (SyncStatus, error)
	SetSyncStatus(ctx context.Context, status SyncStatus) error
	GetCommunityURL(ctx context.Context) (string, error)
	SetCommunityURL(ctx context.Context, url string) error
}

type pgRuntimeRepository struct {
	pool *pgxpool.Pool
}

func NewRuntimeRepository(deps repositoryDependencies) RuntimeRepository {
	return &pgRuntimeRepository{
		pool: deps.pool,
	}
}

func (r *pgRuntimeRepository) InitAppConfig(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
			CREATE TABLE IF NOT EXISTS app_config (
			    key   TEXT PRIMARY KEY,
			    value TEXT NOT NULL
			);
			INSERT INTO app_config (key, value) VALUES
			    ('scan_interval', '60'),
			    ('lookback_days', '180')
			ON CONFLICT (key) DO NOTHING;
		`)
	if err != nil {
		return fmt.Errorf("initializing app config: executing app config initialization: %w", err)
	}
	return nil
}

func (r *pgRuntimeRepository) GetAppConfig(ctx context.Context, key string) (string, error) {
	var value string
	if err := r.pool.QueryRow(ctx, `SELECT value FROM app_config WHERE key = $1`, key).Scan(&value); err != nil {
		return "", fmt.Errorf("getting app config %q: %w", key, err)
	}
	return value, nil
}

func (r *pgRuntimeRepository) SetAppConfig(ctx context.Context, key, value string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO app_config (key, value) VALUES ($1, $2)
			 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("setting app config %q: %w", key, err)
	}
	return nil
}

func (r *pgRuntimeRepository) SetActiveReader(ctx context.Context, reader string) error {
	if strings.TrimSpace(reader) == "" {
		return errors.New("active reader cannot be blank")
	}
	return r.writeAppConfig(ctx, "active_reader", reader)
}

func (r *pgRuntimeRepository) GetActiveReader(ctx context.Context) (string, error) {
	value, err := r.readAppConfig(ctx, "active_reader")
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return value, nil
}

func (r *pgRuntimeRepository) SetReaderSecret(ctx context.Context, reader string, secret []byte) error {
	return r.setReaderJSON(ctx, reader, readerRuntimeClientSecret, secret)
}

func (r *pgRuntimeRepository) GetReaderSecret(ctx context.Context, reader string) (secret []byte, found bool, err error) {
	return r.getReaderJSON(ctx, reader, readerRuntimeClientSecret)
}

func (r *pgRuntimeRepository) SetReaderToken(ctx context.Context, reader string, token []byte) error {
	return r.setReaderJSON(ctx, reader, readerRuntimeOAuthToken, token)
}

func (r *pgRuntimeRepository) GetReaderToken(ctx context.Context, reader string) (token []byte, found bool, err error) {
	return r.getReaderJSON(ctx, reader, readerRuntimeOAuthToken)
}

func (r *pgRuntimeRepository) DeleteReaderToken(ctx context.Context, reader string) error {
	if strings.TrimSpace(reader) == "" {
		return errors.New("reader cannot be blank")
	}
	_, err := r.pool.Exec(ctx, `UPDATE reader_runtime SET oauth_token = NULL, updated_at = NOW() WHERE reader = $1`, reader)
	if err != nil {
		return fmt.Errorf("deleting reader token for %q: %w", reader, err)
	}
	return nil
}

func (r *pgRuntimeRepository) SetReaderConfig(ctx context.Context, reader string, readerConfig json.RawMessage) error {
	return r.setReaderJSON(ctx, reader, readerRuntimeConfig, readerConfig)
}

func (r *pgRuntimeRepository) GetReaderConfig(ctx context.Context, reader string) (json.RawMessage, bool, error) {
	value, ok, err := r.getReaderJSON(ctx, reader, readerRuntimeConfig)
	return json.RawMessage(value), ok, err
}

func (r *pgRuntimeRepository) DeleteReaderRuntime(ctx context.Context, reader string) error {
	if strings.TrimSpace(reader) == "" {
		return errors.New("reader cannot be blank")
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM reader_runtime WHERE reader = $1`, reader)
	if err != nil {
		return fmt.Errorf("deleting reader runtime for %q: %w", reader, err)
	}
	return nil
}

func (r *pgRuntimeRepository) IsMessageProcessed(ctx context.Context, key string) (bool, error) {
	if strings.TrimSpace(key) == "" {
		return false, nil
	}
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM processed_messages WHERE message_key = $1)`, key).Scan(&exists); err != nil {
		return false, fmt.Errorf("checking processed message %q: %w", key, err)
	}
	return exists, nil
}

func (r *pgRuntimeRepository) MarkMessageProcessed(ctx context.Context, key string, at time.Time) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("message key cannot be blank")
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO processed_messages (message_key, processed_at) VALUES ($1, $2)
			 ON CONFLICT (message_key) DO UPDATE SET processed_at = EXCLUDED.processed_at`,
		key, at,
	)
	if err != nil {
		return fmt.Errorf("marking processed message %q: %w", key, err)
	}
	return nil
}

func (r *pgRuntimeRepository) GetSyncStatus(ctx context.Context) (SyncStatus, error) {
	var status SyncStatus
	val, err := r.readAppConfig(ctx, "content_sync_status")
	if err != nil {
		return status, nil //nolint:nilerr // key-not-found on first run is expected; zero value means "never synced"
	}
	if err := json.Unmarshal([]byte(val), &status); err != nil {
		return SyncStatus{}, fmt.Errorf("parsing sync status: %w", err)
	}
	return status, nil
}

func (r *pgRuntimeRepository) SetSyncStatus(ctx context.Context, status SyncStatus) error {
	b, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("marshaling sync status: %w", err)
	}
	return r.writeAppConfig(ctx, "content_sync_status", string(b))
}

func (r *pgRuntimeRepository) GetCommunityURL(ctx context.Context) (string, error) {
	url, err := r.readAppConfig(ctx, "community_content_url")
	if err != nil {
		return "", err
	}
	return url, nil
}

func (r *pgRuntimeRepository) SetCommunityURL(ctx context.Context, url string) error {
	return r.writeAppConfig(ctx, "community_content_url", url)
}

func (r *pgRuntimeRepository) setReaderJSON(ctx context.Context, reader, column string, value []byte) error {
	if strings.TrimSpace(reader) == "" {
		return errors.New("reader cannot be blank")
	}
	if !json.Valid(value) {
		return fmt.Errorf("%s for reader %q must be valid JSON", column, reader)
	}
	query, err := runtimeSetReaderJSONQuery(column)
	if err != nil {
		return err
	}
	if _, err := r.pool.Exec(ctx, query, reader, value); err != nil {
		return fmt.Errorf("setting %s for reader %q: %w", column, reader, err)
	}
	return nil
}

func (r *pgRuntimeRepository) getReaderJSON(ctx context.Context, reader, column string) ([]byte, bool, error) {
	var value []byte
	found := false
	if strings.TrimSpace(reader) == "" {
		return nil, false, errors.New("reader cannot be blank")
	}
	query, err := runtimeGetReaderJSONQuery(column)
	if err != nil {
		return nil, false, err
	}
	err = r.pool.QueryRow(ctx, query, reader).Scan(&value)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("getting %s for reader %q: %w", column, reader, err)
	}
	found = true
	return value, found, nil
}

func (r *pgRuntimeRepository) readAppConfig(ctx context.Context, key string) (string, error) {
	var value string
	err := r.pool.QueryRow(ctx, `SELECT value FROM app_config WHERE key = $1`, key).Scan(&value)
	if err != nil {
		return "", fmt.Errorf("getting app config %q: %w", key, err)
	}
	return value, nil
}

func (r *pgRuntimeRepository) writeAppConfig(ctx context.Context, key, value string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO app_config (key, value) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("setting app config %q: %w", key, err)
	}
	return nil
}

func runtimeSetReaderJSONQuery(column string) (string, error) {
	switch column {
	case readerRuntimeClientSecret:
		return `INSERT INTO reader_runtime (reader, client_secret) VALUES ($1, $2)
			ON CONFLICT (reader) DO UPDATE SET client_secret = EXCLUDED.client_secret, updated_at = NOW()`, nil
	case readerRuntimeOAuthToken:
		return `INSERT INTO reader_runtime (reader, oauth_token) VALUES ($1, $2)
			ON CONFLICT (reader) DO UPDATE SET oauth_token = EXCLUDED.oauth_token, updated_at = NOW()`, nil
	case readerRuntimeConfig:
		return `INSERT INTO reader_runtime (reader, config) VALUES ($1, $2)
			ON CONFLICT (reader) DO UPDATE SET config = EXCLUDED.config, updated_at = NOW()`, nil
	default:
		return "", fmt.Errorf("unsupported reader runtime column %q", column)
	}
}

func runtimeGetReaderJSONQuery(column string) (string, error) {
	switch column {
	case readerRuntimeClientSecret:
		return `SELECT client_secret FROM reader_runtime WHERE reader = $1 AND client_secret IS NOT NULL`, nil
	case readerRuntimeOAuthToken:
		return `SELECT oauth_token FROM reader_runtime WHERE reader = $1 AND oauth_token IS NOT NULL`, nil
	case readerRuntimeConfig:
		return `SELECT config FROM reader_runtime WHERE reader = $1 AND config IS NOT NULL`, nil
	default:
		return "", fmt.Errorf("unsupported reader runtime column %q", column)
	}
}
