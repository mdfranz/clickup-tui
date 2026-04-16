package util

import (
	"os"
	"testing"
)

func TestGetClickUpPAT(t *testing.T) {
	tests := []struct {
		name      string
		setupEnv  func()
		cleanupEnv func()
		expected  string
		shouldErr bool
	}{
		{
			name: "env var set",
			setupEnv: func() {
				os.Setenv("CLICKUP_PAT", "test-token-123")
			},
			cleanupEnv: func() {
				os.Unsetenv("CLICKUP_PAT")
			},
			expected:  "test-token-123",
			shouldErr: false,
		},
		{
			name: "env var not set",
			setupEnv: func() {
				os.Unsetenv("CLICKUP_PAT")
			},
			cleanupEnv: func() {},
			expected:   "",
			shouldErr:  true,
		},
		{
			name: "env var empty",
			setupEnv: func() {
				os.Setenv("CLICKUP_PAT", "")
			},
			cleanupEnv: func() {
				os.Unsetenv("CLICKUP_PAT")
			},
			expected:  "",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv()
			defer tt.cleanupEnv()

			result, err := GetClickUpPAT()
			if tt.shouldErr && err == nil {
				t.Errorf("GetClickUpPAT() expected error, got nil")
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("GetClickUpPAT() unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("GetClickUpPAT() = %q, want %q", result, tt.expected)
			}
		})
	}
}
