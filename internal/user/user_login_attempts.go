package user

import (
	"context"
	"fmt"
	"time"
)

// incrementFailedLoginAttempts increments failed login attempts and locks account if needed
func (s *Service) incrementFailedLoginAttempts(ctx context.Context, userID int) error {
	// Get current failed attempts
	var currentAttempts int
	query := `SELECT failed_login_attempts FROM users WHERE id = ?`
	err := s.db.QueryRowContext(ctx, query, userID).Scan(&currentAttempts)
	if err != nil {
		return fmt.Errorf("failed to get current attempts: %w", err)
	}

	newAttempts := currentAttempts + 1
	maxAttempts := s.getMaxFailedAttempts()

	// Check if we should lock the account
	if newAttempts >= maxAttempts {
		lockDuration := s.getLockDuration()
		lockUntil := time.Now().Add(lockDuration)

		updateQuery := `
			UPDATE users 
			SET failed_login_attempts = ?, 
				account_locked = TRUE,
				locked_until = ?
			WHERE id = ?
		`
		_, err = s.db.ExecContext(ctx, updateQuery, newAttempts, lockUntil, userID)
	} else {
		updateQuery := `
			UPDATE users 
			SET failed_login_attempts = ?
			WHERE id = ?
		`
		_, err = s.db.ExecContext(ctx, updateQuery, newAttempts, userID)
	}

	return err
}

// resetFailedLoginAttempts resets failed login attempts and unlocks account
func (s *Service) resetFailedLoginAttempts(ctx context.Context, userID int) error {
	query := `
		UPDATE users 
		SET failed_login_attempts = 0,
			account_locked = FALSE,
			locked_until = NULL
		WHERE id = ?
	`
	_, err := s.db.ExecContext(ctx, query, userID)
	return err
}

// getMaxFailedAttempts returns the maximum failed login attempts from config
func (s *Service) getMaxFailedAttempts() int {
	if s.sessionConfig != nil && s.sessionConfig.User != nil && s.sessionConfig.User.LoginAttempts != nil {
		return s.sessionConfig.User.LoginAttempts.MaxAttempts
	}
	return 3 // Default to 3 attempts
}

// getLockDuration returns the lock duration from config
func (s *Service) getLockDuration() time.Duration {
	if s.sessionConfig != nil && s.sessionConfig.User != nil && s.sessionConfig.User.LoginAttempts != nil {
		duration, err := time.ParseDuration(s.sessionConfig.User.LoginAttempts.LockDuration)
		if err == nil {
			return duration
		}
	}
	return 15 * time.Minute // Default to 15 minutes
}
