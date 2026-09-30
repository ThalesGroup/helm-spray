package helm

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func installFakeHelm(t *testing.T, listJSON, upgradeJSON string) {
	t.Helper()
	dir := t.TempDir()
	listFile := filepath.Join(dir, "list.json")
	upgradeFile := filepath.Join(dir, "upgrade.json")
	os.WriteFile(listFile, []byte(listJSON), 0600)
	os.WriteFile(upgradeFile, []byte(upgradeJSON), 0600)
	script := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = \"list\" ]; then cat %s; exit 0; fi\nif [ \"$1\" = \"upgrade\" ]; then cat %s; exit 0; fi\nexit 1\n", listFile, upgradeFile)
	os.WriteFile(filepath.Join(dir, "helm"), []byte(script), 0755)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestRemoveTempDir_NonExistentIsNoError(t *testing.T) {
	removeTempDir("/tmp/does-not-exist-helm-spray-test-dir")
}

func TestRemoveTempDir_InvalidPathTriggersError(t *testing.T) {
	// A path with a null byte causes os.RemoveAll to return "invalid argument"
	removeTempDir("/tmp/spray-test\x00invalid")
}

func TestRemoveTempDir_ExistingDir(t *testing.T) {
	dir, err := os.MkdirTemp("", "helm-spray-test-")
	if err != nil {
		t.Fatalf("could not create temp dir: %v", err)
	}
	removeTempDir(dir)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("expected dir to be removed")
	}
}

func TestList_EmptyReleases(t *testing.T) {
	installFakeHelm(t, "[]", "{}")
	releases, err := List(1, "default", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(releases) != 0 {
		t.Errorf("expected empty map, got %v", releases)
	}
}

func TestList_Debug(t *testing.T) {
	installFakeHelm(t, "[]", "{}")
	_, err := List(1, "default", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestList_WithRelease(t *testing.T) {
	listJSON := `[{"name":"myrelease","revision":"2","updated":"","status":"deployed","chart":"mychart-0.1.0","app_version":"1.0","namespace":"default"}]`
	installFakeHelm(t, listJSON, "{}")
	releases, err := List(1, "default", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r, ok := releases["myrelease"]
	if !ok {
		t.Fatal("expected release 'myrelease' in map")
	}
	if r.Revision != "2" {
		t.Errorf("expected revision 2, got %q", r.Revision)
	}
}

func TestUpgradeWithValues_Success(t *testing.T) {
	installFakeHelm(t, "[]", `{"info":{"status":"deployed"},"manifest":""}`)
	r, err := UpgradeWithValues(UpgradeOptions{
		Level:       1,
		Namespace:   "default",
		ReleaseName: "myrelease",
		ChartPath:   "/tmp/chart",
		Timeout:     30,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Info["status"] != "deployed" {
		t.Errorf("expected status=deployed, got %v", r.Info["status"])
	}
}

func TestUpgradeWithValues_AllFlags(t *testing.T) {
	installFakeHelm(t, "[]", `{"info":{"status":"deployed"},"manifest":""}`)
	_, err := UpgradeWithValues(UpgradeOptions{
		Level:           1,
		Namespace:       "default",
		ReleaseName:     "myrelease",
		ChartPath:       "/tmp/chart",
		Timeout:         30,
		ResetValues:     true,
		ReuseValues:     true,
		Force:           true,
		DryRun:          true,
		CreateNamespace: true,
		Debug:           true,
		ValuesSet:       []string{"key=val"},
		ValuesSetString: []string{"str=hello"},
		ValuesSetFile:   []string{"file=/dev/null"},
		ValueFiles:      []string{"/dev/null"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveTempDir_WithContents(t *testing.T) {
	dir, err := os.MkdirTemp("", "helm-spray-test-")
	if err != nil {
		t.Fatalf("could not create temp dir: %v", err)
	}
	f := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(f, []byte("data"), 0600); err != nil {
		t.Fatalf("could not write file: %v", err)
	}
	removeTempDir(dir)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("expected dir with contents to be removed")
	}
}
