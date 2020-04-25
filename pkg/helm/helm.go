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
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gemalto/helm-spray/internal/log"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// Types returned by some of the functions
type Status struct {
	Namespace    string
	Status       string
	Resources    string
	Deployments  []string
	StatefulSets []string
	Jobs         []string
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

func getStringAfterLast(value string, a string) string {
	// Get substring after a string.
	pos := strings.LastIndex(value, a)
	if pos == -1 {
		return ""
	}
	adjustedPos := pos + len(a)
	if adjustedPos >= len(value) {
		return ""
	}
	return value[adjustedPos:]
}

func getStringBetween(value string, a string, b string) string {
	// Get substring between two strings.
	posFirst := strings.Index(value, a)
	if posFirst == -1 {
		return ""
	}

	posFirstAdjusted := posFirst + len(a)
	posLast := strings.Index(value[posFirstAdjusted:], b)
	if posLast == -1 {
		return ""
	}
	posLastAdjusted := posFirstAdjusted + posLast
	return value[posFirstAdjusted:posLastAdjusted]
}

// Parse the "helm status"-like output to extract relevant information
// WARNING: this code has been developed and tested with version 'v2.12.2' of Helm
//          it may need to be adapted to other versions of Helm.
func parseStatusOutput(outs []byte, helmstatus *Status) {
	var outStr = string(outs)

	// Extract the namespace
	var namespace = regexp.MustCompile(`^NAMESPACE: (.*)`)
	result := namespace.FindStringSubmatch(outStr)
	if len(result) > 1 {
		helmstatus.Namespace = result[1]
	}

	// Extract the status
	var status = regexp.MustCompile(`^STATUS: (.*)`)
	result = status.FindStringSubmatch(outStr)
	if len(result) > 1 {
		helmstatus.Status = result[1]
	}

	// Extract the resources
	helmstatus.Resources = getStringAfterLast(outStr, "RESOURCES:")

	// ... and get the Deployments from the resources
	var res = getStringBetween(helmstatus.Resources+"==>", "==> v1/Deployment", "==>") + "\n" +
		getStringBetween(helmstatus.Resources+"==>", "==> v1beta2/Deployment", "==>") + "\n" +
		getStringBetween(helmstatus.Resources+"==>", "==> v1beta1/Deployment", "==>")
	var resAsSlice = make([]string, 0)
	var scanner = bufio.NewScanner(strings.NewReader(res))
	for scanner.Scan() {
		if len(scanner.Text()) > 0 {
			name := strings.Fields(scanner.Text())[0]
			resAsSlice = append(resAsSlice, name)
		}
	}
	if len(resAsSlice) > 0 {
		helmstatus.Deployments = resAsSlice[1:]
	}

	// ... and get the StatefulSets from the resources
	res = getStringBetween(helmstatus.Resources+"==>", "==> v1/StatefulSet", "==>") + "\n" +
		getStringBetween(helmstatus.Resources+"==>", "==> v1beta2/StatefulSet", "==>") + "\n" +
		getStringBetween(helmstatus.Resources+"==>", "==> v1beta1/StatefulSet", "==>")

	resAsSlice = make([]string, 0)
	scanner = bufio.NewScanner(strings.NewReader(res))
	for scanner.Scan() {
		if len(scanner.Text()) > 0 {
			name := strings.Fields(scanner.Text())[0]
			resAsSlice = append(resAsSlice, name)
		}
	}
	if len(resAsSlice) > 0 {
		helmstatus.StatefulSets = resAsSlice[1:]
	}

	// ... and get the Jobs from the resources
	res = getStringBetween(helmstatus.Resources+"==>", "==> v1/Job", "==>")
	resAsSlice = make([]string, 0)
	scanner = bufio.NewScanner(strings.NewReader(res))
	for scanner.Scan() {
		if len(scanner.Text()) > 0 {
			name := strings.Fields(scanner.Text())[0]
			resAsSlice = append(resAsSlice, name)
		}
	}
	if len(resAsSlice) > 0 {
		helmstatus.Jobs = resAsSlice[1:]
	}
}

// Helm functions calls
// --------------------

// List ...
func List(namespace string) (map[string]Release, error) {
	helmlist := make(map[string]Release, 0)

	// Get the list of Releases of the chunk
	cmd := exec.Command("helm", "list", "--namespace", namespace, "-o", "json")
	cmdOutput := &bytes.Buffer{}
	cmd.Stdout = cmdOutput
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	// Transform the received json into structs
	output := cmdOutput.Bytes()
	var releases []Release
	err := json.Unmarshal(output, &releases)
	if err != nil {
		return nil, err
	}

	// Add the Releases into a map
	for _, r := range releases {
		helmlist[r.Name] = r
	}

	return helmlist, nil
}

// UpgradeWithValues ...
func UpgradeWithValues(namespace string, releaseName string, chartPath string, resetValues bool, reuseValues bool, valueFiles []string, valuesSet []string, valuesSetString []string, valuesSetFile []string, force bool, timeout int, dryRun bool, debug bool) (Status, error) {
	// Prepare parameters...
	var myargs = []string{"upgrade", "--install", releaseName, chartPath, "--namespace", namespace, "--timeout", strconv.Itoa(timeout) + "s"}

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
	if debug {
		myargs = append(myargs, "--debug")
		log.Info(1, "running helm command for \"%s\": %v\n", releaseName, myargs)
	}

	// Run the upgrade command
	cmd := exec.Command("helm", myargs...)

	cmdOutput := &bytes.Buffer{}
	cmd.Stderr = os.Stderr
	cmd.Stdout = cmdOutput
	err := cmd.Run()
	output := cmdOutput.Bytes()

	if debug {
		fmt.Printf(string(output))
	}
	if err != nil {
		return Status{}, err
	}

	// Parse the ending helm status.
	status := Status{}
	parseStatusOutput(output, &status)
	return status, nil
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
