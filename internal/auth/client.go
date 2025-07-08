package auth

import (
	"context"
	"fmt"
	"time"

	proto "github.com/dungeongate/pkg/api/auth/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Client is a gRPC client for the Auth service
type Client struct {
	conn   *grpc.ClientConn
	client proto.AuthServiceClient
}

// NewClient creates a new Auth service client
func NewClient(address string) (*Client, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to auth service: %w", err)
	}

	client := proto.NewAuthServiceClient(conn)

	return &Client{
		conn:   conn,
		client: client,
	}, nil
}

// Close closes the gRPC connection
func (c *Client) Close() error {
	return c.conn.Close()
}

// Login authenticates a user
func (c *Client) Login(ctx context.Context, username, password, clientIP string) (*proto.LoginResponse, error) {
	return c.client.Login(ctx, &proto.LoginRequest{
		Username: username,
		Password: password,
		ClientIp: clientIP,
	})
}

// Logout logs out a user
func (c *Client) Logout(ctx context.Context, accessToken, refreshToken string) error {
	_, err := c.client.Logout(ctx, &proto.LogoutRequest{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
	return err
}

// ValidateToken validates an access token
func (c *Client) ValidateToken(ctx context.Context, accessToken string) (*proto.ValidateTokenResponse, error) {
	return c.client.ValidateToken(ctx, &proto.ValidateTokenRequest{
		AccessToken: accessToken,
	})
}

// RefreshToken refreshes an access token
func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (*proto.RefreshTokenResponse, error) {
	return c.client.RefreshToken(ctx, &proto.RefreshTokenRequest{
		RefreshToken: refreshToken,
	})
}

// GetUserInfo gets user information from a token
func (c *Client) GetUserInfo(ctx context.Context, accessToken string) (*proto.GetUserInfoResponse, error) {
	return c.client.GetUserInfo(ctx, &proto.GetUserInfoRequest{
		AccessToken: accessToken,
	})
}

// Health checks the health of the auth service
func (c *Client) Health(ctx context.Context) (*proto.HealthResponse, error) {
	return c.client.Health(ctx, &emptypb.Empty{})
}

// AuthServiceClientImpl implements the AuthServiceClient interface for session service
type AuthServiceClientImpl struct {
	client *Client
}

// NewAuthServiceClient creates a new AuthServiceClient implementation
func NewAuthServiceClient(authServiceAddress string) (*AuthServiceClientImpl, error) {
	client, err := NewClient(authServiceAddress)
	if err != nil {
		return nil, err
	}

	return &AuthServiceClientImpl{
		client: client,
	}, nil
}

// Close closes the client connection
func (a *AuthServiceClientImpl) Close() error {
	return a.client.Close()
}

// Login implements the session service AuthServiceClient interface
func (a *AuthServiceClientImpl) Login(ctx context.Context, req *SessionLoginRequest) (*SessionLoginResponse, error) {
	protoResp, err := a.client.Login(ctx, req.Username, req.Password, req.ClientIP)
	if err != nil {
		return nil, err
	}

	// Convert proto response to session service types
	resp := &SessionLoginResponse{
		Success:      protoResp.Success,
		Error:        protoResp.Error,
		AccessToken:  protoResp.AccessToken,
		RefreshToken: protoResp.RefreshToken,
		ExpiresAt:    time.Unix(protoResp.AccessTokenExpiresAt, 0),
	}

	if protoResp.User != nil {
		sessionUser := &SessionUser{
			ID:              protoResp.User.Id,
			Username:        protoResp.User.Username,
			Email:           protoResp.User.Email,
			IsAuthenticated: protoResp.User.IsAuthenticated,
			IsActive:        protoResp.User.IsActive,
			IsAdmin:         protoResp.User.IsAdmin,
			CreatedAt:       time.Unix(protoResp.User.CreatedAt.Seconds, int64(protoResp.User.CreatedAt.Nanos)),
			UpdatedAt:       time.Unix(protoResp.User.UpdatedAt.Seconds, int64(protoResp.User.UpdatedAt.Nanos)),
		}

		// Handle LastLogin safely - it can be nil for newly registered users
		if protoResp.User.LastLogin != nil {
			sessionUser.LastLogin = time.Unix(protoResp.User.LastLogin.Seconds, int64(protoResp.User.LastLogin.Nanos))
		}

		resp.User = sessionUser
	}

	return resp, nil
}

// Logout implements the session service AuthServiceClient interface
func (a *AuthServiceClientImpl) Logout(ctx context.Context, token string) error {
	return a.client.Logout(ctx, token, "")
}

// ValidateToken implements the session service AuthServiceClient interface
func (a *AuthServiceClientImpl) ValidateToken(ctx context.Context, token string) (*SessionUser, error) {
	protoResp, err := a.client.ValidateToken(ctx, token)
	if err != nil {
		return nil, err
	}

	if !protoResp.Valid {
		return nil, fmt.Errorf("invalid token: %s", protoResp.Error)
	}

	if protoResp.User == nil {
		return nil, fmt.Errorf("no user information in token response")
	}

	return &SessionUser{
		ID:              protoResp.User.Id,
		Username:        protoResp.User.Username,
		Email:           protoResp.User.Email,
		IsAuthenticated: protoResp.User.IsAuthenticated,
		IsActive:        protoResp.User.IsActive,
		IsAdmin:         protoResp.User.IsAdmin,
		CreatedAt:       time.Unix(protoResp.User.CreatedAt.Seconds, int64(protoResp.User.CreatedAt.Nanos)),
		UpdatedAt:       time.Unix(protoResp.User.UpdatedAt.Seconds, int64(protoResp.User.UpdatedAt.Nanos)),
		LastLogin:       time.Unix(protoResp.User.LastLogin.Seconds, int64(protoResp.User.LastLogin.Nanos)),
	}, nil
}

// RefreshToken implements the session service AuthServiceClient interface
func (a *AuthServiceClientImpl) RefreshToken(ctx context.Context, refreshToken string) (*SessionLoginResponse, error) {
	protoResp, err := a.client.RefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, err
	}

	return &SessionLoginResponse{
		Success:      protoResp.Success,
		Error:        protoResp.Error,
		AccessToken:  protoResp.AccessToken,
		RefreshToken: protoResp.RefreshToken,
		ExpiresAt:    time.Unix(protoResp.AccessTokenExpiresAt, 0),
	}, nil
}

// Types for session service compatibility
type SessionLoginRequest struct {
	Username string
	Password string
	ClientIP string
}

type SessionLoginResponse struct {
	Success      bool
	Error        string
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	User         *SessionUser
}

type SessionUser struct {
	ID              string
	Username        string
	Email           string
	IsAuthenticated bool
	IsActive        bool
	IsAdmin         bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
	LastLogin       time.Time
}
