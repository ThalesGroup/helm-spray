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
	"strings"
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"encoding/json"
)


// Types returned by some of the functions
type HelmStatus struct {
	Namespace string
	Status string
	Resources string
	Deployments []string
	StatefulSets []string
	Jobs []string
}

type HelmRelease struct {
	Name			string
	Revision		int
	Updated			string
	Status			string
	Chart			string
	AppVersion		string
	Namespace		string
}


// Printing error or outputs
func printError(err error) {
	if err != nil {
		os.Stderr.WriteString(fmt.Sprintf("==> Error: %s\n", err.Error()))
		os.Exit(1)
	}
}

func printOutput(outs []byte) {
	if len(outs) > 0 {
		fmt.Printf("==> Output: %s\n", string(outs))
	}
}

// Utility functions to parse strings
func getStringAfter(value string, a string) string {
	// Get substring after a string.
	pos := strings.LastIndex(value, a)
	if pos == -1 {
		return ""
	}
	adjustedPos := pos + len(a)
	if adjustedPos >= len(value) {
		return ""
	}
	return value[adjustedPos:len(value)]
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

// Parse the "helm status"-like output to extract releant information
func parseStatusOutput(outs []byte, helmstatus *HelmStatus) {
	var out_str = string(outs)

	// Extract the namespace
	var namespace = regexp.MustCompile(`NAMESPACE: (.*)`)
	result := namespace.FindStringSubmatch(out_str)
	if len(result) > 0 {
		helmstatus.Namespace = string(result[1])
	}

	// Extract the status
	var status = regexp.MustCompile(`STATUS: (.*)`)
	result = status.FindStringSubmatch(out_str)
	if len(result) > 0 {
		helmstatus.Status = string(result[1])
	}

	// Extract the resources
	helmstatus.Resources = getStringAfter (out_str, "RESOURCES:")

	// ... and get the Deployments from the resources
	var res = getStringBetween (helmstatus.Resources + "==>", "==> v1beta1/Deployment", "==>")
	var res_as_slice = make([]string, 0)
	var scanner = bufio.NewScanner(strings.NewReader(res))
	for scanner.Scan() {
		if len (scanner.Text()) > 0 {
			name := strings.Fields(scanner.Text())[0]
			res_as_slice = append (res_as_slice, name)
		}
	}
	if len(res_as_slice) > 0 {
		helmstatus.Deployments = res_as_slice[1:]
	}

	// ... and get the StatefulSets from the resources
	res = getStringBetween (helmstatus.Resources + "==>", "==> v1beta1/StatefulSet", "==>")
	res_as_slice = make([]string, 0)
	scanner = bufio.NewScanner(strings.NewReader(res))
	for scanner.Scan() {
		if len (scanner.Text()) > 0 {
			name := strings.Fields(scanner.Text())[0]
			res_as_slice = append (res_as_slice, name)
		}
	}
	if len(res_as_slice) > 0 {
		helmstatus.StatefulSets = res_as_slice[1:]
	}

	// ... and get the Jobs from the resources
	res = getStringBetween (helmstatus.Resources + "==>", "==> v1/Job", "==>")
	res_as_slice = make([]string, 0)
	scanner = bufio.NewScanner(strings.NewReader(res))
	for scanner.Scan() {
		if len (scanner.Text()) > 0 {
			name := strings.Fields(scanner.Text())[0]
			res_as_slice = append (res_as_slice, name)
		}
	}
	if len(res_as_slice) > 0 {
		helmstatus.Jobs = res_as_slice[1:]
	}
}


// Helm functions calls
// --------------------

// Version ...
func Version() {
	fmt.Print("helm version: ")
	cmd := exec.Command("helm", "version", "--client", "--short")
	cmdOutput := &bytes.Buffer{}
	cmd.Stdout = cmdOutput
	if err := cmd.Run(); err != nil {
		printError(err)
		os.Exit(1)
	}
	output := cmdOutput.Bytes()
	printOutput(output)
}

// List ...
type helmReleasesList struct {
	Next			string
	Releases		[]HelmRelease
}

func List(namespace string) map[string]HelmRelease {
	helmlist := make(map[string]HelmRelease, 0)
	next := "~FIRST"

	// Loop on the chunks returned by the "helm list" command
	for next != "" {
		if next == "~FIRST" {
			next = ""
		}

		// Get the list of Releases of the chunk
		cmd := exec.Command("helm", "list", "--namespace", namespace, "-c", "--output", "json", "-o", next)
		cmdOutput := &bytes.Buffer{}
		cmd.Stdout = cmdOutput
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			printError(err)
			os.Exit(1)
		}

		// Transform the received json into structs
		output := cmdOutput.Bytes()
		var releases helmReleasesList
		json.Unmarshal([]byte(output), &releases)

		// Add the Releases into a map
		for _, r := range releases.Releases {
			helmlist[r.Name] = r
		}

		// Loop on next chunk
		next = releases.Next
	}

	return helmlist
}

// ListAll ...
func ListAll() map[string]HelmRelease {
	return List ("")
}

// Delete chart
func Delete(chart string, dryRun bool) {
	var myargs []string
	if dryRun {
		myargs = []string{"helm", "delete", "--purge", chart, "--dry-run"}
	} else {
		myargs = []string{"delete", "--purge", chart}
	}
	cmd := exec.Command("helm", myargs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		printError(err)
		os.Exit(1)
	}
}

// UpgradeWithValues ...
func UpgradeWithValues(namespace string, release string, chartName string, chartPath string, resetValues bool, reuseValues bool, valueFiles []string, valuesSet string, force bool, dryRun bool, debug bool) HelmStatus {
	var myargs []string = []string{"upgrade", "--install", release, chartPath, "--namespace", namespace, "--set", valuesSet}
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
		fmt.Printf("[spray] running helm command for \"%s\": %v\n", release, myargs)
	}


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
		printError(err)
		os.Exit(1)
	}

	// but in both cases, stdout is required to get and parse the ending helm status.
	helmstatus := HelmStatus{}
	parseStatusOutput(output, &helmstatus)
	return helmstatus
}

// Status ...
func Status(chart string) HelmStatus {
	cmd := exec.Command("helm", "status", chart)
	cmdOutput := &bytes.Buffer{}
	cmd.Stdout = cmdOutput
	if err := cmd.Run(); err != nil {
		printError(err)
		os.Exit(1)
	}
	output := cmdOutput.Bytes()
	helmstatus := HelmStatus{}
	parseStatusOutput(output, &helmstatus)
	return helmstatus
}

// Fetch ...
func Fetch(chart string, version string) {
	cmd := exec.Command("helm", "fetch", chart, "--version", version)
	cmdOutput := &bytes.Buffer{}
	cmd.Stdout = cmdOutput
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		printError(err)
		os.Exit(1)
	}
}
