/*
(c) Copyright 2018, Gemalto. All rights reserved.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helm

import (
	"bytes"
	"encoding/json"
	"github.com/gemalto/helm-spray/v4/internal/log"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

type Status struct {
	Namespace string
	Status    string
}

type UpgradedRelease struct {
	Info     map[string]interface{} `json:"info"`
	Manifest string                 `json:"manifest"`
}

type Release struct {
	Name       string `json:"name"`
	Revision   string `json:"revision"`
	Updated    string `json:"updated"`
	Status     string `json:"status"`
	Chart      string `json:"chart"`
	AppVersion string `json:"app_version"`
	Namespace  string `json:"namespace"`
}

// List ...
func List(level int, namespace string, debug bool) (map[string]Release, error) {
	// Prepare parameters...
	var myargs = []string{"list", "--namespace", namespace, "-o", "json"}

	// Run the list command
	if debug {
		log.Info(level, "running helm command : %v", myargs)
	}
	cmd := exec.Command("helm", myargs...)
	cmdOutput := &bytes.Buffer{}
	cmd.Stdout = cmdOutput
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	output := cmdOutput.Bytes()
	if debug {
		log.Info(level, "helm command returned:\n%s", string(output))
	}
	if err != nil {
		return nil, err
	}

	var releases []Release
	err = json.Unmarshal(output, &releases)
	if err != nil {
		return nil, err
	}

	// Return the Releases into a map
	releasesMap := make(map[string]Release, 0)
	for _, r := range releases {
		releasesMap[r.Name] = r
	}
	return releasesMap, nil
}

// UpgradeWithValues ...
func UpgradeWithValues(level int, namespace string, createNamespace bool, releaseName string, chartPath string, resetValues bool, reuseValues bool, valueFiles []string, valuesSet []string, valuesSetString []string, valuesSetFile []string, force bool, timeout int, dryRun bool, debug bool) (UpgradedRelease, error) {
	// Prepare parameters...
	var myargs = []string{"upgrade", "--install", releaseName, chartPath, "--namespace", namespace, "--timeout", strconv.Itoa(timeout) + "s", "-o", "json"}
	for _, v := range valuesSet {
		myargs = append(myargs, "--set")
		myargs = append(myargs, v)
	}
	for _, v := range valuesSetString {
		myargs = append(myargs, "--set-string")
		myargs = append(myargs, v)
	}
	for _, v := range valuesSetFile {
		myargs = append(myargs, "--set-file")
		myargs = append(myargs, v)
	}
	for _, v := range valueFiles {
		myargs = append(myargs, "-f")
		myargs = append(myargs, v)
	}
	if resetValues {
		myargs = append(myargs, "--reset-values")
	}
	if reuseValues {
		myargs = append(myargs, "--reuse-values")
	}
	if force {
		myargs = append(myargs, "--force")
	}
	if dryRun {
		myargs = append(myargs, "--dry-run")
	}
	if createNamespace {
		myargs = append(myargs, "--create-namespace")
	}

	// Run the upgrade command
	if debug {
		log.Info(level, "running helm command for \"%s\": %v", releaseName, myargs)
	}
	cmd := exec.Command("helm", myargs...)
	cmdOutput := &bytes.Buffer{}
	cmd.Stderr = os.Stderr
	cmd.Stdout = cmdOutput
	err := cmd.Run()
	output := cmdOutput.Bytes()
	if debug {
		log.Info(level, "helm command for \"%s\" returned:\n%s", releaseName, string(output))
	}
	if err != nil {
		return UpgradedRelease{}, err
	}

	var upgradedRelease UpgradedRelease
	err = json.Unmarshal(output, &upgradedRelease)
	if err != nil {
		return UpgradedRelease{}, err
	}

	return upgradedRelease, nil
}

// Fetch ...
func Fetch(chart string, version string) (string, error) {
	tempDir, err := ioutil.TempDir("", "spray-")
	if err != nil {
		return "", err
	}
	defer removeTempDir(tempDir)

	var command string
	var cmd *exec.Cmd
	var endOfLine string
	if runtime.GOOS == "windows" {
		if version != "" {
			command = "helm fetch " + chart + " --destination " + tempDir + " --version " + version
		} else {
			command = "helm fetch " + chart + " --destination " + tempDir
		}
		command = command + " && dir /b " + tempDir + " && copy " + tempDir + "\\* ."
		cmd = exec.Command("cmd", "/C", command)
		endOfLine = "\r\n"
	} else {
		if version != "" {
			command = "helm fetch " + chart + " --destination " + tempDir + " --version " + version
		} else {
			command = "helm fetch " + chart + " --destination " + tempDir
		}
		command = command + " && ls " + tempDir + " && cp " + tempDir + "/* ."
		cmd = exec.Command("sh", "-c", command)
		endOfLine = "\n"
	}

	cmdOutput := &bytes.Buffer{}
	cmd.Stdout = cmdOutput
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}

	output := cmdOutput.Bytes()
	var outputStr = string(output)
	var result = strings.Split(outputStr, endOfLine)
	return result[0], nil
}

func removeTempDir(tempDir string) {
	if err := os.RemoveAll(tempDir); err != nil {
		log.Error("Unable to remove temporary directory: %s", err)
	}
}
