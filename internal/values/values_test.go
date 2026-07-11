package values

import (
	"strings"
	"testing"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	helmvalues "helm.sh/helm/v3/pkg/cli/values"
)

func makeTestChart(valuesYAML string, files map[string]string) *chart.Chart {
	c := &chart.Chart{
		Metadata: &chart.Metadata{Name: "test"},
		Raw: []*chart.File{
			{Name: chartutil.ValuesfileName, Data: []byte(valuesYAML)},
		},
	}
	for name, content := range files {
		c.Files = append(c.Files, &chart.File{
			Name: name,
			Data: []byte(content),
		})
	}
	return c
}

func TestProcessInclude_NoDirective(t *testing.T) {
	c := makeTestChart("key: value\n", nil)
	result, err := processIncludeInValuesFile(c, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "key: value\n" {
		t.Errorf("expected unchanged values, got %q", result)
	}
}

func TestProcessInclude_BasicFilesGet(t *testing.T) {
	c := makeTestChart(
		"#! {{ .Files.Get extra.yaml }}\n",
		map[string]string{"extra.yaml": "port: 8080\n"},
	)
	result, err := processIncludeInValuesFile(c, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "port: 8080") {
		t.Errorf("expected included content, got %q", result)
	}
	if strings.Contains(result, "#!") {
		t.Error("include directive should be replaced")
	}
}

func TestProcessInclude_BackwardCompatFileGet(t *testing.T) {
	// .File.Get (without 's') is a backward-compat alias
	c := makeTestChart(
		"#! {{ .File.Get extra.yaml }}\n",
		map[string]string{"extra.yaml": "port: 9090\n"},
	)
	result, err := processIncludeInValuesFile(c, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "port: 9090") {
		t.Errorf("expected included content, got %q", result)
	}
}

func TestProcessInclude_Indent(t *testing.T) {
	c := makeTestChart(
		"parent:\n#! {{ .Files.Get nested.yaml | indent 2 }}\n",
		map[string]string{"nested.yaml": "child: true\n"},
	)
	result, err := processIncludeInValuesFile(c, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "  child: true") {
		t.Errorf("expected indented content, got %q", result)
	}
}

func TestProcessInclude_PickSubTable(t *testing.T) {
	c := makeTestChart(
		"#! {{ pick (.Files.Get config.yaml) service }}\n",
		map[string]string{"config.yaml": "service:\n  port: 80\nother: ignored\n"},
	)
	result, err := processIncludeInValuesFile(c, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "port: 80") {
		t.Errorf("expected picked subtable, got %q", result)
	}
	if strings.Contains(result, "other") {
		t.Error("picked result should not contain sibling keys")
	}
}

func TestProcessInclude_PickWithIndent(t *testing.T) {
	c := makeTestChart(
		"root:\n#! {{ pick (.Files.Get config.yaml) service | indent 2 }}\n",
		map[string]string{"config.yaml": "service:\n  port: 80\n"},
	)
	result, err := processIncludeInValuesFile(c, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "  port: 80") {
		t.Errorf("expected indented picked content, got %q", result)
	}
}

func TestProcessInclude_MissingFileError(t *testing.T) {
	c := makeTestChart("#! {{ .Files.Get missing.yaml }}\n", nil)
	_, err := processIncludeInValuesFile(c, false)
	if err == nil {
		t.Error("expected error for missing included file")
	}
}

func TestProcessInclude_EmptyValuesFile(t *testing.T) {
	c := makeTestChart("", nil)
	result, err := processIncludeInValuesFile(c, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

func TestProcessInclude_MultipleIncludes(t *testing.T) {
	c := makeTestChart(
		"#! {{ .Files.Get a.yaml }}\n#! {{ .Files.Get b.yaml }}\n",
		map[string]string{
			"a.yaml": "alpha: 1\n",
			"b.yaml": "beta: 2\n",
		},
	)
	result, err := processIncludeInValuesFile(c, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "alpha: 1") || !strings.Contains(result, "beta: 2") {
		t.Errorf("expected both files included, got %q", result)
	}
}

func TestMergeMaps_NoOverlap(t *testing.T) {
	a := map[string]interface{}{"x": 1}
	b := map[string]interface{}{"y": 2}
	out := mergeMaps(a, b)
	if out["x"] != 1 || out["y"] != 2 {
		t.Errorf("expected union of keys, got %v", out)
	}
}

func TestMergeMaps_ScalarBWins(t *testing.T) {
	a := map[string]interface{}{"x": 1}
	b := map[string]interface{}{"x": 2}
	out := mergeMaps(a, b)
	if out["x"] != 2 {
		t.Errorf("expected b to win scalar conflict, got %v", out["x"])
	}
}

func TestMergeMaps_DeepMerge(t *testing.T) {
	a := map[string]interface{}{"nested": map[string]interface{}{"a": 1, "b": 2}}
	b := map[string]interface{}{"nested": map[string]interface{}{"b": 99, "c": 3}}
	out := mergeMaps(a, b)
	nested, ok := out["nested"].(map[string]interface{})
	if !ok {
		t.Fatal("expected nested map in result")
	}
	if nested["a"] != 1 {
		t.Errorf("expected a=1, got %v", nested["a"])
	}
	if nested["b"] != 99 {
		t.Errorf("expected b=99 (b wins), got %v", nested["b"])
	}
	if nested["c"] != 3 {
		t.Errorf("expected c=3, got %v", nested["c"])
	}
}

func TestMergeMaps_AOnlyKey(t *testing.T) {
	a := map[string]interface{}{"only-a": true}
	b := map[string]interface{}{}
	out := mergeMaps(a, b)
	if out["only-a"] != true {
		t.Error("key from a should survive when b is empty")
	}
}

func TestMergeMaps_DoesNotMutateInputs(t *testing.T) {
	a := map[string]interface{}{"k": "original"}
	b := map[string]interface{}{"k": "override"}
	mergeMaps(a, b)
	if a["k"] != "original" {
		t.Error("mergeMaps should not mutate a")
	}
}

func TestProcessInclude_VerboseNoDirective(t *testing.T) {
	c := makeTestChart("key: value\n", nil)
	result, err := processIncludeInValuesFile(c, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "key: value\n" {
		t.Errorf("expected unchanged values, got %q", result)
	}
}

func TestProcessInclude_VerboseWithFile(t *testing.T) {
	c := makeTestChart(
		"#! {{ .Files.Get extra.yaml }}\n",
		map[string]string{"extra.yaml": "port: 8080\n"},
	)
	result, err := processIncludeInValuesFile(c, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "port: 8080") {
		t.Errorf("expected included content, got %q", result)
	}
}

func TestProcessInclude_VerboseWithFileAndIndent(t *testing.T) {
	c := makeTestChart(
		"parent:\n#! {{ .Files.Get nested.yaml | indent 2 }}\n",
		map[string]string{"nested.yaml": "child: true\n"},
	)
	result, err := processIncludeInValuesFile(c, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "  child: true") {
		t.Errorf("expected indented content, got %q", result)
	}
}

func TestProcessInclude_VerbosePickSubTable(t *testing.T) {
	c := makeTestChart(
		"#! {{ pick (.Files.Get config.yaml) service }}\n",
		map[string]string{"config.yaml": "service:\n  port: 80\n"},
	)
	result, err := processIncludeInValuesFile(c, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "port: 80") {
		t.Errorf("expected picked content, got %q", result)
	}
}

func TestProcessInclude_VerbosePickWithIndent(t *testing.T) {
	c := makeTestChart(
		"root:\n#! {{ pick (.Files.Get config.yaml) service | indent 2 }}\n",
		map[string]string{"config.yaml": "service:\n  port: 80\n"},
	)
	result, err := processIncludeInValuesFile(c, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "  port: 80") {
		t.Errorf("expected indented content, got %q", result)
	}
}

func TestProcessInclude_PickLeafStringValue(t *testing.T) {
	// subValuePath targets a scalar string (not a table) — exercises the PathValue branch
	c := makeTestChart(
		"#! {{ pick (.Files.Get config.yaml) mykey }}\n",
		map[string]string{"config.yaml": "mykey: hello_world\n"},
	)
	result, err := processIncludeInValuesFile(c, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "hello_world") {
		t.Errorf("expected leaf value in result, got %q", result)
	}
}

func TestProcessInclude_PickLeafNonStringError(t *testing.T) {
	// Numeric leaf: PathValue returns float64, not string → must error
	c := makeTestChart(
		"#! {{ pick (.Files.Get config.yaml) mykey }}\n",
		map[string]string{"config.yaml": "mykey: 42\n"},
	)
	_, err := processIncludeInValuesFile(c, false)
	if err == nil {
		t.Error("expected error when leaf value is not a string")
	}
}

func TestProcessInclude_PickPathNotFoundError(t *testing.T) {
	c := makeTestChart(
		"#! {{ pick (.Files.Get config.yaml) nonexistent }}\n",
		map[string]string{"config.yaml": "other: value\n"},
	)
	_, err := processIncludeInValuesFile(c, false)
	if err == nil {
		t.Error("expected error when picked path does not exist")
	}
}

func TestMerge_ReuseValues(t *testing.T) {
	c := makeTestChart("key: value\n", nil)
	c.Values = map[string]interface{}{"key": "value"}
	opts := &helmvalues.Options{}
	result, str, err := Merge(c, true, opts, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if str != "" {
		t.Errorf("expected empty updatedChartValuesAsString with reuseValues=true, got %q", str)
	}
	if result == nil {
		t.Error("expected non-nil merged values")
	}
}

func TestMerge_NoReuseValues(t *testing.T) {
	c := makeTestChart("key: value\n", nil)
	opts := &helmvalues.Options{}
	result, str, err := Merge(c, false, opts, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if str != "key: value\n" {
		t.Errorf("expected chart values string, got %q", str)
	}
	if result == nil {
		t.Error("expected non-nil merged values")
	}
}

func TestMerge_NoReuseValues_Verbose(t *testing.T) {
	c := makeTestChart("key: value\n", nil)
	opts := &helmvalues.Options{}
	result, str, err := Merge(c, false, opts, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if str != "key: value\n" {
		t.Errorf("expected chart values string, got %q", str)
	}
	if result == nil {
		t.Error("expected non-nil merged values")
	}
}

func TestMerge_ReuseValues_Verbose(t *testing.T) {
	c := makeTestChart("key: value\n", nil)
	c.Values = map[string]interface{}{"key": "value"}
	opts := &helmvalues.Options{}
	result, _, err := Merge(c, true, opts, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil merged values")
	}
}

func TestMerge_ProcessIncludeError(t *testing.T) {
	// include references a file that doesn't exist → processIncludeInValuesFile returns error
	c := makeTestChart("#! {{ .Files.Get missing.yaml }}\n", nil)
	opts := &helmvalues.Options{}
	_, _, err := Merge(c, false, opts, false)
	if err == nil {
		t.Error("expected error when included file is missing")
	}
}

func TestMerge_MergeValuesError(t *testing.T) {
	// ValueFiles with a non-existent path → MergeValues returns error
	c := makeTestChart("key: value\n", nil)
	opts := &helmvalues.Options{ValueFiles: []string{"/does/not/exist.yaml"}}
	_, _, err := Merge(c, false, opts, false)
	if err == nil {
		t.Error("expected error when values file path does not exist")
	}
}

func TestMerge_InvalidValuesAfterIncludeError(t *testing.T) {
	// processIncludeInValuesFile substitutes the include, then ReadValues fails on bad YAML
	c := makeTestChart(
		"#! {{ .Files.Get extra.yaml }}\n",
		map[string]string{"extra.yaml": "invalid: [\nbad yaml"},
	)
	opts := &helmvalues.Options{}
	_, _, err := Merge(c, false, opts, false)
	if err == nil {
		t.Error("expected error when included file produces invalid YAML for ReadValues")
	}
}

func TestProcessInclude_PickInvalidYAMLInFileError(t *testing.T) {
	// chartutil.ReadValues will fail on invalid YAML — exercises that error path
	c := makeTestChart(
		"#! {{ pick (.Files.Get config.yaml) key }}\n",
		map[string]string{"config.yaml": "invalid: [\nbad yaml"},
	)
	_, err := processIncludeInValuesFile(c, false)
	if err == nil {
		t.Error("expected error when included file contains invalid YAML")
	}
}
