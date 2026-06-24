package values

import (
	"strings"
	"testing"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
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
