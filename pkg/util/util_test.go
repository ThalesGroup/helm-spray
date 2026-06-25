package util

import (
	"testing"
	"time"
)

func TestDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Duration
		expected string
	}{
		{
			name:     "zero duration",
			input:    0,
			expected: "0s",
		},
		{
			name:     "seconds only",
			input:    5 * time.Second,
			expected: "5s",
		},
		{
			name:     "minutes and seconds",
			input:    2*time.Minute + 30*time.Second,
			expected: "2m30s",
		},
		{
			name:     "minutes only",
			input:    3 * time.Minute,
			expected: "3m",
		},
		{
			name:     "hours, minutes, seconds",
			input:    1*time.Hour + 2*time.Minute + 30*time.Second,
			expected: "1h2m30s",
		},
		{
			name:     "hours only",
			input:    2 * time.Hour,
			expected: "2h",
		},
		{
			name:     "hours and seconds (no minutes)",
			input:    1*time.Hour + 30*time.Second,
			expected: "1h30s",
		},
		{
			name:     "milliseconds truncated",
			input:    1*time.Second + 500*time.Millisecond,
			expected: "1s",
		},
		{
			name:     "large duration",
			input:    24*time.Hour + 5*time.Minute,
			expected: "24h5m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Duration(tt.input)
			if result != tt.expected {
				t.Errorf("Duration(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
