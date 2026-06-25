package log

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String()
}

func captureStderr(fn func()) string {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	fn()

	_ = w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String()
}

func TestInfo(t *testing.T) {
	tests := []struct {
		name     string
		level    int
		str      string
		params   []interface{}
		expected string
	}{
		{
			name:     "level 1 no params",
			level:    1,
			str:      "hello world",
			expected: "[spray] hello world",
		},
		{
			name:     "level 2 with prefix",
			level:    2,
			str:      "indented",
			expected: "[spray]   > indented",
		},
		{
			name:     "level 3 with prefix",
			level:    3,
			str:      "more indented",
			expected: "[spray]     o more indented",
		},
		{
			name:     "level 1 with params",
			level:    1,
			str:      "hello %s",
			params:   []interface{}{"world"},
			expected: "[spray] hello world",
		},
		{
			name:     "level 4 with prefix",
			level:    4,
			str:      "deep",
			expected: "[spray]       - deep",
		},
		{
			name:     "level 5 with prefix",
			level:    5,
			str:      "deeper",
			expected: "[spray]         . deeper",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureStdout(func() {
				Info(tt.level, tt.str, tt.params...)
			})
			output = strings.TrimSpace(output)
			if output != tt.expected {
				t.Errorf("Info() output = %q, want %q", output, tt.expected)
			}
		})
	}
}

func TestWithNumberedLines(t *testing.T) {
	tests := []struct {
		name     string
		level    int
		str      string
		expected []string
	}{
		{
			name:  "single line",
			level: 1,
			str:   "hello",
			expected: []string{
				"[spray] [0] hello",
			},
		},
		{
			name:  "multiple lines",
			level: 1,
			str:   "line1\nline2\nline3",
			expected: []string{
				"[spray] [0] line1",
				"[spray] [1] line2",
				"[spray] [2] line3",
			},
		},
		{
			name:     "empty string",
			level:    1,
			str:      "",
			expected: []string{},
		},
		{
			name:  "trailing newline",
			level: 1,
			str:   "line1\nline2\n",
			expected: []string{
				"[spray] [0] line1",
				"[spray] [1] line2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureStdout(func() {
				WithNumberedLines(tt.level, tt.str)
			})
			output = strings.TrimSpace(output)
			var lines []string
			if output != "" {
				lines = strings.Split(output, "\n")
			}
			if len(lines) != len(tt.expected) {
				t.Errorf("WithNumberedLines() produced %d lines, want %d", len(lines), len(tt.expected))
				return
			}
			for i, line := range lines {
				line = strings.TrimSpace(line)
				if line != tt.expected[i] {
					t.Errorf("Line %d = %q, want %q", i, line, tt.expected[i])
				}
			}
		})
	}
}

func TestError(t *testing.T) {
	output := captureStderr(func() {
		Error("error message %s", "details")
	})
	output = strings.TrimSpace(output)
	expected := "error message details"
	if output != expected {
		t.Errorf("Error() output = %q, want %q", output, expected)
	}
}
