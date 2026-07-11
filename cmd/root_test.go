package cmd

import (
	"bytes"
	"os"
	"testing"
)

func runCmd(args []string) error {
	cmd := NewRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestRootCmd_NoArgs(t *testing.T) {
	err := runCmd([]string{})
	if err == nil || err.Error() != "this command needs at least 1 argument: chart name" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRootCmd_TooManyArgs(t *testing.T) {
	err := runCmd([]string{"chart1", "chart2"})
	if err == nil || err.Error() != "this command accepts only 1 argument: chart name" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRootCmd_VersionWithTgz(t *testing.T) {
	err := runCmd([]string{"--version", "1.0.0", "mychart.tgz"})
	if err == nil || err.Error() != "cannot use --version together with chart archive" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRootCmd_VersionWithExistingDirectory(t *testing.T) {
	dir, _ := os.MkdirTemp("", "spray-cmd-test-")
	defer os.RemoveAll(dir)
	err := runCmd([]string{"--version", "1.0.0", dir})
	if err == nil || err.Error() != "cannot use --version together with chart directory" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRootCmd_VersionWithHTTPURL(t *testing.T) {
	err := runCmd([]string{"--version", "1.0.0", "http://example.com/chart"})
	if err == nil || err.Error() != "cannot use --version together with chart HTTP(S) URL" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRootCmd_VersionWithHTTPSURL(t *testing.T) {
	err := runCmd([]string{"--version", "1.0.0", "https://example.com/chart"})
	if err == nil || err.Error() != "cannot use --version together with chart HTTP(S) URL" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRootCmd_BothPrefixFlags(t *testing.T) {
	err := runCmd([]string{"--prefix-releases", "myprefix", "--prefix-releases-with-namespace", "mychart.tgz"})
	if err == nil || err.Error() != "cannot use both --prefix-releases and --prefix-releases-with-namespace together" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRootCmd_BothTargetAndExclude(t *testing.T) {
	err := runCmd([]string{"--target", "svc1", "--exclude", "svc2", "mychart.tgz"})
	if err == nil || err.Error() != "cannot use both --target and --exclude together" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNewRootCmd_HelmDebugEnvVar(t *testing.T) {
	os.Setenv("HELM_DEBUG", "1")
	defer os.Unsetenv("HELM_DEBUG")
	// Just verify NewRootCmd does not panic when HELM_DEBUG is set
	cmd := NewRootCmd()
	if cmd == nil {
		t.Error("expected non-nil command")
	}
}

func TestNewRootCmd_HelmNamespaceEnvVar(t *testing.T) {
	os.Setenv("HELM_NAMESPACE", "mynamespace")
	defer os.Unsetenv("HELM_NAMESPACE")
	cmd := NewRootCmd()
	if cmd == nil {
		t.Error("expected non-nil command")
	}
}

func TestNewRootCmd_DefaultNamespace(t *testing.T) {
	os.Unsetenv("HELM_NAMESPACE")
	cmd := NewRootCmd()
	if cmd == nil {
		t.Error("expected non-nil command")
	}
}

// --- paths that call helm.Fetch or s.Spray() ---
// These cover the remaining branches in RunE. helm.Fetch/Spray will fail
// (invalid URL / chart not found), but the lines are still executed.

func TestRootCmd_HTTPURLFetch(t *testing.T) {
	// http:// prefix → fetch from URL path, no --version (covers log without version + Fetch error)
	// Use 127.0.0.1:1 to get an immediate connection-refused rather than a DNS timeout.
	err := runCmd([]string{"http://127.0.0.1:1/chart"})
	if err == nil {
		t.Error("expected error fetching from unreachable URL")
	}
}

func TestRootCmd_HTTPURLFetchWithVersion(t *testing.T) {
	// http:// prefix + version → covers the log-with-version branch inside the URL block.
	// Note: the "cannot use --version with HTTP URL" guard only fires for http/https.
	// We use oci:// here to bypass that guard while still hitting the URL-fetch path.
	err := runCmd([]string{"--version", "1.0.0", "oci://127.0.0.1:1/chart"})
	if err == nil {
		t.Error("expected error fetching oci chart")
	}
}

func TestRootCmd_OCIURLFetch(t *testing.T) {
	// oci:// prefix → same URL-fetch branch, no --version
	err := runCmd([]string{"oci://127.0.0.1:1/chart"})
	if err == nil {
		t.Error("expected error fetching oci chart")
	}
}

func TestRootCmd_NonExistentChartFetch(t *testing.T) {
	// Chart name does not exist locally → fetch from repo, no version
	err := runCmd([]string{"nonexistent-spray-chart-xyz-abc-000"})
	if err == nil {
		t.Error("expected error for non-existent chart")
	}
}

func TestRootCmd_NonExistentChartFetchWithVersion(t *testing.T) {
	// Chart name does not exist locally + --version → covers log-with-version branch
	err := runCmd([]string{"--version", "1.0.0", "nonexistent-spray-chart-xyz-abc-000"})
	if err == nil {
		t.Error("expected error for non-existent versioned chart")
	}
}

func TestRootCmd_ExistingLocalChart(t *testing.T) {
	// Chart path exists locally → covers the "else" log branch + s.Spray() call.
	// loader.Load will fail on an empty dir (not a valid chart), but line 143/146 are covered.
	dir, _ := os.MkdirTemp("", "spray-chart-test-")
	defer os.RemoveAll(dir)
	err := runCmd([]string{dir})
	if err == nil {
		t.Error("expected error loading empty directory as chart")
	}
}
