package user

import (
	"testing"
	"time"

	"github.com/dungeongate/pkg/config"
)

// TestGetMaxFailedAttempts tests configuration reading
func TestGetMaxFailedAttempts(t *testing.T) {
	tests := []struct {
		name     string
		config   *config.SessionServiceConfig
		expected int
	}{
		{
			name:     "No config",
			config:   nil,
			expected: 3, // default
		},
		{
			name: "With config",
			config: &config.SessionServiceConfig{
				User: &config.UserConfig{
					LoginAttempts: &config.LoginAttemptsConfig{
						MaxAttempts: 5,
					},
				},
			},
			expected: 5,
		},
		{
			name: "Empty config",
			config: &config.SessionServiceConfig{
				User: &config.UserConfig{},
			},
			expected: 3, // default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &Service{
				sessionConfig: tt.config,
			}

			result := service.getMaxFailedAttempts()
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

// TestGetLockDuration tests lock duration configuration
func TestGetLockDuration(t *testing.T) {
	tests := []struct {
		name     string
		config   *config.SessionServiceConfig
		expected time.Duration
	}{
		{
			name:     "No config",
			config:   nil,
			expected: 15 * time.Minute, // default
		},
		{
			name: "With config",
			config: &config.SessionServiceConfig{
				User: &config.UserConfig{
					LoginAttempts: &config.LoginAttemptsConfig{
						LockDuration: "30m",
					},
				},
			},
			expected: 30 * time.Minute,
		},
		{
			name: "Invalid duration",
			config: &config.SessionServiceConfig{
				User: &config.UserConfig{
					LoginAttempts: &config.LoginAttemptsConfig{
						LockDuration: "invalid",
					},
				},
			},
			expected: 15 * time.Minute, // default on error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &Service{
				sessionConfig: tt.config,
			}

			result := service.getLockDuration()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
