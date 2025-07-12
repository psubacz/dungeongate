package auth

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/dungeongate/internal/user"
	proto "github.com/dungeongate/pkg/api/auth/v1"
	"github.com/dungeongate/pkg/config"
	"github.com/dungeongate/pkg/database"
	"github.com/dungeongate/pkg/encryption"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestService(t *testing.T) (*Service, *database.Connection, func()) {
	// Setup test database
	dbConfig := &config.DatabaseConfig{
		Mode: config.DatabaseModeEmbedded,
		Type: "sqlite",
		Embedded: &config.EmbeddedDBConfig{
			Type: "sqlite",
			Path: ":memory:",
		},
	}

	db, err := database.NewConnection(dbConfig)
	require.NoError(t, err)

	// Setup encryption
	encryptionConfig := &config.EncryptionConfig{
		Enabled:             true,
		Algorithm:           "AES-256-GCM",
		KeyRotationInterval: "24h",
	}

	encryptor, err := encryption.New(encryptionConfig)
	require.NoError(t, err)

	// Setup user service
	userServiceConfig := &config.UserServiceConfig{
		Database: dbConfig,
	}
	sessionServiceConfig := config.GetDefaultDevelopmentConfig()

	userService, err := user.NewService(db, userServiceConfig, sessionServiceConfig)
	require.NoError(t, err)

	// Setup logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Reduce noise in tests
	}))

	// Setup auth service
	authConfig := &Config{
		JWTSecret:              "test-secret-key-for-testing-only",
		JWTIssuer:              "dungeongate-test",
		AccessTokenExpiration:  15 * time.Minute,
		RefreshTokenExpiration: 24 * time.Hour,
		MaxLoginAttempts:       3,
		LockoutDuration:        15 * time.Minute,
	}

	authService := NewService(db, userService, *encryptor, authConfig, logger)

	cleanup := func() {
		db.Close()
	}

	return authService, db, cleanup
}

func TestService_Register_Success(t *testing.T) {
	service, _, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	req := &proto.RegisterRequest{
		Username:  "testuser",
		Password:  "testpass123",
		Email:     "test@example.com",
		ClientIp:  "127.0.0.1",
		UserAgent: "test-agent",
	}

	resp, err := service.Register(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Check success response
	assert.True(t, resp.Success)
	assert.Empty(t, resp.Error)
	assert.Empty(t, resp.ErrorCode)

	// Check tokens are generated
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, resp.RefreshToken)
	assert.Greater(t, resp.AccessTokenExpiresAt, time.Now().Unix())
	assert.Greater(t, resp.RefreshTokenExpiresAt, time.Now().Unix())

	// Check user info
	require.NotNil(t, resp.User)
	assert.Equal(t, "testuser", resp.User.Username)
	assert.Equal(t, "test@example.com", resp.User.Email)
	assert.True(t, resp.User.IsActive)
	assert.True(t, resp.User.IsAuthenticated)
	assert.NotEmpty(t, resp.User.Id)
}

func TestService_Register_MissingUsername(t *testing.T) {
	service, _, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	req := &proto.RegisterRequest{
		Username: "", // Missing username
		Password: "testpass123",
		Email:    "test@example.com",
	}

	resp, err := service.Register(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Check failure response
	assert.False(t, resp.Success)
	assert.Equal(t, "Username and password are required", resp.Error)
	assert.Equal(t, "invalid_request", resp.ErrorCode)

	// Check no tokens generated
	assert.Empty(t, resp.AccessToken)
	assert.Empty(t, resp.RefreshToken)
	assert.Nil(t, resp.User)
}

func TestService_Register_MissingPassword(t *testing.T) {
	service, _, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	req := &proto.RegisterRequest{
		Username: "testuser",
		Password: "", // Missing password
		Email:    "test@example.com",
	}

	resp, err := service.Register(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Check failure response
	assert.False(t, resp.Success)
	assert.Equal(t, "Username and password are required", resp.Error)
	assert.Equal(t, "invalid_request", resp.ErrorCode)
}

func TestService_Register_MissingEmail(t *testing.T) {
	service, _, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	req := &proto.RegisterRequest{
		Username: "testuser",
		Password: "testpass123",
		Email:    "", // Missing email
	}

	resp, err := service.Register(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Check failure response
	assert.False(t, resp.Success)
	assert.Equal(t, "Email is required", resp.Error)
	assert.Equal(t, "invalid_request", resp.ErrorCode)
}

func TestService_Register_DuplicateUsername(t *testing.T) {
	service, _, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// First registration
	req1 := &proto.RegisterRequest{
		Username: "testuser",
		Password: "testpass123",
		Email:    "test1@example.com",
	}

	resp1, err := service.Register(ctx, req1)
	require.NoError(t, err)
	require.True(t, resp1.Success)

	// Second registration with same username
	req2 := &proto.RegisterRequest{
		Username: "testuser", // Same username
		Password: "testpass456",
		Email:    "test2@example.com",
	}

	resp2, err := service.Register(ctx, req2)
	require.NoError(t, err)
	require.NotNil(t, resp2)

	// Check failure response
	assert.False(t, resp2.Success)
	assert.Contains(t, resp2.Error, "already exists")
	assert.Equal(t, "username_taken", resp2.ErrorCode)
}

func TestService_Register_InvalidPassword(t *testing.T) {
	service, _, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	req := &proto.RegisterRequest{
		Username: "testuser",
		Password: "123", // Too short
		Email:    "test@example.com",
	}

	resp, err := service.Register(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Check failure response
	assert.False(t, resp.Success)
	assert.Equal(t, "invalid_password", resp.ErrorCode)
}

func TestService_Register_InvalidEmail(t *testing.T) {
	service, _, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	req := &proto.RegisterRequest{
		Username: "testuser",
		Password: "testpass123",
		Email:    "invalid-email", // Invalid email format
	}

	resp, err := service.Register(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Check failure response
	assert.False(t, resp.Success)
	assert.Equal(t, "invalid_email", resp.ErrorCode)
}

func TestService_Register_TokenGeneration(t *testing.T) {
	service, _, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	req := &proto.RegisterRequest{
		Username: "testuser",
		Password: "testpass123",
		Email:    "test@example.com",
	}

	resp, err := service.Register(ctx, req)
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Validate that we can use the generated token
	validateReq := &proto.ValidateTokenRequest{
		AccessToken: resp.AccessToken,
	}

	validateResp, err := service.ValidateToken(ctx, validateReq)
	require.NoError(t, err)
	require.NotNil(t, validateResp)

	assert.True(t, validateResp.Valid)
	assert.Equal(t, resp.User.Id, validateResp.User.Id)
	assert.Equal(t, resp.User.Username, validateResp.User.Username)
}

func TestService_Register_LoginAfterRegistration(t *testing.T) {
	service, _, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Register user
	regReq := &proto.RegisterRequest{
		Username: "testuser",
		Password: "testpass123",
		Email:    "test@example.com",
	}

	regResp, err := service.Register(ctx, regReq)
	require.NoError(t, err)
	require.True(t, regResp.Success)

	// Try to login with same credentials
	loginReq := &proto.LoginRequest{
		Username: "testuser",
		Password: "testpass123",
	}

	loginResp, err := service.Login(ctx, loginReq)
	require.NoError(t, err)
	require.NotNil(t, loginResp)

	assert.True(t, loginResp.Success)
	assert.Equal(t, regResp.User.Id, loginResp.User.Id)
	assert.Equal(t, regResp.User.Username, loginResp.User.Username)
}
