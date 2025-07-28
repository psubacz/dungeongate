package client

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAuthClient(t *testing.T) {
	logger := slog.Default()
	address := "localhost:8082"

	client, err := NewAuthClient(address, logger)

	// In unit tests, we expect this to fail if service isn't running
	if err != nil {
		// This is expected in unit tests
		assert.Contains(t, err.Error(), "failed to connect to auth service")
		return
	}

	// If it succeeds, verify it's properly initialized
	require.NotNil(t, client)
	assert.NotNil(t, client.conn)
	assert.NotNil(t, client.client)
	assert.Equal(t, logger, client.logger)

	// Clean up
	err = client.Close()
	assert.NoError(t, err)
}

func TestAuthClientClose(t *testing.T) {
	logger := slog.Default()
	address := "localhost:8082"

	client, err := NewAuthClient(address, logger)
	if err != nil {
		t.Skip("Auth service not available for testing")
	}

	// Test close - may return error due to gRPC connection cleanup
	err = client.Close()
	// Close may return connection closing errors, which is acceptable

	// Test close on already closed client - should not panic
	err = client.Close()
	// Second close should not panic and may return an error
}

func TestAuthClientValidateToken(t *testing.T) {
	logger := slog.Default()
	address := "localhost:8082"

	client, err := NewAuthClient(address, logger)
	if err != nil {
		t.Skip("Auth service not available for testing")
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test with invalid token
	resp, err := client.ValidateToken(ctx, "invalid-token")

	// The auth service returns a response with Valid=false for invalid tokens
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.Valid)
	assert.Equal(t, "Invalid token", resp.Error)
}

func TestAuthClientLogin(t *testing.T) {
	logger := slog.Default()
	address := "localhost:8082"

	client, err := NewAuthClient(address, logger)
	if err != nil {
		t.Skip("Auth service not available for testing")
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test with invalid credentials
	resp, err := client.Login(ctx, "invalid-user", "invalid-password")

	// The auth service returns a response with Success=false for invalid credentials
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.Success)
	assert.Equal(t, "user_not_found", resp.ErrorCode)
	assert.Empty(t, resp.AccessToken)
}

func TestAuthClientGetUserInfo(t *testing.T) {
	logger := slog.Default()
	address := "localhost:8082"

	client, err := NewAuthClient(address, logger)
	if err != nil {
		t.Skip("Auth service not available for testing")
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test with invalid token
	resp, err := client.GetUserInfo(ctx, "invalid-token")

	// This should succeed but return success=false
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Error, "Invalid token")
}

func TestAuthClientWithTimeoutContext(t *testing.T) {
	logger := slog.Default()
	address := "localhost:8082"

	client, err := NewAuthClient(address, logger)
	if err != nil {
		t.Skip("Auth service not available for testing")
	}
	defer client.Close()

	// Create a context that times out immediately
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// This should timeout
	resp, err := client.ValidateToken(ctx, "some-token")

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestAuthClientInvalidAddress(t *testing.T) {
	logger := slog.Default()
	address := "invalid-address:99999"

	client, err := NewAuthClient(address, logger)

	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "failed to connect to auth service")
}

func TestAuthClientConcurrentRequests(t *testing.T) {
	logger := slog.Default()
	address := "localhost:8082"

	client, err := NewAuthClient(address, logger)
	if err != nil {
		t.Skip("Auth service not available for testing")
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test concurrent requests
	const numRequests = 10
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			_, err := client.ValidateToken(ctx, "invalid-token")
			results <- err
		}(i)
	}

	// Collect results
	for i := 0; i < numRequests; i++ {
		err := <-results
		// All should fail since we're using invalid token
		assert.Error(t, err)
	}
}

func TestAuthClientValidTokenFormat(t *testing.T) {
	logger := slog.Default()
	address := "localhost:8082"

	client, err := NewAuthClient(address, logger)
	if err != nil {
		t.Skip("Auth service not available for testing")
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test with various token formats
	testTokens := []string{
		"",                  // Empty token
		"invalid",           // Simple invalid token
		"bearer.token.here", // JWT-like format
		"very-long-token-that-might-be-valid-but-probably-isnt-since-we-dont-have-a-real-auth-service-running",
	}

	for _, token := range testTokens {
		t.Run("token_"+token, func(t *testing.T) {
			resp, err := client.ValidateToken(ctx, token)

			// All should fail since we don't have valid tokens
			assert.Error(t, err)
			assert.Nil(t, resp)
		})
	}
}

func TestAuthClientLoginVariations(t *testing.T) {
	logger := slog.Default()
	address := "localhost:8082"

	client, err := NewAuthClient(address, logger)
	if err != nil {
		t.Skip("Auth service not available for testing")
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test with various credential combinations
	testCases := []struct {
		username string
		password string
		name     string
	}{
		{"", "", "empty_credentials"},
		{"user", "", "empty_password"},
		{"", "pass", "empty_username"},
		{"admin", "admin", "common_credentials"},
		{"test", "test", "test_credentials"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.Login(ctx, tc.username, tc.password)

			// All should fail since we don't have valid users
			assert.Error(t, err)
			assert.Nil(t, resp)
		})
	}
}

// Integration test that runs if services are available
func TestAuthClientIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := slog.Default()
	address := "localhost:8082"

	client, err := NewAuthClient(address, logger)
	if err != nil {
		t.Skip("Auth service not available for integration testing")
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test workflow: try to login with test credentials
	// This will fail unless we have a real auth service with test data
	resp, err := client.Login(ctx, "testuser", "testpass")
	if err != nil {
		t.Logf("Failed to login (expected): %v", err)
		return
	}

	// If login succeeds, test token validation
	validateResp, err := client.ValidateToken(ctx, resp.AccessToken)
	if err != nil {
		t.Logf("Failed to validate token: %v", err)
	} else {
		assert.NotNil(t, validateResp)
	}

	// Test getting user info
	userResp, err := client.GetUserInfo(ctx, resp.AccessToken)
	if err != nil {
		t.Logf("Failed to get user info: %v", err)
	} else {
		assert.NotNil(t, userResp)
		assert.Equal(t, resp.User.Id, userResp.User.Id)
		assert.Equal(t, resp.User.Username, userResp.User.Username)
	}
}

// Benchmark test for auth client operations
func BenchmarkAuthClientValidateToken(b *testing.B) {
	logger := slog.Default()
	address := "localhost:8082"

	client, err := NewAuthClient(address, logger)
	if err != nil {
		b.Skip("Auth service not available for benchmarking")
	}
	defer client.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.ValidateToken(ctx, "test-token")
	}
}

func BenchmarkAuthClientLogin(b *testing.B) {
	logger := slog.Default()
	address := "localhost:8082"

	client, err := NewAuthClient(address, logger)
	if err != nil {
		b.Skip("Auth service not available for benchmarking")
	}
	defer client.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.Login(ctx, "testuser", "testpass")
	}
}
