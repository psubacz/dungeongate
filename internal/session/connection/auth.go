package connection

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/dungeongate/internal/session/client"
	"github.com/dungeongate/internal/session/menu"
	"github.com/dungeongate/internal/session/terminal"
	authv1 "github.com/dungeongate/pkg/api/auth/v1"
	"golang.org/x/crypto/ssh"
)

// SSHAuthHandler provides SSH authentication callbacks
type SSHAuthHandler struct {
	authClient      *client.AuthClient
	logger          *slog.Logger
	envVars         map[string]string // Store environment variables from SSH connection
	allowedUsername string            // Only allow connections from this username
	sshPassword     string            // SSH password for the allowed username (config-based)
}

// NewSSHAuthHandler creates a new SSH auth handler
func NewSSHAuthHandler(authClient *client.AuthClient, logger *slog.Logger, allowedUsername, sshPassword string) *SSHAuthHandler {
	return &SSHAuthHandler{
		authClient:      authClient,
		logger:          logger,
		envVars:         make(map[string]string),
		allowedUsername: allowedUsername,
		sshPassword:     sshPassword,
	}
}

// isUsernameAllowed checks if the username is allowed to connect
func (a *SSHAuthHandler) isUsernameAllowed(username string) bool {
	// If no allowed username is configured, allow all usernames
	if a.allowedUsername == "" {
		return true
	}
	// Otherwise, only allow the specific configured username
	return username == a.allowedUsername
}

// PasswordCallback handles password authentication
func (a *SSHAuthHandler) PasswordCallback(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
	username := conn.User()

	// Check if username is allowed
	if !a.isUsernameAllowed(username) {
		a.logger.Warn("SSH connection rejected: username not allowed", 
			"username", username, 
			"allowed_username", a.allowedUsername,
			"remote_addr", conn.RemoteAddr())
		return nil, fmt.Errorf("authentication failed: username not allowed")
	}

	// If SSH password is configured and this is the allowed username, use config-based auth
	if a.sshPassword != "" && username == a.allowedUsername {
		if string(password) == a.sshPassword {
			// SSH-level authentication successful - create minimal permissions
			permissions := &ssh.Permissions{
				Extensions: map[string]string{
					"ssh_auth_method": "config_password",
					"ssh_username":    username,
				},
			}
			a.logger.Info("SSH authentication successful", "username", username, "method", "config_password")
			return permissions, nil
		} else {
			a.logger.Warn("SSH authentication failed: incorrect password", "username", username)
			return nil, fmt.Errorf("authentication failed")
		}
	}

	// Fallback to DungeonGate auth service authentication
	ctx := context.Background()
	resp, err := a.authClient.Login(ctx, username, string(password))
	if err != nil {
		a.logger.Warn("DungeonGate auth failed", "username", username, "error", err)
		return nil, fmt.Errorf("authentication failed")
	}

	// Validate response
	if resp == nil {
		a.logger.Warn("DungeonGate auth failed: empty response", "username", username)
		return nil, fmt.Errorf("authentication failed")
	}
	if resp.User == nil {
		a.logger.Warn("DungeonGate auth failed: empty user in response", "username", username)
		return nil, fmt.Errorf("authentication failed")
	}

	// Store user info in permissions
	permissions := &ssh.Permissions{
		Extensions: map[string]string{
			"user_id":         resp.User.Id,
			"username":        resp.User.Username,
			"access_token":    resp.AccessToken,
			"ssh_auth_method": "dungeongate_auth",
		},
	}

	a.logger.Info("DungeonGate authentication successful", "username", username, "user_id", resp.User.Id)
	return permissions, nil
}

// PublicKeyCallback handles public key authentication
func (a *SSHAuthHandler) PublicKeyCallback(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	var username string
	if conn != nil {
		username = conn.User()
	}

	// Check if username is allowed
	if !a.isUsernameAllowed(username) {
		a.logger.Warn("SSH public key connection rejected: username not allowed", 
			"username", username, 
			"allowed_username", a.allowedUsername,
			"remote_addr", conn.RemoteAddr())
		return nil, fmt.Errorf("authentication failed: username not allowed")
	}

	// For now, reject public key authentication
	// In a real implementation, we'd validate the key against stored public keys
	a.logger.Debug("Public key authentication attempted", "username", username)
	return nil, fmt.Errorf("public key authentication not supported")
}

// SetEnvironmentVariable stores an environment variable for this SSH connection
func (a *SSHAuthHandler) SetEnvironmentVariable(name, value string) {
	a.envVars[name] = value
	a.logger.Debug("Environment variable set", "name", name, "value_length", len(value))
}

// authenticateWithDGAUTH attempts authentication using DGAUTH environment variable
func (a *SSHAuthHandler) authenticateWithDGAUTH(ctx context.Context) (*ssh.Permissions, error) {
	dgauth, exists := a.envVars["DGAUTH"]
	if !exists {
		return nil, fmt.Errorf("DGAUTH environment variable not found")
	}

	// Parse DGAUTH format: Username:Password
	parts := strings.SplitN(dgauth, ":", 2)
	if len(parts) != 2 {
		a.logger.Warn("Invalid DGAUTH format", "format", "expected Username:Password")
		return nil, fmt.Errorf("invalid DGAUTH format, expected Username:Password")
	}

	username := parts[0]
	password := parts[1]

	if username == "" || password == "" {
		a.logger.Warn("Empty username or password in DGAUTH")
		return nil, fmt.Errorf("username and password cannot be empty")
	}

	// Authenticate with auth service
	resp, err := a.authClient.Login(ctx, username, password)
	if err != nil {
		a.logger.Warn("DGAUTH login failed", "username", username, "error", err)
		return nil, fmt.Errorf("authentication failed")
	}

	// Validate response
	if resp == nil {
		a.logger.Warn("DGAUTH login failed: empty response", "username", username)
		return nil, fmt.Errorf("authentication failed")
	}
	if resp.User == nil {
		a.logger.Warn("DGAUTH login failed: empty user in response", "username", username)
		return nil, fmt.Errorf("authentication failed")
	}

	// Store user info in permissions
	permissions := &ssh.Permissions{
		Extensions: map[string]string{
			"user_id":      resp.User.Id,
			"username":     resp.User.Username,
			"access_token": resp.AccessToken,
			"auth_method":  "dgauth",
		},
	}

	a.logger.Info("DGAUTH authentication successful", "username", username, "user_id", resp.User.Id)
	return permissions, nil
}

// UserAuthManager handles user authentication and account management
type UserAuthManager struct {
	authClient *client.AuthClient
	logger     *slog.Logger
}

// NewUserAuthManager creates a new user auth manager
func NewUserAuthManager(authClient *client.AuthClient, logger *slog.Logger) *UserAuthManager {
	return &UserAuthManager{
		authClient: authClient,
		logger:     logger,
	}
}

// GetUserInfo retrieves user information from the auth service
func (m *UserAuthManager) GetUserInfo(ctx context.Context, sshConn *ssh.ServerConn) (*authv1.User, error) {
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
	resp, err := m.authClient.GetUserInfo(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	return resp.User, nil
}

// HandleLogin handles the login process
func (m *UserAuthManager) HandleLogin(ctx context.Context, channel ssh.Channel, connID, currentUsername string, sshConn *ssh.ServerConn) error {
	// Clear screen for login form
	channel.Write([]byte("\033[2J\033[H"))
	channel.Write([]byte("\r\n=== Login ===\r\n\r\n"))

	// Flush any pending input from menu selection
	m.flushInput(channel)

	// Get username
	channel.Write([]byte("Username: "))
	username, err := m.readLineWithTerminal(ctx, channel)
	if err != nil {
		if err.Error() == "user cancelled" {
			channel.Write([]byte("\r\nLogin cancelled.\r\n"))
			time.Sleep(1 * time.Second)
			return nil
		}
		return err
	}

	// Get password (hidden input)
	channel.Write([]byte("Password: "))
	password, err := m.readPasswordWithTerminal(ctx, channel)
	if err != nil {
		if err.Error() == "user cancelled" {
			channel.Write([]byte("\r\nLogin cancelled.\r\n"))
			time.Sleep(1 * time.Second)
			return nil
		}
		return err
	}

	// Attempt login with auth service
	resp, err := m.authClient.Login(ctx, username, password)
	if err != nil {
		m.logger.Warn("Login failed", "username", username, "error", err)
		channel.Write([]byte("\r\nLogin failed. Please check your credentials.\r\n"))
		// Brief pause to let user read the message
		time.Sleep(2 * time.Second)
		return nil
	}

	// Check if response is valid
	if resp == nil || resp.User == nil {
		m.logger.Error("Invalid login response", "username", username)
		channel.Write([]byte("\r\nLogin failed. Server error.\r\n"))
		// Brief pause to let user read the message
		time.Sleep(2 * time.Second)
		return nil
	}

	// Login successful - store access token in SSH connection
	if sshConn.Permissions == nil {
		sshConn.Permissions = &ssh.Permissions{}
	}
	if sshConn.Permissions.Extensions == nil {
		sshConn.Permissions.Extensions = make(map[string]string)
	}
	sshConn.Permissions.Extensions["access_token"] = resp.AccessToken

	m.logger.Info("User logged in successfully", "username", username, "user_id", resp.User.Id)
	channel.Write([]byte("\r\nLogin successful! Welcome back to the gate, " + resp.User.Username + "...\r\n"))

	// Brief pause to show success message
	time.Sleep(3 * time.Second)

	return nil
}

// HandleRegister handles the registration process
func (m *UserAuthManager) HandleRegister(ctx context.Context, channel ssh.Channel, connID, currentUsername string, sshConn *ssh.ServerConn) error {
	// Clear screen for registration form
	channel.Write([]byte("\033[2J\033[H"))
	channel.Write([]byte("\r\n=== Registration ===\r\n\r\n"))

	// Flush any pending input from menu selection
	m.flushInput(channel)

	// Get username
	channel.Write([]byte("Choose a username: "))
	username, err := m.readLineWithTerminal(ctx, channel)
	if err != nil {
		if err.Error() == "user cancelled" {
			channel.Write([]byte("\r\nRegistration cancelled.\r\n"))
			time.Sleep(1 * time.Second)
			return nil
		}
		return err
	}

	// Get password
	channel.Write([]byte("Choose a password: "))
	password, err := m.readPasswordWithTerminal(ctx, channel)
	if err != nil {
		if err.Error() == "user cancelled" {
			channel.Write([]byte("\r\nRegistration cancelled.\r\n"))
			time.Sleep(1 * time.Second)
			return nil
		}
		return err
	}

	// Confirm password
	channel.Write([]byte("Confirm password: "))
	confirmPassword, err := m.readPasswordWithTerminal(ctx, channel)
	if err != nil {
		if err.Error() == "user cancelled" {
			channel.Write([]byte("\r\nRegistration cancelled.\r\n"))
			time.Sleep(1 * time.Second)
			return nil
		}
		return err
	}

	// Check if passwords match
	if password != confirmPassword {
		channel.Write([]byte("\r\nPasswords do not match.\r\n"))
		return m.handleRegistrationRetry(ctx, channel, "password mismatch")
	}

	// Get email (optional)
	channel.Write([]byte("Email (optional - leave blank to skip): "))
	email, err := m.readOptionalLineWithTerminal(ctx, channel)
	if err != nil {
		return err
	}

	// Attempt registration with auth service
	resp, err := m.authClient.Register(ctx, username, password, email)
	if err != nil {
		m.logger.Warn("Registration failed", "username", username, "error", err)
		channel.Write([]byte("\r\nRegistration failed. Please try again later.\r\n"))
		return m.handleRegistrationRetry(ctx, channel, "network error")
	}

	if !resp.Success {
		m.logger.Warn("Registration rejected", "username", username, "error", resp.Error, "error_code", resp.ErrorCode)

		// Show detailed validation message
		detailedMessage := m.getDetailedValidationMessage(resp.ErrorCode, resp.Error)
		channel.Write([]byte("\r\nRegistration failed:\r\n" + detailedMessage))
		return m.handleRegistrationRetryWithCode(ctx, channel, resp.Error, resp.ErrorCode)
	}

	// Registration successful - store access token in SSH connection
	if sshConn.Permissions == nil {
		sshConn.Permissions = &ssh.Permissions{}
	}
	if sshConn.Permissions.Extensions == nil {
		sshConn.Permissions.Extensions = make(map[string]string)
	}
	sshConn.Permissions.Extensions["access_token"] = resp.AccessToken

	m.logger.Info("User registered successfully", "username", username, "user_id", resp.User.Id)
	channel.Write([]byte("\r\nRegistration successful! Welcome, " + resp.User.Username + "!\r\n"))
	channel.Write([]byte("You are now logged in.\r\n"))

	// Brief pause to show success message
	time.Sleep(1 * time.Second)

	return nil
}

// HandleRequiredPasswordChange handles the forced password change for one-time passwords
func (m *UserAuthManager) HandleRequiredPasswordChange(ctx context.Context, channel ssh.Channel, user *authv1.User, sshConn *ssh.ServerConn) (*menu.MenuChoice, error) {
	// Clear screen and display password change prompt
	channel.Write([]byte("\033[2J\033[H"))
	channel.Write([]byte("\r\n=== PASSWORD CHANGE REQUIRED ===\r\n\r\n"))
	channel.Write([]byte("Your account is using a one-time password and must be changed before you can access the system.\r\n\r\n"))

	// Flush any pending input
	m.flushInput(channel)

	for {
		// Get current password
		channel.Write([]byte("Enter your current password: "))
		currentPassword, err := m.readPasswordWithTerminal(ctx, channel)
		if err != nil {
			if err.Error() == "user cancelled" {
				return &menu.MenuChoice{Action: "quit", Value: ""}, nil
			}
			return nil, err
		}

		// Get new password
		channel.Write([]byte("Enter your new password: "))
		newPassword, err := m.readPasswordWithTerminal(ctx, channel)
		if err != nil {
			if err.Error() == "user cancelled" {
				return &menu.MenuChoice{Action: "quit", Value: ""}, nil
			}
			return nil, err
		}

		// Confirm new password
		channel.Write([]byte("Confirm your new password: "))
		confirmPassword, err := m.readPasswordWithTerminal(ctx, channel)
		if err != nil {
			if err.Error() == "user cancelled" {
				return &menu.MenuChoice{Action: "quit", Value: ""}, nil
			}
			return nil, err
		}

		// Check if new passwords match
		if newPassword != confirmPassword {
			channel.Write([]byte("\r\nError: New passwords do not match. Please try again.\r\n\r\n"))
			continue
		}

		// Check if new password is different from current
		if newPassword == currentPassword {
			channel.Write([]byte("\r\nError: New password must be different from current password. Please try again.\r\n\r\n"))
			continue
		}

		// Get access token for API call
		permissions := sshConn.Permissions
		if permissions == nil || permissions.Extensions == nil {
			return nil, fmt.Errorf("no authentication token available")
		}

		accessToken, ok := permissions.Extensions["access_token"]
		if !ok || accessToken == "" {
			return nil, fmt.Errorf("no access token in session")
		}

		// Call auth service to change password
		changeReq := &authv1.ChangePasswordRequest{
			AccessToken:     accessToken,
			CurrentPassword: currentPassword,
			NewPassword:     newPassword,
		}

		changeResp, err := m.authClient.ChangePassword(ctx, changeReq)
		if err != nil {
			m.logger.Error("Password change failed", "error", err, "username", user.Username)
			channel.Write([]byte("\r\nError: Failed to change password. Please try again.\r\n\r\n"))
			continue
		}

		if !changeResp.Success {
			channel.Write([]byte(fmt.Sprintf("\r\nError: %s\r\n\r\n", changeResp.Error)))
			continue
		}

		// Password changed successfully
		channel.Write([]byte("\r\nPassword changed successfully! You can now access the system.\r\n"))
		m.logger.Info("Password changed successfully for one-time password user", "username", user.Username)

		// Brief pause to show success message
		time.Sleep(2 * time.Second)

		return nil, nil // Return to main menu loop to refresh user info
	}
}

// getDetailedValidationMessage provides specific validation feedback
func (m *UserAuthManager) getDetailedValidationMessage(errorCode, errorMessage string) string {
	// For debugging: log the actual error code and message
	m.logger.Debug("Registration validation error", "error_code", errorCode, "error_message", errorMessage)

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
		// This might be the generic error we're seeing
		if strings.Contains(errorMessage, "Validation failed") {
			return "Registration validation failed. Please check your input:\r\n" +
				"  • Username must be unique\r\n" +
				"  • Password must be at least 6 characters long\r\n" +
				"  • Email format must be valid (if provided)\r\n"
		}
		return errorMessage + "\r\n"
	default:
		// Return the original error message with debug info
		return fmt.Sprintf("%s\r\n(Error code: %s)\r\n", errorMessage, errorCode)
	}
}

// handleRegistrationRetryWithCode gives user options after registration failure with detailed error info
func (m *UserAuthManager) handleRegistrationRetryWithCode(ctx context.Context, channel ssh.Channel, errorReason, errorCode string) error {
	return m.handleRegistrationRetry(ctx, channel, errorReason)
}

// handleRegistrationRetry gives user options after registration failure
func (m *UserAuthManager) handleRegistrationRetry(ctx context.Context, channel ssh.Channel, errorReason string) error {
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
				// Retry registration - this will cause the menu to call handleRegister again
				channel.Write([]byte("\r\n\r\nRetrying registration...\r\n"))
				time.Sleep(1 * time.Second)
				return fmt.Errorf("retry_register") // Special error to indicate retry
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

// Terminal input helper methods
func (m *UserAuthManager) readLineWithTerminal(ctx context.Context, channel ssh.Channel) (string, error) {
	editor := terminal.NewLineEditor(channel, terminal.InputTypeText)
	return editor.ReadLine(ctx)
}

func (m *UserAuthManager) readPasswordWithTerminal(ctx context.Context, channel ssh.Channel) (string, error) {
	editor := terminal.NewLineEditor(channel, terminal.InputTypePassword)
	return editor.ReadLine(ctx)
}

func (m *UserAuthManager) readOptionalLineWithTerminal(ctx context.Context, channel ssh.Channel) (string, error) {
	editor := terminal.NewLineEditor(channel, terminal.InputTypeOptional)
	return editor.ReadLine(ctx)
}

func (m *UserAuthManager) flushInput(channel ssh.Channel) {
	// Skip flushing input - it was causing hangs
	// The menu input handling should be sufficient
	return
}
