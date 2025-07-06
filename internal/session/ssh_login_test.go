package session

import (
	"testing"

	"github.com/dungeongate/pkg/config"
)

// TestGetMaxLoginAttempts tests configuration of max login attempts
func TestGetMaxLoginAttemptsWithDefault(t *testing.T) {
	tests := []struct {
		name     string
		config   *config.SessionServiceConfig
		expected int
	}{
		{
			name:     "Default config",
			config:   &config.SessionServiceConfig{},
			expected: 3,
		},
		{
			name: "Custom config",
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
			name:     "Nil config",
			config:   nil,
			expected: 3,
		},
		{
			name: "Zero max attempts defaults to 3",
			config: &config.SessionServiceConfig{
				User: &config.UserConfig{
					LoginAttempts: &config.LoginAttemptsConfig{
						MaxAttempts: 0,
					},
				},
			},
			expected: 3,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &SSHServer{
				config: tt.config,
			}
			
			result := server.getMaxLoginAttemptsWithDefault()
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}