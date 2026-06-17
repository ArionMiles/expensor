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
		return nil, ErrBootstrapUnavailable
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

func (r *pgAuthRepository) FindUserByEmail(ctx context.Context, email string) (*User, error) {
	user, err := scanUser(r.pool.QueryRow(ctx, `
		SELECT id, id AS tenant_id, email, COALESCE(password_hash, ''), display_name, role, avatar_key,
		       disabled_at, created_at, updated_at
		FROM users
		WHERE lower(email) = lower($1)
	`, email))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
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
			return nil, ErrNotFound
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
			return nil, ErrNotFound
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
		return ErrNotFound
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
		return nil, fmt.Errorf("creating access token: %w", err)
	}
	return token, nil
}

func (r *pgAuthRepository) FindAccessTokenByHash(ctx context.Context, tokenHash string) (*AccessToken, error) {
	token, err := scanAccessToken(r.pool.QueryRow(ctx, `
		SELECT id, user_id, name, token_hash, created_at, expires_at, last_used_at, revoked_at
		FROM access_tokens
		WHERE token_hash = $1
	`, tokenHash))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
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
		return ErrNotFound
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
			return nil, ErrNotFound
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
		return ErrNotFound
	}
	return nil
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
