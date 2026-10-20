package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/auth"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const (
	readerRuntimeClientSecret = "client_secret"
	readerRuntimeOAuthToken   = "oauth_token"
	readerRuntimeConfig       = "config"
	llmProviderCredentials    = "credentials"
)

func (s *Store) GetAppConfig(ctx context.Context, tenant store.Tenant, key string) (string, error) {
	return s.readAppConfig(ctx, tenant, key)
}

func (s *Store) SetAppConfig(ctx context.Context, tenant store.Tenant, key, value string) error {
	return s.writeAppConfig(ctx, tenant, key, value)
}

func (s *Store) SetReaderSecret(ctx context.Context, tenant store.Tenant, reader string, secret []byte) error {
	return s.writeReaderEncryptedJSON(ctx, tenant, reader, readerRuntimeClientSecret, secret)
}

func (s *Store) GetReaderSecret(
	ctx context.Context,
	tenant store.Tenant,
	reader string,
) (value []byte, found bool, err error) {
	return s.readReaderEncryptedJSON(ctx, tenant, reader, readerRuntimeClientSecret)
}

func (s *Store) SetReaderToken(ctx context.Context, tenant store.Tenant, reader string, token []byte) error {
	return s.writeReaderEncryptedJSON(ctx, tenant, reader, readerRuntimeOAuthToken, token)
}

func (s *Store) GetReaderToken(
	ctx context.Context,
	tenant store.Tenant,
	reader string,
) (value []byte, found bool, err error) {
	return s.readReaderEncryptedJSON(ctx, tenant, reader, readerRuntimeOAuthToken)
}

func (s *Store) DeleteReaderToken(ctx context.Context, tenant store.Tenant, reader string) error {
	now := timeToText(s.repositories.now())
	return s.repositories.writeTx(ctx, func(query queryer) error {
		_, err := query.ExecContext(ctx, `
			UPDATE reader_runtime
			SET oauth_token_ciphertext = NULL, updated_at = ?
			WHERE tenant_id = ? AND reader = ?
		`, now, tenant.ID, reader)
		return mapSQLiteError("sqlite.runtime.delete_reader_token", err)
	})
}

func (s *Store) SetReaderConfig(ctx context.Context, tenant store.Tenant, reader string, config json.RawMessage) error {
	if err := validateRuntimeJSON(config, readerRuntimeConfig+` for reader "`+reader+`"`); err != nil {
		return err
	}
	now := timeToText(s.repositories.now())
	return s.repositories.writeTx(ctx, func(query queryer) error {
		_, err := query.ExecContext(ctx, `
			INSERT INTO reader_runtime (tenant_id, reader, config, updated_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT (tenant_id, reader) WHERE tenant_id IS NOT NULL
			DO UPDATE SET config = excluded.config, updated_at = excluded.updated_at
		`, tenant.ID, reader, string(config), now)
		return mapSQLiteError("sqlite.runtime.set_reader_config", err)
	})
}

func (s *Store) GetReaderConfig(
	ctx context.Context,
	tenant store.Tenant,
	reader string,
) (config json.RawMessage, found bool, err error) {
	var value string
	err = s.repositories.query.QueryRowContext(ctx, `
		SELECT config FROM reader_runtime
		WHERE tenant_id = ? AND reader = ? AND config IS NOT NULL
	`, tenant.ID, reader).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, mapSQLiteError("sqlite.runtime.get_reader_config", err)
	}
	if err := validateStoredRuntimeJSON(value); err != nil {
		return nil, false, errors.B.Op("sqlite.runtime.get_reader_config").Text("reading reader config").Err(err).Build()
	}
	return json.RawMessage(value), true, nil
}

func (s *Store) DeleteReaderRuntime(ctx context.Context, tenant store.Tenant, reader string) error {
	return s.runtimeDelete(ctx, "sqlite.runtime.delete_reader_runtime", `DELETE FROM reader_runtime WHERE tenant_id = ? AND reader = ?`, tenant.ID, reader)
}

func (s *Store) SetLLMProviderConfig(ctx context.Context, tenant store.Tenant, provider string, config json.RawMessage) error {
	if err := validateRuntimeJSON(config, `config for llm provider "`+provider+`"`); err != nil {
		return err
	}
	now := timeToText(s.repositories.now())
	return s.repositories.writeTx(ctx, func(query queryer) error {
		_, err := query.ExecContext(ctx, `
			INSERT INTO llm_provider_runtime (tenant_id, provider, config, updated_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT (tenant_id, provider) WHERE tenant_id IS NOT NULL
			DO UPDATE SET config = excluded.config, updated_at = excluded.updated_at
		`, tenant.ID, provider, string(config), now)
		return mapSQLiteError("sqlite.runtime.set_llm_provider_config", err)
	})
}

func (s *Store) GetLLMProviderConfig(
	ctx context.Context,
	tenant store.Tenant,
	provider string,
) (config json.RawMessage, found bool, err error) {
	var value string
	err = s.repositories.query.QueryRowContext(ctx, `
		SELECT config FROM llm_provider_runtime WHERE tenant_id = ? AND provider = ?
	`, tenant.ID, provider).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, mapSQLiteError("sqlite.runtime.get_llm_provider_config", err)
	}
	if err := validateStoredRuntimeJSON(value); err != nil {
		return nil, false, errors.B.Op("sqlite.runtime.get_llm_provider_config").Text("reading llm provider config").Err(err).Build()
	}
	return json.RawMessage(value), true, nil
}

func (s *Store) SetLLMProviderCredentials(ctx context.Context, tenant store.Tenant, provider string, credentials []byte) error {
	if err := validateRuntimeJSON(credentials, llmProviderCredentials+` for llm provider "`+provider+`"`); err != nil {
		return err
	}
	if s.repositories.secretBox == nil {
		return errors.B.KindFailedPrecondition().Text("store secret box is not initialized").Build()
	}
	ciphertext, err := s.repositories.secretBox.Seal(credentials, llmProviderAssociatedData(tenant, provider))
	if err != nil {
		return errors.B.Op("sqlite.runtime.set_llm_provider_credentials").Text("encrypting llm provider credentials").Err(err).Build()
	}
	now := timeToText(s.repositories.now())
	return s.repositories.writeTx(ctx, func(query queryer) error {
		_, err := query.ExecContext(ctx, `
			INSERT INTO llm_provider_runtime (tenant_id, provider, credentials_ciphertext, updated_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT (tenant_id, provider) WHERE tenant_id IS NOT NULL
			DO UPDATE SET credentials_ciphertext = excluded.credentials_ciphertext, updated_at = excluded.updated_at
		`, tenant.ID, provider, ciphertext, now)
		return mapSQLiteError("sqlite.runtime.set_llm_provider_credentials", err)
	})
}

func (s *Store) GetLLMProviderCredentials(
	ctx context.Context,
	tenant store.Tenant,
	provider string,
) (credentials []byte, found bool, err error) {
	if s.repositories.secretBox == nil {
		return nil, false, errors.B.KindFailedPrecondition().Text("store secret box is not initialized").Build()
	}
	var ciphertext []byte
	err = s.repositories.query.QueryRowContext(ctx, `
		SELECT credentials_ciphertext FROM llm_provider_runtime
		WHERE tenant_id = ? AND provider = ? AND credentials_ciphertext IS NOT NULL
	`, tenant.ID, provider).Scan(&ciphertext)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, mapSQLiteError("sqlite.runtime.get_llm_provider_credentials", err)
	}
	plaintext, err := s.repositories.secretBox.Open(ciphertext, llmProviderAssociatedData(tenant, provider))
	if err != nil {
		return nil, false, errors.B.Op("sqlite.runtime.get_llm_provider_credentials").Text("decrypting llm provider credentials").Err(err).Build()
	}
	if err := validateStoredRuntimeJSON(string(plaintext)); err != nil {
		return nil, false, errors.B.Op("sqlite.runtime.get_llm_provider_credentials").Text("reading llm provider credentials").Err(err).Build()
	}
	return plaintext, true, nil
}

func (s *Store) DeleteLLMProviderRuntime(ctx context.Context, tenant store.Tenant, provider string) error {
	return s.runtimeDelete(
		ctx,
		"sqlite.runtime.delete_llm_provider_runtime",
		`DELETE FROM llm_provider_runtime WHERE tenant_id = ? AND provider = ?`,
		tenant.ID,
		provider,
	)
}

func (s *Store) SetActiveLLMProvider(ctx context.Context, tenant store.Tenant, provider string) error {
	now := timeToText(s.repositories.now())
	return s.repositories.writeTx(ctx, func(query queryer) error {
		if _, err := query.ExecContext(
			ctx,
			`UPDATE llm_provider_runtime SET active = 0, updated_at = ? WHERE tenant_id = ? AND active = 1`,
			now,
			tenant.ID,
		); err != nil {
			return mapSQLiteError("sqlite.runtime.set_active_llm_provider.clear", err)
		}
		_, err := query.ExecContext(ctx, `
			INSERT INTO llm_provider_runtime (tenant_id, provider, active, updated_at)
			VALUES (?, ?, 1, ?)
			ON CONFLICT (tenant_id, provider) WHERE tenant_id IS NOT NULL
			DO UPDATE SET active = 1, updated_at = excluded.updated_at
		`, tenant.ID, provider, now)
		return mapSQLiteError("sqlite.runtime.set_active_llm_provider.set", err)
	})
}

func (s *Store) ClearActiveLLMProvider(ctx context.Context, tenant store.Tenant) error {
	now := timeToText(s.repositories.now())
	return s.repositories.writeTx(ctx, func(query queryer) error {
		_, err := query.ExecContext(ctx, `UPDATE llm_provider_runtime SET active = 0, updated_at = ? WHERE tenant_id = ? AND active = 1`, now, tenant.ID)
		return mapSQLiteError("sqlite.runtime.clear_active_llm_provider", err)
	})
}

func (s *Store) GetActiveLLMProviderRuntime(ctx context.Context, tenant store.Tenant) (store.LLMProviderRuntime, bool, error) {
	var runtime store.LLMProviderRuntime
	var config, createdAt, updatedAt string
	var ciphertext []byte
	err := s.repositories.query.QueryRowContext(ctx, `
		SELECT provider, config, COALESCE(credentials_ciphertext, X''),
		       credentials_ciphertext IS NOT NULL, active, created_at, updated_at
		FROM llm_provider_runtime WHERE tenant_id = ? AND active = 1
	`, tenant.ID).Scan(
		&runtime.Provider, &config, &ciphertext, &runtime.HasCredentials,
		&runtime.Active, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return store.LLMProviderRuntime{}, false, nil
	}
	if err != nil {
		return store.LLMProviderRuntime{}, false, mapSQLiteError("sqlite.runtime.get_active_llm_provider_runtime", err)
	}
	if err := validateStoredRuntimeJSON(config); err != nil {
		return store.LLMProviderRuntime{}, false, errors.B.Op("sqlite.runtime.get_active_llm_provider_runtime").Text("reading llm provider config").Err(err).Build()
	}
	runtime.Config = []byte(config)
	runtime.CreatedAt, err = timeFromText(createdAt)
	if err != nil {
		return store.LLMProviderRuntime{}, false, errors.B.Op("sqlite.runtime.get_active_llm_provider_runtime").Text("reading created timestamp").Err(err).Build()
	}
	runtime.UpdatedAt, err = timeFromText(updatedAt)
	if err != nil {
		return store.LLMProviderRuntime{}, false, errors.B.Op("sqlite.runtime.get_active_llm_provider_runtime").Text("reading updated timestamp").Err(err).Build()
	}
	if runtime.HasCredentials {
		if s.repositories.secretBox == nil {
			return store.LLMProviderRuntime{}, false, errors.B.KindFailedPrecondition().Text("store secret box is not initialized").Build()
		}
		runtime.Credentials, err = s.repositories.secretBox.Open(ciphertext, llmProviderAssociatedData(tenant, runtime.Provider))
		if err != nil {
			return store.LLMProviderRuntime{}, false, errors.B.
				Op("sqlite.runtime.get_active_llm_provider_runtime").
				Text("decrypting llm provider credentials").
				Err(err).
				Build()
		}
		if err := validateStoredRuntimeJSON(string(runtime.Credentials)); err != nil {
			return store.LLMProviderRuntime{}, false, errors.B.
				Op("sqlite.runtime.get_active_llm_provider_runtime").
				Text("reading llm provider credentials").
				Err(err).
				Build()
		}
	}
	return runtime, true, nil
}

func (s *Store) IsMessageProcessed(ctx context.Context, tenant store.Tenant, key string) (bool, error) {
	if strings.TrimSpace(key) == "" {
		return false, nil
	}
	var exists bool
	err := s.repositories.query.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM processed_messages WHERE tenant_id = ? AND message_key = ?)
	`, tenant.ID, key).Scan(&exists)
	if err != nil {
		return false, mapSQLiteError("sqlite.runtime.is_message_processed", err)
	}
	return exists, nil
}

func (s *Store) MarkMessageProcessed(ctx context.Context, tenant store.Tenant, key string, at time.Time) error {
	if strings.TrimSpace(key) == "" {
		return errors.B.KindInvalidInput().Text("message key cannot be blank").Build()
	}
	return s.repositories.writeTx(ctx, func(query queryer) error {
		_, err := query.ExecContext(ctx, `
			INSERT INTO processed_messages (tenant_id, message_key, processed_at)
			VALUES (?, ?, ?)
			ON CONFLICT (tenant_id, message_key) WHERE tenant_id IS NOT NULL
			DO UPDATE SET processed_at = excluded.processed_at
		`, tenant.ID, key, timeToText(at))
		return mapSQLiteError("sqlite.runtime.mark_message_processed", err)
	})
}

func (s *Store) GetSyncStatus(ctx context.Context) (store.SyncStatus, error) {
	value, err := s.readGlobalAppConfig(ctx, "content_sync_status")
	if errors.Is(err, sql.ErrNoRows) || errors.WhatKind(err) == errors.NotFound {
		return store.SyncStatus{}, nil
	}
	if err != nil {
		return store.SyncStatus{}, err
	}
	if err := validateStoredRuntimeJSON(value); err != nil {
		return store.SyncStatus{}, errors.B.Op("sqlite.runtime.get_sync_status").Text("reading sync status").Err(err).Build()
	}
	var status store.SyncStatus
	if err := json.Unmarshal([]byte(value), &status); err != nil {
		return store.SyncStatus{}, errors.B.Op("sqlite.runtime.get_sync_status").Text("parsing sync status").Err(err).Build()
	}
	return status, nil
}

func (s *Store) SetSyncStatus(ctx context.Context, status store.SyncStatus) error {
	value, err := json.Marshal(status)
	if err != nil {
		return errors.B.Op("sqlite.runtime.set_sync_status").KindInvalidInput().Text("marshaling sync status").Err(err).Build()
	}
	return s.writeGlobalAppConfig(ctx, "content_sync_status", string(value))
}

func (s *Store) GetCommunitySyncSettings(ctx context.Context) (store.CommunitySyncSettings, error) {
	enabled := true
	value, err := s.readGlobalAppConfig(ctx, "community_auto_sync_enabled")
	if errors.Is(err, sql.ErrNoRows) || errors.WhatKind(err) == errors.NotFound {
		return store.CommunitySyncSettings{AutomaticSyncEnabled: &enabled}, nil
	}
	if err != nil {
		return store.CommunitySyncSettings{}, err
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return store.CommunitySyncSettings{}, errors.B.Op("sqlite.runtime.get_community_sync_settings").Text("parsing community auto sync setting").Err(err).Build()
	}
	return store.CommunitySyncSettings{AutomaticSyncEnabled: &parsed}, nil
}

func (s *Store) PatchCommunitySyncSettings(ctx context.Context, patch store.CommunitySyncSettingsPatch) (store.CommunitySyncSettings, error) {
	if patch.AutomaticSyncEnabled != nil {
		if err := s.writeGlobalAppConfig(ctx, "community_auto_sync_enabled", strconv.FormatBool(*patch.AutomaticSyncEnabled)); err != nil {
			return store.CommunitySyncSettings{}, err
		}
	}
	return s.GetCommunitySyncSettings(ctx)
}

func (s *Store) GetCommunityURL(ctx context.Context) (string, error) {
	return s.readGlobalAppConfig(ctx, "community_content_url")
}

func (s *Store) SetCommunityURL(ctx context.Context, url string) error {
	return s.writeGlobalAppConfig(ctx, "community_content_url", url)
}

func (s *Store) writeReaderEncryptedJSON(ctx context.Context, tenant store.Tenant, reader, kind string, value []byte) error {
	if err := validateRuntimeJSON(value, kind+` for reader "`+reader+`"`); err != nil {
		return err
	}
	if s.repositories.secretBox == nil {
		return errors.B.KindFailedPrecondition().Text("store secret box is not initialized").Build()
	}
	ciphertext, err := s.repositories.secretBox.Seal(value, readerAssociatedData(tenant, reader, kind))
	if err != nil {
		return errors.B.Op("sqlite.runtime.write_reader_encrypted_json").Text("encrypting reader secret").Err(err).Build()
	}
	column, err := readerCiphertextColumn(kind)
	if err != nil {
		return err
	}
	now := timeToText(s.repositories.now())
	queryText := `INSERT INTO reader_runtime (tenant_id, reader, ` + column + `, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (tenant_id, reader) WHERE tenant_id IS NOT NULL
		DO UPDATE SET ` + column + ` = excluded.` + column + `, updated_at = excluded.updated_at`
	return s.repositories.writeTx(ctx, func(query queryer) error {
		_, err := query.ExecContext(ctx, queryText, tenant.ID, reader, ciphertext, now)
		return mapSQLiteError("sqlite.runtime.write_reader_encrypted_json", err)
	})
}

func (s *Store) readReaderEncryptedJSON(ctx context.Context, tenant store.Tenant, reader, kind string) ([]byte, bool, error) {
	if s.repositories.secretBox == nil {
		return nil, false, errors.B.KindFailedPrecondition().Text("store secret box is not initialized").Build()
	}
	column, err := readerCiphertextColumn(kind)
	if err != nil {
		return nil, false, err
	}
	var ciphertext []byte
	err = s.repositories.query.QueryRowContext(ctx, `SELECT `+column+` FROM reader_runtime
		WHERE tenant_id = ? AND reader = ? AND `+column+` IS NOT NULL`, tenant.ID, reader).Scan(&ciphertext)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, mapSQLiteError("sqlite.runtime.read_reader_encrypted_json", err)
	}
	plaintext, err := s.repositories.secretBox.Open(ciphertext, readerAssociatedData(tenant, reader, kind))
	if err != nil {
		return nil, false, errors.B.Op("sqlite.runtime.read_reader_encrypted_json").Text("decrypting reader secret").Err(err).Build()
	}
	if err := validateStoredRuntimeJSON(string(plaintext)); err != nil {
		return nil, false, errors.B.Op("sqlite.runtime.read_reader_encrypted_json").Text("reading reader secret").Err(err).Build()
	}
	return plaintext, true, nil
}

func (s *Store) readAppConfig(ctx context.Context, tenant store.Tenant, key string) (string, error) {
	var value string
	err := s.repositories.query.QueryRowContext(ctx, `SELECT value FROM app_config WHERE tenant_id = ? AND key = ?`, tenant.ID, key).Scan(&value)
	if err != nil {
		return "", mapSQLiteError("sqlite.runtime.read_app_config", err)
	}
	return value, nil
}

func (s *Store) writeAppConfig(ctx context.Context, tenant store.Tenant, key, value string) error {
	return s.repositories.writeTx(ctx, func(query queryer) error {
		_, err := query.ExecContext(ctx, `
			INSERT INTO app_config (tenant_id, key, value) VALUES (?, ?, ?)
			ON CONFLICT (tenant_id, key) WHERE tenant_id IS NOT NULL
			DO UPDATE SET value = excluded.value
		`, tenant.ID, key, value)
		return mapSQLiteError("sqlite.runtime.write_app_config", err)
	})
}

func (s *Store) readGlobalAppConfig(ctx context.Context, key string) (string, error) {
	var value string
	err := s.repositories.query.QueryRowContext(ctx, `SELECT value FROM app_config WHERE tenant_id IS NULL AND key = ?`, key).Scan(&value)
	if err != nil {
		return "", mapSQLiteError("sqlite.runtime.read_global_app_config", err)
	}
	return value, nil
}

func (s *Store) writeGlobalAppConfig(ctx context.Context, key, value string) error {
	return s.repositories.writeTx(ctx, func(query queryer) error {
		_, err := query.ExecContext(ctx, `
			INSERT INTO app_config (tenant_id, key, value) VALUES (NULL, ?, ?)
			ON CONFLICT (key) WHERE tenant_id IS NULL DO UPDATE SET value = excluded.value
		`, key, value)
		return mapSQLiteError("sqlite.runtime.write_global_app_config", err)
	})
}

func (s *Store) runtimeDelete(ctx context.Context, op, statement string, args ...any) error {
	return s.repositories.writeTx(ctx, func(query queryer) error {
		_, err := query.ExecContext(ctx, statement, args...)
		return mapSQLiteError(op, err)
	})
}

func validateRuntimeJSON(value []byte, name string) error {
	if !json.Valid(value) {
		return errors.B.KindInvalidInput().Text(name + " must be valid JSON").Build()
	}
	return nil
}

func validateStoredRuntimeJSON(value string) error {
	if !json.Valid([]byte(value)) {
		return errors.B.KindInternal().Text("stored JSON is invalid").Build()
	}
	return nil
}

func readerCiphertextColumn(kind string) (string, error) {
	switch kind {
	case readerRuntimeClientSecret:
		return "client_secret_ciphertext", nil
	case readerRuntimeOAuthToken:
		return "oauth_token_ciphertext", nil
	default:
		return "", errors.B.KindInternal().Textf("unsupported reader runtime kind %q", kind).Build()
	}
}

func readerAssociatedData(tenant store.Tenant, reader, kind string) auth.SecretAssociatedData {
	return auth.SecretAssociatedData{TenantID: tenant.ID, Scope: "reader", Name: reader, Kind: kind}
}

func llmProviderAssociatedData(tenant store.Tenant, provider string) auth.SecretAssociatedData {
	return auth.SecretAssociatedData{TenantID: tenant.ID, Scope: "llm_provider", Name: provider, Kind: llmProviderCredentials}
}
