package client

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	authv1 "github.com/dungeongate/pkg/api/auth/v1"
)

// AuthClient provides stateless access to Auth Service
type AuthClient struct {
	conn   *grpc.ClientConn
	client authv1.AuthServiceClient
	logger *slog.Logger
}

// NewAuthClient creates a new Auth Service client
func NewAuthClient(address string, logger *slog.Logger) (*AuthClient, error) {
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to auth service: %w", err)
	}

	client := authv1.NewAuthServiceClient(conn)

	return &AuthClient{
		conn:   conn,
		client: client,
		logger: logger,
	}, nil
}

// Close closes the client connection
func (c *AuthClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ValidateToken validates a JWT token
func (c *AuthClient) ValidateToken(ctx context.Context, token string) (*authv1.ValidateTokenResponse, error) {
	req := &authv1.ValidateTokenRequest{
		AccessToken: token,
	}

	resp, err := c.client.ValidateToken(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to validate token: %w", err)
	}

	return resp, nil
}

// Login authenticates a user with username/password
func (c *AuthClient) Login(ctx context.Context, username, password string) (*authv1.LoginResponse, error) {
	req := &authv1.LoginRequest{
		Username: username,
		Password: password,
	}

	resp, err := c.client.Login(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to login user: %w", err)
	}

	return resp, nil
}

// GetUserInfo retrieves user information
func (c *AuthClient) GetUserInfo(ctx context.Context, token string) (*authv1.GetUserInfoResponse, error) {
	req := &authv1.GetUserInfoRequest{
		AccessToken: token,
	}

	resp, err := c.client.GetUserInfo(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	return resp, nil
}

// Register creates a new user account
func (c *AuthClient) Register(ctx context.Context, username, password, email string) (*authv1.RegisterResponse, error) {
	req := &authv1.RegisterRequest{
		Username: username,
		Password: password,
		Email:    email,
	}

	resp, err := c.client.Register(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to register user: %w", err)
	}

	return resp, nil
}

// ChangePassword changes a user's password
func (c *AuthClient) ChangePassword(ctx context.Context, req *authv1.ChangePasswordRequest) (*authv1.ChangePasswordResponse, error) {
	resp, err := c.client.ChangePassword(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to change password: %w", err)
	}

	return resp, nil
}

// IsHealthy checks if the auth service is available and healthy
func (c *AuthClient) IsHealthy(ctx context.Context) bool {
	// Use a simple ping mechanism - try to call an endpoint that should always be available
	// We'll use ValidateToken with an empty token which should return an error but indicates service is up
	_, err := c.ValidateToken(ctx, "")

	// Service is healthy if we get any response (even an error response means the service is up)
	// Only connection-level errors indicate the service is down
	if err != nil {
		// Check if this is a gRPC connection error
		errStr := err.Error()
		if containsSubstring(errStr, "connection refused") ||
			containsSubstring(errStr, "no such host") ||
			containsSubstring(errStr, "connection error") ||
			containsSubstring(errStr, "transport") {
			return false
		}
	}

	return true
}

// Admin Functions

// UnlockUserAccount unlocks a user account (admin only)
func (c *AuthClient) UnlockUserAccount(ctx context.Context, adminToken, targetUsername string) (*authv1.AdminActionResponse, error) {
	req := &authv1.AdminActionRequest{
		AdminToken:     adminToken,
		TargetUsername: targetUsername,
	}

	resp, err := c.client.UnlockUserAccount(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to unlock user account: %w", err)
	}

	return resp, nil
}

// DeleteUserAccount deletes a user account (admin only)
func (c *AuthClient) DeleteUserAccount(ctx context.Context, adminToken, targetUsername string) (*authv1.AdminActionResponse, error) {
	req := &authv1.AdminActionRequest{
		AdminToken:     adminToken,
		TargetUsername: targetUsername,
	}

	resp, err := c.client.DeleteUserAccount(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to delete user account: %w", err)
	}

	return resp, nil
}

// ResetUserPassword resets a user's password (admin only)
func (c *AuthClient) ResetUserPassword(ctx context.Context, adminToken, targetUsername, newPassword string) (*authv1.AdminActionResponse, error) {
	req := &authv1.ResetPasswordAdminRequest{
		AdminToken:     adminToken,
		TargetUsername: targetUsername,
		NewPassword:    newPassword,
	}

	resp, err := c.client.ResetUserPassword(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to reset user password: %w", err)
	}

	return resp, nil
}

// PromoteUserToAdmin promotes a user to admin status (admin only)
func (c *AuthClient) PromoteUserToAdmin(ctx context.Context, adminToken, targetUsername string) (*authv1.AdminActionResponse, error) {
	req := &authv1.AdminActionRequest{
		AdminToken:     adminToken,
		TargetUsername: targetUsername,
	}

	resp, err := c.client.PromoteUserToAdmin(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to promote user to admin: %w", err)
	}

	return resp, nil
}

// GetServerStatistics returns server statistics (admin only)
func (c *AuthClient) GetServerStatistics(ctx context.Context, adminToken string) (*authv1.ServerStatsResponse, error) {
	req := &authv1.ServerStatsRequest{
		AdminToken: adminToken,
	}

	resp, err := c.client.GetServerStatistics(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get server statistics: %w", err)
	}

	return resp, nil
}

// containsSubstring checks if a string contains a substring
func containsSubstring(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
