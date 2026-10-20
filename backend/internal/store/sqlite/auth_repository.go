package sqlite

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const (
	userColumns = `id, id, email, COALESCE(password_hash, ''), display_name, role, avatar_key,
		disabled_at, created_at, updated_at`
	sessionColumns           = `id, user_id, token_hash, created_at, expires_at, last_used_at, revoked_at`
	accessTokenColumns       = `id, user_id, name, token_hash, created_at, expires_at, last_used_at, revoked_at`
	accountSetupTokenColumns = `id, user_id, token_hash, created_at, expires_at, used_at`
)

var _ store.AuthStore = (*Store)(nil)

// BootstrapRequired reports whether the instance has no users.
func (s *Store) BootstrapRequired(ctx context.Context) (bool, error) {
	var exists bool
	if err := s.repositories.query.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users)`).Scan(&exists); err != nil {
		return false, mapSQLiteError("sqlite.auth.bootstrap_required", err)
	}
	return !exists, nil
}

// CreateBootstrapAdmin creates the first user as an administrator.
func (s *Store) CreateBootstrapAdmin(ctx context.Context, input store.CreateBootstrapAdminInput) (*store.User, error) {
	var user *store.User
	err := s.repositories.writeTx(ctx, func(query queryer) error {
		var exists bool
		if err := query.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users)`).Scan(&exists); err != nil {
			return mapSQLiteError("sqlite.auth.create_bootstrap_admin", err)
		}
		if exists {
			return errors.B.Op("store.auth.create_bootstrap_admin").KindConflict().UserMsg("bootstrap is already complete for this instance").Build()
		}

		var err error
		user, err = insertAuthUser(ctx, query, s.repositories.now(), store.CreateUserInput{
			Email:        input.Email,
			DisplayName:  input.DisplayName,
			Role:         store.UserRoleAdmin,
			AvatarKey:    input.AvatarKey,
			PasswordHash: input.PasswordHash,
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return user, nil
}

// CreateUser creates an instance user.
func (s *Store) CreateUser(ctx context.Context, input store.CreateUserInput) (*store.User, error) {
	var user *store.User
	err := s.repositories.writeTx(ctx, func(query queryer) error {
		var err error
		user, err = insertAuthUser(ctx, query, s.repositories.now(), input)
		return err
	})
	if err != nil {
		return nil, err
	}
	return user, nil
}

// ListUsers lists all instance users in creation order.
func (s *Store) ListUsers(ctx context.Context) ([]store.User, error) {
	rows, err := s.repositories.query.QueryContext(ctx, `SELECT `+userColumns+` FROM users ORDER BY created_at, id`)
	if err != nil {
		return nil, mapSQLiteError("sqlite.auth.list_users", err)
	}
	defer rows.Close()

	users := make([]store.User, 0)
	for rows.Next() {
		user, err := scanAuthUser(rows)
		if err != nil {
			return nil, errors.B.Op("sqlite.auth.list_users").KindInternal().Text("scanning user").Err(err).Build()
		}
		users = append(users, *user)
	}
	if err := rows.Err(); err != nil {
		return nil, mapSQLiteError("sqlite.auth.list_users", err)
	}
	return users, nil
}

// UpdateUser updates mutable user metadata.
func (s *Store) UpdateUser(ctx context.Context, id string, input store.UpdateUserInput) (*store.User, error) {
	var user *store.User
	err := s.repositories.writeTx(ctx, func(query queryer) error {
		now := timeToText(s.repositories.now())
		var disabled any
		if input.Disabled != nil {
			disabled = *input.Disabled
		}
		result, err := query.ExecContext(ctx, `
			UPDATE users
			SET display_name = COALESCE(?, display_name),
			    role = COALESCE(?, role),
			    avatar_key = COALESCE(?, avatar_key),
			    disabled_at = CASE
			        WHEN ? IS NULL THEN disabled_at
			        WHEN ? THEN COALESCE(disabled_at, ?)
			        ELSE NULL
			    END,
			    updated_at = ?
			WHERE id = ?
		`, input.DisplayName, input.Role, input.AvatarKey, disabled, disabled, now, now, id)
		if err != nil {
			return mapSQLiteError("sqlite.auth.update_user", err)
		}
		if err := requireAffected(result, "store.auth.update_user", "user not found"); err != nil {
			return err
		}
		user, err = findAuthUserByID(ctx, query, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return user, nil
}

// UpdateUserPassword replaces a user's password hash.
func (s *Store) UpdateUserPassword(ctx context.Context, id string, input store.UpdateUserPasswordInput) error {
	return s.repositories.writeTx(ctx, func(query queryer) error {
		result, err := query.ExecContext(
			ctx,
			`UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`,
			input.PasswordHash,
			timeToText(s.repositories.now()),
			id,
		)
		if err != nil {
			return mapSQLiteError("sqlite.auth.update_user_password", err)
		}
		return requireAffected(result, "store.auth.update_user_password", "user not found")
	})
}

// DeleteUser deletes a user and all rows that depend on it.
func (s *Store) DeleteUser(ctx context.Context, id string) error {
	return s.repositories.writeTx(ctx, func(query queryer) error {
		result, err := query.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
		if err != nil {
			return mapSQLiteError("sqlite.auth.delete_user", err)
		}
		return requireAffected(result, "store.auth.delete_user", "user not found")
	})
}

// FindUserByEmail finds a user with case-insensitive email matching.
func (s *Store) FindUserByEmail(ctx context.Context, email string) (*store.User, error) {
	row := s.repositories.query.QueryRowContext(
		ctx,
		`SELECT `+userColumns+` FROM users WHERE expensor_casefold(email) = expensor_casefold(?)`,
		email,
	)
	user, err := scanAuthUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.B.Op("store.auth.find_user_by_email").KindNotFound().UserMsg("user not found").Err(err).Build()
		}
		return nil, errors.B.Op("sqlite.auth.find_user_by_email").KindInternal().Text("finding user by email").Err(err).Build()
	}
	return user, nil
}

// FindUserByID finds a user by ID.
func (s *Store) FindUserByID(ctx context.Context, id string) (*store.User, error) {
	return findAuthUserByID(ctx, s.repositories.query, id)
}

// CreateSession creates a browser session.
func (s *Store) CreateSession(ctx context.Context, input store.CreateSessionInput) (*store.Session, error) {
	var session *store.Session
	err := s.repositories.writeTx(ctx, func(query queryer) error {
		id := uuid.NewString()
		now := timeToText(s.repositories.now())
		_, err := query.ExecContext(
			ctx,
			`INSERT INTO sessions (id, user_id, token_hash, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
			id,
			input.UserID,
			input.TokenHash,
			now,
			timeToText(input.ExpiresAt),
		)
		if err != nil {
			return mapSQLiteError("store.auth.create_session", err)
		}
		session, err = scanAuthSession(query.QueryRowContext(ctx, `SELECT `+sessionColumns+` FROM sessions WHERE id = ?`, id))
		return err
	})
	if err != nil {
		return nil, err
	}
	return session, nil
}

// FindSessionByHash finds a session by its token hash.
func (s *Store) FindSessionByHash(ctx context.Context, tokenHash string) (*store.Session, error) {
	session, err := scanAuthSession(s.repositories.query.QueryRowContext(ctx, `SELECT `+sessionColumns+` FROM sessions WHERE token_hash = ?`, tokenHash))
	if err != nil {
		return nil, authScanError("store.auth.find_session_by_hash", "finding session by hash", err)
	}
	return session, nil
}

// RevokeSession revokes a browser session.
func (s *Store) RevokeSession(ctx context.Context, id string) error {
	return s.repositories.writeTx(ctx, func(query queryer) error {
		result, err := query.ExecContext(ctx, `UPDATE sessions SET revoked_at = ? WHERE id = ?`, timeToText(s.repositories.now()), id)
		if err != nil {
			return mapSQLiteError("sqlite.auth.revoke_session", err)
		}
		return requireAffected(result, "store.auth.revoke_session", "")
	})
}

// CreateAccessToken creates a programmatic access token record.
func (s *Store) CreateAccessToken(ctx context.Context, input store.CreateAccessTokenInput) (*store.AccessToken, error) {
	var token *store.AccessToken
	err := s.repositories.writeTx(ctx, func(query queryer) error {
		id := uuid.NewString()
		now := timeToText(s.repositories.now())
		_, err := query.ExecContext(
			ctx,
			`INSERT INTO access_tokens (id, user_id, name, token_hash, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?)`,
			id,
			input.UserID,
			input.Name,
			input.TokenHash,
			now,
			optionalTimeToText(input.ExpiresAt),
		)
		if err != nil {
			mapped := mapSQLiteError("store.auth.create_access_token", err)
			if errors.WhatKind(mapped) == errors.Conflict {
				return errors.B.
					Op("store.auth.create_access_token").
					KindConflict().
					UserMsg("Token " + input.Name + " already exists.").
					Text("access token conflict").
					Err(err).
					Build()
			}
			return mapped
		}
		token, err = scanAuthAccessToken(query.QueryRowContext(ctx, `SELECT `+accessTokenColumns+` FROM access_tokens WHERE id = ?`, id))
		return err
	})
	if err != nil {
		return nil, err
	}
	return token, nil
}

// ListAccessTokens lists active access tokens for a user.
func (s *Store) ListAccessTokens(ctx context.Context, userID string) ([]store.AccessToken, error) {
	rows, err := s.repositories.query.QueryContext(
		ctx,
		`SELECT `+accessTokenColumns+` FROM access_tokens WHERE user_id = ? AND revoked_at IS NULL ORDER BY created_at DESC, id DESC`,
		userID,
	)
	if err != nil {
		return nil, mapSQLiteError("sqlite.auth.list_access_tokens", err)
	}
	defer rows.Close()

	tokens := make([]store.AccessToken, 0)
	for rows.Next() {
		token, err := scanAuthAccessToken(rows)
		if err != nil {
			return nil, errors.B.Op("sqlite.auth.list_access_tokens").KindInternal().Text("scanning access token").Err(err).Build()
		}
		tokens = append(tokens, *token)
	}
	if err := rows.Err(); err != nil {
		return nil, mapSQLiteError("sqlite.auth.list_access_tokens", err)
	}
	return tokens, nil
}

// FindAccessTokenByHash finds an access token by its token hash.
func (s *Store) FindAccessTokenByHash(ctx context.Context, tokenHash string) (*store.AccessToken, error) {
	row := s.repositories.query.QueryRowContext(
		ctx,
		`SELECT `+accessTokenColumns+` FROM access_tokens WHERE token_hash = ?`,
		tokenHash,
	)
	token, err := scanAuthAccessToken(row)
	if err != nil {
		return nil, authScanError("store.auth.find_access_token_by_hash", "finding access token by hash", err)
	}
	return token, nil
}

// MarkAccessTokenUsed records use of an active access token.
func (s *Store) MarkAccessTokenUsed(ctx context.Context, id string) error {
	return s.repositories.writeTx(ctx, func(query queryer) error {
		result, err := query.ExecContext(ctx, `UPDATE access_tokens SET last_used_at = ? WHERE id = ? AND revoked_at IS NULL`, timeToText(s.repositories.now()), id)
		if err != nil {
			return mapSQLiteError("sqlite.auth.mark_access_token_used", err)
		}
		return requireAffected(result, "store.auth.mark_access_token_used", "")
	})
}

// RevokeAccessToken revokes an access token owned by a user.
func (s *Store) RevokeAccessToken(ctx context.Context, id, userID string) error {
	return s.repositories.writeTx(ctx, func(query queryer) error {
		result, err := query.ExecContext(ctx, `UPDATE access_tokens SET revoked_at = ? WHERE id = ? AND user_id = ?`, timeToText(s.repositories.now()), id, userID)
		if err != nil {
			return mapSQLiteError("sqlite.auth.revoke_access_token", err)
		}
		return requireAffected(result, "store.auth.revoke_access_token", "token not found")
	})
}

// CreateAccountSetupToken creates a one-time account setup token record.
func (s *Store) CreateAccountSetupToken(ctx context.Context, input store.CreateAccountSetupTokenInput) (*store.AccountSetupToken, error) {
	var token *store.AccountSetupToken
	err := s.repositories.writeTx(ctx, func(query queryer) error {
		id := uuid.NewString()
		now := timeToText(s.repositories.now())
		_, err := query.ExecContext(
			ctx,
			`INSERT INTO account_setup_tokens (id, user_id, token_hash, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
			id,
			input.UserID,
			input.TokenHash,
			now,
			timeToText(input.ExpiresAt),
		)
		if err != nil {
			return mapSQLiteError("store.auth.create_account_setup_token", err)
		}
		token, err = scanAuthAccountSetupToken(query.QueryRowContext(ctx, `SELECT `+accountSetupTokenColumns+` FROM account_setup_tokens WHERE id = ?`, id))
		return err
	})
	if err != nil {
		return nil, err
	}
	return token, nil
}

// FindAccountSetupTokenByHash finds an account setup token by its hash.
func (s *Store) FindAccountSetupTokenByHash(ctx context.Context, tokenHash string) (*store.AccountSetupToken, error) {
	row := s.repositories.query.QueryRowContext(
		ctx,
		`SELECT `+accountSetupTokenColumns+` FROM account_setup_tokens WHERE token_hash = ?`,
		tokenHash,
	)
	token, err := scanAuthAccountSetupToken(row)
	if err != nil {
		return nil, authScanError("store.auth.find_account_setup_token_by_hash", "finding account setup token by hash", err)
	}
	return token, nil
}

// MarkAccountSetupTokenUsed marks an account setup token as used.
func (s *Store) MarkAccountSetupTokenUsed(ctx context.Context, id string) error {
	return s.repositories.writeTx(ctx, func(query queryer) error {
		result, err := query.ExecContext(ctx, `UPDATE account_setup_tokens SET used_at = ? WHERE id = ?`, timeToText(s.repositories.now()), id)
		if err != nil {
			return mapSQLiteError("sqlite.auth.mark_account_setup_token_used", err)
		}
		return requireAffected(result, "store.auth.mark_account_setup_token_used", "")
	})
}

// CompleteAccountSetup updates a user and consumes a valid setup token atomically.
func (s *Store) CompleteAccountSetup(ctx context.Context, input store.CompleteAccountSetupInput) (*store.User, error) {
	var user *store.User
	err := s.repositories.writeTx(ctx, func(query queryer) error {
		now := timeToText(s.repositories.now())
		token, err := scanAuthAccountSetupToken(query.QueryRowContext(ctx, `
			SELECT `+accountSetupTokenColumns+`
			FROM account_setup_tokens
			WHERE token_hash = ? AND used_at IS NULL AND expires_at > ?
		`, input.TokenHash, now))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errors.B.Op("store.auth.complete_account_setup").KindNotFound().Err(err).Build()
			}
			return errors.B.Op("sqlite.auth.complete_account_setup").KindInternal().Text("finding active account setup token").Err(err).Build()
		}

		result, err := query.ExecContext(ctx, `
			UPDATE users
			SET password_hash = ?, display_name = ?, avatar_key = ?, updated_at = ?
			WHERE id = ?
		`, input.PasswordHash, input.DisplayName, input.AvatarKey, now, token.UserID)
		if err != nil {
			return mapSQLiteError("sqlite.auth.complete_account_setup", err)
		}
		if err := requireAffected(result, "store.auth.complete_account_setup", "user not found"); err != nil {
			return err
		}
		result, err = query.ExecContext(ctx, `UPDATE account_setup_tokens SET used_at = ? WHERE id = ? AND used_at IS NULL`, now, token.ID)
		if err != nil {
			return mapSQLiteError("sqlite.auth.complete_account_setup", err)
		}
		if err := requireAffected(result, "store.auth.complete_account_setup", ""); err != nil {
			return err
		}
		user, err = findAuthUserByID(ctx, query, token.UserID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return user, nil
}

func insertAuthUser(ctx context.Context, query queryer, now time.Time, input store.CreateUserInput) (*store.User, error) {
	role := input.Role
	if role == "" {
		role = store.UserRoleUser
	}
	avatarKey := strings.TrimSpace(input.AvatarKey)
	if avatarKey == "" {
		avatarKey = "default"
	}
	id := uuid.NewString()
	timestamp := timeToText(now)
	_, err := query.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, display_name, role, avatar_key, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, id, strings.ToLower(input.Email), input.PasswordHash, input.DisplayName, role, avatarKey, timestamp, timestamp)
	if err != nil {
		mapped := mapSQLiteError("sqlite.auth.insert_user", err)
		if errors.WhatKind(mapped) == errors.Conflict {
			return nil, errors.B.
				Op("store.auth.create_user").
				KindConflict().
				UserMsg("User " + strings.ToLower(strings.TrimSpace(input.Email)) + " already exists.").
				Text("user email conflict").
				Err(err).
				Build()
		}
		return nil, mapped
	}
	return findAuthUserByID(ctx, query, id)
}

func findAuthUserByID(ctx context.Context, query queryer, id string) (*store.User, error) {
	user, err := scanAuthUser(query.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = ?`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.B.Op("store.auth.find_user_by_id").KindNotFound().UserMsg("user not found").Err(err).Build()
		}
		return nil, errors.B.Op("sqlite.auth.find_user_by_id").KindInternal().Text("finding user by id").Err(err).Build()
	}
	return user, nil
}

type authScanner interface {
	Scan(dest ...any) error
}

func scanAuthUser(row authScanner) (*store.User, error) {
	var (
		user                 store.User
		role                 string
		disabledAt           sql.NullString
		createdAt, updatedAt string
	)
	if err := row.Scan(
		&user.ID,
		&user.TenantID,
		&user.Email,
		&user.PasswordHash,
		&user.DisplayName,
		&role,
		&user.AvatarKey,
		&disabledAt,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, err
	}
	user.Role = store.UserRole(role)
	var err error
	user.DisabledAt, err = authNullableTime(disabledAt)
	if err != nil {
		return nil, err
	}
	if user.CreatedAt, err = timeFromText(createdAt); err != nil {
		return nil, err
	}
	if user.UpdatedAt, err = timeFromText(updatedAt); err != nil {
		return nil, err
	}
	return &user, nil
}

func scanAuthSession(row authScanner) (*store.Session, error) {
	var (
		session               store.Session
		createdAt, expiresAt  string
		lastUsedAt, revokedAt sql.NullString
	)
	if err := row.Scan(&session.ID, &session.UserID, &session.TokenHash, &createdAt, &expiresAt, &lastUsedAt, &revokedAt); err != nil {
		return nil, err
	}
	var err error
	if session.CreatedAt, err = timeFromText(createdAt); err != nil {
		return nil, err
	}
	if session.ExpiresAt, err = timeFromText(expiresAt); err != nil {
		return nil, err
	}
	if session.LastUsedAt, err = authNullableTime(lastUsedAt); err != nil {
		return nil, err
	}
	if session.RevokedAt, err = authNullableTime(revokedAt); err != nil {
		return nil, err
	}
	return &session, nil
}

func scanAuthAccessToken(row authScanner) (*store.AccessToken, error) {
	var (
		token                            store.AccessToken
		createdAt                        string
		expiresAt, lastUsedAt, revokedAt sql.NullString
	)
	if err := row.Scan(&token.ID, &token.UserID, &token.Name, &token.TokenHash, &createdAt, &expiresAt, &lastUsedAt, &revokedAt); err != nil {
		return nil, err
	}
	var err error
	if token.CreatedAt, err = timeFromText(createdAt); err != nil {
		return nil, err
	}
	if token.ExpiresAt, err = authNullableTime(expiresAt); err != nil {
		return nil, err
	}
	if token.LastUsedAt, err = authNullableTime(lastUsedAt); err != nil {
		return nil, err
	}
	if token.RevokedAt, err = authNullableTime(revokedAt); err != nil {
		return nil, err
	}
	return &token, nil
}

func scanAuthAccountSetupToken(row authScanner) (*store.AccountSetupToken, error) {
	var (
		token                store.AccountSetupToken
		createdAt, expiresAt string
		usedAt               sql.NullString
	)
	if err := row.Scan(&token.ID, &token.UserID, &token.TokenHash, &createdAt, &expiresAt, &usedAt); err != nil {
		return nil, err
	}
	var err error
	if token.CreatedAt, err = timeFromText(createdAt); err != nil {
		return nil, err
	}
	if token.ExpiresAt, err = timeFromText(expiresAt); err != nil {
		return nil, err
	}
	if token.UsedAt, err = authNullableTime(usedAt); err != nil {
		return nil, err
	}
	return &token, nil
}

func authNullableTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	parsed, err := timeFromText(value.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func optionalTimeToText(value *time.Time) any {
	if value == nil {
		return nil
	}
	return timeToText(*value)
}

func requireAffected(result sql.Result, op, userMessage string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return errors.B.Op(op).KindInternal().Text("checking affected rows").Err(err).Build()
	}
	if affected != 0 {
		return nil
	}
	builder := errors.B.Op(op).KindNotFound()
	if userMessage != "" {
		builder = builder.UserMsg(userMessage)
	}
	return builder.Build()
}

func authScanError(op, text string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return errors.B.Op(op).KindNotFound().Err(err).Build()
	}
	return errors.B.Op("sqlite.auth.scan").KindInternal().Text(text).Err(err).Build()
}
