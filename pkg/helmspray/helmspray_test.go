package helmspray

import (
	"testing"

	"github.com/gemalto/helm-spray/v4/internal/dependencies"
)

func TestMaxWeight_Nil(t *testing.T) {
	if maxWeight(nil) != 0 {
		t.Error("expected 0 for nil deps")
	}
}

func TestMaxWeight_Empty(t *testing.T) {
	if maxWeight([]dependencies.Dependency{}) != 0 {
		t.Error("expected 0 for empty deps")
	}
}

func TestMaxWeight_Single(t *testing.T) {
	deps := []dependencies.Dependency{{Weight: 5}}
	if maxWeight(deps) != 5 {
		t.Errorf("expected 5, got %d", maxWeight(deps))
	}
}

func TestMaxWeight_ReturnsMax(t *testing.T) {
	deps := []dependencies.Dependency{{Weight: 1}, {Weight: 3}, {Weight: 2}}
	if maxWeight(deps) != 3 {
		t.Errorf("expected 3, got %d", maxWeight(deps))
	}
}

func TestMaxWeight_AllZero(t *testing.T) {
	deps := []dependencies.Dependency{{Weight: 0}, {Weight: 0}}
	if maxWeight(deps) != 0 {
		t.Errorf("expected 0, got %d", maxWeight(deps))
	}
}

func TestMaxWeight_FirstIsMax(t *testing.T) {
	deps := []dependencies.Dependency{{Weight: 10}, {Weight: 3}, {Weight: 7}}
	if maxWeight(deps) != 10 {
		t.Errorf("expected 10, got %d", maxWeight(deps))
	}
}

func TestCheckTargetsAndExcludes_NoneSpecified(t *testing.T) {
	deps := []dependencies.Dependency{{UsedName: "svc"}}
	if err := checkTargetsAndExcludes(deps, nil, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckTargetsAndExcludes_ValidTarget(t *testing.T) {
	deps := []dependencies.Dependency{{UsedName: "svc"}}
	if err := checkTargetsAndExcludes(deps, []string{"svc"}, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckTargetsAndExcludes_InvalidTarget(t *testing.T) {
	deps := []dependencies.Dependency{{UsedName: "svc"}}
	if err := checkTargetsAndExcludes(deps, []string{"unknown"}, nil); err == nil {
		t.Error("expected error for unknown target")
	}
}

func TestCheckTargetsAndExcludes_ValidExclude(t *testing.T) {
	deps := []dependencies.Dependency{{UsedName: "svc"}}
	if err := checkTargetsAndExcludes(deps, nil, []string{"svc"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckTargetsAndExcludes_InvalidExclude(t *testing.T) {
	deps := []dependencies.Dependency{{UsedName: "svc"}}
	if err := checkTargetsAndExcludes(deps, nil, []string{"unknown"}); err == nil {
		t.Error("expected error for unknown exclude")
	}
}

func TestCheckTargetsAndExcludes_MultipleTargetsAllValid(t *testing.T) {
	deps := []dependencies.Dependency{{UsedName: "svc1"}, {UsedName: "svc2"}}
	if err := checkTargetsAndExcludes(deps, []string{"svc1", "svc2"}, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckTargetsAndExcludes_TargetMatchesAlias(t *testing.T) {
	deps := []dependencies.Dependency{{UsedName: "myalias", Name: "svc"}}
	if err := checkTargetsAndExcludes(deps, []string{"myalias"}, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckTargetsAndExcludes_OneOfMultipleTargetsInvalid(t *testing.T) {
	deps := []dependencies.Dependency{{UsedName: "svc1"}, {UsedName: "svc2"}}
	if err := checkTargetsAndExcludes(deps, []string{"svc1", "ghost"}, nil); err == nil {
		t.Error("expected error when one of multiple targets is invalid")
	}
}
