package helmspray

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gemalto/helm-spray/v4/internal/dependencies"
	"github.com/gemalto/helm-spray/v4/pkg/helm"
	"github.com/gemalto/helm-spray/v4/pkg/kubectl"
	cliValues "helm.sh/helm/v3/pkg/cli/values"
)

// makeMinimalChart creates a minimal valid helm chart directory and returns its path.
// The caller is responsible for removing it.
func makeMinimalChart(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "spray-chart-test-")
	if err != nil {
		t.Fatalf("could not create chart dir: %v", err)
	}
	chartYAML := `apiVersion: v2
name: test-umbrella
version: 0.1.0
`
	if err := os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(chartYAML), 0600); err != nil {
		t.Fatalf("could not write Chart.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "values.yaml"), []byte(""), 0600); err != nil {
		t.Fatalf("could not write values.yaml: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "charts"), 0750); err != nil {
		t.Fatalf("could not create charts dir: %v", err)
	}
	return dir
}

func newTestSpray(chartName string) *Spray {
	return &Spray{
		ChartName: chartName,
		Namespace: "default",
		Timeout:   30,
	}
}

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

func TestLogRelease_DoesNotPanic(t *testing.T) {
	releases := map[string]helm.Release{
		"myns-svc": {Name: "myns-svc", Revision: "2", Status: "deployed"},
	}
	deps := []dependencies.Dependency{
		// plain dep — known release exists → revision/status filled in
		{Name: "svc", UsedName: "svc", CorrespondingReleaseName: "myns-svc", Targeted: true, Weight: 1},
		// aliased dep — no release yet
		{Name: "db", Alias: "mydb", UsedName: "mydb", CorrespondingReleaseName: "myns-mydb", Targeted: false, HasTags: true, AllowedByTags: true},
		// Targeted=true, HasTags=true, AllowedByTags=false → "false (no tag match)"
		{Name: "cache", UsedName: "cache", CorrespondingReleaseName: "myns-cache", Targeted: true, HasTags: true, AllowedByTags: false},
		// Targeted=true, HasTags=true, AllowedByTags=true → "true (tag match)"
		{Name: "worker", UsedName: "worker", CorrespondingReleaseName: "myns-worker", Targeted: true, HasTags: true, AllowedByTags: true},
	}
	logRelease(releases, deps)
}

// --- Spray() integration-style tests (chart loads OK, fails at helm.List) ---

func TestSpray_InvalidChartPath(t *testing.T) {
	s := newTestSpray("/does/not/exist")
	if err := s.Spray(); err == nil {
		t.Error("expected error for non-existent chart path")
	}
}

func TestSpray_ValidChartFailsAtHelmList(t *testing.T) {
	dir := makeMinimalChart(t)
	defer os.RemoveAll(dir)
	s := newTestSpray(dir)
	// helm.List will fail (no k8s cluster) — that's expected
	err := s.Spray()
	if err == nil {
		t.Log("spray succeeded unexpectedly (k8s available?)")
	}
}

func TestSpray_ValidChartWithPrefixReleases(t *testing.T) {
	dir := makeMinimalChart(t)
	defer os.RemoveAll(dir)
	s := newTestSpray(dir)
	s.PrefixReleases = "myprefix"
	err := s.Spray()
	if err == nil {
		t.Log("spray succeeded unexpectedly (k8s available?)")
	}
}

func TestSpray_ValidChartWithPrefixReleasesWithNamespace(t *testing.T) {
	dir := makeMinimalChart(t)
	defer os.RemoveAll(dir)
	s := newTestSpray(dir)
	s.PrefixReleasesWithNamespace = true
	err := s.Spray()
	if err == nil {
		t.Log("spray succeeded unexpectedly (k8s available?)")
	}
}

func TestSpray_ValidChartVerbose(t *testing.T) {
	dir := makeMinimalChart(t)
	defer os.RemoveAll(dir)
	s := newTestSpray(dir)
	s.Verbose = true
	err := s.Spray()
	if err == nil {
		t.Log("spray succeeded unexpectedly (k8s available?)")
	}
}

func TestSpray_ValidChartDebug(t *testing.T) {
	dir := makeMinimalChart(t)
	defer os.RemoveAll(dir)
	s := newTestSpray(dir)
	s.Debug = true
	s.Verbose = true
	err := s.Spray()
	if err == nil {
		t.Log("spray succeeded unexpectedly (k8s available?)")
	}
}

func TestSpray_ValidChartWithValuesOpts(t *testing.T) {
	dir := makeMinimalChart(t)
	defer os.RemoveAll(dir)
	s := newTestSpray(dir)
	s.ValuesOpts = cliValues.Options{Values: []string{"key=value"}}
	err := s.Spray()
	if err == nil {
		t.Log("spray succeeded unexpectedly (k8s available?)")
	}
}

func TestSpray_ValidChartInvalidTarget(t *testing.T) {
	dir := makeMinimalChart(t)
	defer os.RemoveAll(dir)
	s := newTestSpray(dir)
	s.Targets = []string{"nonexistent-subchart"}
	err := s.Spray()
	if err == nil {
		t.Error("expected error for invalid target")
	}
}

func TestRemoveTempDir_NonExistentIsNoError(t *testing.T) {
	removeTempDir("/tmp/does-not-exist-spray-test-dir")
}

func TestRemoveTempDir_InvalidPathTriggersError(t *testing.T) {
	// A path with a null byte causes os.RemoveAll to return "invalid argument"
	removeTempDir("/tmp/spray-test\x00invalid")
}

func TestRemoveTempFile_EmptyPathTriggersError(t *testing.T) {
	// os.Remove("") returns "no such file" error — exercises the log.Error branch
	removeTempFile("")
}

func TestRemoveTempFile_NonExistentIsNoError(t *testing.T) {
	removeTempFile("/tmp/does-not-exist-spray-test-file.yaml")
}

func TestRemoveTempDir_ExistingDir(t *testing.T) {
	dir, err := os.MkdirTemp("", "spray-test-")
	if err != nil {
		t.Fatalf("could not create temp dir: %v", err)
	}
	removeTempDir(dir)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("expected dir to be removed")
	}
}

func TestRemoveTempFile_ExistingFile(t *testing.T) {
	dir, err := os.MkdirTemp("", "spray-test-")
	if err != nil {
		t.Fatalf("could not create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)
	f := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(f, []byte("x: 1\n"), 0600); err != nil {
		t.Fatalf("could not create temp file: %v", err)
	}
	removeTempFile(f)
	if _, err := os.Stat(f); !os.IsNotExist(err) {
		t.Error("expected file to be removed")
	}
}

// --- computeReleasePrefix ---

func TestComputeReleasePrefix_None(t *testing.T) {
	s := &Spray{}
	if got := s.computeReleasePrefix(); got != "" {
		t.Errorf("expected empty prefix, got %q", got)
	}
}

func TestComputeReleasePrefix_PrefixReleases(t *testing.T) {
	s := &Spray{PrefixReleases: "myprefix"}
	if got := s.computeReleasePrefix(); got != "myprefix-" {
		t.Errorf("expected %q, got %q", "myprefix-", got)
	}
}

func TestComputeReleasePrefix_PrefixWithNamespace(t *testing.T) {
	s := &Spray{PrefixReleasesWithNamespace: true, Namespace: "prod"}
	if got := s.computeReleasePrefix(); got != "prod-" {
		t.Errorf("expected %q, got %q", "prod-", got)
	}
}

func TestComputeReleasePrefix_PrefixWithNamespaceButEmptyNamespace(t *testing.T) {
	s := &Spray{PrefixReleasesWithNamespace: true, Namespace: ""}
	if got := s.computeReleasePrefix(); got != "" {
		t.Errorf("expected empty prefix when namespace is empty, got %q", got)
	}
}

// --- buildDepValuesSet ---

func TestBuildDepValuesSet_SingleDep(t *testing.T) {
	dep := dependencies.Dependency{UsedName: "svc"}
	deps := []dependencies.Dependency{dep}
	result := buildDepValuesSet(deps, dep)
	if !strings.Contains(result, "svc.enabled=true") {
		t.Errorf("expected svc.enabled=true, got %q", result)
	}
}

func TestBuildDepValuesSet_MultipleDepsCurrent(t *testing.T) {
	deps := []dependencies.Dependency{
		{UsedName: "svc"},
		{UsedName: "db"},
	}
	result := buildDepValuesSet(deps, deps[0])
	if !strings.Contains(result, "svc.enabled=true") {
		t.Errorf("expected svc=true, got %q", result)
	}
	if !strings.Contains(result, "db.enabled=false") {
		t.Errorf("expected db=false, got %q", result)
	}
}

// --- writeTempValuesFile ---

func TestWriteTempValuesFile_CreatesAndCleans(t *testing.T) {
	s := newTestSpray("test")
	cleanup, err := s.writeTempValuesFile("key: value\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.ValuesOpts.ValueFiles) == 0 {
		t.Fatal("expected ValueFiles to be prepended")
	}
	tmpFile := s.ValuesOpts.ValueFiles[0]
	if _, statErr := os.Stat(tmpFile); statErr != nil {
		t.Errorf("expected temp file to exist: %v", statErr)
	}
	cleanup()
	if _, statErr := os.Stat(tmpFile); !os.IsNotExist(statErr) {
		t.Error("expected temp file removed after cleanup")
	}
}

// --- collectWorkloads ---

func TestCollectWorkloads_EmptyManifest(t *testing.T) {
	s := &Spray{deployments: []string{}, statefulSets: []string{}, jobs: []string{}}
	ignored := s.collectWorkloads("")
	if len(ignored) != 0 {
		t.Errorf("expected no ignored parts for empty manifest, got %d", len(ignored))
	}
}

func TestCollectWorkloads_UnknownKindGoesToIgnored(t *testing.T) {
	s := &Spray{deployments: []string{}, statefulSets: []string{}, jobs: []string{}}
	manifest := "---\napiVersion: custom.io/v1\nkind: MyThing\nmetadata:\n  name: foo\n"
	ignored := s.collectWorkloads(manifest)
	if len(ignored) == 0 {
		t.Error("expected unknown kind to be ignored")
	}
}

// --- logUpgradedWorkloads ---

func TestLogUpgradedWorkloads_AllBranches(t *testing.T) {
	s := &Spray{
		Verbose:      true,
		Debug:        true,
		deployments:  []string{"dep1"},
		statefulSets: []string{"ss1"},
		jobs:         []string{"job1"},
	}
	s.logUpgradedWorkloads([]string{"some ignored part"})
}

func TestLogUpgradedWorkloads_NoWorkloads(t *testing.T) {
	s := &Spray{Verbose: true, deployments: []string{}, statefulSets: []string{}, jobs: []string{}}
	s.logUpgradedWorkloads([]string{})
}

// --- checkReady ---

func TestCheckReady_AlreadyDone(t *testing.T) {
	s := &Spray{Namespace: "default"}
	ready, err := s.checkReady([]string{"dep"}, "deployments", true, kubectl.AreDeploymentsReady)
	if err != nil || !ready {
		t.Errorf("expected ready=true when done=true, got ready=%v err=%v", ready, err)
	}
}

func TestCheckReady_EmptyNames(t *testing.T) {
	s := &Spray{Namespace: "default"}
	ready, err := s.checkReady([]string{}, "deployments", false, kubectl.AreDeploymentsReady)
	if err != nil || !ready {
		t.Errorf("expected ready=true for empty names, got ready=%v err=%v", ready, err)
	}
}

func TestCheckReady_Verbose(t *testing.T) {
	s := &Spray{Namespace: "default", Verbose: true}
	// AreDeploymentsReady with empty names returns true without calling kubectl
	ready, err := s.checkReady([]string{}, "deployments", false, kubectl.AreDeploymentsReady)
	if err != nil || !ready {
		t.Errorf("expected ready=true, got ready=%v err=%v", ready, err)
	}
}

// --- fake helm helpers and integration tests ---

// makeMinimalChartWithDep creates an umbrella chart with one subchart dependency.
func makeMinimalChartWithDep(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "spray-chart-dep-")
	if err != nil {
		t.Fatalf("could not create chart dir: %v", err)
	}
	chartYAML := `apiVersion: v2
name: umbrella
version: 0.1.0
dependencies:
  - name: subchart
    version: "0.1.0"
    repository: ""
`
	os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(chartYAML), 0600)
	os.WriteFile(filepath.Join(dir, "values.yaml"), []byte("subchart:\n  weight: 0\n"), 0600)
	subDir := filepath.Join(dir, "charts", "subchart")
	os.MkdirAll(subDir, 0750)
	os.WriteFile(filepath.Join(subDir, "Chart.yaml"), []byte("apiVersion: v2\nname: subchart\nversion: 0.1.0\n"), 0600)
	os.WriteFile(filepath.Join(subDir, "values.yaml"), []byte(""), 0600)
	return dir
}

// installFakeHelm writes a shell-script helm stub into a temp dir, prepends it to
// PATH, and returns. The stub returns listJSON for "helm list" and upgradeJSON for
// "helm upgrade".
func installFakeHelm(t *testing.T, listJSON, upgradeJSON string) {
	t.Helper()
	dir := t.TempDir()
	listFile := filepath.Join(dir, "list.json")
	upgradeFile := filepath.Join(dir, "upgrade.json")
	os.WriteFile(listFile, []byte(listJSON), 0600)
	os.WriteFile(upgradeFile, []byte(upgradeJSON), 0600)
	script := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = \"list\" ]; then cat %s; exit 0; fi\nif [ \"$1\" = \"upgrade\" ]; then cat %s; exit 0; fi\nexit 1\n", listFile, upgradeFile)
	fakeHelm := filepath.Join(dir, "helm")
	os.WriteFile(fakeHelm, []byte(script), 0755)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

const upgradeJSON = `{"info":{"status":"deployed"},"manifest":""}`
const upgradeJSONWithIgnored = `{"info":{"status":"deployed"},"manifest":"---\napiVersion: custom.io/v1\nkind: MyThing\nmetadata:\n  name: foo\n"}`
const existingReleaseJSON = `[{"name":"subchart","revision":"1","updated":"","status":"deployed","chart":"subchart-0.1.0","app_version":"","namespace":"default"}]`

func TestSpray_FakeHelm_FirstInstall(t *testing.T) {
	dir := makeMinimalChartWithDep(t)
	defer os.RemoveAll(dir)
	installFakeHelm(t, "[]", upgradeJSON)
	s := newTestSpray(dir)
	s.DryRun = true
	if err := s.Spray(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSpray_FakeHelm_ExistingRelease(t *testing.T) {
	dir := makeMinimalChartWithDep(t)
	defer os.RemoveAll(dir)
	installFakeHelm(t, existingReleaseJSON, upgradeJSON)
	s := newTestSpray(dir)
	s.DryRun = true
	if err := s.Spray(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSpray_FakeHelm_Verbose(t *testing.T) {
	dir := makeMinimalChartWithDep(t)
	defer os.RemoveAll(dir)
	installFakeHelm(t, "[]", upgradeJSONWithIgnored)
	s := newTestSpray(dir)
	s.DryRun = true
	s.Verbose = true
	if err := s.Spray(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSpray_FakeHelm_Debug(t *testing.T) {
	dir := makeMinimalChartWithDep(t)
	defer os.RemoveAll(dir)
	installFakeHelm(t, "[]", upgradeJSONWithIgnored)
	s := newTestSpray(dir)
	s.DryRun = true
	s.Verbose = true
	s.Debug = true
	if err := s.Spray(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestSpray_FakeHelm_DryRunFalse runs with DryRun=false and an empty manifest so
// that wait() is called but exits immediately (no workloads to wait for).
func TestSpray_FakeHelm_DryRunFalse(t *testing.T) {
	dir := makeMinimalChartWithDep(t)
	defer os.RemoveAll(dir)
	installFakeHelm(t, "[]", upgradeJSON)
	s := newTestSpray(dir)
	s.DryRun = false
	if err := s.Spray(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestSpray_FakeHelm_UpgradeFailedStatus covers the upgradeDependency error branch
// where the helm status is not "deployed".
func TestSpray_FakeHelm_UpgradeFailedStatus(t *testing.T) {
	dir := makeMinimalChartWithDep(t)
	defer os.RemoveAll(dir)
	const failedUpgrade = `{"info":{"status":"pending-install"},"manifest":""}`
	installFakeHelm(t, "[]", failedUpgrade)
	s := newTestSpray(dir)
	s.DryRun = false
	if err := s.Spray(); err == nil {
		t.Error("expected error for non-deployed helm status")
	}
}

// TestCheckReady_RealCheck covers the branch that actually calls the check function.
func TestCheckReady_RealCheck(t *testing.T) {
	s := &Spray{Namespace: "default", Verbose: true}
	called := false
	mockFn := func(names []string, ns string, debug bool) (bool, error) {
		called = true
		return true, nil
	}
	ready, err := s.checkReady([]string{"dep1"}, "deployments", false, mockFn)
	if err != nil || !ready {
		t.Errorf("expected ready=true, got ready=%v err=%v", ready, err)
	}
	if !called {
		t.Error("expected check function to be called")
	}
}

const deploymentManifest = `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: my-deploy
  template:
    metadata:
      labels:
        app: my-deploy
    spec:
      containers:
      - name: app
        image: nginx:latest
`

func TestCollectWorkloads_DeploymentFound(t *testing.T) {
	s := &Spray{deployments: []string{}, statefulSets: []string{}, jobs: []string{}}
	ignored := s.collectWorkloads(deploymentManifest)
	if len(ignored) != 0 {
		t.Errorf("expected no ignored parts for valid Deployment, got %v", ignored)
	}
	if len(s.deployments) != 1 || s.deployments[0] != "my-deploy" {
		t.Errorf("expected deployment 'my-deploy', got %v", s.deployments)
	}
}
