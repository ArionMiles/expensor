package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/auth"
	"github.com/ArionMiles/expensor/backend/internal/store"
)

const (
	sessionCookieName  = "expensor_session"
	sessionTokenPrefix = "expensor_session"
	accessTokenPrefix  = "expensor_pat"
	setupTokenPrefix   = "expensor_setup"
	sessionTTL         = 30 * 24 * time.Hour
	setupTokenTTL      = 24 * time.Hour
)

type bootstrapRequest struct {
	Email       string `json:"email" validate:"required,email" example:"new-admin@example.com"`
	Password    string `json:"password" validate:"required,min=12" example:"correct horse battery staple"`
	DisplayName string `json:"display_name" validate:"required,no_control_chars" example:"New Admin"`
	AvatarKey   string `json:"avatar_key" validate:"omitempty,no_control_chars,oneof=default ledger wallet" example:"default"`
}

type loginRequest struct {
	Email    string `json:"email" validate:"required,email" example:"component-admin@example.com"`
	Password string `json:"password" validate:"required" example:"component admin password"`
}

type createAccessTokenRequest struct {
	Name string `json:"name" validate:"required,no_control_chars" example:"contract"`
}

type updateProfileRequest struct {
	DisplayName *string `json:"display_name" validate:"omitempty,no_control_chars" example:"New Name"`
	AvatarKey   *string `json:"avatar_key" validate:"omitempty,no_control_chars" example:"wallet" enums:"default,ledger,wallet"`
}

type updatePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required" example:"component admin password"`
	NewPassword     string `json:"new_password" validate:"required,min=12" example:"correct horse battery staple"`
}

type createUserRequest struct {
	Email string `json:"email" validate:"required,email" example:"contract-user@example.com"`
	Role  string `json:"role" validate:"omitempty,oneof=admin user" example:"user"`
}

type updateUserRequest struct {
	DisplayName *string `json:"display_name" validate:"omitempty,no_control_chars" example:"Contract User"`
	Role        *string `json:"role" validate:"omitempty,no_control_chars" example:"user" enums:"admin,user"`
	AvatarKey   *string `json:"avatar_key" validate:"omitempty,no_control_chars" example:"wallet" enums:"default,ledger,wallet"`
	Disabled    *bool   `json:"disabled" example:"false"`
}

type completeAccountSetupRequest struct {
	Token       string `json:"token" validate:"required" example:"expensor_setup_invalid"`
	DisplayName string `json:"display_name" validate:"required,no_control_chars" example:"Contract User"`
	Password    string `json:"password" validate:"required,min=12" example:"correct horse battery staple"`
	AvatarKey   string `json:"avatar_key" validate:"omitempty,no_control_chars,oneof=default ledger wallet" example:"default"`
}

type bootstrapResponse struct {
	Required bool `json:"required"`
}

type principalResponse struct {
	UserID      string `json:"user_id"`
	TenantID    string `json:"tenant_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	AvatarKey   string `json:"avatar_key"`
}

type accessTokenResponse struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Token      string     `json:"token,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

type userResponse struct {
	UserID       string     `json:"user_id"`
	TenantID     string     `json:"tenant_id"`
	Email        string     `json:"email"`
	DisplayName  string     `json:"display_name"`
	Role         string     `json:"role"`
	AvatarKey    string     `json:"avatar_key"`
	SetupPending bool       `json:"setup_pending"`
	DisabledAt   *time.Time `json:"disabled_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type accountSetupResponse struct {
	Email     string `json:"email"`
	AvatarKey string `json:"avatar_key"`
}

type setupTokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// GetBootstrap reports whether first-admin bootstrap is available.
// @Summary Get bootstrap status
// @Tags Auth
// @Produce json
// @Success 200 {object} bootstrapResponse
// @Failure 500 {object} ErrorResponse
// @Router /bootstrap [get]
func (h *Handlers) GetBootstrap(w http.ResponseWriter, r *http.Request) {
	required, err := h.authStore.BootstrapRequired(r.Context())
	if err != nil {
		h.logger.Error("bootstrap status", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to check bootstrap status")
		return
	}
	writeJSON(w, http.StatusOK, bootstrapResponse{Required: required})
}

// Bootstrap creates the first administrator account.
// @Summary Create the first administrator account
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body bootstrapRequest true "Bootstrap account"
// @Success 201 {object} principalResponse
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /bootstrap [post]
func (h *Handlers) Bootstrap(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeAndValidateJSON[bootstrapRequest](h, w, r)
	if !ok {
		return
	}
	passwordHash, err := auth.HashPassword(body.Password)
	if err != nil {
		h.logger.Error("hash bootstrap password", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create admin")
		return
	}
	user, err := h.authStore.CreateBootstrapAdmin(r.Context(), store.CreateBootstrapAdminInput{
		Email:        strings.ToLower(strings.TrimSpace(body.Email)),
		DisplayName:  strings.TrimSpace(body.DisplayName),
		PasswordHash: passwordHash,
		AvatarKey:    normalizeAvatarKey(body.AvatarKey),
	})
	if err != nil {
		if errors.Is(err, store.ErrBootstrapUnavailable) {
			writeError(w, http.StatusConflict, "bootstrap unavailable")
			return
		}
		h.logger.Error("create bootstrap admin", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create admin")
		return
	}
	if !h.createSessionCookie(w, r, user) {
		return
	}
	writeJSON(w, http.StatusCreated, principalFromUser(user))
}

// Login creates a browser session.
// @Summary Create a browser session
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body loginRequest true "Login credentials"
// @Success 201 {object} principalResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /session [post]
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeAndValidateJSON[loginRequest](h, w, r)
	if !ok {
		return
	}
	user, err := h.authStore.FindUserByEmail(r.Context(), strings.ToLower(strings.TrimSpace(body.Email)))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if user.DisabledAt != nil || auth.VerifyPassword(user.PasswordHash, body.Password) != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if !h.createSessionCookie(w, r, user) {
		return
	}
	writeJSON(w, http.StatusCreated, principalFromUser(user))
}

// GetSession returns the current authenticated user.
// @Summary Get the current session
// @Tags Auth
// @Produce json
// @Success 200 {object} principalResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /session [get]
func (h *Handlers) GetSession(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, principalFromUser(user))
}

// Logout revokes the current browser session cookie.
// @Summary Delete the current browser session
// @Tags Auth
// @Success 204
// @Failure 500 {object} ErrorResponse
// @Router /session [delete]
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		session, findErr := h.authStore.FindSessionByHash(r.Context(), auth.HashOpaqueToken(cookie.Value))
		if findErr == nil {
			if err := h.authStore.RevokeSession(r.Context(), session.ID); err != nil && !errors.Is(err, store.ErrNotFound) {
				h.logger.Error("revoke session", "error", err)
				writeError(w, http.StatusInternalServerError, "failed to log out")
				return
			}
		}
	}
	clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

// GetProfile returns the current authenticated user profile.
// @Summary Get the current user profile
// @Tags Auth
// @Produce json
// @Success 200 {object} principalResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /profile [get]
func (h *Handlers) GetProfile(w http.ResponseWriter, r *http.Request) {
	h.GetSession(w, r)
}

// UpdateProfile updates the current authenticated user profile.
// @Summary Update the current user profile
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body updateProfileRequest true "Profile update"
// @Success 200 {object} principalResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /profile [patch]
func (h *Handlers) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	body, ok := decodeAndValidateJSON[updateProfileRequest](h, w, r)
	if !ok {
		return
	}
	input, ok := updateUserInputFromProfile(w, body)
	if !ok {
		return
	}
	user, err := h.authStore.UpdateUser(r.Context(), principal.UserID, input)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		h.logger.Error("update profile", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update profile")
		return
	}
	writeJSON(w, http.StatusOK, principalFromUser(user))
}

// UpdatePassword updates the current authenticated user's password.
// @Summary Update the current user password
// @Tags Auth
// @Accept json
// @Param request body updatePasswordRequest true "Password update"
// @Success 204
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /profile/password [patch]
func (h *Handlers) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	body, ok := decodeAndValidateJSON[updatePasswordRequest](h, w, r)
	if !ok {
		return
	}
	if auth.VerifyPassword(user.PasswordHash, body.CurrentPassword) != nil {
		writeError(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}
	passwordHash, err := auth.HashPassword(body.NewPassword)
	if err != nil {
		h.logger.Error("hash updated password", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update password")
		return
	}
	if err := h.authStore.UpdateUserPassword(r.Context(), user.ID, store.UpdateUserPasswordInput{PasswordHash: passwordHash}); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		h.logger.Error("update password", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update password")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CreateAccessToken creates a programmatic access token.
// @Summary Create a programmatic access token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body createAccessTokenRequest true "Access token metadata"
// @Success 201 {object} accessTokenResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /tokens [post]
func (h *Handlers) CreateAccessToken(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	body, ok := decodeAndValidateJSON[createAccessTokenRequest](h, w, r)
	if !ok {
		return
	}
	raw, hash, err := auth.NewOpaqueToken(accessTokenPrefix)
	if err != nil {
		h.logger.Error("generate access token", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create token")
		return
	}
	token, err := h.authStore.CreateAccessToken(r.Context(), store.CreateAccessTokenInput{
		UserID:    principal.UserID,
		Name:      strings.TrimSpace(body.Name),
		TokenHash: hash,
	})
	if err != nil {
		if errors.Is(err, store.ErrAccessTokenNameConflict) {
			writeError(w, http.StatusConflict, fmt.Sprintf("Token %s already exists.", strings.TrimSpace(body.Name)))
			return
		}
		h.logger.Error("create access token", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create token")
		return
	}
	resp := accessTokenFromStore(token)
	resp.Token = raw
	writeJSON(w, http.StatusCreated, resp)
}

// ListAccessTokens returns programmatic access token metadata for the current user.
// @Summary List programmatic access tokens
// @Tags Auth
// @Produce json
// @Success 200 {array} accessTokenResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /tokens [get]
func (h *Handlers) ListAccessTokens(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	tokens, err := h.authStore.ListAccessTokens(r.Context(), principal.UserID)
	if err != nil {
		h.logger.Error("list access tokens", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list tokens")
		return
	}
	resp := make([]accessTokenResponse, 0, len(tokens))
	for _, token := range tokens {
		resp = append(resp, accessTokenFromStore(&token))
	}
	writeJSON(w, http.StatusOK, resp)
}

// RevokeAccessToken revokes one of the current user's programmatic access tokens.
// @Summary Revoke a programmatic access token
// @Tags Auth
// @Param id path string true "Token ID" example(00000000-0000-0000-0000-00000000c0de)
// @Success 204
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /tokens/{id} [delete]
func (h *Handlers) RevokeAccessToken(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if err := h.authStore.RevokeAccessToken(r.Context(), r.PathValue("id"), principal.UserID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "token not found")
			return
		}
		h.logger.Error("revoke access token", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to revoke token")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CreateUser creates an instance user.
// @Summary Create an instance user
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body createUserRequest true "User account"
// @Success 201 {object} principalResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /admin/users [post]
func (h *Handlers) CreateUser(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	body, ok := decodeAndValidateJSON[createUserRequest](h, w, r)
	if !ok {
		return
	}
	role := store.UserRole(body.Role)
	if role == "" {
		role = store.UserRoleUser
	}
	user, err := h.authStore.CreateUser(r.Context(), store.CreateUserInput{
		Email:     strings.ToLower(strings.TrimSpace(body.Email)),
		Role:      role,
		AvatarKey: "default",
	})
	if err != nil {
		if errors.Is(err, store.ErrUserEmailConflict) {
			writeError(w, http.StatusConflict, fmt.Sprintf("User %s already exists.", strings.ToLower(strings.TrimSpace(body.Email))))
			return
		}
		h.logger.Error("create user", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}
	writeJSON(w, http.StatusCreated, userFromStore(user))
}

// ListUsers returns instance users for administrators.
// @Summary List instance users
// @Tags Auth
// @Produce json
// @Success 200 {array} userResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /admin/users [get]
func (h *Handlers) ListUsers(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	users, err := h.authStore.ListUsers(r.Context())
	if err != nil {
		h.logger.Error("list users", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	resp := make([]userResponse, 0, len(users))
	for _, user := range users {
		resp = append(resp, userFromStore(&user))
	}
	writeJSON(w, http.StatusOK, resp)
}

// UpdateUser updates an instance user for administrators.
// @Summary Update an instance user
// @Tags Auth
// @Accept json
// @Produce json
// @Param id path string true "User ID" example(00000000-0000-0000-0000-00000000c0de)
// @Param request body updateUserRequest true "User update"
// @Success 200 {object} userResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /admin/users/{id} [patch]
func (h *Handlers) UpdateUser(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	body, ok := decodeAndValidateJSON[updateUserRequest](h, w, r)
	if !ok {
		return
	}
	input, ok := updateUserInputFromAdmin(w, body)
	if !ok {
		return
	}
	userID := r.PathValue("id")
	if input.Role != nil && userID == principal.UserID {
		writeError(w, http.StatusForbidden, "cannot change your own role")
		return
	}
	if input.Disabled != nil && *input.Disabled && userID == principal.UserID {
		writeError(w, http.StatusForbidden, "cannot disable your own account")
		return
	}
	user, err := h.authStore.UpdateUser(r.Context(), userID, input)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		h.logger.Error("update user", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update user")
		return
	}
	writeJSON(w, http.StatusOK, userFromStore(user))
}

// DeleteUser deletes an instance user for administrators.
// @Summary Delete an instance user
// @Tags Auth
// @Param id path string true "User ID" example(00000000-0000-0000-0000-00000000c0de)
// @Success 204
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /admin/users/{id} [delete]
func (h *Handlers) DeleteUser(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	userID := r.PathValue("id")
	if userID == principal.UserID {
		writeError(w, http.StatusForbidden, "cannot delete your own account")
		return
	}
	if err := h.authStore.DeleteUser(r.Context(), userID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		h.logger.Error("delete user", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CreateSetupToken creates a one-time setup token for an instance user.
// @Summary Create a user setup token
// @Tags Auth
// @Produce json
// @Param id path string true "User ID" example(00000000-0000-0000-0000-00000000c0de)
// @Success 201 {object} setupTokenResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /admin/users/{id}/setup-tokens [post]
func (h *Handlers) CreateSetupToken(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	userID := r.PathValue("id")
	user, err := h.authStore.FindUserByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		h.logger.Error("find setup token user", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create setup token")
		return
	}
	if strings.TrimSpace(user.PasswordHash) != "" {
		writeError(w, http.StatusConflict, "account already set up")
		return
	}
	raw, hash, err := auth.NewOpaqueToken(setupTokenPrefix)
	if err != nil {
		h.logger.Error("generate setup token", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create setup token")
		return
	}
	expiresAt := time.Now().Add(setupTokenTTL)
	token, err := h.authStore.CreateAccountSetupToken(r.Context(), store.CreateAccountSetupTokenInput{
		UserID:    userID,
		TokenHash: hash,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		h.logger.Error("create setup token", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create setup token")
		return
	}
	writeJSON(w, http.StatusCreated, setupTokenResponse{Token: raw, ExpiresAt: token.ExpiresAt})
}

// GetAccountSetup returns public setup metadata for a valid one-time setup token.
// @Summary Get account setup metadata
// @Tags Auth
// @Produce json
// @Param token query string true "Setup token" example(expensor_setup_invalid)
// @Success 200 {object} accountSetupResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /account-setup [get]
func (h *Handlers) GetAccountSetup(w http.ResponseWriter, r *http.Request) {
	user, ok := h.accountSetupUserFromToken(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, accountSetupResponse{
		Email:     user.Email,
		AvatarKey: normalizeAvatarKey(user.AvatarKey),
	})
}

// CompleteAccountSetup sets a password from a one-time setup token.
// @Summary Complete account setup
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body completeAccountSetupRequest true "Setup token and password"
// @Success 201 {object} principalResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /account-setup [post]
func (h *Handlers) CompleteAccountSetup(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeAndValidateJSON[completeAccountSetupRequest](h, w, r)
	if !ok {
		return
	}
	passwordHash, err := auth.HashPassword(body.Password)
	if err != nil {
		h.logger.Error("hash account setup password", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to complete account setup")
		return
	}
	user, err := h.authStore.CompleteAccountSetup(r.Context(), store.CompleteAccountSetupInput{
		TokenHash:    auth.HashOpaqueToken(strings.TrimSpace(body.Token)),
		PasswordHash: passwordHash,
		DisplayName:  strings.TrimSpace(body.DisplayName),
		AvatarKey:    normalizeAvatarKey(body.AvatarKey),
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "invalid or expired setup token")
			return
		}
		h.logger.Error("complete account setup", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to complete account setup")
		return
	}
	if !h.createSessionCookie(w, r, user) {
		return
	}
	writeJSON(w, http.StatusCreated, principalFromUser(user))
}

func (h *Handlers) accountSetupUserFromToken(w http.ResponseWriter, r *http.Request) (*store.User, bool) {
	tokenValue := strings.TrimSpace(r.URL.Query().Get("token"))
	if tokenValue == "" {
		writeError(w, http.StatusUnauthorized, "invalid or expired setup token")
		return nil, false
	}
	setupToken, err := h.authStore.FindAccountSetupTokenByHash(r.Context(), auth.HashOpaqueToken(tokenValue))
	if err != nil || setupToken.UsedAt != nil || !setupToken.ExpiresAt.After(time.Now()) {
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			h.logger.Error("find account setup token", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to fetch account setup")
			return nil, false
		}
		writeError(w, http.StatusUnauthorized, "invalid or expired setup token")
		return nil, false
	}
	user, err := h.authStore.FindUserByID(r.Context(), setupToken.UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "invalid or expired setup token")
			return nil, false
		}
		h.logger.Error("find setup token user", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch account setup")
		return nil, false
	}
	if strings.TrimSpace(user.PasswordHash) != "" {
		writeError(w, http.StatusUnauthorized, "invalid or expired setup token")
		return nil, false
	}
	return user, true
}

func (h *Handlers) createSessionCookie(w http.ResponseWriter, r *http.Request, user *store.User) bool {
	raw, hash, err := auth.NewOpaqueToken(sessionTokenPrefix)
	if err != nil {
		h.logger.Error("generate session token", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return false
	}
	expiresAt := time.Now().Add(sessionTTL)
	if _, err := h.authStore.CreateSession(r.Context(), store.CreateSessionInput{
		UserID:    user.ID,
		TokenHash: hash,
		ExpiresAt: expiresAt,
	}); err != nil {
		h.logger.Error("create session", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return false
	}
	http.SetCookie(w, sessionCookie(r, raw, expiresAt))
	return true
}

func (h *Handlers) currentUser(w http.ResponseWriter, r *http.Request) (*store.User, bool) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return nil, false
	}
	user, err := h.authStore.FindUserByID(r.Context(), principal.UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return nil, false
		}
		h.logger.Error("get current user", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch current user")
		return nil, false
	}
	return user, true
}

func sessionCookie(r *http.Request, value string, expiresAt time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	}
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	cookie := sessionCookie(r, "", time.Unix(0, 0))
	cookie.MaxAge = -1
	http.SetCookie(w, cookie)
}

func principalFromUser(user *store.User) principalResponse {
	return principalResponse{
		UserID:      user.ID,
		TenantID:    user.TenantID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Role:        string(user.Role),
		AvatarKey:   user.AvatarKey,
	}
}

func userFromStore(user *store.User) userResponse {
	return userResponse{
		UserID:       user.ID,
		TenantID:     user.TenantID,
		Email:        user.Email,
		DisplayName:  user.DisplayName,
		Role:         string(user.Role),
		AvatarKey:    user.AvatarKey,
		SetupPending: strings.TrimSpace(user.PasswordHash) == "",
		DisabledAt:   user.DisabledAt,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}
}

func accessTokenFromStore(token *store.AccessToken) accessTokenResponse {
	return accessTokenResponse{
		ID:         token.ID,
		Name:       token.Name,
		CreatedAt:  token.CreatedAt,
		ExpiresAt:  token.ExpiresAt,
		LastUsedAt: token.LastUsedAt,
	}
}

func updateUserInputFromProfile(w http.ResponseWriter, request updateProfileRequest) (store.UpdateUserInput, bool) {
	var input store.UpdateUserInput
	var details []ValidationErrorDetail
	applyProfileUserUpdates(request.DisplayName, request.AvatarKey, &input, &details)
	if len(details) > 0 {
		writeValidationErrors(w, details)
		return store.UpdateUserInput{}, false
	}
	return input, true
}

func updateUserInputFromAdmin(w http.ResponseWriter, request updateUserRequest) (store.UpdateUserInput, bool) {
	var input store.UpdateUserInput
	var details []ValidationErrorDetail
	applyProfileUserUpdates(request.DisplayName, request.AvatarKey, &input, &details)
	if request.Role != nil {
		role := store.UserRole(strings.TrimSpace(*request.Role))
		switch role {
		case store.UserRoleAdmin, store.UserRoleUser:
			input.Role = &role
		default:
			details = append(details, ValidationErrorDetail{Field: "role", Location: "body", Message: "must be one of: admin, user"})
		}
	}
	input.Disabled = request.Disabled
	if len(details) > 0 {
		writeValidationErrors(w, details)
		return store.UpdateUserInput{}, false
	}
	return input, true
}

func applyProfileUserUpdates(
	displayNameValue *string,
	avatarKeyValue *string,
	input *store.UpdateUserInput,
	details *[]ValidationErrorDetail,
) {
	if displayNameValue != nil {
		displayName := strings.TrimSpace(*displayNameValue)
		if displayName == "" {
			*details = append(*details, ValidationErrorDetail{Field: "display_name", Location: "body", Message: "is required"})
		}
		input.DisplayName = &displayName
	}
	if avatarKeyValue != nil {
		avatarKey := strings.TrimSpace(*avatarKeyValue)
		if !ValidAvatarKey(avatarKey) {
			*details = append(*details, ValidationErrorDetail{
				Field:    "avatar_key",
				Location: "body",
				Message:  "must be one of: default, ledger, wallet",
			})
		}
		input.AvatarKey = &avatarKey
	}
}

func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return false
	}
	if principal.Role != auth.RoleAdmin {
		writeError(w, http.StatusForbidden, "admin role required")
		return false
	}
	return true
}
