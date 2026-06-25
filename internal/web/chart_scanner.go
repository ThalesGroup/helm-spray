package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

// ChartInfo represents a chart's metadata
type ChartInfo struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	AppVersion  string            `json:"appVersion"`
	Description string            `json:"description"`
	Dependencies []DependencyInfo `json:"dependencies"`
	ExecutionOrder []string       `json:"executionOrder"`
	Path        string            `json:"path"`
}

// DependencyInfo represents a chart dependency
type DependencyInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Alias   string `json:"alias"`
	Weight  int    `json:"weight"`
	Tags    []string `json:"tags"`
}

// ReleaseInfo represents a Helm release
type ReleaseInfo struct {
	Name       string `json:"name"`
	Revision   string `json:"revision"`
	Updated    string `json:"updated"`
	Status     string `json:"status"`
	Chart      string `json:"chart"`
	AppVersion string `json:"appVersion"`
	Namespace  string `json:"namespace"`
}

// SprayRequest represents a spray execution request
type SprayRequest struct {
	ChartName              string   `json:"chartName"`
	Namespace              string   `json:"namespace"`
	Targets                []string `json:"targets"`
	Excludes               []string `json:"excludes"`
	PrefixReleases         string   `json:"prefixReleases"`
	CreateNamespace        bool     `json:"createNamespace"`
	ResetValues            bool     `json:"resetValues"`
	ReuseValues            bool     `json:"reuseValues"`
	Force                  bool     `json:"force"`
	DryRun                 bool     `json:"dryRun"`
	Verbose                bool     `json:"verbose"`
	Debug                  bool     `json:"debug"`
	Timeout                int      `json:"timeout"`
	ValueFiles             []string `json:"valueFiles"`
	Values                 []string `json:"values"`
}

// ChartMeta is used to parse Chart.yaml
type ChartMeta struct {
	APIVersion    string                 `json:"apiVersion" yaml:"apiVersion"`
	Name          string                 `json:"name" yaml:"name"`
	Version       string                 `json:"version" yaml:"version"`
	AppVersion    string                 `json:"appVersion" yaml:"appVersion"`
	Description   string                 `json:"description" yaml:"description"`
	Dependencies  []DependencyMeta       `json:"dependencies" yaml:"dependencies"`
}

// DependencyMeta is used to parse chart dependencies
type DependencyMeta struct {
	Name    string   `json:"name" yaml:"name"`
	Version string   `json:"version" yaml:"version"`
	Alias   string   `json:"alias" yaml:"alias"`
	Tags    []string `json:"tags" yaml:"tags"`
	Condition string `json:"condition" yaml:"condition"`
}

// ScanCharts scans the chart directory for umbrella charts
func ScanCharts(chartDir string) ([]ChartInfo, error) {
	if chartDir == "" {
		chartDir = "."
	}

	var charts []ChartInfo

	entries, err := os.ReadDir(chartDir)
	if err != nil {
		return nil, fmt.Errorf("reading chart directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		chartPath := filepath.Join(chartDir, entry.Name())
		chartFile := filepath.Join(chartPath, "Chart.yaml")

		if _, err := os.Stat(chartFile); os.IsNotExist(err) {
			continue
		}

		info, err := parseChartFile(chartFile, chartPath)
		if err != nil {
			continue
		}

		// Only include charts with dependencies (umbrella charts)
		if len(info.Dependencies) > 0 {
			charts = append(charts, *info)
		}
	}

	return charts, nil
}

// GetChartInfo gets detailed info for a specific chart
func GetChartInfo(chartDir, chartName string) (*ChartInfo, error) {
	if chartDir == "" {
		chartDir = "."
	}

	chartPath := filepath.Join(chartDir, chartName)
	chartFile := filepath.Join(chartPath, "Chart.yaml")

	if _, err := os.Stat(chartFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("chart %s not found", chartName)
	}

	info, err := parseChartFile(chartFile, chartPath)
	if err != nil {
		return nil, err
	}

	// Parse values.yaml to get weights
	valuesFile := filepath.Join(chartPath, "values.yaml")
	weights := parseWeights(valuesFile)

	// Apply weights to dependencies
	for i := range info.Dependencies {
		dep := &info.Dependencies[i]
		usedName := dep.Name
		if dep.Alias != "" {
			usedName = dep.Alias
		}
		if w, ok := weights[usedName]; ok {
			dep.Weight = w
		}
	}

	// Compute execution order
	info.ExecutionOrder = computeExecutionOrder(info.Dependencies)

	return info, nil
}

// GetReleases gets the list of Helm releases
func GetReleases(namespace string) ([]ReleaseInfo, error) {
	args := []string{"list", "--namespace", namespace, "-o", "json"}
	if namespace == "" {
		args = []string{"list", "-o", "json"}
	}

	output, err := ExecCommand("helm", args...)
	if err != nil {
		return nil, fmt.Errorf("listing releases: %w", err)
	}

	var releases []ReleaseInfo
	if err := json.Unmarshal([]byte(output), &releases); err != nil {
		return nil, fmt.Errorf("parsing releases: %w", err)
	}

	return releases, nil
}

// ExecCommand executes a command and returns its output
func ExecCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}
	return string(output), nil
}

func parseChartFile(path, chartPath string) (*ChartInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var meta ChartMeta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	info := &ChartInfo{
		Name:        meta.Name,
		Version:     meta.Version,
		AppVersion:  meta.AppVersion,
		Description: meta.Description,
		Path:        chartPath,
	}

	for _, dep := range meta.Dependencies {
		info.Dependencies = append(info.Dependencies, DependencyInfo{
			Name:    dep.Name,
			Version: dep.Version,
			Alias:   dep.Alias,
			Tags:    dep.Tags,
		})
	}

	return info, nil
}

func parseWeights(valuesFile string) map[string]int {
	data, err := os.ReadFile(valuesFile)
	if err != nil {
		return nil
	}

	var values map[string]interface{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		return nil
	}

	weights := make(map[string]int)
	for key, val := range values {
		if m, ok := val.(map[string]interface{}); ok {
			if w, ok := m["weight"]; ok {
				switch v := w.(type) {
				case int:
					weights[key] = v
				case float64:
					weights[key] = int(v)
				}
			}
		}
	}

	return weights
}

func computeExecutionOrder(deps []DependencyInfo) []string {
	type depWithWeight struct {
		Name   string
		Weight int
	}

	var depList []depWithWeight
	for _, dep := range deps {
		name := dep.Name
		if dep.Alias != "" {
			name = dep.Alias
		}
		depList = append(depList, depWithWeight{Name: name, Weight: dep.Weight})
	}

	// Sort by weight
	for i := 0; i < len(depList); i++ {
		for j := i + 1; j < len(depList); j++ {
			if depList[j].Weight < depList[i].Weight {
				depList[i], depList[j] = depList[j], depList[i]
			}
		}
	}

	var order []string
	for _, d := range depList {
		order = append(order, fmt.Sprintf("%s (weight: %d)", d.Name, d.Weight))
	}

	return order
}

// writeJSON writes a JSON object to the response
func writeJSON(w http.ResponseWriter, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		fmt.Fprintf(w, `{"error":"%s"}`, strings.Replace(err.Error(), `"`, `\"`, -1))
		return
	}
	w.Write(data)
}

// readJSON reads a JSON object from the request
func readJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}
