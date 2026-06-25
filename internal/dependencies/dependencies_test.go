package dependencies

import (
	"testing"

	"helm.sh/helm/v4/pkg/chart"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
)

func createTestChart(name string, deps []*chartv2.Dependency) *chartv2.Chart {
	c := &chartv2.Chart{
		Metadata: &chartv2.Metadata{
			Name:        name,
			Version:     "1.0.0",
			APIVersion:  "v2",
			Dependencies: deps,
		},
	}
	return c
}

func TestGet(t *testing.T) {
	tests := []struct {
		name        string
		chart       *chartv2.Chart
		targets     []string
		excludes    []string
		expectCount int
		expectError bool
	}{
		{
			name: "all targeted",
			chart: createTestChart("test", []*chartv2.Dependency{
				{Name: "sub1", Version: "0.1.0"},
				{Name: "sub2", Version: "0.2.0"},
			}),
			targets:     []string{},
			excludes:    []string{},
			expectCount: 2,
			expectError: false,
		},
		{
			name: "with targets",
			chart: createTestChart("test", []*chartv2.Dependency{
				{Name: "sub1", Version: "0.1.0"},
				{Name: "sub2", Version: "0.2.0"},
			}),
			targets:     []string{"sub1"},
			excludes:    []string{},
			expectCount: 2, // Both returned, but only sub1 is targeted
			expectError: false,
		},
		{
			name: "with excludes",
			chart: createTestChart("test", []*chartv2.Dependency{
				{Name: "sub1", Version: "0.1.0"},
				{Name: "sub2", Version: "0.2.0"},
			}),
			targets:     []string{},
			excludes:    []string{"sub2"},
			expectCount: 2,
			expectError: false,
		},
		{
			name: "with alias",
			chart: createTestChart("test", []*chartv2.Dependency{
				{Name: "sub1", Version: "0.1.0", Alias: "ms1"},
			}),
			targets:     []string{},
			excludes:    []string{},
			expectCount: 1,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chrt := chart.Charter(tt.chart)
			values := make(map[string]interface{})
			vals := &values

			result, err := Get(chrt, vals, tt.targets, tt.excludes, "", false)
			if tt.expectError {
				if err == nil {
					t.Error("Get() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Get() error = %v", err)
			}

			if len(result) != tt.expectCount {
				t.Errorf("Get() returned %d dependencies, want %d", len(result), tt.expectCount)
			}
		})
	}
}

func TestTags(t *testing.T) {
	tests := []struct {
		name     string
		values   map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "with tags",
			values: map[string]interface{}{
				"tags": map[string]interface{}{
					"tag1": true,
					"tag2": false,
				},
			},
			expected: map[string]interface{}{
				"tag1": true,
				"tag2": false,
			},
		},
		{
			name:     "no tags",
			values:   map[string]interface{}{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vals := &tt.values
			result := tags(vals, false)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("tags() = %v, want nil", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("tags() returned %d entries, want %d", len(result), len(tt.expected))
				return
			}

			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("tags()[%q] = %v, want %v", k, result[k], v)
				}
			}
		})
	}
}
