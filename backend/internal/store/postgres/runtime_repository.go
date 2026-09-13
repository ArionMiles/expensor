package postgres

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArionMiles/expensor/backend/internal/auth"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const (
	readerRuntimeClientSecret = "client_secret"
	readerRuntimeOAuthToken   = "oauth_token"
	readerRuntimeConfig       = "config"
	llmProviderCredentials    = "credentials"

	appConfigUpsertSQL = `
		INSERT INTO app_config (tenant_id, key, value)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id, key) WHERE tenant_id IS NOT NULL
		DO UPDATE SET value = EXCLUDED.value
	`
)

type runtimeRepository struct {
	pool      *pgxpool.Pool
	secretBox *auth.SecretBox
}

func newRuntimeRepository(deps repositoryDependencies) *runtimeRepository {
	return &runtimeRepository{
		pool:      deps.pool,
		secretBox: deps.secretBox,
	}
}

func (r *runtimeRepository) GetAppConfig(ctx context.Context, tenant store.Tenant, key string) (string, error) {
	var value string
	if err := r.pool.QueryRow(ctx,
		`SELECT value FROM app_config WHERE tenant_id = $1 AND key = $2`,
		tenant.ID, key,
	).Scan(&value); err != nil {
		return "", errors.B.Op("postgres.runtime.get_app_config").Textf("getting app config %q", key).Err(err).Build()
	}
	return value, nil
}

func (r *runtimeRepository) SetAppConfig(ctx context.Context, tenant store.Tenant, key, value string) error {
	_, err := r.pool.Exec(ctx,
		appConfigUpsertSQL,
		tenant.ID, key, value,
	)
	if err != nil {
		return errors.B.Op("postgres.runtime.set_app_config").Textf("setting app config %q", key).Err(err).Build()
	}
	return nil
}

func (r *runtimeRepository) SetReaderSecret(ctx context.Context, tenant store.Tenant, reader string, secret []byte) error {
	return r.writeReaderEncryptedJSON(ctx, tenant, reader, readerRuntimeClientSecret, secret)
}

func (r *runtimeRepository) GetReaderSecret(ctx context.Context, tenant store.Tenant, reader string) (secret []byte, found bool, err error) {
	return r.readReaderEncryptedJSON(ctx, tenant, reader, readerRuntimeClientSecret)
}

func (r *runtimeRepository) SetReaderToken(ctx context.Context, tenant store.Tenant, reader string, token []byte) error {
	return r.writeReaderEncryptedJSON(ctx, tenant, reader, readerRuntimeOAuthToken, token)
}

func (r *runtimeRepository) GetReaderToken(ctx context.Context, tenant store.Tenant, reader string) (token []byte, found bool, err error) {
	return r.readReaderEncryptedJSON(ctx, tenant, reader, readerRuntimeOAuthToken)
}

func (r *runtimeRepository) DeleteReaderToken(ctx context.Context, tenant store.Tenant, reader string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE reader_runtime
		SET oauth_token = NULL, oauth_token_ciphertext = NULL, updated_at = NOW()
		WHERE tenant_id = $1 AND reader = $2
	`, tenant.ID, reader)
	if err != nil {
		return errors.B.Op("postgres.runtime.delete_reader_token").Textf("deleting reader token for %q", reader).Err(err).Build()
	}
	return nil
}

func (r *runtimeRepository) SetReaderConfig(ctx context.Context, tenant store.Tenant, reader string, readerConfig json.RawMessage) error {
	return r.writeReaderConfigJSON(ctx, tenant, reader, readerConfig)
}

func (r *runtimeRepository) GetReaderConfig(
	ctx context.Context,
	tenant store.Tenant,
	reader string,
) (config json.RawMessage, ok bool, err error) {
	value, ok, err := r.readReaderConfigJSON(ctx, tenant, reader)
	return json.RawMessage(value), ok, err
}

func (r *runtimeRepository) SetLLMProviderConfig(ctx context.Context, tenant store.Tenant, provider string, config json.RawMessage) error {
	return r.writeLLMProviderConfigJSON(ctx, tenant, provider, config)
}

func (r *runtimeRepository) GetLLMProviderConfig(
	ctx context.Context,
	tenant store.Tenant,
	provider string,
) (config json.RawMessage, ok bool, err error) {
	value, ok, err := r.readLLMProviderConfigJSON(ctx, tenant, provider)
	return json.RawMessage(value), ok, err
}

func (r *runtimeRepository) SetLLMProviderCredentials(ctx context.Context, tenant store.Tenant, provider string, credentials []byte) error {
	return r.writeLLMProviderEncryptedJSON(ctx, tenant, provider, credentials)
}

func (r *runtimeRepository) GetLLMProviderCredentials(ctx context.Context, tenant store.Tenant, provider string) (credentials []byte, found bool, err error) {
	return r.readLLMProviderEncryptedJSON(ctx, tenant, provider)
}

func (r *runtimeRepository) DeleteLLMProviderRuntime(ctx context.Context, tenant store.Tenant, provider string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM llm_provider_runtime WHERE tenant_id = $1 AND provider = $2`, tenant.ID, provider)
	if err != nil {
		return errors.B.Op("postgres.runtime.delete_llm_provider_runtime").Textf("deleting llm provider runtime for %q", provider).Err(err).Build()
	}
	return nil
}

func (r *runtimeRepository) SetActiveLLMProvider(ctx context.Context, tenant store.Tenant, provider string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return errors.B.Op("postgres.runtime.set_active_llm_provider").Text("starting llm provider activation transaction").Err(err).Build()
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	if _, err := tx.Exec(ctx,
		`UPDATE llm_provider_runtime SET active = false, updated_at = NOW() WHERE tenant_id = $1 AND active = true`,
		tenant.ID,
	); err != nil {
		return errors.B.Op("postgres.runtime.set_active_llm_provider").Text("clearing active llm provider").Err(err).Build()
	}
	if _, err := tx.Exec(ctx, llmProviderRuntimeUpsertActiveSQL, tenant.ID, provider); err != nil {
		return errors.B.Op("postgres.runtime.set_active_llm_provider").Textf("setting active llm provider %q", provider).Err(err).Build()
	}
	if err := tx.Commit(ctx); err != nil {
		return errors.B.Op("postgres.runtime.set_active_llm_provider").Text("committing llm provider activation").Err(err).Build()
	}
	return nil
}

func (r *runtimeRepository) ClearActiveLLMProvider(ctx context.Context, tenant store.Tenant) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE llm_provider_runtime SET active = false, updated_at = NOW() WHERE tenant_id = $1 AND active = true`,
		tenant.ID,
	)
	if err != nil {
		return errors.B.Op("postgres.runtime.clear_active_llm_provider").Text("clearing active llm provider").Err(err).Build()
	}
	return nil
}

func (r *runtimeRepository) GetActiveLLMProviderRuntime(ctx context.Context, tenant store.Tenant) (runtime store.LLMProviderRuntime, found bool, err error) {
	var credentialsCiphertext []byte
	err = r.pool.QueryRow(ctx, `
		SELECT provider, COALESCE(config, '{}'::jsonb), COALESCE(credentials_ciphertext, '\x'::bytea),
		       credentials_ciphertext IS NOT NULL, active, created_at, updated_at
		FROM llm_provider_runtime
		WHERE tenant_id = $1 AND active = true
	`, tenant.ID).Scan(
		&runtime.Provider, &runtime.Config, &credentialsCiphertext, &runtime.HasCredentials,
		&runtime.Active, &runtime.CreatedAt, &runtime.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.LLMProviderRuntime{}, false, nil
		}
		return store.LLMProviderRuntime{}, false, errors.B.Op("postgres.runtime.get_active_llm_provider_runtime").Text("getting active llm provider").Err(err).Build()
	}
	if runtime.HasCredentials {
		if r.secretBox == nil {
			return store.LLMProviderRuntime{}, false, errors.B.KindFailedPrecondition().Text("store secret box is not initialized").Build()
		}
		credentials, err := r.secretBox.Open(credentialsCiphertext, llmProviderAssociatedData(tenant, runtime.Provider))
		if err != nil {
			return store.LLMProviderRuntime{}, false, errors.B.
				Op("postgres.runtime.get_active_llm_provider_runtime").
				Textf("decrypting llm provider %q credentials", runtime.Provider).
				Err(err).
				Build()
		}
		runtime.Credentials = credentials
	}
	return runtime, true, nil
}

func (r *runtimeRepository) DeleteReaderRuntime(ctx context.Context, tenant store.Tenant, reader string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM reader_runtime WHERE tenant_id = $1 AND reader = $2`, tenant.ID, reader)
	if err != nil {
		return errors.B.Op("postgres.runtime.delete_reader_runtime").Textf("deleting reader runtime for %q", reader).Err(err).Build()
	}
	return nil
}

func (r *runtimeRepository) IsMessageProcessed(ctx context.Context, tenant store.Tenant, key string) (bool, error) {
	if strings.TrimSpace(key) == "" {
		return false, nil
	}
	var exists bool
	if err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM processed_messages
			WHERE tenant_id = $1 AND message_key = $2
		)
	`, tenant.ID, key).Scan(&exists); err != nil {
		return false, errors.B.Op("postgres.runtime.is_message_processed").Textf("checking processed message %q", key).Err(err).Build()
	}
	return exists, nil
}

func (r *runtimeRepository) MarkMessageProcessed(ctx context.Context, tenant store.Tenant, key string, at time.Time) error {
	if strings.TrimSpace(key) == "" {
		return errors.B.KindInvalidInput().Text("message key cannot be blank").Build()
	}
	const query = `
		INSERT INTO processed_messages (tenant_id, message_key, processed_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id, message_key) WHERE tenant_id IS NOT NULL
		DO UPDATE SET processed_at = EXCLUDED.processed_at
	`
	_, err := r.pool.Exec(ctx, query, tenant.ID, key, at)
	if err != nil {
		return errors.B.Op("postgres.runtime.mark_message_processed").Textf("marking processed message %q", key).Err(err).Build()
	}
	return nil
}

func (r *runtimeRepository) GetSyncStatus(ctx context.Context) (store.SyncStatus, error) {
	var status store.SyncStatus
	val, err := r.readGlobalAppConfig(ctx, "content_sync_status")
	if err != nil {
		return status, nil //nolint:nilerr // key-not-found on first run is expected; zero value means "never synced"
	}
	if err := json.Unmarshal([]byte(val), &status); err != nil {
		return store.SyncStatus{}, errors.B.Op("postgres.runtime.get_sync_status").Text("parsing sync status").Err(err).Build()
	}
	return status, nil
}

func (r *runtimeRepository) SetSyncStatus(ctx context.Context, status store.SyncStatus) error {
	b, err := json.Marshal(status)
	if err != nil {
		return errors.B.Op("postgres.runtime.set_sync_status").Text("marshaling sync status").Err(err).Build()
	}
	return r.writeGlobalAppConfig(ctx, "content_sync_status", string(b))
}

func (r *runtimeRepository) GetCommunitySyncSettings(ctx context.Context) (store.CommunitySyncSettings, error) {
	enabled := true
	value, err := r.readGlobalAppConfig(ctx, "community_auto_sync_enabled")
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.CommunitySyncSettings{AutomaticSyncEnabled: &enabled}, nil
		}
		return store.CommunitySyncSettings{}, err
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return store.CommunitySyncSettings{}, errors.B.Op("postgres.runtime.get_community_sync_settings").Text("parsing community auto sync setting").Err(err).Build()
	}
	return store.CommunitySyncSettings{AutomaticSyncEnabled: &parsed}, nil
}

func (r *runtimeRepository) PatchCommunitySyncSettings(
	ctx context.Context,
	patch store.CommunitySyncSettingsPatch,
) (store.CommunitySyncSettings, error) {
	if patch.AutomaticSyncEnabled != nil {
		if err := r.writeGlobalAppConfig(ctx, "community_auto_sync_enabled", strconv.FormatBool(*patch.AutomaticSyncEnabled)); err != nil {
			return store.CommunitySyncSettings{}, err
		}
	}
	return r.GetCommunitySyncSettings(ctx)
}

func (r *runtimeRepository) GetCommunityURL(ctx context.Context) (string, error) {
	url, err := r.readGlobalAppConfig(ctx, "community_content_url")
	if err != nil {
		return "", err
	}
	return url, nil
}

func (r *runtimeRepository) SetCommunityURL(ctx context.Context, url string) error {
	return r.writeGlobalAppConfig(ctx, "community_content_url", url)
}

func (r *runtimeRepository) writeReaderEncryptedJSON(ctx context.Context, tenant store.Tenant, reader, column string, value []byte) error {
	if !json.Valid(value) {
		return errors.B.KindInvalidInput().Textf("%s for reader %q must be valid JSON", column, reader).Build()
	}
	if r.secretBox == nil {
		return errors.B.KindFailedPrecondition().Text("store secret box is not initialized").Build()
	}
	ciphertext, err := r.secretBox.Seal(value, auth.SecretAssociatedData{
		TenantID: tenant.ID,
		Scope:    "reader",
		Name:     reader,
		Kind:     column,
	})
	if err != nil {
		return errors.B.Op("postgres.runtime.write_reader_encrypted_json").Textf("encrypting %s for reader %q", column, reader).Err(err).Build()
	}
	query, err := runtimeSetReaderSecretQuery(column)
	if err != nil {
		return err
	}
	if _, err := r.pool.Exec(ctx, query, tenant.ID, reader, ciphertext); err != nil {
		return errors.B.Op("postgres.runtime.write_reader_encrypted_json").Textf("setting %s for reader %q", column, reader).Err(err).Build()
	}
	return nil
}

func (r *runtimeRepository) readReaderEncryptedJSON(ctx context.Context, tenant store.Tenant, reader, column string) ([]byte, bool, error) {
	var ciphertext []byte
	if r.secretBox == nil {
		return nil, false, errors.B.KindFailedPrecondition().Text("store secret box is not initialized").Build()
	}
	query, err := runtimeGetReaderSecretQuery(column)
	if err != nil {
		return nil, false, err
	}
	err = r.pool.QueryRow(ctx, query, tenant.ID, reader).Scan(&ciphertext)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, errors.B.Op("postgres.runtime.read_reader_encrypted_json").Textf("getting %s for reader %q", column, reader).Err(err).Build()
	}
	plaintext, err := r.secretBox.Open(ciphertext, auth.SecretAssociatedData{
		TenantID: tenant.ID,
		Scope:    "reader",
		Name:     reader,
		Kind:     column,
	})
	if err != nil {
		return nil, false, errors.B.Op("postgres.runtime.read_reader_encrypted_json").Textf("decrypting %s for reader %q", column, reader).Err(err).Build()
	}
	return plaintext, true, nil
}

func (r *runtimeRepository) writeReaderConfigJSON(ctx context.Context, tenant store.Tenant, reader string, value []byte) error {
	if !json.Valid(value) {
		return errors.B.KindInvalidInput().Textf("%s for reader %q must be valid JSON", readerRuntimeConfig, reader).Build()
	}
	if _, err := r.pool.Exec(ctx, runtimeSetReaderConfigSQL, tenant.ID, reader, value); err != nil {
		return errors.B.Op("postgres.runtime.write_reader_config_json").Textf("setting %s for reader %q", readerRuntimeConfig, reader).Err(err).Build()
	}
	return nil
}

func (r *runtimeRepository) readReaderConfigJSON(ctx context.Context, tenant store.Tenant, reader string) ([]byte, bool, error) {
	var value []byte
	err := r.pool.QueryRow(ctx, `
		SELECT config
		FROM reader_runtime
		WHERE tenant_id = $1 AND reader = $2 AND config IS NOT NULL
	`, tenant.ID, reader).Scan(&value)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, errors.B.Op("postgres.runtime.read_reader_config_json").Textf("getting %s for reader %q", readerRuntimeConfig, reader).Err(err).Build()
	}
	return value, true, nil
}

func (r *runtimeRepository) writeLLMProviderConfigJSON(ctx context.Context, tenant store.Tenant, provider string, value []byte) error {
	if !json.Valid(value) {
		return errors.B.KindInvalidInput().Textf("config for llm provider %q must be valid JSON", provider).Build()
	}
	if _, err := r.pool.Exec(ctx, llmProviderRuntimeSetConfigSQL, tenant.ID, provider, value); err != nil {
		return errors.B.Op("postgres.runtime.write_llm_provider_config_json").Textf("setting config for llm provider %q", provider).Err(err).Build()
	}
	return nil
}

func (r *runtimeRepository) readLLMProviderConfigJSON(ctx context.Context, tenant store.Tenant, provider string) ([]byte, bool, error) {
	var value []byte
	err := r.pool.QueryRow(ctx, `
		SELECT config
		FROM llm_provider_runtime
		WHERE tenant_id = $1 AND provider = $2 AND config IS NOT NULL
	`, tenant.ID, provider).Scan(&value)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, errors.B.Op("postgres.runtime.read_llm_provider_config_json").Textf("getting config for llm provider %q", provider).Err(err).Build()
	}
	return value, true, nil
}

func (r *runtimeRepository) writeLLMProviderEncryptedJSON(ctx context.Context, tenant store.Tenant, provider string, value []byte) error {
	if !json.Valid(value) {
		return errors.B.KindInvalidInput().Textf("%s for llm provider %q must be valid JSON", llmProviderCredentials, provider).Build()
	}
	if r.secretBox == nil {
		return errors.B.KindFailedPrecondition().Text("store secret box is not initialized").Build()
	}
	ciphertext, err := r.secretBox.Seal(value, llmProviderAssociatedData(tenant, provider))
	if err != nil {
		return errors.B.Op("postgres.runtime.write_llm_provider_encrypted_json").Textf("encrypting llm provider %q credentials", provider).Err(err).Build()
	}
	if _, err := r.pool.Exec(ctx, llmProviderRuntimeSetCredentialsSQL, tenant.ID, provider, ciphertext); err != nil {
		return errors.B.Op("postgres.runtime.write_llm_provider_encrypted_json").Textf("setting credentials for llm provider %q", provider).Err(err).Build()
	}
	return nil
}

func (r *runtimeRepository) readLLMProviderEncryptedJSON(ctx context.Context, tenant store.Tenant, provider string) ([]byte, bool, error) {
	var ciphertext []byte
	if r.secretBox == nil {
		return nil, false, errors.B.KindFailedPrecondition().Text("store secret box is not initialized").Build()
	}
	err := r.pool.QueryRow(ctx, `
		SELECT credentials_ciphertext
		FROM llm_provider_runtime
		WHERE tenant_id = $1 AND provider = $2 AND credentials_ciphertext IS NOT NULL
	`, tenant.ID, provider).Scan(&ciphertext)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, errors.B.
			Op("postgres.runtime.read_llm_provider_encrypted_json").
			Textf("getting credentials for llm provider %q", provider).
			Err(err).
			Build()
	}
	plaintext, err := r.secretBox.Open(ciphertext, llmProviderAssociatedData(tenant, provider))
	if err != nil {
		return nil, false, errors.B.Op("postgres.runtime.read_llm_provider_encrypted_json").Textf("decrypting llm provider %q credentials", provider).Err(err).Build()
	}
	return plaintext, true, nil
}

func llmProviderAssociatedData(tenant store.Tenant, provider string) auth.SecretAssociatedData {
	return auth.SecretAssociatedData{
		TenantID: tenant.ID,
		Scope:    "llm_provider",
		Name:     provider,
		Kind:     llmProviderCredentials,
	}
}

func (r *runtimeRepository) readAppConfig(ctx context.Context, tenant store.Tenant, key string) (string, error) {
	var value string
	err := r.pool.QueryRow(ctx,
		`SELECT value FROM app_config WHERE tenant_id = $1 AND key = $2`,
		tenant.ID, key,
	).Scan(&value)
	if err != nil {
		return "", errors.B.Op("postgres.runtime.read_app_config").Textf("getting app config %q", key).Err(err).Build()
	}
	return value, nil
}

func (r *runtimeRepository) writeAppConfig(ctx context.Context, tenant store.Tenant, key, value string) error {
	_, err := r.pool.Exec(ctx,
		appConfigUpsertSQL,
		tenant.ID, key, value,
	)
	if err != nil {
		return errors.B.Op("postgres.runtime.write_app_config").Textf("setting app config %q", key).Err(err).Build()
	}
	return nil
}

func (r *runtimeRepository) readGlobalAppConfig(ctx context.Context, key string) (string, error) {
	var value string
	err := r.pool.QueryRow(ctx,
		`SELECT value FROM app_config WHERE tenant_id IS NULL AND key = $1`,
		key,
	).Scan(&value)
	if err != nil {
		return "", errors.B.Op("postgres.runtime.read_global_app_config").Textf("getting global app config %q", key).Err(err).Build()
	}
	return value, nil
}

func (r *runtimeRepository) writeGlobalAppConfig(ctx context.Context, key, value string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO app_config (tenant_id, key, value)
		VALUES (NULL, $1, $2)
		ON CONFLICT (key) WHERE tenant_id IS NULL
		DO UPDATE SET value = EXCLUDED.value
	`, key, value)
	if err != nil {
		return errors.B.Op("postgres.runtime.write_global_app_config").Textf("setting global app config %q", key).Err(err).Build()
	}
	return nil
}

func runtimeSetReaderSecretQuery(column string) (string, error) {
	switch column {
	case readerRuntimeClientSecret:
		return runtimeSetReaderClientSecretSQL, nil
	case readerRuntimeOAuthToken:
		return runtimeSetReaderOAuthTokenSQL, nil
	default:
		return "", errors.B.KindInternal().Textf("unsupported reader runtime column %q", column).Build()
	}
}

func runtimeGetReaderSecretQuery(column string) (string, error) {
	switch column {
	case readerRuntimeClientSecret:
		return `
			SELECT client_secret_ciphertext
			FROM reader_runtime
			WHERE tenant_id = $1 AND reader = $2 AND client_secret_ciphertext IS NOT NULL
		`, nil
	case readerRuntimeOAuthToken:
		return `
			SELECT oauth_token_ciphertext
			FROM reader_runtime
			WHERE tenant_id = $1 AND reader = $2 AND oauth_token_ciphertext IS NOT NULL
		`, nil
	default:
		return "", errors.B.KindInternal().Textf("unsupported reader runtime column %q", column).Build()
	}
}

const runtimeSetReaderClientSecretSQL = `
	INSERT INTO reader_runtime (tenant_id, reader, client_secret_ciphertext)
	VALUES ($1, $2, $3)
	ON CONFLICT (tenant_id, reader) WHERE tenant_id IS NOT NULL
	DO UPDATE SET client_secret = NULL, client_secret_ciphertext = EXCLUDED.client_secret_ciphertext, updated_at = NOW()
`

const runtimeSetReaderOAuthTokenSQL = `
	INSERT INTO reader_runtime (tenant_id, reader, oauth_token_ciphertext)
	VALUES ($1, $2, $3)
	ON CONFLICT (tenant_id, reader) WHERE tenant_id IS NOT NULL
	DO UPDATE SET oauth_token = NULL, oauth_token_ciphertext = EXCLUDED.oauth_token_ciphertext, updated_at = NOW()
`

const runtimeSetReaderConfigSQL = `
	INSERT INTO reader_runtime (tenant_id, reader, config)
	VALUES ($1, $2, $3)
	ON CONFLICT (tenant_id, reader) WHERE tenant_id IS NOT NULL
	DO UPDATE SET config = EXCLUDED.config, updated_at = NOW()
`

const llmProviderRuntimeSetConfigSQL = `
	INSERT INTO llm_provider_runtime (tenant_id, provider, config)
	VALUES ($1, $2, $3)
	ON CONFLICT (tenant_id, provider) WHERE tenant_id IS NOT NULL
	DO UPDATE SET config = EXCLUDED.config, updated_at = NOW()
`

const llmProviderRuntimeSetCredentialsSQL = `
	INSERT INTO llm_provider_runtime (tenant_id, provider, credentials_ciphertext)
	VALUES ($1, $2, $3)
	ON CONFLICT (tenant_id, provider) WHERE tenant_id IS NOT NULL
	DO UPDATE SET credentials_ciphertext = EXCLUDED.credentials_ciphertext, updated_at = NOW()
`

const llmProviderRuntimeUpsertActiveSQL = `
	INSERT INTO llm_provider_runtime (tenant_id, provider, active)
	VALUES ($1, $2, true)
	ON CONFLICT (tenant_id, provider) WHERE tenant_id IS NOT NULL
	DO UPDATE SET active = true, updated_at = NOW()
`
