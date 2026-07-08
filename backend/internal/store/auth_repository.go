package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgAuthRepository struct {
	pool *pgxpool.Pool
}

func newPGAuthRepository(deps repositoryDependencies) *pgAuthRepository {
	return &pgAuthRepository{pool: deps.pool}
}

func (r *pgAuthRepository) BootstrapRequired(ctx context.Context) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users)`).Scan(&exists); err != nil {
		return false, fmt.Errorf("checking bootstrap status: %w", err)
	}
	return !exists, nil
}

func (r *pgAuthRepository) CreateBootstrapAdmin(ctx context.Context, input CreateBootstrapAdminInput) (*User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning bootstrap admin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var existingUsers int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&existingUsers); err != nil {
		return nil, fmt.Errorf("counting users for bootstrap: %w", err)
	}
	if existingUsers > 0 {
		return nil, conflict("store.auth.create_bootstrap_admin", messageBootstrapUnavailable)
	}

	user, err := insertUser(ctx, tx, CreateUserInput{
		Email:        input.Email,
		DisplayName:  input.DisplayName,
		Role:         UserRoleAdmin,
		AvatarKey:    input.AvatarKey,
		PasswordHash: input.PasswordHash,
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing bootstrap admin transaction: %w", err)
	}
	return user, nil
}

func (r *pgAuthRepository) CreateUser(ctx context.Context, input CreateUserInput) (*User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning create user transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	user, err := insertUser(ctx, tx, input)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing create user transaction: %w", err)
	}
	return user, nil
}

func (r *pgAuthRepository) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, id AS tenant_id, email, COALESCE(password_hash, ''), display_name, role, avatar_key,
		       disabled_at, created_at, updated_at
		FROM users
		ORDER BY created_at, id
	`)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning user: %w", err)
		}
		users = append(users, *user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating users: %w", err)
	}
	return users, nil
}

func (r *pgAuthRepository) UpdateUser(ctx context.Context, id string, input UpdateUserInput) (*User, error) {
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
			return nil, notFound("store.auth.update_user")
		}
		return nil, fmt.Errorf("updating user: %w", err)
	}
	return user, nil
}

func (r *pgAuthRepository) UpdateUserPassword(ctx context.Context, id string, input UpdateUserPasswordInput) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE users
		SET password_hash = $2,
		    updated_at = NOW()
		WHERE id = $1
	`, id, input.PasswordHash)
	if err != nil {
		return fmt.Errorf("updating user password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return notFound("store.auth.update_user_password")
	}
	return nil
}

func (r *pgAuthRepository) DeleteUser(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return notFound("store.auth.delete_user")
	}
	return nil
}

func (r *pgAuthRepository) FindUserByEmail(ctx context.Context, email string) (*User, error) {
	user, err := scanUser(r.pool.QueryRow(ctx, `
		SELECT id, id AS tenant_id, email, COALESCE(password_hash, ''), display_name, role, avatar_key,
		       disabled_at, created_at, updated_at
		FROM users
		WHERE lower(email) = lower($1)
	`, email))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, notFound("store.auth.find_user_by_email")
		}
		return nil, fmt.Errorf("finding user by email: %w", err)
	}
	return user, nil
}

func (r *pgAuthRepository) FindUserByID(ctx context.Context, id string) (*User, error) {
	user, err := scanUser(r.pool.QueryRow(ctx, `
		SELECT id, id AS tenant_id, email, COALESCE(password_hash, ''), display_name, role, avatar_key,
		       disabled_at, created_at, updated_at
		FROM users
		WHERE id = $1
	`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, notFound("store.auth.find_user_by_id")
		}
		return nil, fmt.Errorf("finding user by id: %w", err)
	}
	return user, nil
}

func (r *pgAuthRepository) CreateSession(ctx context.Context, input CreateSessionInput) (*Session, error) {
	session, err := scanSession(r.pool.QueryRow(ctx, `
		INSERT INTO sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, token_hash, created_at, expires_at, last_used_at, revoked_at
	`, input.UserID, input.TokenHash, input.ExpiresAt))
	if err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}
	return session, nil
}

func (r *pgAuthRepository) FindSessionByHash(ctx context.Context, tokenHash string) (*Session, error) {
	session, err := scanSession(r.pool.QueryRow(ctx, `
		SELECT id, user_id, token_hash, created_at, expires_at, last_used_at, revoked_at
		FROM sessions
		WHERE token_hash = $1
	`, tokenHash))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, notFound("store.auth.find_session_by_hash")
		}
		return nil, fmt.Errorf("finding session by hash: %w", err)
	}
	return session, nil
}

func (r *pgAuthRepository) RevokeSession(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE sessions SET revoked_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("revoking session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return notFound("store.auth.revoke_session")
	}
	return nil
}

func (r *pgAuthRepository) CreateAccessToken(ctx context.Context, input CreateAccessTokenInput) (*AccessToken, error) {
	token, err := scanAccessToken(r.pool.QueryRow(ctx, `
		INSERT INTO access_tokens (user_id, name, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, name, token_hash, created_at, expires_at, last_used_at, revoked_at
	`, input.UserID, input.Name, input.TokenHash, input.ExpiresAt))
	if err != nil {
		if isAccessTokenNameConflict(err) {
			return nil, conflict("store.auth.create_access_token", messageAccessTokenNameConflict)
		}
		return nil, fmt.Errorf("creating access token: %w", err)
	}
	return token, nil
}

func (r *pgAuthRepository) ListAccessTokens(ctx context.Context, userID string) ([]AccessToken, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, name, token_hash, created_at, expires_at, last_used_at, revoked_at
		FROM access_tokens
		WHERE user_id = $1 AND revoked_at IS NULL
		ORDER BY created_at DESC, id DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("listing access tokens: %w", err)
	}
	defer rows.Close()

	tokens := make([]AccessToken, 0)
	for rows.Next() {
		token, err := scanAccessToken(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning access token: %w", err)
		}
		tokens = append(tokens, *token)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating access tokens: %w", err)
	}
	return tokens, nil
}

func (r *pgAuthRepository) FindAccessTokenByHash(ctx context.Context, tokenHash string) (*AccessToken, error) {
	token, err := scanAccessToken(r.pool.QueryRow(ctx, `
		SELECT id, user_id, name, token_hash, created_at, expires_at, last_used_at, revoked_at
		FROM access_tokens
		WHERE token_hash = $1
	`, tokenHash))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, notFound("store.auth.find_access_token_by_hash")
		}
		return nil, fmt.Errorf("finding access token by hash: %w", err)
	}
	return token, nil
}

func (r *pgAuthRepository) RevokeAccessToken(ctx context.Context, id, userID string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE access_tokens SET revoked_at = NOW() WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("revoking access token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return notFound("store.auth.revoke_access_token")
	}
	return nil
}

func (r *pgAuthRepository) CreateAccountSetupToken(ctx context.Context, input CreateAccountSetupTokenInput) (*AccountSetupToken, error) {
	token, err := scanAccountSetupToken(r.pool.QueryRow(ctx, `
		INSERT INTO account_setup_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, token_hash, created_at, expires_at, used_at
	`, input.UserID, input.TokenHash, input.ExpiresAt))
	if err != nil {
		return nil, fmt.Errorf("creating account setup token: %w", err)
	}
	return token, nil
}

func (r *pgAuthRepository) FindAccountSetupTokenByHash(ctx context.Context, tokenHash string) (*AccountSetupToken, error) {
	token, err := scanAccountSetupToken(r.pool.QueryRow(ctx, `
		SELECT id, user_id, token_hash, created_at, expires_at, used_at
		FROM account_setup_tokens
		WHERE token_hash = $1
	`, tokenHash))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, notFound("store.auth.find_account_setup_token_by_hash")
		}
		return nil, fmt.Errorf("finding account setup token by hash: %w", err)
	}
	return token, nil
}

func (r *pgAuthRepository) MarkAccountSetupTokenUsed(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE account_setup_tokens SET used_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("marking account setup token used: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return notFound("store.auth.mark_account_setup_token_used")
	}
	return nil
}

func (r *pgAuthRepository) CompleteAccountSetup(ctx context.Context, input CompleteAccountSetupInput) (*User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning account setup transaction: %w", err)
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
			return nil, notFound("store.auth.complete_account_setup")
		}
		return nil, fmt.Errorf("finding active account setup token: %w", err)
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
		return nil, fmt.Errorf("updating setup password: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE account_setup_tokens SET used_at = NOW() WHERE id = $1`, token.ID); err != nil {
		return nil, fmt.Errorf("marking account setup token used: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing account setup transaction: %w", err)
	}
	return user, nil
}

func insertUser(ctx context.Context, tx pgx.Tx, input CreateUserInput) (*User, error) {
	role := input.Role
	if role == "" {
		role = UserRoleUser
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
		if isUserEmailConflict(err) {
			return nil, conflict("store.auth.create_user", messageUserEmailConflict)
		}
		return nil, fmt.Errorf("creating user: %w", err)
	}
	return user, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanUser(row scanner) (*User, error) {
	var user User
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

func scanSession(row scanner) (*Session, error) {
	var session Session
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

func scanAccessToken(row scanner) (*AccessToken, error) {
	var token AccessToken
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

func scanAccountSetupToken(row scanner) (*AccountSetupToken, error) {
	var token AccountSetupToken
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
