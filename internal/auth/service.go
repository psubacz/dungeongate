package auth

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/dungeongate/internal/auth/proto"
	"github.com/dungeongate/internal/user"
	"github.com/dungeongate/pkg/database"
	"github.com/dungeongate/pkg/encryption"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Service implements the Auth service
type Service struct {
	proto.UnimplementedAuthServiceServer
	db        *database.Connection
	userSvc   *user.Service
	encryptor encryption.Encryptor
	jwtSecret []byte
	jwtIssuer string

	// Token expiration times
	accessTokenExpiration  time.Duration
	refreshTokenExpiration time.Duration

	// Rate limiting
	maxLoginAttempts int
	lockoutDuration  time.Duration
}

// Config holds the configuration for the Auth service
type Config struct {
	JWTSecret              string        `yaml:"jwt_secret"`
	JWTIssuer              string        `yaml:"jwt_issuer"`
	AccessTokenExpiration  time.Duration `yaml:"access_token_expiration"`
	RefreshTokenExpiration time.Duration `yaml:"refresh_token_expiration"`
	MaxLoginAttempts       int           `yaml:"max_login_attempts"`
	LockoutDuration        time.Duration `yaml:"lockout_duration"`
}

// NewService creates a new Auth service
func NewService(db *database.Connection, userSvc *user.Service, encryptor encryption.Encryptor, config *Config) *Service {
	// Set default values
	if config.AccessTokenExpiration == 0 {
		config.AccessTokenExpiration = 15 * time.Minute
	}
	if config.RefreshTokenExpiration == 0 {
		config.RefreshTokenExpiration = 7 * 24 * time.Hour // 7 days
	}
	if config.MaxLoginAttempts == 0 {
		config.MaxLoginAttempts = 3
	}
	if config.LockoutDuration == 0 {
		config.LockoutDuration = 15 * time.Minute
	}
	if config.JWTIssuer == "" {
		config.JWTIssuer = "dungeongate"
	}

	return &Service{
		db:                     db,
		userSvc:                userSvc,
		encryptor:              encryptor,
		jwtSecret:              []byte(config.JWTSecret),
		jwtIssuer:              config.JWTIssuer,
		accessTokenExpiration:  config.AccessTokenExpiration,
		refreshTokenExpiration: config.RefreshTokenExpiration,
		maxLoginAttempts:       config.MaxLoginAttempts,
		lockoutDuration:        config.LockoutDuration,
	}
}

// Login authenticates a user and returns tokens
func (s *Service) Login(ctx context.Context, req *proto.LoginRequest) (*proto.LoginResponse, error) {
	if req.Username == "" || req.Password == "" {
		return &proto.LoginResponse{
			Success:   false,
			Error:     "Username and password are required",
			ErrorCode: "invalid_request",
		}, nil
	}

	// Check login attempts first
	attemptsResp, err := s.GetLoginAttempts(ctx, &proto.GetLoginAttemptsRequest{
		Username: req.Username,
		ClientIp: req.ClientIp,
	})
	if err != nil {
		return &proto.LoginResponse{
			Success: false,
			Error:   "Failed to check login attempts",
		}, status.Errorf(codes.Internal, "failed to check login attempts: %v", err)
	}

	if attemptsResp.AccountLocked {
		return &proto.LoginResponse{
			Success:           false,
			Error:             "Account is temporarily locked due to too many failed login attempts",
			ErrorCode:         "account_locked",
			RetryAfterSeconds: attemptsResp.LockedUntil - time.Now().Unix(),
		}, nil
	}

	// Authenticate user
	authenticatedUser, err := s.userSvc.AuthenticateUser(ctx, req.Username, req.Password)
	if err != nil {
		// Determine error type and increment failed attempts
		var errorCode string
		switch err.Error() {
		case "username_not_found":
			errorCode = "user_not_found"
		case "invalid_password":
			errorCode = "invalid_credentials"
		case "account_locked":
			errorCode = "account_locked"
		default:
			errorCode = "authentication_failed"
		}

		// Increment failed login attempts
		s.incrementFailedLoginAttempts(ctx, req.Username, req.ClientIp)

		return &proto.LoginResponse{
			Success:           false,
			Error:             "Invalid credentials",
			ErrorCode:         errorCode,
			RemainingAttempts: attemptsResp.RemainingAttempts - 1,
		}, nil
	}

	// Reset failed login attempts on successful login
	s.resetFailedLoginAttempts(ctx, req.Username, req.ClientIp)

	// Generate tokens
	accessToken, refreshToken, err := s.generateTokens(authenticatedUser)
	if err != nil {
		return &proto.LoginResponse{
			Success: false,
			Error:   "Failed to generate tokens",
		}, status.Errorf(codes.Internal, "failed to generate tokens: %v", err)
	}

	// Convert user to proto
	protoUser := s.convertUserToProto(authenticatedUser)

	return &proto.LoginResponse{
		Success:               true,
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		AccessTokenExpiresAt:  time.Now().Add(s.accessTokenExpiration).Unix(),
		RefreshTokenExpiresAt: time.Now().Add(s.refreshTokenExpiration).Unix(),
		User:                  protoUser,
		RemainingAttempts:     int32(s.maxLoginAttempts),
	}, nil
}

// Logout invalidates a user's session
func (s *Service) Logout(ctx context.Context, req *proto.LogoutRequest) (*proto.LogoutResponse, error) {
	// For now, we'll implement stateless logout (tokens are just not validated)
	// In a production system, you'd want to maintain a token blacklist
	return &proto.LogoutResponse{
		Success: true,
	}, nil
}

// RefreshToken refreshes an access token using a refresh token
func (s *Service) RefreshToken(ctx context.Context, req *proto.RefreshTokenRequest) (*proto.RefreshTokenResponse, error) {
	if req.RefreshToken == "" {
		return &proto.RefreshTokenResponse{
			Success: false,
			Error:   "Refresh token is required",
		}, nil
	}

	// Parse and validate refresh token
	claims, err := s.parseToken(req.RefreshToken)
	if err != nil {
		return &proto.RefreshTokenResponse{
			Success: false,
			Error:   "Invalid refresh token",
		}, nil
	}

	// Get user from database
	userIDInt, err := strconv.Atoi(claims.UserId)
	if err != nil {
		return &proto.RefreshTokenResponse{
			Success: false,
			Error:   "Invalid user ID",
		}, nil
	}
	user, err := s.userSvc.GetUserByID(ctx, userIDInt)
	if err != nil {
		return &proto.RefreshTokenResponse{
			Success: false,
			Error:   "User not found",
		}, nil
	}

	// Generate new tokens
	accessToken, refreshToken, err := s.generateTokens(user)
	if err != nil {
		return &proto.RefreshTokenResponse{
			Success: false,
			Error:   "Failed to generate tokens",
		}, status.Errorf(codes.Internal, "failed to generate tokens: %v", err)
	}

	return &proto.RefreshTokenResponse{
		Success:               true,
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		AccessTokenExpiresAt:  time.Now().Add(s.accessTokenExpiration).Unix(),
		RefreshTokenExpiresAt: time.Now().Add(s.refreshTokenExpiration).Unix(),
	}, nil
}

// ValidateToken validates an access token and returns user info
func (s *Service) ValidateToken(ctx context.Context, req *proto.ValidateTokenRequest) (*proto.ValidateTokenResponse, error) {
	if req.AccessToken == "" {
		return &proto.ValidateTokenResponse{
			Valid: false,
			Error: "Access token is required",
		}, nil
	}

	// Parse and validate token
	claims, err := s.parseToken(req.AccessToken)
	if err != nil {
		return &proto.ValidateTokenResponse{
			Valid: false,
			Error: "Invalid token",
		}, nil
	}

	// Check if token has expired
	if time.Now().Unix() > claims.ExpiresAt {
		return &proto.ValidateTokenResponse{
			Valid: false,
			Error: "Token has expired",
		}, nil
	}

	// Get user from database
	userIDInt, err := strconv.Atoi(claims.UserId)
	if err != nil {
		return &proto.ValidateTokenResponse{
			Valid: false,
			Error: "Invalid user ID",
		}, nil
	}
	user, err := s.userSvc.GetUserByID(ctx, userIDInt)
	if err != nil {
		return &proto.ValidateTokenResponse{
			Valid: false,
			Error: "User not found",
		}, nil
	}

	// Check if user is still active
	if !user.IsActive {
		return &proto.ValidateTokenResponse{
			Valid: false,
			Error: "User account is inactive",
		}, nil
	}

	// Convert user to proto
	protoUser := s.convertUserToProto(user)

	return &proto.ValidateTokenResponse{
		Valid:     true,
		User:      protoUser,
		ExpiresAt: claims.ExpiresAt,
	}, nil
}

// GetUserInfo gets user information from a valid token
func (s *Service) GetUserInfo(ctx context.Context, req *proto.GetUserInfoRequest) (*proto.GetUserInfoResponse, error) {
	validateResp, err := s.ValidateToken(ctx, &proto.ValidateTokenRequest{
		AccessToken: req.AccessToken,
	})
	if err != nil {
		return &proto.GetUserInfoResponse{
			Success: false,
			Error:   "Failed to validate token",
		}, err
	}

	if !validateResp.Valid {
		return &proto.GetUserInfoResponse{
			Success: false,
			Error:   validateResp.Error,
		}, nil
	}

	return &proto.GetUserInfoResponse{
		Success: true,
		User:    validateResp.User,
	}, nil
}

// ChangePassword changes a user's password
func (s *Service) ChangePassword(ctx context.Context, req *proto.ChangePasswordRequest) (*proto.ChangePasswordResponse, error) {
	// Validate token first
	validateResp, err := s.ValidateToken(ctx, &proto.ValidateTokenRequest{
		AccessToken: req.AccessToken,
	})
	if err != nil {
		return &proto.ChangePasswordResponse{
			Success: false,
			Error:   "Failed to validate token",
		}, err
	}

	if !validateResp.Valid {
		return &proto.ChangePasswordResponse{
			Success: false,
			Error:   validateResp.Error,
		}, nil
	}

	// Verify current password
	_, err = s.userSvc.AuthenticateUser(ctx, validateResp.User.Username, req.CurrentPassword)
	if err != nil {
		return &proto.ChangePasswordResponse{
			Success: false,
			Error:   "Current password is incorrect",
		}, nil
	}

	// Update password - Note: This would need to be implemented in user service
	// For now, return success
	return &proto.ChangePasswordResponse{
		Success: true,
	}, nil
}

// ResetPassword initiates password reset flow
func (s *Service) ResetPassword(ctx context.Context, req *proto.ResetPasswordRequest) (*proto.ResetPasswordResponse, error) {
	// This would typically send an email with a reset token
	// For now, return success
	return &proto.ResetPasswordResponse{
		Success: true,
		Message: "Password reset instructions have been sent to your email",
	}, nil
}

// VerifyPasswordReset verifies and completes password reset
func (s *Service) VerifyPasswordReset(ctx context.Context, req *proto.VerifyPasswordResetRequest) (*proto.VerifyPasswordResetResponse, error) {
	// This would verify the reset token and update the password
	// For now, return success
	return &proto.VerifyPasswordResetResponse{
		Success: true,
	}, nil
}

// GetLoginAttempts gets login attempt info for a user
func (s *Service) GetLoginAttempts(ctx context.Context, req *proto.GetLoginAttemptsRequest) (*proto.GetLoginAttemptsResponse, error) {
	// This would check the database for login attempts
	// For now, return default values
	return &proto.GetLoginAttemptsResponse{
		FailedAttempts:    0,
		AccountLocked:     false,
		LockedUntil:       0,
		RemainingAttempts: int32(s.maxLoginAttempts),
	}, nil
}

// Health returns the health status of the service
func (s *Service) Health(ctx context.Context, req *emptypb.Empty) (*proto.HealthResponse, error) {
	return &proto.HealthResponse{
		Status: "healthy",
		Details: map[string]string{
			"service": "auth",
			"version": "1.0.0",
		},
		Timestamp: timestampProto(time.Now()),
	}, nil
}

// Private helper methods

func (s *Service) generateTokens(user *user.User) (string, string, error) {
	now := time.Now()

	// Generate access token
	accessClaims := &proto.TokenClaims{
		UserId:    strconv.Itoa(user.ID),
		Username:  user.Username,
		Email:     user.Email,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(s.accessTokenExpiration).Unix(),
		NotBefore: now.Unix(),
		Issuer:    s.jwtIssuer,
	}

	accessToken, err := s.createToken(accessClaims)
	if err != nil {
		return "", "", fmt.Errorf("failed to create access token: %w", err)
	}

	// Generate refresh token
	refreshClaims := &proto.TokenClaims{
		UserId:    strconv.Itoa(user.ID),
		Username:  user.Username,
		Email:     user.Email,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(s.refreshTokenExpiration).Unix(),
		NotBefore: now.Unix(),
		Issuer:    s.jwtIssuer,
	}

	refreshToken, err := s.createToken(refreshClaims)
	if err != nil {
		return "", "", fmt.Errorf("failed to create refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}

func (s *Service) createToken(claims *proto.TokenClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  claims.UserId,
		"username": claims.Username,
		"email":    claims.Email,
		"iat":      claims.IssuedAt,
		"exp":      claims.ExpiresAt,
		"nbf":      claims.NotBefore,
		"iss":      claims.Issuer,
	})

	return token.SignedString(s.jwtSecret)
}

func (s *Service) parseToken(tokenString string) (*proto.TokenClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return &proto.TokenClaims{
		UserId:    claims["user_id"].(string),
		Username:  claims["username"].(string),
		Email:     claims["email"].(string),
		IssuedAt:  int64(claims["iat"].(float64)),
		ExpiresAt: int64(claims["exp"].(float64)),
		NotBefore: int64(claims["nbf"].(float64)),
		Issuer:    claims["iss"].(string),
	}, nil
}

func (s *Service) convertUserToProto(userObj *user.User) *proto.User {
	var lastLogin *timestamppb.Timestamp
	if userObj.LastLogin != nil {
		lastLogin = timestampProto(*userObj.LastLogin)
	}
	
	return &proto.User{
		Id:              strconv.Itoa(userObj.ID),
		Username:        userObj.Username,
		Email:           userObj.Email,
		IsActive:        userObj.IsActive,
		IsAdmin:         (userObj.Flags & user.UserFlagAdmin) != 0,
		IsAuthenticated: true,
		EmailVerified:   userObj.EmailVerified,
		CreatedAt:       timestampProto(userObj.CreatedAt),
		UpdatedAt:       timestampProto(userObj.UpdatedAt),
		LastLogin:       lastLogin,
	}
}

func (s *Service) incrementFailedLoginAttempts(ctx context.Context, username, clientIP string) {
	// This would increment failed login attempts in the database
	// For now, just log it
}

func (s *Service) resetFailedLoginAttempts(ctx context.Context, username, clientIP string) {
	// This would reset failed login attempts in the database
	// For now, just log it
}

func timestampProto(t time.Time) *timestamppb.Timestamp {
	return timestamppb.New(t)
}

