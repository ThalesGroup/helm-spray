package kubectl

import (
	"strings"
	"testing"
)

func TestGenerateTemplate_SingleName(t *testing.T) {
	result := generateTemplate([]string{"my-deploy"}, "<TEST>")
	expected := `{{range .items}}{{if eq "my-deploy" .metadata.name}}<TEST>{{end}}{{end}}`
	if result != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
	}
}

func TestGenerateTemplate_MultipleNamesUsesOr(t *testing.T) {
	result := generateTemplate([]string{"dep-a", "dep-b"}, "<BODY>")
	if !strings.HasPrefix(result, "{{range .items}}{{if or ") {
		t.Errorf("expected 'or' for multiple names, got %q", result)
	}
	if !strings.Contains(result, `eq "dep-a" .metadata.name`) {
		t.Error("missing condition for dep-a")
	}
	if !strings.Contains(result, `eq "dep-b" .metadata.name`) {
		t.Error("missing condition for dep-b")
	}
	if !strings.HasSuffix(result, "{{end}}{{end}}") {
		t.Error("expected correct template suffix")
	}
}

func TestGenerateTemplate_ThreeNames(t *testing.T) {
	result := generateTemplate([]string{"a", "b", "c"}, "<BODY>")
	for _, name := range []string{"a", "b", "c"} {
		if !strings.Contains(result, `eq "`+name+`" .metadata.name`) {
			t.Errorf("missing condition for name %q", name)
		}
	}
	if !strings.Contains(result, "<BODY>") {
		t.Error("expected body present in template")
	}
}

func TestGenerateTemplate_AlwaysWrapsInRange(t *testing.T) {
	result := generateTemplate([]string{"x"}, "body")
	if !strings.HasPrefix(result, "{{range .items}}") {
		t.Error("template should start with range directive")
	}
	if !strings.HasSuffix(result, "{{end}}") {
		t.Error("template should end with end directive")
	}
}

func TestGenerateTemplate_BodyIsEmbedded(t *testing.T) {
	body := "{{printf \"%s\" .metadata.name}}"
	result := generateTemplate([]string{"svc"}, body)
	if !strings.Contains(result, body) {
		t.Errorf("expected body %q in result %q", body, result)
	}
}

func TestGenerateTemplate_SingleNameDoesNotUseOr(t *testing.T) {
	result := generateTemplate([]string{"only"}, "body")
	if strings.Contains(result, " or ") {
		t.Error("single name should not use 'or' combinator")
	}
}

func TestAreDeploymentsReady_EmptyNamesReturnsTrue(t *testing.T) {
	// Empty names list hits the early-return path without any kubectl call
	ready, err := AreDeploymentsReady([]string{}, "default", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ready {
		t.Error("expected ready=true for empty names list")
	}
}

func TestAreStatefulSetsReady_EmptyNamesReturnsTrue(t *testing.T) {
	ready, err := AreStatefulSetsReady([]string{}, "default", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ready {
		t.Error("expected ready=true for empty names list")
	}
}

func TestAreJobsReady_EmptyNamesReturnsTrue(t *testing.T) {
	ready, err := AreJobsReady([]string{}, "default", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ready {
		t.Error("expected ready=true for empty names list")
	}
}
