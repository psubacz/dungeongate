package session

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dungeongate/internal/auth"
)

// AuthMiddleware handles authentication for the session service
type AuthMiddleware struct {
	authClient *auth.AuthServiceClientImpl
	enabled    bool
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(authServiceAddress string, enabled bool) (*AuthMiddleware, error) {
	if !enabled {
		return &AuthMiddleware{enabled: false}, nil
	}

	authClient, err := auth.NewAuthServiceClient(authServiceAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth client: %w", err)
	}

	return &AuthMiddleware{
		authClient: authClient,
		enabled:    true,
	}, nil
}

// Close closes the auth client connection
func (m *AuthMiddleware) Close() error {
	if m.authClient != nil {
		return m.authClient.Close()
	}
	return nil
}

// AuthenticateUser authenticates a user via the auth service
func (m *AuthMiddleware) AuthenticateUser(ctx context.Context, username, password, clientIP string) (*User, error) {
	if !m.enabled {
		return nil, fmt.Errorf("authentication disabled")
	}

	loginReq := &auth.SessionLoginRequest{
		Username: username,
		Password: password,
		ClientIP: clientIP,
	}

	resp, err := m.authClient.Login(ctx, loginReq)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}

	// Convert from SessionUser to session User
	// Convert string ID to int
	userID, err := strconv.Atoi(resp.User.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %v", err)
	}
	return &User{
		ID:              userID,
		Username:        resp.User.Username,
		Email:           resp.User.Email,
		IsAuthenticated: resp.User.IsAuthenticated,
		IsActive:        resp.User.IsActive,
		IsAdmin:         resp.User.IsAdmin,
		CreatedAt:       resp.User.CreatedAt,
		UpdatedAt:       resp.User.UpdatedAt,
		LastLogin:       resp.User.LastLogin,
	}, nil
}

// ValidateToken validates an access token
func (m *AuthMiddleware) ValidateToken(ctx context.Context, token string) (*User, error) {
	if !m.enabled {
		return nil, fmt.Errorf("authentication disabled")
	}

	// Remove Bearer prefix if present
	token = strings.TrimPrefix(token, "Bearer ")

	sessionUser, err := m.authClient.ValidateToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	// Convert from SessionUser to session User
	// Convert string ID to int
	userID, err := strconv.Atoi(sessionUser.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %v", err)
	}
	return &User{
		ID:              userID,
		Username:        sessionUser.Username,
		Email:           sessionUser.Email,
		IsAuthenticated: sessionUser.IsAuthenticated,
		IsActive:        sessionUser.IsActive,
		IsAdmin:         sessionUser.IsAdmin,
		CreatedAt:       sessionUser.CreatedAt,
		UpdatedAt:       sessionUser.UpdatedAt,
		LastLogin:       sessionUser.LastLogin,
	}, nil
}

// RefreshToken refreshes an access token
func (m *AuthMiddleware) RefreshToken(ctx context.Context, refreshToken string) (*auth.SessionLoginResponse, error) {
	if !m.enabled {
		return nil, fmt.Errorf("authentication disabled")
	}

	return m.authClient.RefreshToken(ctx, refreshToken)
}

// Logout logs out a user
func (m *AuthMiddleware) Logout(ctx context.Context, accessToken string) error {
	if !m.enabled {
		return nil
	}

	return m.authClient.Logout(ctx, accessToken)
}

// AuthenticatedSSHHandler wraps SSH session context with authentication
type AuthenticatedSSHHandler struct {
	middleware *AuthMiddleware
	next       func(ctx context.Context, sessionCtx *SSHSessionContext) error
}

// NewAuthenticatedSSHHandler creates a new authenticated SSH handler
func NewAuthenticatedSSHHandler(middleware *AuthMiddleware, next func(ctx context.Context, sessionCtx *SSHSessionContext) error) *AuthenticatedSSHHandler {
	return &AuthenticatedSSHHandler{
		middleware: middleware,
		next:       next,
	}
}

// Handle processes the SSH session with authentication
func (h *AuthenticatedSSHHandler) Handle(ctx context.Context, sessionCtx *SSHSessionContext) error {
	// If already authenticated, proceed
	if sessionCtx.IsAuthenticated {
		return h.next(ctx, sessionCtx)
	}

	// Authentication required - this would be called from the menu system
	// For now, just proceed (authentication happens in the menu)
	return h.next(ctx, sessionCtx)
}

// TokenAuthenticator provides token-based authentication methods
type TokenAuthenticator struct {
	middleware *AuthMiddleware
}

// NewTokenAuthenticator creates a new token authenticator
func NewTokenAuthenticator(middleware *AuthMiddleware) *TokenAuthenticator {
	return &TokenAuthenticator{
		middleware: middleware,
	}
}

// AuthenticateWithToken authenticates a user with a token
func (t *TokenAuthenticator) AuthenticateWithToken(ctx context.Context, token string) (*User, error) {
	return t.middleware.ValidateToken(ctx, token)
}

// AuthenticateWithCredentials authenticates a user with credentials
func (t *TokenAuthenticator) AuthenticateWithCredentials(ctx context.Context, username, password, clientIP string) (*User, string, error) {
	user, err := t.middleware.AuthenticateUser(ctx, username, password, clientIP)
	if err != nil {
		return nil, "", err
	}

	// In a real implementation, we'd return the access token from the login response
	// For now, return empty token
	return user, "", nil
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Enabled            bool          `yaml:"enabled"`
	AuthServiceAddress string        `yaml:"auth_service_address"`
	TokenExpiration    time.Duration `yaml:"token_expiration"`
	RequireTokenForAPI bool          `yaml:"require_token_for_api"`
	RequireTokenForSSH bool          `yaml:"require_token_for_ssh"`
}

// DefaultAuthConfig returns default authentication configuration
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		Enabled:            true,
		AuthServiceAddress: "localhost:8082",
		TokenExpiration:    15 * time.Minute,
		RequireTokenForAPI: false,
		RequireTokenForSSH: false,
	}
}

// HTTPAuthMiddleware provides HTTP authentication middleware
type HTTPAuthMiddleware struct {
	authenticator *TokenAuthenticator
	config        *AuthConfig
}

// NewHTTPAuthMiddleware creates a new HTTP authentication middleware
func NewHTTPAuthMiddleware(authenticator *TokenAuthenticator, config *AuthConfig) *HTTPAuthMiddleware {
	return &HTTPAuthMiddleware{
		authenticator: authenticator,
		config:        config,
	}
}

// AuthenticateHTTPRequest authenticates an HTTP request
func (h *HTTPAuthMiddleware) AuthenticateHTTPRequest(ctx context.Context, authHeader string) (*User, error) {
	if !h.config.Enabled || !h.config.RequireTokenForAPI {
		return nil, nil // Authentication not required
	}

	if authHeader == "" {
		return nil, fmt.Errorf("authorization header required")
	}

	// Extract token from Authorization header
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return nil, fmt.Errorf("authorization header must start with 'Bearer '")
	}

	token := strings.TrimPrefix(authHeader, bearerPrefix)
	if token == "" {
		return nil, fmt.Errorf("empty token in authorization header")
	}

	return h.authenticator.AuthenticateWithToken(ctx, token)
}

// AuthenticationResult holds the result of authentication
type AuthenticationResult struct {
	User         *User
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	Error        string
	ErrorCode    string
}

// LoginResult holds the result of a login attempt
type LoginResult struct {
	Success           bool
	User              *User
	AccessToken       string
	RefreshToken      string
	ExpiresAt         time.Time
	Error             string
	ErrorCode         string
	RemainingAttempts int
	RetryAfter        time.Duration
}

// AuthenticationError represents authentication errors
type AuthenticationError struct {
	Code    string
	Message string
}

func (e *AuthenticationError) Error() string {
	return e.Message
}

// NewAuthenticationError creates a new authentication error
func NewAuthenticationError(code, message string) *AuthenticationError {
	return &AuthenticationError{
		Code:    code,
		Message: message,
	}
}

// Common authentication error codes
const (
	AuthErrorInvalidCredentials = "invalid_credentials"
	AuthErrorUserNotFound       = "user_not_found"
	AuthErrorAccountLocked      = "account_locked"
	AuthErrorTokenExpired       = "token_expired"
	AuthErrorTokenInvalid       = "token_invalid"
	AuthErrorInternalError      = "internal_error"
	AuthErrorServiceUnavailable = "service_unavailable"
)
