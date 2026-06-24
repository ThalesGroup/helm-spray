package dependencies

import (
	"encoding/json"
	"testing"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
)

func makeChart(deps []*chart.Dependency, subCharts []*chart.Chart) *chart.Chart {
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:         "umbrella",
			Dependencies: deps,
		},
	}
	for _, sub := range subCharts {
		c.AddDependency(sub)
	}
	return c
}

func makeValues(weights map[string]float64) *chartutil.Values {
	m := make(chartutil.Values, len(weights))
	for name, w := range weights {
		m[name] = map[string]interface{}{"weight": w}
	}
	return &m
}

func TestGet_EmptyDependencies(t *testing.T) {
	c := makeChart(nil, nil)
	v := &chartutil.Values{}
	deps, err := Get(c, v, nil, nil, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("expected 0 deps, got %d", len(deps))
	}
}

func TestGet_SingleDepDefaultWeight(t *testing.T) {
	c := makeChart([]*chart.Dependency{{Name: "svc"}}, nil)
	v := makeValues(map[string]float64{"svc": 0})
	deps, err := Get(c, v, nil, nil, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(deps))
	}
	d := deps[0]
	if d.Name != "svc" || d.UsedName != "svc" || d.Alias != "" {
		t.Errorf("unexpected name fields: %+v", d)
	}
	if d.Weight != 0 {
		t.Errorf("expected weight 0, got %d", d.Weight)
	}
	if !d.Targeted {
		t.Error("expected Targeted=true with no targets/excludes")
	}
	if d.CorrespondingReleaseName != "svc" {
		t.Errorf("unexpected release name %q", d.CorrespondingReleaseName)
	}
}

func TestGet_AliasUsedAsName(t *testing.T) {
	c := makeChart([]*chart.Dependency{{Name: "svc", Alias: "mysvc"}}, nil)
	v := makeValues(map[string]float64{"mysvc": 0})
	deps, err := Get(c, v, nil, nil, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	d := deps[0]
	if d.Name != "svc" {
		t.Errorf("expected Name=svc, got %q", d.Name)
	}
	if d.Alias != "mysvc" {
		t.Errorf("expected Alias=mysvc, got %q", d.Alias)
	}
	if d.UsedName != "mysvc" {
		t.Errorf("expected UsedName=mysvc, got %q", d.UsedName)
	}
}

func TestGet_WeightNonZero(t *testing.T) {
	c := makeChart([]*chart.Dependency{{Name: "db"}}, nil)
	v := makeValues(map[string]float64{"db": 3})
	deps, err := Get(c, v, nil, nil, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deps[0].Weight != 3 {
		t.Errorf("expected weight 3, got %d", deps[0].Weight)
	}
}

func TestGet_WeightAsJsonNumber(t *testing.T) {
	c := makeChart([]*chart.Dependency{{Name: "svc"}}, nil)
	v := &chartutil.Values{"svc": map[string]interface{}{"weight": json.Number("5")}}
	deps, err := Get(c, v, nil, nil, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deps[0].Weight != 5 {
		t.Errorf("expected weight 5, got %d", deps[0].Weight)
	}
}

func TestGet_NegativeWeightError(t *testing.T) {
	c := makeChart([]*chart.Dependency{{Name: "svc"}}, nil)
	v := makeValues(map[string]float64{"svc": -1})
	_, err := Get(c, v, nil, nil, "", false)
	if err == nil {
		t.Error("expected error for negative weight")
	}
}

func TestGet_InvalidWeightTypeError(t *testing.T) {
	c := makeChart([]*chart.Dependency{{Name: "svc"}}, nil)
	v := &chartutil.Values{"svc": map[string]interface{}{"weight": "notanumber"}}
	_, err := Get(c, v, nil, nil, "", false)
	if err == nil {
		t.Error("expected error for non-numeric weight type")
	}
}

func TestGet_TargetsIncludeOnlyMatched(t *testing.T) {
	c := makeChart([]*chart.Dependency{{Name: "svc1"}, {Name: "svc2"}}, nil)
	v := makeValues(map[string]float64{"svc1": 0, "svc2": 0})
	deps, err := Get(c, v, []string{"svc1"}, nil, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deps[0].Targeted {
		t.Error("svc1 should be targeted")
	}
	if deps[1].Targeted {
		t.Error("svc2 should not be targeted")
	}
}

func TestGet_ExcludesRemovesMatched(t *testing.T) {
	c := makeChart([]*chart.Dependency{{Name: "svc1"}, {Name: "svc2"}}, nil)
	v := makeValues(map[string]float64{"svc1": 0, "svc2": 0})
	deps, err := Get(c, v, nil, []string{"svc1"}, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deps[0].Targeted {
		t.Error("svc1 should not be targeted (excluded)")
	}
	if !deps[1].Targeted {
		t.Error("svc2 should be targeted")
	}
}

func TestGet_TargetByAlias(t *testing.T) {
	c := makeChart([]*chart.Dependency{{Name: "svc", Alias: "myalias"}}, nil)
	v := makeValues(map[string]float64{"myalias": 0})
	deps, err := Get(c, v, []string{"myalias"}, nil, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deps[0].Targeted {
		t.Error("aliased dep should be targeted when alias is in targets")
	}
}

func TestGet_ReleasePrefix(t *testing.T) {
	c := makeChart([]*chart.Dependency{{Name: "svc"}}, nil)
	v := makeValues(map[string]float64{"svc": 0})
	deps, err := Get(c, v, nil, nil, "myns-", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deps[0].CorrespondingReleaseName != "myns-svc" {
		t.Errorf("expected release name myns-svc, got %q", deps[0].CorrespondingReleaseName)
	}
}

func TestGet_AppVersionFromSubChart(t *testing.T) {
	sub := &chart.Chart{Metadata: &chart.Metadata{Name: "svc", AppVersion: "1.2.3"}}
	c := makeChart([]*chart.Dependency{{Name: "svc"}}, []*chart.Chart{sub})
	v := makeValues(map[string]float64{"svc": 0})
	deps, err := Get(c, v, nil, nil, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deps[0].AppVersion != "1.2.3" {
		t.Errorf("expected AppVersion 1.2.3, got %q", deps[0].AppVersion)
	}
}

func TestGet_NoTagsMeansAlwaysAllowed(t *testing.T) {
	c := makeChart([]*chart.Dependency{{Name: "svc"}}, nil)
	v := makeValues(map[string]float64{"svc": 0})
	deps, err := Get(c, v, nil, nil, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deps[0].HasTags {
		t.Error("expected HasTags=false for dep with no tags")
	}
	if !deps[0].AllowedByTags {
		t.Error("expected AllowedByTags=true for dep with no tags")
	}
}

func TestGet_TagEnabledAllowsDep(t *testing.T) {
	c := makeChart([]*chart.Dependency{{Name: "svc", Tags: []string{"optional"}}}, nil)
	v := &chartutil.Values{
		"svc":  map[string]interface{}{"weight": float64(0)},
		"tags": map[string]interface{}{"optional": true},
	}
	deps, err := Get(c, v, nil, nil, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deps[0].HasTags {
		t.Error("expected HasTags=true")
	}
	if !deps[0].AllowedByTags {
		t.Error("expected AllowedByTags=true when tag is enabled in values")
	}
}

func TestGet_TagNotInValuesBlocksDep(t *testing.T) {
	c := makeChart([]*chart.Dependency{{Name: "svc", Tags: []string{"optional"}}}, nil)
	v := makeValues(map[string]float64{"svc": 0})
	deps, err := Get(c, v, nil, nil, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deps[0].HasTags {
		t.Error("expected HasTags=true")
	}
	if deps[0].AllowedByTags {
		t.Error("expected AllowedByTags=false when tag not present in values")
	}
}

func TestGet_MultipleWeights(t *testing.T) {
	c := makeChart([]*chart.Dependency{{Name: "db"}, {Name: "cache"}, {Name: "app"}}, nil)
	v := makeValues(map[string]float64{"db": 0, "cache": 1, "app": 2})
	deps, err := Get(c, v, nil, nil, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	weights := []int{0, 1, 2}
	for i, d := range deps {
		if d.Weight != weights[i] {
			t.Errorf("dep %d: expected weight %d, got %d", i, weights[i], d.Weight)
		}
	}
}
