package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArionMiles/expensor/backend/internal/auth"
)

const (
	readerRuntimeClientSecret = "client_secret"
	readerRuntimeOAuthToken   = "oauth_token"
	readerRuntimeConfig       = "config"
	llmProviderCredentials    = "credentials"
)

type pgRuntimeRepository struct {
	pool      *pgxpool.Pool
	secretBox *auth.SecretBox
}

func newPGRuntimeRepository(deps repositoryDependencies) *pgRuntimeRepository {
	return &pgRuntimeRepository{
		pool:      deps.pool,
		secretBox: deps.secretBox,
	}
}

func (r *pgRuntimeRepository) GetAppConfig(ctx context.Context, tenant Tenant, key string) (string, error) {
	var value string
	if err := r.pool.QueryRow(ctx,
		`SELECT value FROM app_config WHERE tenant_id IS NOT DISTINCT FROM $1 AND key = $2`,
		tenantIDParam(tenant), key,
	).Scan(&value); err != nil {
		return "", fmt.Errorf("getting app config %q: %w", key, err)
	}
	return value, nil
}

func (r *pgRuntimeRepository) SetAppConfig(ctx context.Context, tenant Tenant, key, value string) error {
	_, err := r.pool.Exec(ctx,
		appConfigUpsertSQL(tenant),
		tenantIDParam(tenant), key, value,
	)
	if err != nil {
		return fmt.Errorf("setting app config %q: %w", key, err)
	}
	return nil
}

func (r *pgRuntimeRepository) SetReaderSecret(ctx context.Context, tenant Tenant, reader string, secret []byte) error {
	return r.writeReaderEncryptedJSON(ctx, tenant, reader, readerRuntimeClientSecret, secret)
}

func (r *pgRuntimeRepository) GetReaderSecret(ctx context.Context, tenant Tenant, reader string) (secret []byte, found bool, err error) {
	return r.readReaderEncryptedJSON(ctx, tenant, reader, readerRuntimeClientSecret)
}

func (r *pgRuntimeRepository) SetReaderToken(ctx context.Context, tenant Tenant, reader string, token []byte) error {
	return r.writeReaderEncryptedJSON(ctx, tenant, reader, readerRuntimeOAuthToken, token)
}

func (r *pgRuntimeRepository) GetReaderToken(ctx context.Context, tenant Tenant, reader string) (token []byte, found bool, err error) {
	return r.readReaderEncryptedJSON(ctx, tenant, reader, readerRuntimeOAuthToken)
}

func (r *pgRuntimeRepository) DeleteReaderToken(ctx context.Context, tenant Tenant, reader string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE reader_runtime
		SET oauth_token = NULL, oauth_token_ciphertext = NULL, updated_at = NOW()
		WHERE tenant_id IS NOT DISTINCT FROM $1 AND reader = $2
	`, tenantIDParam(tenant), reader)
	if err != nil {
		return fmt.Errorf("deleting reader token for %q: %w", reader, err)
	}
	return nil
}

func (r *pgRuntimeRepository) SetReaderConfig(ctx context.Context, tenant Tenant, reader string, readerConfig json.RawMessage) error {
	return r.writeReaderConfigJSON(ctx, tenant, reader, readerConfig)
}

func (r *pgRuntimeRepository) GetReaderConfig(ctx context.Context, tenant Tenant, reader string) (json.RawMessage, bool, error) {
	value, ok, err := r.readReaderConfigJSON(ctx, tenant, reader)
	return json.RawMessage(value), ok, err
}

func (r *pgRuntimeRepository) SetLLMProviderConfig(ctx context.Context, tenant Tenant, provider string, config json.RawMessage) error {
	return r.writeLLMProviderConfigJSON(ctx, tenant, provider, config)
}

func (r *pgRuntimeRepository) GetLLMProviderConfig(ctx context.Context, tenant Tenant, provider string) (json.RawMessage, bool, error) {
	value, ok, err := r.readLLMProviderConfigJSON(ctx, tenant, provider)
	return json.RawMessage(value), ok, err
}

func (r *pgRuntimeRepository) SetLLMProviderCredentials(ctx context.Context, tenant Tenant, provider string, credentials []byte) error {
	return r.writeLLMProviderEncryptedJSON(ctx, tenant, provider, credentials)
}

func (r *pgRuntimeRepository) GetLLMProviderCredentials(ctx context.Context, tenant Tenant, provider string) (credentials []byte, found bool, err error) {
	return r.readLLMProviderEncryptedJSON(ctx, tenant, provider)
}

func (r *pgRuntimeRepository) DeleteLLMProviderRuntime(ctx context.Context, tenant Tenant, provider string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM llm_provider_runtime WHERE tenant_id IS NOT DISTINCT FROM $1 AND provider = $2`, tenantIDParam(tenant), provider)
	if err != nil {
		return fmt.Errorf("deleting llm provider runtime for %q: %w", provider, err)
	}
	return nil
}

func (r *pgRuntimeRepository) SetActiveLLMProvider(ctx context.Context, tenant Tenant, provider string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting llm provider activation transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	if _, err := tx.Exec(ctx,
		`UPDATE llm_provider_runtime SET active = false, updated_at = NOW() WHERE tenant_id IS NOT DISTINCT FROM $1 AND active = true`,
		tenantIDParam(tenant),
	); err != nil {
		return fmt.Errorf("clearing active llm provider: %w", err)
	}
	query := llmProviderRuntimeUpsertActiveTenantQuery
	if strings.TrimSpace(tenant.ID) == "" {
		query = llmProviderRuntimeUpsertActiveLegacyQuery
	}
	if _, err := tx.Exec(ctx, query, tenantIDParam(tenant), provider); err != nil {
		return fmt.Errorf("setting active llm provider %q: %w", provider, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing llm provider activation: %w", err)
	}
	return nil
}

func (r *pgRuntimeRepository) ClearActiveLLMProvider(ctx context.Context, tenant Tenant) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE llm_provider_runtime SET active = false, updated_at = NOW() WHERE tenant_id IS NOT DISTINCT FROM $1 AND active = true`,
		tenantIDParam(tenant),
	)
	if err != nil {
		return fmt.Errorf("clearing active llm provider: %w", err)
	}
	return nil
}

func (r *pgRuntimeRepository) GetActiveLLMProviderRuntime(ctx context.Context, tenant Tenant) (LLMProviderRuntime, bool, error) {
	var runtime LLMProviderRuntime
	var credentialsCiphertext []byte
	err := r.pool.QueryRow(ctx, `
		SELECT provider, COALESCE(config, '{}'::jsonb), COALESCE(credentials_ciphertext, '\x'::bytea),
		       credentials_ciphertext IS NOT NULL, active, created_at, updated_at
		FROM llm_provider_runtime
		WHERE tenant_id IS NOT DISTINCT FROM $1 AND active = true
	`, tenantIDParam(tenant)).Scan(
		&runtime.Provider, &runtime.Config, &credentialsCiphertext, &runtime.HasCredentials,
		&runtime.Active, &runtime.CreatedAt, &runtime.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LLMProviderRuntime{}, false, nil
		}
		return LLMProviderRuntime{}, false, fmt.Errorf("getting active llm provider: %w", err)
	}
	if runtime.HasCredentials {
		if r.secretBox == nil {
			return LLMProviderRuntime{}, false, errors.New("store secret box is not initialized")
		}
		credentials, err := r.secretBox.Open(credentialsCiphertext, llmProviderAssociatedData(tenant, runtime.Provider))
		if err != nil {
			return LLMProviderRuntime{}, false, fmt.Errorf("decrypting llm provider %q credentials: %w", runtime.Provider, err)
		}
		runtime.Credentials = credentials
	}
	return runtime, true, nil
}

func (r *pgRuntimeRepository) DeleteReaderRuntime(ctx context.Context, tenant Tenant, reader string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM reader_runtime WHERE tenant_id IS NOT DISTINCT FROM $1 AND reader = $2`, tenantIDParam(tenant), reader)
	if err != nil {
		return fmt.Errorf("deleting reader runtime for %q: %w", reader, err)
	}
	return nil
}

func (r *pgRuntimeRepository) IsMessageProcessed(ctx context.Context, tenant Tenant, key string) (bool, error) {
	if strings.TrimSpace(key) == "" {
		return false, nil
	}
	var exists bool
	if err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM processed_messages
			WHERE tenant_id IS NOT DISTINCT FROM $1 AND message_key = $2
		)
	`, tenantIDParam(tenant), key).Scan(&exists); err != nil {
		return false, fmt.Errorf("checking processed message %q: %w", key, err)
	}
	return exists, nil
}

func (r *pgRuntimeRepository) MarkMessageProcessed(ctx context.Context, tenant Tenant, key string, at time.Time) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("message key cannot be blank")
	}
	query := `
		INSERT INTO processed_messages (tenant_id, message_key, processed_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id, message_key) WHERE tenant_id IS NOT NULL
		DO UPDATE SET processed_at = EXCLUDED.processed_at
	`
	if strings.TrimSpace(tenant.ID) == "" {
		query = `
			INSERT INTO processed_messages (tenant_id, message_key, processed_at)
			VALUES ($1, $2, $3)
			ON CONFLICT (message_key) WHERE tenant_id IS NULL
			DO UPDATE SET processed_at = EXCLUDED.processed_at
		`
	}
	_, err := r.pool.Exec(ctx, query, tenantIDParam(tenant), key, at)
	if err != nil {
		return fmt.Errorf("marking processed message %q: %w", key, err)
	}
	return nil
}

func (r *pgRuntimeRepository) GetSyncStatus(ctx context.Context) (SyncStatus, error) {
	var status SyncStatus
	val, err := r.readAppConfig(ctx, Tenant{}, "content_sync_status")
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
	return r.writeAppConfig(ctx, Tenant{}, "content_sync_status", string(b))
}

func (r *pgRuntimeRepository) GetCommunitySyncSettings(ctx context.Context) (CommunitySyncSettings, error) {
	enabled := true
	value, err := r.readAppConfig(ctx, Tenant{}, "community_auto_sync_enabled")
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CommunitySyncSettings{AutomaticSyncEnabled: &enabled}, nil
		}
		return CommunitySyncSettings{}, err
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return CommunitySyncSettings{}, fmt.Errorf("parsing community auto sync setting: %w", err)
	}
	return CommunitySyncSettings{AutomaticSyncEnabled: &parsed}, nil
}

func (r *pgRuntimeRepository) PatchCommunitySyncSettings(
	ctx context.Context,
	patch CommunitySyncSettingsPatch,
) (CommunitySyncSettings, error) {
	if patch.AutomaticSyncEnabled != nil {
		if err := r.writeAppConfig(ctx, Tenant{}, "community_auto_sync_enabled", strconv.FormatBool(*patch.AutomaticSyncEnabled)); err != nil {
			return CommunitySyncSettings{}, err
		}
	}
	return r.GetCommunitySyncSettings(ctx)
}

func (r *pgRuntimeRepository) GetCommunityURL(ctx context.Context) (string, error) {
	url, err := r.readAppConfig(ctx, Tenant{}, "community_content_url")
	if err != nil {
		return "", err
	}
	return url, nil
}

func (r *pgRuntimeRepository) SetCommunityURL(ctx context.Context, url string) error {
	return r.writeAppConfig(ctx, Tenant{}, "community_content_url", url)
}

func (r *pgRuntimeRepository) writeReaderEncryptedJSON(ctx context.Context, tenant Tenant, reader, column string, value []byte) error {
	if !json.Valid(value) {
		return fmt.Errorf("%s for reader %q must be valid JSON", column, reader)
	}
	if r.secretBox == nil {
		return errors.New("store secret box is not initialized")
	}
	ciphertext, err := r.secretBox.Seal(value, auth.SecretAssociatedData{
		TenantID: strings.TrimSpace(tenant.ID),
		Scope:    "reader",
		Name:     reader,
		Kind:     column,
	})
	if err != nil {
		return fmt.Errorf("encrypting %s for reader %q: %w", column, reader, err)
	}
	query, err := runtimeSetReaderSecretQuery(tenant, column)
	if err != nil {
		return err
	}
	if _, err := r.pool.Exec(ctx, query, tenantIDParam(tenant), reader, ciphertext); err != nil {
		return fmt.Errorf("setting %s for reader %q: %w", column, reader, err)
	}
	return nil
}

func (r *pgRuntimeRepository) readReaderEncryptedJSON(ctx context.Context, tenant Tenant, reader, column string) ([]byte, bool, error) {
	var ciphertext []byte
	if r.secretBox == nil {
		return nil, false, errors.New("store secret box is not initialized")
	}
	query, err := runtimeGetReaderSecretQuery(column)
	if err != nil {
		return nil, false, err
	}
	err = r.pool.QueryRow(ctx, query, tenantIDParam(tenant), reader).Scan(&ciphertext)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("getting %s for reader %q: %w", column, reader, err)
	}
	plaintext, err := r.secretBox.Open(ciphertext, auth.SecretAssociatedData{
		TenantID: strings.TrimSpace(tenant.ID),
		Scope:    "reader",
		Name:     reader,
		Kind:     column,
	})
	if err != nil {
		return nil, false, fmt.Errorf("decrypting %s for reader %q: %w", column, reader, err)
	}
	return plaintext, true, nil
}

func (r *pgRuntimeRepository) writeReaderConfigJSON(ctx context.Context, tenant Tenant, reader string, value []byte) error {
	if !json.Valid(value) {
		return fmt.Errorf("%s for reader %q must be valid JSON", readerRuntimeConfig, reader)
	}
	query := runtimeSetReaderConfigTenantQuery
	if strings.TrimSpace(tenant.ID) == "" {
		query = runtimeSetReaderConfigLegacyQuery
	}
	if _, err := r.pool.Exec(ctx, query, tenantIDParam(tenant), reader, value); err != nil {
		return fmt.Errorf("setting %s for reader %q: %w", readerRuntimeConfig, reader, err)
	}
	return nil
}

func (r *pgRuntimeRepository) readReaderConfigJSON(ctx context.Context, tenant Tenant, reader string) ([]byte, bool, error) {
	var value []byte
	err := r.pool.QueryRow(ctx, `
		SELECT config
		FROM reader_runtime
		WHERE tenant_id IS NOT DISTINCT FROM $1 AND reader = $2 AND config IS NOT NULL
	`, tenantIDParam(tenant), reader).Scan(&value)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("getting %s for reader %q: %w", readerRuntimeConfig, reader, err)
	}
	return value, true, nil
}

func (r *pgRuntimeRepository) writeLLMProviderConfigJSON(ctx context.Context, tenant Tenant, provider string, value []byte) error {
	if !json.Valid(value) {
		return fmt.Errorf("config for llm provider %q must be valid JSON", provider)
	}
	query := llmProviderRuntimeSetConfigTenantQuery
	if strings.TrimSpace(tenant.ID) == "" {
		query = llmProviderRuntimeSetConfigLegacyQuery
	}
	if _, err := r.pool.Exec(ctx, query, tenantIDParam(tenant), provider, value); err != nil {
		return fmt.Errorf("setting config for llm provider %q: %w", provider, err)
	}
	return nil
}

func (r *pgRuntimeRepository) readLLMProviderConfigJSON(ctx context.Context, tenant Tenant, provider string) ([]byte, bool, error) {
	var value []byte
	err := r.pool.QueryRow(ctx, `
		SELECT config
		FROM llm_provider_runtime
		WHERE tenant_id IS NOT DISTINCT FROM $1 AND provider = $2 AND config IS NOT NULL
	`, tenantIDParam(tenant), provider).Scan(&value)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("getting config for llm provider %q: %w", provider, err)
	}
	return value, true, nil
}

func (r *pgRuntimeRepository) writeLLMProviderEncryptedJSON(ctx context.Context, tenant Tenant, provider string, value []byte) error {
	if !json.Valid(value) {
		return fmt.Errorf("%s for llm provider %q must be valid JSON", llmProviderCredentials, provider)
	}
	if r.secretBox == nil {
		return errors.New("store secret box is not initialized")
	}
	ciphertext, err := r.secretBox.Seal(value, llmProviderAssociatedData(tenant, provider))
	if err != nil {
		return fmt.Errorf("encrypting llm provider %q credentials: %w", provider, err)
	}
	query := llmProviderRuntimeSetCredentialsTenantQuery
	if strings.TrimSpace(tenant.ID) == "" {
		query = llmProviderRuntimeSetCredentialsLegacyQuery
	}
	if _, err := r.pool.Exec(ctx, query, tenantIDParam(tenant), provider, ciphertext); err != nil {
		return fmt.Errorf("setting credentials for llm provider %q: %w", provider, err)
	}
	return nil
}

func (r *pgRuntimeRepository) readLLMProviderEncryptedJSON(ctx context.Context, tenant Tenant, provider string) ([]byte, bool, error) {
	var ciphertext []byte
	if r.secretBox == nil {
		return nil, false, errors.New("store secret box is not initialized")
	}
	err := r.pool.QueryRow(ctx, `
		SELECT credentials_ciphertext
		FROM llm_provider_runtime
		WHERE tenant_id IS NOT DISTINCT FROM $1 AND provider = $2 AND credentials_ciphertext IS NOT NULL
	`, tenantIDParam(tenant), provider).Scan(&ciphertext)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("getting credentials for llm provider %q: %w", provider, err)
	}
	plaintext, err := r.secretBox.Open(ciphertext, llmProviderAssociatedData(tenant, provider))
	if err != nil {
		return nil, false, fmt.Errorf("decrypting llm provider %q credentials: %w", provider, err)
	}
	return plaintext, true, nil
}

func llmProviderAssociatedData(tenant Tenant, provider string) auth.SecretAssociatedData {
	return auth.SecretAssociatedData{
		TenantID: strings.TrimSpace(tenant.ID),
		Scope:    "llm_provider",
		Name:     provider,
		Kind:     llmProviderCredentials,
	}
}

func (r *pgRuntimeRepository) readAppConfig(ctx context.Context, tenant Tenant, key string) (string, error) {
	var value string
	err := r.pool.QueryRow(ctx,
		`SELECT value FROM app_config WHERE tenant_id IS NOT DISTINCT FROM $1 AND key = $2`,
		tenantIDParam(tenant), key,
	).Scan(&value)
	if err != nil {
		return "", fmt.Errorf("getting app config %q: %w", key, err)
	}
	return value, nil
}

func (r *pgRuntimeRepository) writeAppConfig(ctx context.Context, tenant Tenant, key, value string) error {
	_, err := r.pool.Exec(ctx,
		appConfigUpsertSQL(tenant),
		tenantIDParam(tenant), key, value,
	)
	if err != nil {
		return fmt.Errorf("setting app config %q: %w", key, err)
	}
	return nil
}

func appConfigUpsertSQL(tenant Tenant) string {
	if tenantIDParam(tenant) == nil {
		return `INSERT INTO app_config (tenant_id, key, value) VALUES ($1, $2, $3)
		 ON CONFLICT (key) WHERE tenant_id IS NULL DO UPDATE SET value = EXCLUDED.value`
	}
	return `INSERT INTO app_config (tenant_id, key, value) VALUES ($1, $2, $3)
		 ON CONFLICT (tenant_id, key) WHERE tenant_id IS NOT NULL DO UPDATE SET value = EXCLUDED.value`
}

func runtimeSetReaderSecretQuery(tenant Tenant, column string) (string, error) {
	legacy := strings.TrimSpace(tenant.ID) == ""
	switch column {
	case readerRuntimeClientSecret:
		if legacy {
			return runtimeSetReaderClientSecretLegacyQuery, nil
		}
		return runtimeSetReaderClientSecretTenantQuery, nil
	case readerRuntimeOAuthToken:
		if legacy {
			return runtimeSetReaderOAuthTokenLegacyQuery, nil
		}
		return runtimeSetReaderOAuthTokenTenantQuery, nil
	default:
		return "", fmt.Errorf("unsupported reader runtime column %q", column)
	}
}

func runtimeGetReaderSecretQuery(column string) (string, error) {
	switch column {
	case readerRuntimeClientSecret:
		return `
			SELECT client_secret_ciphertext
			FROM reader_runtime
			WHERE tenant_id IS NOT DISTINCT FROM $1 AND reader = $2 AND client_secret_ciphertext IS NOT NULL
		`, nil
	case readerRuntimeOAuthToken:
		return `
			SELECT oauth_token_ciphertext
			FROM reader_runtime
			WHERE tenant_id IS NOT DISTINCT FROM $1 AND reader = $2 AND oauth_token_ciphertext IS NOT NULL
		`, nil
	default:
		return "", fmt.Errorf("unsupported reader runtime column %q", column)
	}
}

const runtimeSetReaderClientSecretTenantQuery = `
	INSERT INTO reader_runtime (tenant_id, reader, client_secret_ciphertext)
	VALUES ($1, $2, $3)
	ON CONFLICT (tenant_id, reader) WHERE tenant_id IS NOT NULL
	DO UPDATE SET client_secret = NULL, client_secret_ciphertext = EXCLUDED.client_secret_ciphertext, updated_at = NOW()
`

const runtimeSetReaderClientSecretLegacyQuery = `
	INSERT INTO reader_runtime (tenant_id, reader, client_secret_ciphertext)
	VALUES ($1, $2, $3)
	ON CONFLICT (reader) WHERE tenant_id IS NULL
	DO UPDATE SET client_secret = NULL, client_secret_ciphertext = EXCLUDED.client_secret_ciphertext, updated_at = NOW()
`

const runtimeSetReaderOAuthTokenTenantQuery = `
	INSERT INTO reader_runtime (tenant_id, reader, oauth_token_ciphertext)
	VALUES ($1, $2, $3)
	ON CONFLICT (tenant_id, reader) WHERE tenant_id IS NOT NULL
	DO UPDATE SET oauth_token = NULL, oauth_token_ciphertext = EXCLUDED.oauth_token_ciphertext, updated_at = NOW()
`

const runtimeSetReaderOAuthTokenLegacyQuery = `
	INSERT INTO reader_runtime (tenant_id, reader, oauth_token_ciphertext)
	VALUES ($1, $2, $3)
	ON CONFLICT (reader) WHERE tenant_id IS NULL
	DO UPDATE SET oauth_token = NULL, oauth_token_ciphertext = EXCLUDED.oauth_token_ciphertext, updated_at = NOW()
`

const runtimeSetReaderConfigTenantQuery = `
	INSERT INTO reader_runtime (tenant_id, reader, config)
	VALUES ($1, $2, $3)
	ON CONFLICT (tenant_id, reader) WHERE tenant_id IS NOT NULL
	DO UPDATE SET config = EXCLUDED.config, updated_at = NOW()
`

const runtimeSetReaderConfigLegacyQuery = `
	INSERT INTO reader_runtime (tenant_id, reader, config)
	VALUES ($1, $2, $3)
	ON CONFLICT (reader) WHERE tenant_id IS NULL
	DO UPDATE SET config = EXCLUDED.config, updated_at = NOW()
`

const llmProviderRuntimeSetConfigTenantQuery = `
	INSERT INTO llm_provider_runtime (tenant_id, provider, config)
	VALUES ($1, $2, $3)
	ON CONFLICT (tenant_id, provider) WHERE tenant_id IS NOT NULL
	DO UPDATE SET config = EXCLUDED.config, updated_at = NOW()
`

const llmProviderRuntimeSetConfigLegacyQuery = `
	INSERT INTO llm_provider_runtime (tenant_id, provider, config)
	VALUES ($1, $2, $3)
	ON CONFLICT (provider) WHERE tenant_id IS NULL
	DO UPDATE SET config = EXCLUDED.config, updated_at = NOW()
`

const llmProviderRuntimeSetCredentialsTenantQuery = `
	INSERT INTO llm_provider_runtime (tenant_id, provider, credentials_ciphertext)
	VALUES ($1, $2, $3)
	ON CONFLICT (tenant_id, provider) WHERE tenant_id IS NOT NULL
	DO UPDATE SET credentials_ciphertext = EXCLUDED.credentials_ciphertext, updated_at = NOW()
`

const llmProviderRuntimeSetCredentialsLegacyQuery = `
	INSERT INTO llm_provider_runtime (tenant_id, provider, credentials_ciphertext)
	VALUES ($1, $2, $3)
	ON CONFLICT (provider) WHERE tenant_id IS NULL
	DO UPDATE SET credentials_ciphertext = EXCLUDED.credentials_ciphertext, updated_at = NOW()
`

const llmProviderRuntimeUpsertActiveTenantQuery = `
	INSERT INTO llm_provider_runtime (tenant_id, provider, active)
	VALUES ($1, $2, true)
	ON CONFLICT (tenant_id, provider) WHERE tenant_id IS NOT NULL
	DO UPDATE SET active = true, updated_at = NOW()
`

const llmProviderRuntimeUpsertActiveLegacyQuery = `
	INSERT INTO llm_provider_runtime (tenant_id, provider, active)
	VALUES ($1, $2, true)
	ON CONFLICT (provider) WHERE tenant_id IS NULL
	DO UPDATE SET active = true, updated_at = NOW()
`
