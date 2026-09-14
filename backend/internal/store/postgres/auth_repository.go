package postgres

import (
	"context"
	"strings"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

type authRepository struct {
	pool *pgxpool.Pool
}

func newAuthRepository(deps repositoryDependencies) *authRepository {
	return &authRepository{pool: deps.pool}
}

func (r *authRepository) BootstrapRequired(ctx context.Context) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users)`).Scan(&exists); err != nil {
		return false, errors.B.Op("postgres.auth.bootstrap_required").Text("checking bootstrap status").Err(err).Build()
	}
	return !exists, nil
}

func (r *authRepository) CreateBootstrapAdmin(ctx context.Context, input store.CreateBootstrapAdminInput) (*store.User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, errors.B.Op("postgres.auth.create_bootstrap_admin").Text("beginning bootstrap admin transaction").Err(err).Build()
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var existingUsers int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&existingUsers); err != nil {
		return nil, errors.B.Op("postgres.auth.create_bootstrap_admin").Text("counting users for bootstrap").Err(err).Build()
	}
	if existingUsers > 0 {
		return nil, errors.B.Op("store.auth.create_bootstrap_admin").KindConflict().UserMsg("bootstrap is already complete for this instance").Build()
	}

	user, err := insertUser(ctx, tx, store.CreateUserInput{
		Email:        input.Email,
		DisplayName:  input.DisplayName,
		Role:         store.UserRoleAdmin,
		AvatarKey:    input.AvatarKey,
		PasswordHash: input.PasswordHash,
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, errors.B.Op("postgres.auth.create_bootstrap_admin").Text("committing bootstrap admin transaction").Err(err).Build()
	}
	return user, nil
}

func (r *authRepository) CreateUser(ctx context.Context, input store.CreateUserInput) (*store.User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, errors.B.Op("postgres.auth.create_user").Text("beginning create user transaction").Err(err).Build()
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	user, err := insertUser(ctx, tx, input)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, errors.B.Op("postgres.auth.create_user").Text("committing create user transaction").Err(err).Build()
	}
	return user, nil
}

func (r *authRepository) ListUsers(ctx context.Context) ([]store.User, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, id AS tenant_id, email, COALESCE(password_hash, ''), display_name, role, avatar_key,
		       disabled_at, created_at, updated_at
		FROM users
		ORDER BY created_at, id
	`)
	if err != nil {
		return nil, errors.B.Op("postgres.auth.list_users").Text("listing users").Err(err).Build()
	}
	defer rows.Close()

	users := make([]store.User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, errors.B.Op("postgres.auth.list_users").Text("scanning user").Err(err).Build()
		}
		users = append(users, *user)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.B.Op("postgres.auth.list_users").Text("iterating users").Err(err).Build()
	}
	return users, nil
}

func (r *authRepository) UpdateUser(ctx context.Context, id string, input store.UpdateUserInput) (*store.User, error) {
	user, err := scanUser(r.pool.QueryRow(ctx, `
		UPDATE users
		SET display_name = COALESCE($2, display_name),
		    role = COALESCE($3, role),
		    avatar_key = COALESCE($4, avatar_key),
		    disabled_at = CASE
		        WHEN $5::boolean IS NULL THEN disabled_at
		        WHEN $5 THEN COALESCE(disabled_at, NOW())
		        ELSE NULL
		    END,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, id AS tenant_id, email, COALESCE(password_hash, ''), display_name, role, avatar_key,
		          disabled_at, created_at, updated_at
	`, id, input.DisplayName, input.Role, input.AvatarKey, input.Disabled))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.B.Op("store.auth.update_user").KindNotFound().UserMsg("user not found").Build()
		}
		return nil, errors.B.Op("postgres.auth.update_user").Text("updating user").Err(err).Build()
	}
	return user, nil
}

func (r *authRepository) UpdateUserPassword(ctx context.Context, id string, input store.UpdateUserPasswordInput) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE users
		SET password_hash = $2,
		    updated_at = NOW()
		WHERE id = $1
	`, id, input.PasswordHash)
	if err != nil {
		return errors.B.Op("postgres.auth.update_user_password").Text("updating user password").Err(err).Build()
	}
	if tag.RowsAffected() == 0 {
		return errors.B.Op("store.auth.update_user_password").KindNotFound().UserMsg("user not found").Build()
	}
	return nil
}

func (r *authRepository) DeleteUser(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return errors.B.Op("postgres.auth.delete_user").Text("deleting user").Err(err).Build()
	}
	if tag.RowsAffected() == 0 {
		return errors.B.Op("store.auth.delete_user").KindNotFound().UserMsg("user not found").Build()
	}
	return nil
}

func (r *authRepository) FindUserByEmail(ctx context.Context, email string) (*store.User, error) {
	user, err := scanUser(r.pool.QueryRow(ctx, `
		SELECT id, id AS tenant_id, email, COALESCE(password_hash, ''), display_name, role, avatar_key,
		       disabled_at, created_at, updated_at
		FROM users
		WHERE lower(email) = lower($1)
	`, email))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.B.Op("store.auth.find_user_by_email").KindNotFound().UserMsg("user not found").Build()
		}
		return nil, errors.B.Op("postgres.auth.find_user_by_email").Text("finding user by email").Err(err).Build()
	}
	return user, nil
}

func (r *authRepository) FindUserByID(ctx context.Context, id string) (*store.User, error) {
	user, err := scanUser(r.pool.QueryRow(ctx, `
		SELECT id, id AS tenant_id, email, COALESCE(password_hash, ''), display_name, role, avatar_key,
		       disabled_at, created_at, updated_at
		FROM users
		WHERE id = $1
	`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.B.Op("store.auth.find_user_by_id").KindNotFound().UserMsg("user not found").Build()
		}
		return nil, errors.B.Op("postgres.auth.find_user_by_id").Text("finding user by id").Err(err).Build()
	}
	return user, nil
}

func (r *authRepository) CreateSession(ctx context.Context, input store.CreateSessionInput) (*store.Session, error) {
	session, err := scanSession(r.pool.QueryRow(ctx, `
		INSERT INTO sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, token_hash, created_at, expires_at, last_used_at, revoked_at
	`, input.UserID, input.TokenHash, input.ExpiresAt))
	if err != nil {
		return nil, errors.B.Op("postgres.auth.create_session").Text("creating session").Err(err).Build()
	}
	return session, nil
}

func (r *authRepository) FindSessionByHash(ctx context.Context, tokenHash string) (*store.Session, error) {
	session, err := scanSession(r.pool.QueryRow(ctx, `
		SELECT id, user_id, token_hash, created_at, expires_at, last_used_at, revoked_at
		FROM sessions
		WHERE token_hash = $1
	`, tokenHash))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.B.Op("store.auth.find_session_by_hash").KindNotFound().Build()
		}
		return nil, errors.B.Op("postgres.auth.find_session_by_hash").Text("finding session by hash").Err(err).Build()
	}
	return session, nil
}

func (r *authRepository) RevokeSession(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE sessions SET revoked_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return errors.B.Op("postgres.auth.revoke_session").Text("revoking session").Err(err).Build()
	}
	if tag.RowsAffected() == 0 {
		return errors.B.Op("store.auth.revoke_session").KindNotFound().Build()
	}
	return nil
}

func (r *authRepository) CreateAccessToken(ctx context.Context, input store.CreateAccessTokenInput) (*store.AccessToken, error) {
	token, err := scanAccessToken(r.pool.QueryRow(ctx, `
		INSERT INTO access_tokens (user_id, name, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, name, token_hash, created_at, expires_at, last_used_at, revoked_at
	`, input.UserID, input.Name, input.TokenHash, input.ExpiresAt))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return nil, errors.B.
				Op("store.auth.create_access_token").
				KindConflict().
				UserMsg("Token " + input.Name + " already exists.").
				Text("access token name conflict").
				Err(err).
				Build()
		}
		return nil, errors.B.Op("postgres.auth.create_access_token").Text("creating access token").Err(err).Build()
	}
	return token, nil
}

func (r *authRepository) ListAccessTokens(ctx context.Context, userID string) ([]store.AccessToken, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, name, token_hash, created_at, expires_at, last_used_at, revoked_at
		FROM access_tokens
		WHERE user_id = $1 AND revoked_at IS NULL
		ORDER BY created_at DESC, id DESC
	`, userID)
	if err != nil {
		return nil, errors.B.Op("postgres.auth.list_access_tokens").Text("listing access tokens").Err(err).Build()
	}
	defer rows.Close()

	tokens := make([]store.AccessToken, 0)
	for rows.Next() {
		token, err := scanAccessToken(rows)
		if err != nil {
			return nil, errors.B.Op("postgres.auth.list_access_tokens").Text("scanning access token").Err(err).Build()
		}
		tokens = append(tokens, *token)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.B.Op("postgres.auth.list_access_tokens").Text("iterating access tokens").Err(err).Build()
	}
	return tokens, nil
}

func (r *authRepository) FindAccessTokenByHash(ctx context.Context, tokenHash string) (*store.AccessToken, error) {
	token, err := scanAccessToken(r.pool.QueryRow(ctx, `
		SELECT id, user_id, name, token_hash, created_at, expires_at, last_used_at, revoked_at
		FROM access_tokens
		WHERE token_hash = $1
	`, tokenHash))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.B.Op("store.auth.find_access_token_by_hash").KindNotFound().Build()
		}
		return nil, errors.B.Op("postgres.auth.find_access_token_by_hash").Text("finding access token by hash").Err(err).Build()
	}
	return token, nil
}

func (r *authRepository) MarkAccessTokenUsed(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE access_tokens SET last_used_at = NOW() WHERE id = $1 AND revoked_at IS NULL`, id)
	if err != nil {
		return errors.B.Op("postgres.auth.mark_access_token_used").Text("marking access token used").Err(err).Build()
	}
	if tag.RowsAffected() == 0 {
		return errors.B.Op("store.auth.mark_access_token_used").KindNotFound().Build()
	}
	return nil
}

func (r *authRepository) RevokeAccessToken(ctx context.Context, id, userID string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE access_tokens SET revoked_at = NOW() WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return errors.B.Op("postgres.auth.revoke_access_token").Text("revoking access token").Err(err).Build()
	}
	if tag.RowsAffected() == 0 {
		return errors.B.Op("store.auth.revoke_access_token").KindNotFound().UserMsg("token not found").Build()
	}
	return nil
}

func (r *authRepository) CreateAccountSetupToken(ctx context.Context, input store.CreateAccountSetupTokenInput) (*store.AccountSetupToken, error) {
	token, err := scanAccountSetupToken(r.pool.QueryRow(ctx, `
		INSERT INTO account_setup_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, token_hash, created_at, expires_at, used_at
	`, input.UserID, input.TokenHash, input.ExpiresAt))
	if err != nil {
		return nil, errors.B.Op("postgres.auth.create_account_setup_token").Text("creating account setup token").Err(err).Build()
	}
	return token, nil
}

func (r *authRepository) FindAccountSetupTokenByHash(ctx context.Context, tokenHash string) (*store.AccountSetupToken, error) {
	token, err := scanAccountSetupToken(r.pool.QueryRow(ctx, `
		SELECT id, user_id, token_hash, created_at, expires_at, used_at
		FROM account_setup_tokens
		WHERE token_hash = $1
	`, tokenHash))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.B.Op("store.auth.find_account_setup_token_by_hash").KindNotFound().Build()
		}
		return nil, errors.B.Op("postgres.auth.find_account_setup_token_by_hash").Text("finding account setup token by hash").Err(err).Build()
	}
	return token, nil
}

func (r *authRepository) MarkAccountSetupTokenUsed(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE account_setup_tokens SET used_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return errors.B.Op("postgres.auth.mark_account_setup_token_used").Text("marking account setup token used").Err(err).Build()
	}
	if tag.RowsAffected() == 0 {
		return errors.B.Op("store.auth.mark_account_setup_token_used").KindNotFound().Build()
	}
	return nil
}

func (r *authRepository) CompleteAccountSetup(ctx context.Context, input store.CompleteAccountSetupInput) (*store.User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, errors.B.Op("postgres.auth.complete_account_setup").Text("beginning account setup transaction").Err(err).Build()
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	token, err := scanAccountSetupToken(tx.QueryRow(ctx, `
		SELECT id, user_id, token_hash, created_at, expires_at, used_at
		FROM account_setup_tokens
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > NOW()
		FOR UPDATE
	`, input.TokenHash))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.B.Op("store.auth.complete_account_setup").KindNotFound().Build()
		}
		return nil, errors.B.Op("postgres.auth.complete_account_setup").Text("finding active account setup token").Err(err).Build()
	}

	user, err := scanUser(tx.QueryRow(ctx, `
		UPDATE users
		SET password_hash = $2,
		    display_name = $3,
		    avatar_key = $4,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, id AS tenant_id, email, COALESCE(password_hash, ''), display_name, role, avatar_key,
		          disabled_at, created_at, updated_at
	`, token.UserID, input.PasswordHash, input.DisplayName, input.AvatarKey))
	if err != nil {
		return nil, errors.B.Op("postgres.auth.complete_account_setup").Text("updating setup password").Err(err).Build()
	}
	if _, err := tx.Exec(ctx, `UPDATE account_setup_tokens SET used_at = NOW() WHERE id = $1`, token.ID); err != nil {
		return nil, errors.B.Op("postgres.auth.complete_account_setup").Text("marking account setup token used").Err(err).Build()
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, errors.B.Op("postgres.auth.complete_account_setup").Text("committing account setup transaction").Err(err).Build()
	}
	return user, nil
}

func insertUser(ctx context.Context, tx pgx.Tx, input store.CreateUserInput) (*store.User, error) {
	role := input.Role
	if role == "" {
		role = store.UserRoleUser
	}
	avatarKey := strings.TrimSpace(input.AvatarKey)
	if avatarKey == "" {
		avatarKey = "default"
	}
	user, err := scanUser(tx.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name, role, avatar_key)
		VALUES (lower($1), $2, $3, $4, $5)
		RETURNING id, id AS tenant_id, email, COALESCE(password_hash, ''), display_name, role, avatar_key,
		          disabled_at, created_at, updated_at
	`, input.Email, input.PasswordHash, input.DisplayName, role, avatarKey))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return nil, errors.B.
				Op("store.auth.create_user").
				KindConflict().
				UserMsg("User " + strings.ToLower(strings.TrimSpace(input.Email)) + " already exists.").
				Text("user email conflict").
				Err(err).
				Build()
		}
		return nil, errors.B.Op("postgres.auth.insert_user").Text("creating user").Err(err).Build()
	}
	return user, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanUser(row scanner) (*store.User, error) {
	var user store.User
	if err := row.Scan(
		&user.ID,
		&user.TenantID,
		&user.Email,
		&user.PasswordHash,
		&user.DisplayName,
		&user.Role,
		&user.AvatarKey,
		&user.DisabledAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &user, nil
}

func scanSession(row scanner) (*store.Session, error) {
	var session store.Session
	if err := row.Scan(
		&session.ID,
		&session.UserID,
		&session.TokenHash,
		&session.CreatedAt,
		&session.ExpiresAt,
		&session.LastUsedAt,
		&session.RevokedAt,
	); err != nil {
		return nil, err
	}
	return &session, nil
}

func scanAccessToken(row scanner) (*store.AccessToken, error) {
	var token store.AccessToken
	if err := row.Scan(
		&token.ID,
		&token.UserID,
		&token.Name,
		&token.TokenHash,
		&token.CreatedAt,
		&token.ExpiresAt,
		&token.LastUsedAt,
		&token.RevokedAt,
	); err != nil {
		return nil, err
	}
	return &token, nil
}

func scanAccountSetupToken(row scanner) (*store.AccountSetupToken, error) {
	var token store.AccountSetupToken
	if err := row.Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.CreatedAt,
		&token.ExpiresAt,
		&token.UsedAt,
	); err != nil {
		return nil, err
	}
	return &token, nil
}
