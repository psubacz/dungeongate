package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/dungeongate/internal/session-old/client"
	"github.com/dungeongate/internal/session/pools"
	"github.com/dungeongate/internal/session/resources"
	"github.com/dungeongate/internal/session-old/terminal"
	authv1 "github.com/dungeongate/pkg/api/auth/v1"
	"golang.org/x/crypto/ssh"
)

// AuthHandler handles authentication workflows with resource limits
type AuthHandler struct {
	authClient      *client.AuthClient
	resourceLimiter *resources.ResourceLimiter
	workerPool      *pools.WorkerPool
	logger          *slog.Logger

	// Metrics
	loginAttempts    *resources.CounterMetric
	loginSuccesses   *resources.CounterMetric
	loginFailures    *resources.CounterMetric
	loginDuration    *resources.HistogramMetric
	registerAttempts *resources.CounterMetric
	registerSuccesses *resources.CounterMetric
	registerFailures *resources.CounterMetric
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(
	authClient *client.AuthClient,
	resourceLimiter *resources.ResourceLimiter,
	workerPool *pools.WorkerPool,
	metricsRegistry *resources.MetricsRegistry,
	logger *slog.Logger,
) *AuthHandler {
	ah := &AuthHandler{
		authClient:      authClient,
		resourceLimiter: resourceLimiter,
		workerPool:      workerPool,
		logger:          logger,
	}

	ah.initializeMetrics(metricsRegistry)
	return ah
}

// initializeMetrics sets up metrics for the auth handler
func (ah *AuthHandler) initializeMetrics(registry *resources.MetricsRegistry) {
	ah.loginAttempts = registry.RegisterCounter(
		"session_auth_login_attempts_total",
		"Total number of login attempts",
		map[string]string{"handler": "auth"})

	ah.loginSuccesses = registry.RegisterCounter(
		"session_auth_login_successes_total",
		"Total number of successful logins",
		map[string]string{"handler": "auth"})

	ah.loginFailures = registry.RegisterCounter(
		"session_auth_login_failures_total",
		"Total number of failed logins",
		map[string]string{"handler": "auth"})

	ah.loginDuration = registry.RegisterHistogram(
		"session_auth_login_duration_seconds",
		"Time spent processing login requests",
		nil,
		map[string]string{"handler": "auth"})

	ah.registerAttempts = registry.RegisterCounter(
		"session_auth_register_attempts_total",
		"Total number of registration attempts",
		map[string]string{"handler": "auth"})

	ah.registerSuccesses = registry.RegisterCounter(
		"session_auth_register_successes_total",
		"Total number of successful registrations",
		map[string]string{"handler": "auth"})

	ah.registerFailures = registry.RegisterCounter(
		"session_auth_register_failures_total",
		"Total number of failed registrations",
		map[string]string{"handler": "auth"})
}

// HandleLogin handles authentication workflows with resource limits
func (ah *AuthHandler) HandleLogin(ctx context.Context, conn *pools.Connection, channel ssh.Channel) error {
	startTime := time.Now()
	ah.loginAttempts.Inc()
	defer func() {
		duration := time.Since(startTime)
		ah.loginDuration.Observe(duration.Seconds())
	}()

	// Check resource limits before allowing login
	if !ah.resourceLimiter.CanExecute(conn.UserID, "login") {
		ah.logger.Warn("Login blocked by resource limiter",
			"user_id", conn.UserID,
			"connection_id", conn.ID)
		ah.loginFailures.Inc()
		channel.Write([]byte("Rate limit exceeded. Please try again later.\r\n"))
		time.Sleep(2 * time.Second)
		return fmt.Errorf("rate limit exceeded")
	}

	// Clear screen for login form
	channel.Write([]byte("\033[2J\033[H"))
	channel.Write([]byte("\r\n=== Login ===\r\n\r\n"))

	// Get username
	channel.Write([]byte("Username: "))
	username, err := ah.readLineWithTerminal(ctx, channel)
	if err != nil {
		if err.Error() == "user cancelled" {
			channel.Write([]byte("\r\nLogin cancelled.\r\n"))
			time.Sleep(1 * time.Second)
			return nil
		}
		ah.loginFailures.Inc()
		return err
	}

	// Get password (hidden input)
	channel.Write([]byte("Password: "))
	password, err := ah.readPasswordWithTerminal(ctx, channel)
	if err != nil {
		if err.Error() == "user cancelled" {
			channel.Write([]byte("\r\nLogin cancelled.\r\n"))
			time.Sleep(1 * time.Second)
			return nil
		}
		ah.loginFailures.Inc()
		return err
	}

	// Attempt login with auth service
	resp, err := ah.authClient.Login(ctx, username, password)
	if err != nil {
		ah.logger.Warn("Login failed",
			"username", username,
			"error", err,
			"connection_id", conn.ID)
		ah.loginFailures.Inc()
		channel.Write([]byte("\r\nLogin failed. Please check your credentials.\r\n"))
		time.Sleep(2 * time.Second)
		return nil
	}

	// Check if response is valid
	if resp == nil || resp.User == nil {
		ah.logger.Error("Invalid login response",
			"username", username,
			"connection_id", conn.ID)
		ah.loginFailures.Inc()
		channel.Write([]byte("\r\nLogin failed. Server error.\r\n"))
		time.Sleep(2 * time.Second)
		return nil
	}

	// Login successful - store access token in SSH connection
	if conn.SSHConn.Permissions == nil {
		conn.SSHConn.Permissions = &ssh.Permissions{}
	}
	if conn.SSHConn.Permissions.Extensions == nil {
		conn.SSHConn.Permissions.Extensions = make(map[string]string)
	}
	conn.SSHConn.Permissions.Extensions["access_token"] = resp.AccessToken

	// Update connection with user info
	conn.UserID = resp.User.Id
	conn.Username = resp.User.Username

	ah.logger.Info("User logged in successfully",
		"username", username,
		"user_id", resp.User.Id,
		"connection_id", conn.ID)
	ah.loginSuccesses.Inc()

	channel.Write([]byte("\r\nLogin successful! Welcome back to the gate, " + resp.User.Username + "\r\n"))
	time.Sleep(1 * time.Second)

	return nil
}

// HandleRegister handles the registration process with resource limits
func (ah *AuthHandler) HandleRegister(ctx context.Context, conn *pools.Connection, channel ssh.Channel) error {
	startTime := time.Now()
	ah.registerAttempts.Inc()

	// Check resource limits before allowing registration
	if !ah.resourceLimiter.CanExecute(conn.UserID, "register") {
		ah.logger.Warn("Registration blocked by resource limiter",
			"user_id", conn.UserID,
			"connection_id", conn.ID)
		ah.registerFailures.Inc()
		channel.Write([]byte("Rate limit exceeded. Please try again later.\r\n"))
		time.Sleep(2 * time.Second)
		return fmt.Errorf("rate limit exceeded")
	}

	// Clear screen for registration form
	channel.Write([]byte("\033[2J\033[H"))
	channel.Write([]byte("\r\n=== Registration ===\r\n\r\n"))

	// Get username
	channel.Write([]byte("Choose a username: "))
	username, err := ah.readLineWithTerminal(ctx, channel)
	if err != nil {
		if err.Error() == "user cancelled" {
			channel.Write([]byte("\r\nRegistration cancelled.\r\n"))
			time.Sleep(1 * time.Second)
			return nil
		}
		ah.registerFailures.Inc()
		return err
	}

	// Get password
	channel.Write([]byte("Choose a password: "))
	password, err := ah.readPasswordWithTerminal(ctx, channel)
	if err != nil {
		if err.Error() == "user cancelled" {
			channel.Write([]byte("\r\nRegistration cancelled.\r\n"))
			time.Sleep(1 * time.Second)
			return nil
		}
		ah.registerFailures.Inc()
		return err
	}

	// Confirm password
	channel.Write([]byte("Confirm password: "))
	confirmPassword, err := ah.readPasswordWithTerminal(ctx, channel)
	if err != nil {
		if err.Error() == "user cancelled" {
			channel.Write([]byte("\r\nRegistration cancelled.\r\n"))
			time.Sleep(1 * time.Second)
			return nil
		}
		ah.registerFailures.Inc()
		return err
	}

	// Check if passwords match
	if password != confirmPassword {
		channel.Write([]byte("\r\nPasswords do not match.\r\n"))
		ah.registerFailures.Inc()
		return ah.handleRegistrationRetry(ctx, channel, "password mismatch")
	}

	// Get email (optional)
	channel.Write([]byte("Email (optional - leave blank to skip): "))
	email, err := ah.readOptionalLineWithTerminal(ctx, channel)
	if err != nil {
		ah.registerFailures.Inc()
		return err
	}

	// Attempt registration with auth service
	resp, err := ah.authClient.Register(ctx, username, password, email)
	if err != nil {
		ah.logger.Warn("Registration failed",
			"username", username,
			"error", err,
			"connection_id", conn.ID)
		ah.registerFailures.Inc()
		channel.Write([]byte("\r\nRegistration failed. Please try again later.\r\n"))
		return ah.handleRegistrationRetry(ctx, channel, "network error")
	}

	if !resp.Success {
		ah.logger.Warn("Registration rejected",
			"username", username,
			"error", resp.Error,
			"error_code", resp.ErrorCode,
			"connection_id", conn.ID)

		ah.registerFailures.Inc()
		detailedMessage := ah.getDetailedValidationMessage(resp.ErrorCode, resp.Error)
		channel.Write([]byte("\r\nRegistration failed:\r\n" + detailedMessage))
		return ah.handleRegistrationRetry(ctx, channel, resp.Error)
	}

	// Registration successful - store access token in SSH connection
	if conn.SSHConn.Permissions == nil {
		conn.SSHConn.Permissions = &ssh.Permissions{}
	}
	if conn.SSHConn.Permissions.Extensions == nil {
		conn.SSHConn.Permissions.Extensions = make(map[string]string)
	}
	conn.SSHConn.Permissions.Extensions["access_token"] = resp.AccessToken

	// Update connection with user info
	conn.UserID = resp.User.Id
	conn.Username = resp.User.Username

	ah.logger.Info("User registered successfully",
		"username", username,
		"user_id", resp.User.Id,
		"connection_id", conn.ID)
	ah.registerSuccesses.Inc()

	channel.Write([]byte("\r\nRegistration successful! Welcome, " + resp.User.Username + "!\r\n"))
	channel.Write([]byte("You are now logged in.\r\n"))

	duration := time.Since(startTime)
	ah.logger.Info("Registration completed",
		"username", username,
		"duration", duration,
		"connection_id", conn.ID)

	time.Sleep(1 * time.Second)
	return nil
}

// ValidateToken validates a token with the auth service
func (ah *AuthHandler) ValidateToken(ctx context.Context, token string) (*authv1.User, error) {
	resp, err := ah.authClient.GetUserInfo(ctx, token)
	if err != nil {
		return nil, err
	}
	return resp.User, nil
}

// GetUserInfo retrieves user information from the auth service
func (ah *AuthHandler) GetUserInfo(ctx context.Context, sshConn *ssh.ServerConn) (*authv1.User, error) {
	// Get the access token from SSH permissions (set during authentication)
	permissions := sshConn.Permissions
	if permissions == nil || permissions.Extensions == nil {
		return nil, fmt.Errorf("no authentication token available")
	}

	accessToken, ok := permissions.Extensions["access_token"]
	if !ok || accessToken == "" {
		return nil, fmt.Errorf("no access token in session")
	}

	// Validate token with auth service
	return ah.ValidateToken(ctx, accessToken)
}

// CheckServiceHealth checks the health of the auth service
func (ah *AuthHandler) CheckServiceHealth(ctx context.Context) error {
	if !ah.authClient.IsHealthy(ctx) {
		return fmt.Errorf("auth service unavailable")
	}
	return nil
}

// PasswordCallback handles password authentication for SSH
func (ah *AuthHandler) PasswordCallback(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
	username := conn.User()
	ah.loginAttempts.Inc()

	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		ah.loginDuration.Observe(duration.Seconds())
	}()

	// Authenticate with auth service
	ctx := context.Background()
	resp, err := ah.authClient.Login(ctx, username, string(password))
	if err != nil {
		ah.logger.Warn("SSH password authentication failed",
			"username", username,
			"error", err)
		ah.loginFailures.Inc()
		return nil, fmt.Errorf("authentication failed")
	}

	// Validate response
	if resp == nil || resp.User == nil {
		ah.logger.Warn("SSH password authentication failed: empty response",
			"username", username)
		ah.loginFailures.Inc()
		return nil, fmt.Errorf("authentication failed")
	}

	// Store user info in permissions
	permissions := &ssh.Permissions{
		Extensions: map[string]string{
			"user_id":      resp.User.Id,
			"username":     resp.User.Username,
			"access_token": resp.AccessToken,
		},
	}

	ah.loginSuccesses.Inc()
	ah.logger.Info("SSH password authentication successful",
		"username", username,
		"user_id", resp.User.Id)

	return permissions, nil
}

// PublicKeyCallback handles public key authentication for SSH
func (ah *AuthHandler) PublicKeyCallback(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	// For now, reject public key authentication
	username := ""
	if conn != nil {
		username = conn.User()
	}
	ah.logger.Debug("Public key authentication attempted", "username", username)
	return nil, fmt.Errorf("public key authentication not supported")
}

// Helper functions using the terminal package
func (ah *AuthHandler) readLineWithTerminal(ctx context.Context, channel ssh.Channel) (string, error) {
	editor := terminal.NewLineEditor(channel, terminal.InputTypeText)
	return editor.ReadLine(ctx)
}

func (ah *AuthHandler) readPasswordWithTerminal(ctx context.Context, channel ssh.Channel) (string, error) {
	editor := terminal.NewLineEditor(channel, terminal.InputTypePassword)
	return editor.ReadLine(ctx)
}

func (ah *AuthHandler) readOptionalLineWithTerminal(ctx context.Context, channel ssh.Channel) (string, error) {
	editor := terminal.NewLineEditor(channel, terminal.InputTypeOptional)
	return editor.ReadLine(ctx)
}

// getDetailedValidationMessage provides specific validation feedback
func (ah *AuthHandler) getDetailedValidationMessage(errorCode, errorMessage string) string {
	ah.logger.Debug("Registration validation error",
		"error_code", errorCode,
		"error_message", errorMessage)

	switch errorCode {
	case "invalid_password":
		return "Password validation failed:\r\n" +
			"  • Password must be at least 6 characters long\r\n" +
			"  • Please choose a stronger password\r\n"
	case "username_taken":
		return "Username is already taken. Please choose a different username.\r\n"
	case "invalid_username":
		return "Username can only contain letters, numbers, and underscores.\r\n"
	case "invalid_email":
		return "Invalid email format. Please enter a valid email address or leave blank.\r\n"
	case "invalid_request":
		if strings.Contains(errorMessage, "Username") {
			return "Username and password are required fields.\r\n"
		}
		return errorMessage + "\r\n"
	case "registration_failed":
		if strings.Contains(errorMessage, "Validation failed") {
			return "Registration validation failed. Please check your input:\r\n" +
				"  • Username must be unique\r\n" +
				"  • Password must be at least 6 characters long\r\n" +
				"  • Email format must be valid (if provided)\r\n"
		}
		return errorMessage + "\r\n"
	default:
		return fmt.Sprintf("%s\r\n(Error code: %s)\r\n", errorMessage, errorCode)
	}
}

// handleRegistrationRetry gives user options after registration failure
func (ah *AuthHandler) handleRegistrationRetry(ctx context.Context, channel ssh.Channel, errorReason string) error {
	channel.Write([]byte("\r\n"))
	channel.Write([]byte("Options:\r\n"))
	channel.Write([]byte("  [r] Try registration again\r\n"))
	channel.Write([]byte("  [m] Return to main menu\r\n"))
	channel.Write([]byte("  [q] Quit\r\n\r\n"))
	channel.Write([]byte("Choice: "))

	// Wait for user choice
	buffer := make([]byte, 1)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, err := channel.Read(buffer)
		if err != nil {
			return err
		}

		if n > 0 {
			choice := strings.ToLower(string(buffer[:1]))
			switch choice {
			case "r":
				// Retry registration
				channel.Write([]byte("\r\n\r\nRetrying registration...\r\n"))
				time.Sleep(1 * time.Second)
				return fmt.Errorf("retry_register")
			case "m":
				// Return to main menu
				channel.Write([]byte("\r\n\r\nReturning to main menu...\r\n"))
				time.Sleep(1 * time.Second)
				return nil
			case "q":
				// Quit
				channel.Write([]byte("\r\n\r\nGoodbye!\r\n"))
				return fmt.Errorf("user quit")
			default:
				// Invalid choice
				channel.Write([]byte("\r\nInvalid choice. Please enter 'r', 'm', or 'q': "))
			}
		}
	}
}