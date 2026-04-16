package format

import (
	"testing"
	"time"
)

func TestFormatTaskDate(t *testing.T) {
	// Use Unix timestamp to be timezone-independent
	// Create a known timestamp and format it
	baseTime := time.Unix(0, 1000*int64(time.Millisecond)) // Jan 1, 1970 + 1 second
	expectedDate := baseTime.Format("01/02")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"valid date", "1000", expectedDate},
		{"empty string", "", ""},
		{"invalid number", "not-a-number", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTaskDate(tt.input)
			if result != tt.expected {
				t.Errorf("FormatTaskDate(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatCommentDate(t *testing.T) {
	// Use Unix timestamp to be timezone-independent
	baseTime := time.Unix(0, 1000*int64(time.Millisecond))
	expectedDate := baseTime.Format("01/02 15:04")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"valid date", "1000", expectedDate},
		{"empty string", "", ""},
		{"invalid number", "not-a-number", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatCommentDate(tt.input)
			if result != tt.expected {
				t.Errorf("FormatCommentDate(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseUnixMillis(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		shouldErr bool
		checkMs   int64 // milliseconds to check
	}{
		{
			name:      "valid timestamp",
			input:     "1000",
			shouldErr: false,
			checkMs:   1000,
		},
		{
			name:      "zero timestamp",
			input:     "0",
			shouldErr: false,
			checkMs:   0,
		},
		{
			name:      "invalid input",
			input:     "not-a-number",
			shouldErr: true,
		},
		{
			name:      "empty string",
			input:     "",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseUnixMillis(tt.input)
			if tt.shouldErr && err == nil {
				t.Errorf("parseUnixMillis(%q) expected error, got nil", tt.input)
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("parseUnixMillis(%q) unexpected error: %v", tt.input, err)
			}
			if !tt.shouldErr {
				// Check by comparing the Unix nanoseconds
				expectedTime := time.Unix(0, tt.checkMs*int64(time.Millisecond))
				if result.Unix() != expectedTime.Unix() {
					t.Errorf("parseUnixMillis(%q) = %v, want %v", tt.input, result, expectedTime)
				}
			}
		})
	}
}
