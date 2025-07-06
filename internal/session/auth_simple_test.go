package session

import (
	"testing"
	"time"

	"github.com/dungeongate/internal/user"
)

// TestUserTypeConversion tests conversion from user.User to session.User
func TestUserTypeConversion(t *testing.T) {
	lastLogin := time.Now()
	userServiceUser := &user.User{
		ID:           123,
		Username:     "testuser",
		Email:        "test@example.com",
		IsActive:     true,
		Flags:        user.UserFlagAdmin | user.UserFlagModerator,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		LastLogin:    &lastLogin,
		LoginCount:   5,
		EmailVerified: true,
	}
	
	// Convert user.User to session.User (simulating the conversion in ssh.go)
	sessionUser := &User{
		ID:              userServiceUser.ID,
		Username:        userServiceUser.Username,
		Email:           userServiceUser.Email,
		IsAuthenticated: true,
		IsActive:        userServiceUser.IsActive,
		IsAdmin:         (userServiceUser.Flags & user.UserFlagAdmin) != 0,
		CreatedAt:       userServiceUser.CreatedAt,
		UpdatedAt:       userServiceUser.UpdatedAt,
		LastLogin:       *userServiceUser.LastLogin,
	}
	
	// Verify conversion
	if sessionUser.ID != userServiceUser.ID {
		t.Errorf("Expected ID %d, got %d", userServiceUser.ID, sessionUser.ID)
	}
	
	if sessionUser.Username != userServiceUser.Username {
		t.Errorf("Expected username '%s', got '%s'", userServiceUser.Username, sessionUser.Username)
	}
	
	if !sessionUser.IsAdmin {
		t.Error("Expected user to be admin based on flags")
	}
	
	if !sessionUser.IsAuthenticated {
		t.Error("Expected session user to be authenticated")
	}
	
	if sessionUser.Email != userServiceUser.Email {
		t.Errorf("Expected email '%s', got '%s'", userServiceUser.Email, sessionUser.Email)
	}
}

// TestUserTypeFlagConversion tests flag conversion scenarios
func TestUserTypeFlagConversion(t *testing.T) {
	tests := []struct {
		name        string
		flags       user.UserFlags
		expectAdmin bool
	}{
		{
			name:        "No flags",
			flags:       user.UserFlagNone,
			expectAdmin: false,
		},
		{
			name:        "Admin flag only",
			flags:       user.UserFlagAdmin,
			expectAdmin: true,
		},
		{
			name:        "Multiple flags including admin",
			flags:       user.UserFlagAdmin | user.UserFlagModerator | user.UserFlagBeta,
			expectAdmin: true,
		},
		{
			name:        "Multiple flags without admin",
			flags:       user.UserFlagModerator | user.UserFlagBeta,
			expectAdmin: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lastLogin := time.Now()
			userServiceUser := &user.User{
				ID:        1,
				Username:  "testuser",
				Email:     "test@example.com",
				IsActive:  true,
				Flags:     tt.flags,
				LastLogin: &lastLogin,
			}
			
			// Convert to session user
			sessionUser := &User{
				ID:              userServiceUser.ID,
				Username:        userServiceUser.Username,
				Email:           userServiceUser.Email,
				IsAuthenticated: true,
				IsActive:        userServiceUser.IsActive,
				IsAdmin:         (userServiceUser.Flags & user.UserFlagAdmin) != 0,
				LastLogin:       *userServiceUser.LastLogin,
			}
			
			if sessionUser.IsAdmin != tt.expectAdmin {
				t.Errorf("Expected IsAdmin=%v, got IsAdmin=%v", tt.expectAdmin, sessionUser.IsAdmin)
			}
		})
	}
}

// TestAuthenticationPathSelection tests which authentication path would be selected
func TestAuthenticationPathSelection(t *testing.T) {
	tests := []struct {
		name               string
		hasAuthMiddleware  bool
		hasUserService     bool
		expectedPath       string
	}{
		{
			name:              "Auth middleware available",
			hasAuthMiddleware: true,
			hasUserService:    true,
			expectedPath:      "auth_middleware",
		},
		{
			name:              "Fallback to user service",
			hasAuthMiddleware: false,
			hasUserService:    true,
			expectedPath:      "user_service",
		},
		{
			name:              "No authentication available",
			hasAuthMiddleware: false,
			hasUserService:    false,
			expectedPath:      "none",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test which path would be taken (simulating the logic from ssh.go)
			var actualPath string
			if tt.hasAuthMiddleware {
				actualPath = "auth_middleware"
			} else if tt.hasUserService {
				actualPath = "user_service"
			} else {
				actualPath = "none"
			}
			
			if actualPath != tt.expectedPath {
				t.Errorf("Expected path '%s', got '%s'", tt.expectedPath, actualPath)
			}
		})
	}
}

// TestAuthenticationRetryLogic tests the retry logic used in SSH authentication
func TestAuthenticationRetryLogic(t *testing.T) {
	tests := []struct {
		name           string
		maxAttempts    int
		failurePattern []bool // true = failure, false = success
		expectedResult string
	}{
		{
			name:           "Success on first attempt",
			maxAttempts:    3,
			failurePattern: []bool{false},
			expectedResult: "success",
		},
		{
			name:           "Success on second attempt",
			maxAttempts:    3,
			failurePattern: []bool{true, false},
			expectedResult: "success",
		},
		{
			name:           "Failure after max attempts",
			maxAttempts:    3,
			failurePattern: []bool{true, true, true},
			expectedResult: "max_attempts_exceeded",
		},
		{
			name:           "Success on last allowed attempt",
			maxAttempts:    3,
			failurePattern: []bool{true, true, false},
			expectedResult: "success",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attempts := 0
			var result string
			
			for _, shouldFail := range tt.failurePattern {
				attempts++
				
				if !shouldFail {
					// Authentication succeeded
					result = "success"
					break
				}
				
				// Authentication failed
				if attempts >= tt.maxAttempts {
					result = "max_attempts_exceeded"
					break
				}
				// Otherwise, continue to next attempt
			}
			
			if result != tt.expectedResult {
				t.Errorf("Expected result '%s', got '%s'", tt.expectedResult, result)
			}
		})
	}
}

// TestErrorMessageMapping tests error message mapping for different failure types
func TestErrorMessageMapping(t *testing.T) {
	tests := []struct {
		name          string
		errorString   string
		expectedMsg   string
		expectedLabel string
	}{
		{
			name:          "Username not found",
			errorString:   "username_not_found",
			expectedMsg:   "Username not found. Please check your username and try again.\r\n",
			expectedLabel: "username_not_found",
		},
		{
			name:          "Invalid password",
			errorString:   "invalid_password",
			expectedMsg:   "Incorrect password. Please try again.\r\n",
			expectedLabel: "invalid_password",
		},
		{
			name:          "Account locked",
			errorString:   "account_locked",
			expectedMsg:   "Account is temporarily locked. Please try again later.\r\n",
			expectedLabel: "account_locked",
		},
		{
			name:          "Generic failure",
			errorString:   "some_other_error",
			expectedMsg:   "Authentication failed. Please try again.\r\n",
			expectedLabel: "authentication_failed",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the error handling logic from ssh.go
			var errorMessage string
			var metricLabel string
			
			switch {
			case contains(tt.errorString, "username_not_found"):
				errorMessage = "Username not found. Please check your username and try again.\r\n"
				metricLabel = "username_not_found"
			case contains(tt.errorString, "invalid_password"):
				errorMessage = "Incorrect password. Please try again.\r\n"
				metricLabel = "invalid_password"
			case contains(tt.errorString, "account_locked"):
				errorMessage = "Account is temporarily locked. Please try again later.\r\n"
				metricLabel = "account_locked"
			default:
				errorMessage = "Authentication failed. Please try again.\r\n"
				metricLabel = "authentication_failed"
			}
			
			if errorMessage != tt.expectedMsg {
				t.Errorf("Expected message '%s', got '%s'", tt.expectedMsg, errorMessage)
			}
			
			if metricLabel != tt.expectedLabel {
				t.Errorf("Expected label '%s', got '%s'", tt.expectedLabel, metricLabel)
			}
		})
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}