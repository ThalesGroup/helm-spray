package web

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseWeights(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected map[string]int
	}{
		{
			name: "simple weights",
			content: `
micro-service-1:
  weight: 0
micro-service-2:
  weight: 1
ms3:
  weight: 2
`,
			expected: map[string]int{
				"micro-service-1": 0,
				"micro-service-2": 1,
				"ms3":             2,
			},
		},
		{
			name: "float weights",
			content: `
service1:
  weight: 1.5
service2:
  weight: 2.0
`,
			expected: map[string]int{
				"service1": 1,
				"service2": 2,
			},
		},
		{
			name:     "no weights",
			content:  `key: value`,
			expected: map[string]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			valuesFile := filepath.Join(tmpDir, "values.yaml")
			if err := os.WriteFile(valuesFile, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			result := parseWeights(valuesFile)
			if len(result) != len(tt.expected) {
				t.Errorf("parseWeights() returned %d entries, want %d", len(result), len(tt.expected))
				return
			}

			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("parseWeights()[%q] = %d, want %d", k, result[k], v)
				}
			}
		})
	}
}

func TestComputeExecutionOrder(t *testing.T) {
	tests := []struct {
		name     string
		deps     []DependencyInfo
		expected []string
	}{
		{
			name: "sorted by weight",
			deps: []DependencyInfo{
				{Name: "service-c", Weight: 2},
				{Name: "service-a", Weight: 0},
				{Name: "service-b", Weight: 1},
			},
			expected: []string{
				"service-a (weight: 0)",
				"service-b (weight: 1)",
				"service-c (weight: 2)",
			},
		},
		{
			name: "same weight",
			deps: []DependencyInfo{
				{Name: "service-a", Weight: 0},
				{Name: "service-b", Weight: 0},
			},
			expected: []string{
				"service-a (weight: 0)",
				"service-b (weight: 0)",
			},
		},
		{
			name: "with aliases",
			deps: []DependencyInfo{
				{Name: "real-name", Alias: "alias-name", Weight: 1},
			},
			expected: []string{
				"alias-name (weight: 1)",
			},
		},
		{
			name:     "empty",
			deps:     []DependencyInfo{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeExecutionOrder(tt.deps)
			if len(result) != len(tt.expected) {
				t.Errorf("computeExecutionOrder() returned %d items, want %d", len(result), len(tt.expected))
				return
			}
			for i, item := range result {
				if item != tt.expected[i] {
					t.Errorf("computeExecutionOrder()[%d] = %q, want %q", i, item, tt.expected[i])
				}
			}
		})
	}
}

func TestParseChartFile(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		chartPath   string
		expectError bool
		expectName  string
		expectDeps  int
	}{
		{
			name: "valid chart",
			content: `
apiVersion: v2
name: my-chart
version: 1.0.0
appVersion: 1.0.0
description: A test chart
dependencies:
  - name: subchart1
    version: 0.1.0
  - name: subchart2
    version: 0.2.0
    alias: sc2
`,
			chartPath:   "/tmp/test-chart",
			expectError: false,
			expectName:  "my-chart",
			expectDeps:  2,
		},
		{
			name: "no dependencies",
			content: `
apiVersion: v2
name: simple-chart
version: 1.0.0
`,
			chartPath:   "/tmp/simple",
			expectError: false,
			expectName:  "simple-chart",
			expectDeps:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			chartFile := filepath.Join(tmpDir, "Chart.yaml")
			if err := os.WriteFile(chartFile, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			info, err := parseChartFile(chartFile, tt.chartPath)
			if tt.expectError {
				if err == nil {
					t.Error("parseChartFile() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("parseChartFile() error = %v", err)
			}

			if info.Name != tt.expectName {
				t.Errorf("parseChartFile().Name = %q, want %q", info.Name, tt.expectName)
			}

			if len(info.Dependencies) != tt.expectDeps {
				t.Errorf("parseChartFile().Dependencies has %d items, want %d", len(info.Dependencies), tt.expectDeps)
			}
		})
	}
}

func TestExecCommand(t *testing.T) {
	// Test with a simple command
	output, err := ExecCommand("echo", "hello")
	if err != nil {
		t.Fatalf("ExecCommand() error = %v", err)
	}

	if output != "hello\n" {
		t.Errorf("ExecCommand() output = %q, want %q", output, "hello\n")
	}
}
