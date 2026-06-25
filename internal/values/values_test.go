package values

import (
	"reflect"
	"testing"
)

func TestMergeMaps(t *testing.T) {
	tests := []struct {
		name     string
		a        map[string]interface{}
		b        map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name:     "empty maps",
			a:        map[string]interface{}{},
			b:        map[string]interface{}{},
			expected: map[string]interface{}{},
		},
		{
			name:     "a empty",
			a:        map[string]interface{}{},
			b:        map[string]interface{}{"key": "value"},
			expected: map[string]interface{}{"key": "value"},
		},
		{
			name:     "b empty",
			a:        map[string]interface{}{"key": "value"},
			b:        map[string]interface{}{},
			expected: map[string]interface{}{"key": "value"},
		},
		{
			name:     "no overlap",
			a:        map[string]interface{}{"a": 1},
			b:        map[string]interface{}{"b": 2},
			expected: map[string]interface{}{"a": 1, "b": 2},
		},
		{
			name:     "b overrides a",
			a:        map[string]interface{}{"key": "old"},
			b:        map[string]interface{}{"key": "new"},
			expected: map[string]interface{}{"key": "new"},
		},
		{
			name: "nested merge",
			a: map[string]interface{}{
				"outer": map[string]interface{}{
					"a": 1,
					"b": 2,
				},
			},
			b: map[string]interface{}{
				"outer": map[string]interface{}{
					"b": 3,
					"c": 4,
				},
			},
			expected: map[string]interface{}{
				"outer": map[string]interface{}{
					"a": 1,
					"b": 3,
					"c": 4,
				},
			},
		},
		{
			name:     "b non-map overrides a map",
			a:        map[string]interface{}{"key": map[string]interface{}{"nested": true}},
			b:        map[string]interface{}{"key": "string"},
			expected: map[string]interface{}{"key": "string"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeMaps(tt.a, tt.b)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("mergeMaps() = %v, want %v", result, tt.expected)
			}
		})
	}
}
