package user

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/mail"
	"regexp"

	// "strings"

	"time"

	"github.com/dungeongate/pkg/config"
	"github.com/dungeongate/pkg/database"
	"golang.org/x/crypto/argon2"
)

// UserFlags represents user account flags
type UserFlags int

const (
	UserFlagNone         UserFlags = 0
	UserFlagAdmin        UserFlags = 1 << 0 // 0x01
	UserFlagLoginLock    UserFlags = 1 << 1 // 0x02
	UserFlagPasswordLock UserFlags = 1 << 2 // 0x04
	UserFlagEmailLock    UserFlags = 1 << 3 // 0x08
	UserFlagModerator    UserFlags = 1 << 4 // 0x10
	UserFlagBeta         UserFlags = 1 << 5 // 0x20
)

// Enhanced User model
type User struct {
	ID                    int                    `json:"id" db:"id"`
	Username              string                 `json:"username" db:"username"`
	Email                 string                 `json:"email,omitempty" db:"email"`
	PasswordHash          string                 `json:"-" db:"password_hash"`
	Salt                  string                 `json:"-" db:"salt"`
	Environment           string                 `json:"environment,omitempty" db:"environment"`
	Flags                 UserFlags              `json:"flags" db:"flags"`
	CreatedAt             time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time              `json:"updated_at" db:"updated_at"`
	LastLogin             *time.Time             `json:"last_login,omitempty" db:"last_login"`
	LoginCount            int                    `json:"login_count" db:"login_count"`
	FailedLoginAttempts   int                    `json:"-" db:"failed_login_attempts"`
	AccountLocked         bool                   `json:"account_locked" db:"account_locked"`
	LockedUntil           *time.Time             `json:"-" db:"locked_until"`
	EmailVerified         bool                   `json:"email_verified" db:"email_verified"`
	IsActive              bool                   `json:"is_active" db:"is_active"`
	RequirePasswordChange bool                   `json:"require_password_change" db:"require_password_change"`
	Profile               *UserProfile           `json:"profile,omitempty"`
	Preferences           map[string]interface{} `json:"preferences,omitempty"`
	Roles                 []string               `json:"roles,omitempty"`
}

// UserProfile represents extended user profile information
type UserProfile struct {
	UserID             int    `json:"user_id" db:"user_id"`
	RealName           string `json:"real_name,omitempty" db:"real_name"`
	Location           string `json:"location,omitempty" db:"location"`
	Website            string `json:"website,omitempty" db:"website"`
	Bio                string `json:"bio,omitempty" db:"bio"`
	AvatarURL          string `json:"avatar_url,omitempty" db:"avatar_url"`
	Timezone           string `json:"timezone" db:"timezone"`
	Language           string `json:"language" db:"language"`
	Theme              string `json:"theme" db:"theme"`
	TerminalSize       string `json:"terminal_size" db:"terminal_size"`
	ColorMode          string `json:"color_mode" db:"color_mode"`
	EmailNotifications bool   `json:"email_notifications" db:"email_notifications"`
	PublicProfile      bool   `json:"public_profile" db:"public_profile"`
	AllowSpectators    bool   `json:"allow_spectators" db:"allow_spectators"`
	ShowOnlineStatus   bool   `json:"show_online_status" db:"show_online_status"`
}

// RegistrationRequest represents a user registration request
type RegistrationRequest struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	PasswordConfirm string `json:"password_confirm"`
	Email           string `json:"email,omitempty"`
	RealName        string `json:"real_name,omitempty"`
	AcceptTerms     bool   `json:"accept_terms"`
	CaptchaResponse string `json:"captcha_response,omitempty"`
	Source          string `json:"source"` // "ssh", "web", "api"
	IPAddress       string `json:"ip_address,omitempty"`
	UserAgent       string `json:"user_agent,omitempty"`
}

// RegistrationResponse represents a registration response
type RegistrationResponse struct {
	Success              bool              `json:"success"`
	User                 *User             `json:"user,omitempty"`
	Message              string            `json:"message"`
	Errors               []ValidationError `json:"errors,omitempty"`
	RequiresVerification bool              `json:"requires_verification"`
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// Enhanced Service with flexible database configuration
type Service struct {
	db            *database.Connection
	config        *config.UserServiceConfig
	sessionConfig *config.SessionServiceConfig
}

// NewService creates a new user service with enhanced configuration
func NewService(db *database.Connection, cfg *config.UserServiceConfig, sessionCfg *config.SessionServiceConfig) (*Service, error) {
	service := &Service{
		db:            db,
		config:        cfg,
		sessionConfig: sessionCfg,
	}

	// Initialize database schema
	if err := service.initializeSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize database schema: %w", err)
	}

	// Create default admin user if it doesn't exist
	if err := service.createDefaultAdminUser(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to create default admin user: %w", err)
	}

	return service, nil
}

// initializeSchema creates the necessary database tables
func (s *Service) initializeSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username VARCHAR(30) UNIQUE NOT NULL,
			email VARCHAR(80),
			password_hash VARCHAR(255) NOT NULL,
			salt VARCHAR(32) NOT NULL,
			environment TEXT DEFAULT '',
			flags INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_login TIMESTAMP,
			login_count INTEGER DEFAULT 0,
			failed_login_attempts INTEGER DEFAULT 0,
			account_locked BOOLEAN DEFAULT FALSE,
			locked_until TIMESTAMP,
			email_verified BOOLEAN DEFAULT FALSE,
			is_active BOOLEAN DEFAULT TRUE,
			require_password_change BOOLEAN DEFAULT FALSE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute schema query: %w", err)
		}
	}

	return nil
}

// RegisterUser registers a new user
func (s *Service) RegisterUser(ctx context.Context, req *RegistrationRequest) (*RegistrationResponse, error) {
	// Validate registration request
	if errors := s.validateRegistrationRequest(req); len(errors) > 0 {
		return &RegistrationResponse{
			Success: false,
			Message: "Validation failed",
			Errors:  errors,
		}, nil
	}

	// Check if username exists
	if exists, err := s.usernameExists(ctx, req.Username); err != nil {
		return nil, fmt.Errorf("failed to check username existence: %w", err)
	} else if exists {
		return &RegistrationResponse{
			Success: false,
			Message: "Username already exists",
			Errors: []ValidationError{
				{Field: "username", Message: "Username already taken", Code: "USERNAME_EXISTS"},
			},
		}, nil
	}

	// Hash password
	passwordHash, salt, err := s.hashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	now := time.Now()
	user := &User{
		Username:      req.Username,
		Email:         req.Email,
		PasswordHash:  passwordHash,
		Salt:          salt,
		Environment:   "",
		Flags:         UserFlagNone,
		CreatedAt:     now,
		UpdatedAt:     now,
		IsActive:      true,
		EmailVerified: true,
	}

	// Insert user into database
	query := `
		INSERT INTO users (username, email, password_hash, salt, environment, flags, 
						  created_at, updated_at, is_active, email_verified, require_password_change)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := s.db.ExecContext(ctx, query,
		user.Username, user.Email, user.PasswordHash, user.Salt, user.Environment,
		user.Flags, user.CreatedAt, user.UpdatedAt, user.IsActive, user.EmailVerified, user.RequirePasswordChange)
	if err != nil {
		return nil, fmt.Errorf("failed to insert user: %w", err)
	}

	userID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get user ID: %w", err)
	}
	user.ID = int(userID)

	return &RegistrationResponse{
		Success: true,
		User:    user,
		Message: "Registration successful",
	}, nil
}

// validateRegistrationRequest validates the registration request
func (s *Service) validateRegistrationRequest(req *RegistrationRequest) []ValidationError {
	var errors []ValidationError

	// Validate username
	if usernameErrors := s.validateUsername(req.Username); len(usernameErrors) > 0 {
		errors = append(errors, usernameErrors...)
	}

	// Validate password
	if passwordErrors := s.validatePassword(req.Password); len(passwordErrors) > 0 {
		errors = append(errors, passwordErrors...)
	}

	// Validate password confirmation
	if req.Password != req.PasswordConfirm {
		errors = append(errors, ValidationError{
			Field:   "password_confirm",
			Message: "Passwords do not match",
			Code:    "PASSWORD_MISMATCH",
		})
	}

	// Validate email if provided
	if req.Email != "" {
		if emailErrors := s.validateEmail(req.Email); len(emailErrors) > 0 {
			errors = append(errors, emailErrors...)
		}
	}

	return errors
}

// validateUsername validates username
func (s *Service) validateUsername(username string) []ValidationError {
	var errors []ValidationError

	if username == "" {
		errors = append(errors, ValidationError{
			Field:   "username",
			Message: "Username is required",
			Code:    "USERNAME_REQUIRED",
		})
		return errors
	}

	if len(username) < 3 {
		errors = append(errors, ValidationError{
			Field:   "username",
			Message: "Username must be at least 3 characters long",
			Code:    "USERNAME_TOO_SHORT",
		})
	}

	if len(username) > 30 {
		errors = append(errors, ValidationError{
			Field:   "username",
			Message: "Username must be no more than 30 characters long",
			Code:    "USERNAME_TOO_LONG",
		})
	}

	// Check valid characters (alphanumeric and underscore only)
	validUsername := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	if !validUsername.MatchString(username) {
		errors = append(errors, ValidationError{
			Field:   "username",
			Message: "Username can only contain letters, numbers, and underscores",
			Code:    "USERNAME_INVALID_CHARS",
		})
	}

	return errors
}

// validatePassword validates password
func (s *Service) validatePassword(password string) []ValidationError {
	var errors []ValidationError

	if password == "" {
		errors = append(errors, ValidationError{
			Field:   "password",
			Message: "Password is required",
			Code:    "PASSWORD_REQUIRED",
		})
		return errors
	}

	if len(password) < 6 {
		errors = append(errors, ValidationError{
			Field:   "password",
			Message: "Password must be at least 6 characters long",
			Code:    "PASSWORD_TOO_SHORT",
		})
	}

	return errors
}

// validateEmail validates email
func (s *Service) validateEmail(email string) []ValidationError {
	var errors []ValidationError

	if email == "" {
		return errors // Email is optional
	}

	if _, err := mail.ParseAddress(email); err != nil {
		errors = append(errors, ValidationError{
			Field:   "email",
			Message: "Invalid email format",
			Code:    "EMAIL_INVALID",
		})
	}

	return errors
}

// usernameExists checks if username already exists
func (s *Service) usernameExists(ctx context.Context, username string) (bool, error) {
	var count int
	query := "SELECT COUNT(*) FROM users WHERE username = ?"
	err := s.db.QueryRowContext(ctx, query, username).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// hashPassword hashes a password using Argon2
func (s *Service) hashPassword(password string) (string, string, error) {
	// Generate salt
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", "", err
	}

	// Hash password
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)

	return hex.EncodeToString(hash), hex.EncodeToString(salt), nil
}

// verifyPassword verifies a password against a hash
func verifyPassword(password, saltHex, hashHex string) bool {
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return false
	}

	hash, err := hex.DecodeString(hashHex)
	if err != nil {
		return false
	}

	// Hash the provided password
	providedHash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)

	// Compare hashes
	return subtle.ConstantTimeCompare(hash, providedHash) == 1
}

// AuthenticateUser authenticates a user with enhanced error handling and attempt tracking
func (s *Service) AuthenticateUser(ctx context.Context, username, password string) (*User, error) {
	query := `
		SELECT id, username, email, password_hash, salt, environment, flags,
			   created_at, updated_at, last_login, login_count, failed_login_attempts,
			   account_locked, locked_until, email_verified, is_active, require_password_change
		FROM users 
		WHERE username = ? AND is_active = TRUE
	`

	var user User
	var lastLogin, lockedUntil sql.NullTime

	err := s.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Salt,
		&user.Environment, &user.Flags, &user.CreatedAt, &user.UpdatedAt,
		&lastLogin, &user.LoginCount, &user.FailedLoginAttempts,
		&user.AccountLocked, &lockedUntil, &user.EmailVerified, &user.IsActive, &user.RequirePasswordChange,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("username_not_found")
		}
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	// Convert nullable times
	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}
	if lockedUntil.Valid {
		user.LockedUntil = &lockedUntil.Time
	}

	// Check if account is locked
	if user.AccountLocked && user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
		return nil, fmt.Errorf("account_locked")
	}

	// Verify password
	if !verifyPassword(password, user.Salt, user.PasswordHash) {
		// Increment failed login attempts
		if err := s.incrementFailedLoginAttempts(ctx, user.ID); err != nil {
			// Log error but don't expose it to user
			fmt.Printf("Error incrementing failed login attempts: %v\n", err)
		}
		return nil, fmt.Errorf("invalid_password")
	}

	// Password is correct - reset failed attempts and unlock account if needed
	if err := s.resetFailedLoginAttempts(ctx, user.ID); err != nil {
		// Log error but don't fail authentication
		fmt.Printf("Error resetting failed login attempts: %v\n", err)
	}

	// Update last login
	if err := s.updateLastLogin(ctx, user.ID); err != nil {
		// Log error but don't fail authentication
		fmt.Printf("Error updating last login: %v\n", err)
	}

	return &user, nil
}

// updateLastLogin updates user's last login time
func (s *Service) updateLastLogin(ctx context.Context, userID int) error {
	query := `
		UPDATE users 
		SET last_login = CURRENT_TIMESTAMP, 
			login_count = login_count + 1
		WHERE id = ?
	`
	_, err := s.db.ExecContext(ctx, query, userID)
	return err
}

// GetUserByID retrieves a user by ID
func (s *Service) GetUserByID(ctx context.Context, userID int) (*User, error) {
	query := `
		SELECT id, username, email, password_hash, salt, environment, flags,
			   created_at, updated_at, last_login, login_count, failed_login_attempts,
			   account_locked, locked_until, email_verified, is_active, require_password_change
		FROM users 
		WHERE id = ?
	`

	var user User
	var lastLogin, lockedUntil sql.NullTime

	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Salt,
		&user.Environment, &user.Flags, &user.CreatedAt, &user.UpdatedAt,
		&lastLogin, &user.LoginCount, &user.FailedLoginAttempts,
		&user.AccountLocked, &lockedUntil, &user.EmailVerified, &user.IsActive, &user.RequirePasswordChange,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	// Convert nullable times
	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}
	if lockedUntil.Valid {
		user.LockedUntil = &lockedUntil.Time
	}

	return &user, nil
}

// GetUserByUsername retrieves a user by username
func (s *Service) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	query := `
		SELECT id, username, email, password_hash, salt, environment, flags,
			   created_at, updated_at, last_login, login_count, failed_login_attempts,
			   account_locked, locked_until, email_verified, is_active
		FROM users 
		WHERE username = ?
	`

	var user User
	var lastLogin, lockedUntil sql.NullTime

	err := s.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Salt,
		&user.Environment, &user.Flags, &user.CreatedAt, &user.UpdatedAt,
		&lastLogin, &user.LoginCount, &user.FailedLoginAttempts,
		&user.AccountLocked, &lockedUntil, &user.EmailVerified, &user.IsActive,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	// Convert nullable times
	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}
	if lockedUntil.Valid {
		user.LockedUntil = &lockedUntil.Time
	}

	return &user, nil
}

// createDefaultAdminUser creates default admin users based on configuration
func (s *Service) createDefaultAdminUser(ctx context.Context) error {
	// Check if any admin user already exists
	hasAdmin, err := s.hasAdminUser(ctx)
	if err != nil {
		return fmt.Errorf("failed to check for existing admin users: %w", err)
	}

	if hasAdmin {
		return nil // Admin user already exists - skip creation
	}

	var adminUsersCreated []string

	// Create root admin user if configured
	if s.config.Authentication != nil &&
		s.config.Authentication.RootAdminUser != nil &&
		s.config.Authentication.RootAdminUser.Enabled {

		username := s.config.Authentication.RootAdminUser.Name
		if username == "" {
			username = "admin" // Default username
		}

		password := s.config.Authentication.RootAdminUser.OneTimePassword
		if password == "" {
			return fmt.Errorf("root admin user one_time_password is required when enabled")
		}

		if err := s.createAdminUser(ctx, username, password, s.config.Authentication.RootAdminUser.RecoveryEmail); err != nil {
			return fmt.Errorf("failed to create root admin user: %w", err)
		}

		adminUsersCreated = append(adminUsersCreated, username)
		fmt.Printf("Root admin user created: username=%s\n", username)
	}

	// Create additional admin users if configured
	if s.config.Authentication != nil && len(s.config.Authentication.AdminUsers) > 0 {
		for _, adminConfig := range s.config.Authentication.AdminUsers {
			if adminConfig.Name == "" {
				fmt.Printf("Warning: Skipping admin user with empty name\n")
				continue
			}

			if adminConfig.OneTimePassword == "" {
				fmt.Printf("Warning: Skipping admin user '%s' with empty one_time_password\n", adminConfig.Name)
				continue
			}

			if err := s.createAdminUser(ctx, adminConfig.Name, adminConfig.OneTimePassword, adminConfig.RecoveryEmail); err != nil {
				fmt.Printf("Warning: Failed to create admin user '%s': %v\n", adminConfig.Name, err)
				continue
			}

			adminUsersCreated = append(adminUsersCreated, adminConfig.Name)
			fmt.Printf("Admin user created: username=%s\n", adminConfig.Name)
		}
	}

	// Fallback: create default admin if no users were configured and none exist
	if len(adminUsersCreated) == 0 {
		defaultPassword := "admin123" // Basic fallback
		if err := s.createAdminUser(ctx, "admin", defaultPassword, "admin@dungeongate.local"); err != nil {
			return fmt.Errorf("failed to create fallback admin user: %w", err)
		}
		adminUsersCreated = append(adminUsersCreated, "admin")
		fmt.Printf("Fallback admin user created: username=admin, password=%s\n", defaultPassword)
	}

	if len(adminUsersCreated) > 0 {
		fmt.Println("IMPORTANT: Please change admin passwords immediately after first login!")
		fmt.Println("SECURITY: One-time passwords are visible in logs - secure your log files!")
	}

	return nil
}

// createAdminUser creates a single admin user with the given credentials
func (s *Service) createAdminUser(ctx context.Context, username, password, email string) error {
	// Check if user already exists
	existingUser, err := s.GetUserByUsername(ctx, username)
	if err == nil {
		// User exists - check if it's still using one-time password
		if existingUser.RequirePasswordChange {
			// User still has one-time password, we can reset it
			if err := s.ResetUserPassword(ctx, username, password); err != nil {
				return fmt.Errorf("failed to reset one-time password: %w", err)
			}
		}

		// Promote to admin if not already
		if !existingUser.IsAdmin() {
			if err := s.promoteUserToAdmin(ctx, username); err != nil {
				return fmt.Errorf("failed to promote existing user to admin: %w", err)
			}
		}

		// If user already exists and has changed password, leave them alone
		return nil
	}

	// Create new user with one-time password flag
	req := &RegistrationRequest{
		Username:        username,
		Password:        password,
		PasswordConfirm: password,
		Email:           email,
		AcceptTerms:     true,
		Source:          "system",
	}

	resp, err := s.RegisterUser(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to register admin user: %w", err)
	}

	if !resp.Success {
		if len(resp.Errors) > 0 && resp.Errors[0].Code == "USERNAME_EXISTS" {
			// User was created between our check and now - promote to admin
			return s.promoteUserToAdmin(ctx, username)
		}
		return fmt.Errorf("failed to create admin user: %s", resp.Message)
	}

	// Promote user to admin
	if err := s.promoteUserToAdmin(ctx, username); err != nil {
		return fmt.Errorf("failed to promote user to admin: %w", err)
	}

	// Mark user as requiring password change (one-time password)
	if err := s.setRequirePasswordChange(ctx, username, true); err != nil {
		return fmt.Errorf("failed to set password change requirement: %w", err)
	}

	return nil
}

// hasAdminUser checks if any admin user exists in the database
func (s *Service) hasAdminUser(ctx context.Context) (bool, error) {
	var count int
	query := "SELECT COUNT(*) FROM users WHERE (flags & ?) != 0 AND is_active = TRUE"
	err := s.db.QueryRowContext(ctx, query, int(UserFlagAdmin)).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// promoteUserToAdmin promotes a user to admin status
func (s *Service) promoteUserToAdmin(ctx context.Context, username string) error {
	query := "UPDATE users SET flags = flags | ? WHERE username = ?"
	_, err := s.db.ExecContext(ctx, query, int(UserFlagAdmin), username)
	return err
}

// IsAdmin checks if a user has admin privileges
func (u *User) IsAdmin() bool {
	return (u.Flags & UserFlagAdmin) != 0
}

// Admin Management Methods

// UnlockUserAccount unlocks a user account
func (s *Service) UnlockUserAccount(ctx context.Context, username string) error {
	query := `
		UPDATE users 
		SET account_locked = FALSE, 
			locked_until = NULL, 
			failed_login_attempts = 0 
		WHERE username = ?
	`
	result, err := s.db.ExecContext(ctx, query, username)
	if err != nil {
		return fmt.Errorf("failed to unlock user account: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %s", username)
	}

	return nil
}

// DeleteUserAccount deletes a user account
func (s *Service) DeleteUserAccount(ctx context.Context, username string) error {
	// First check if user exists and is not the only admin
	user, err := s.GetUserByUsername(ctx, username)
	if err != nil {
		return fmt.Errorf("user not found: %s", username)
	}

	// Prevent deleting the last admin
	if user.IsAdmin() {
		adminCount, err := s.getAdminCount(ctx)
		if err != nil {
			return fmt.Errorf("failed to check admin count: %w", err)
		}
		if adminCount <= 1 {
			return fmt.Errorf("cannot delete the last admin user")
		}
	}

	query := "DELETE FROM users WHERE username = ?"
	result, err := s.db.ExecContext(ctx, query, username)
	if err != nil {
		return fmt.Errorf("failed to delete user account: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %s", username)
	}

	return nil
}

// ResetUserPassword resets a user's password
func (s *Service) ResetUserPassword(ctx context.Context, username, newPassword string) error {
	// Validate new password
	if errors := s.validatePassword(newPassword); len(errors) > 0 {
		return fmt.Errorf("invalid password: %s", errors[0].Message)
	}

	// Hash new password
	passwordHash, salt, err := s.hashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	query := `
		UPDATE users 
		SET password_hash = ?, 
			salt = ?, 
			updated_at = CURRENT_TIMESTAMP 
		WHERE username = ?
	`
	result, err := s.db.ExecContext(ctx, query, passwordHash, salt, username)
	if err != nil {
		return fmt.Errorf("failed to reset user password: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %s", username)
	}

	return nil
}

// PromoteUserToAdmin promotes a user to admin
func (s *Service) PromoteUserToAdmin(ctx context.Context, username string) error {
	return s.promoteUserToAdmin(ctx, username)
}

// getAdminCount returns the number of active admin users
func (s *Service) getAdminCount(ctx context.Context) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM users WHERE (flags & ?) != 0 AND is_active = TRUE"
	err := s.db.QueryRowContext(ctx, query, int(UserFlagAdmin)).Scan(&count)
	return count, err
}

// GetServerStatistics returns server statistics
func (s *Service) GetServerStatistics(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total users
	var totalUsers int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&totalUsers)
	if err != nil {
		return nil, fmt.Errorf("failed to get total users: %w", err)
	}
	stats["total_users"] = totalUsers

	// Active users
	var activeUsers int
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE is_active = TRUE").Scan(&activeUsers)
	if err != nil {
		return nil, fmt.Errorf("failed to get active users: %w", err)
	}
	stats["active_users"] = activeUsers

	// Admin users
	adminCount, err := s.getAdminCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin count: %w", err)
	}
	stats["admin_users"] = adminCount

	// Locked users
	var lockedUsers int
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE account_locked = TRUE").Scan(&lockedUsers)
	if err != nil {
		return nil, fmt.Errorf("failed to get locked users: %w", err)
	}
	stats["locked_users"] = lockedUsers

	// Users created today
	var usersToday int
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE DATE(created_at) = DATE('now')").Scan(&usersToday)
	if err != nil {
		return nil, fmt.Errorf("failed to get users created today: %w", err)
	}
	stats["users_created_today"] = usersToday

	return stats, nil
}

// setRequirePasswordChange sets or unsets the password change requirement for a user
func (s *Service) setRequirePasswordChange(ctx context.Context, username string, required bool) error {
	query := `
		UPDATE users 
		SET require_password_change = ?, 
			updated_at = CURRENT_TIMESTAMP 
		WHERE username = ?
	`
	result, err := s.db.ExecContext(ctx, query, required, username)
	if err != nil {
		return fmt.Errorf("failed to update password change requirement: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %s", username)
	}

	return nil
}

// ClearPasswordChangeRequirement clears the password change requirement after successful password change
func (s *Service) ClearPasswordChangeRequirement(ctx context.Context, username string) error {
	return s.setRequirePasswordChange(ctx, username, false)
}

// RequiresPasswordChange checks if a user needs to change their password
func (u *User) RequiresPasswordChange() bool {
	return u.RequirePasswordChange
}

// ChangePassword changes a user's password after verifying their current password
func (s *Service) ChangePassword(ctx context.Context, username, currentPassword, newPassword string) error {
	// First authenticate with current password
	user, err := s.AuthenticateUser(ctx, username, currentPassword)
	if err != nil {
		return fmt.Errorf("current password verification failed: %w", err)
	}

	// Validate new password
	if errors := s.validatePassword(newPassword); len(errors) > 0 {
		return fmt.Errorf("invalid new password: %s", errors[0].Message)
	}

	// Hash new password
	passwordHash, salt, err := s.hashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	// Update password in database
	query := `
		UPDATE users 
		SET password_hash = ?, 
			salt = ?, 
			require_password_change = FALSE,
			updated_at = CURRENT_TIMESTAMP 
		WHERE username = ?
	`
	result, err := s.db.ExecContext(ctx, query, passwordHash, salt, username)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %s", username)
	}

	// Clear the require_password_change flag (for one-time passwords)
	// This is already handled in the query above, but we also reset failed login attempts
	if err := s.resetFailedLoginAttempts(ctx, user.ID); err != nil {
		// Log error but don't fail the password change
		fmt.Printf("Warning: Failed to reset failed login attempts after password change: %v\n", err)
	}

	return nil
}
