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
	"fmt"
	"os"
	"os/exec"
	"regexp"
)

type helmStatus struct {
	status string
}

func printError(err error) {
	if err != nil {
		os.Stderr.WriteString(fmt.Sprintf("==> Error: %s\n", err.Error()))
		os.Exit(-1)
	}
}

func printOutput(outs []byte) {
	if len(outs) > 0 {
		fmt.Printf("==> Output: %s\n", string(outs))
	}
}

func parseOutput(outs []byte, helmstatus *helmStatus) {
	var status = regexp.MustCompile(`STATUS: (.*)`)
	result := status.FindStringSubmatch(string(outs))
	if len(result) > 0 {
		helmstatus.status = string(result[1])
	}
}

// Version ...
func Version() {
	fmt.Print("helm version: ")
	cmd := exec.Command("helm", "version", "--client", "--short")
	cmdOutput := &bytes.Buffer{}
	cmd.Stdout = cmdOutput
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
	output := cmdOutput.Bytes()
	printOutput(output)
}

// List ...
func List(namespace string) {
	cmd := exec.Command("helm", "list", "--namespace", namespace, "-c")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

// ListAll ...
func ListAll() {
	cmd := exec.Command("helm", "list")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
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
		os.Exit(1)
	}
}

// UpgradeWithValues ...
func UpgradeWithValues(namespace string, release string, chartName string, chartPath string, valueFiles []string, valuesSet string, dryRun bool, debug bool) {
	var myargs []string = []string{"upgrade", "--install", release, chartPath, "--namespace", namespace, "--set", chartName + ".enabled=true," + valuesSet}
    for _, v := range valueFiles {
        myargs = append(myargs, "-f")
        myargs = append(myargs, v)
    }
	if dryRun {
        myargs = append(myargs, "--dry-run")
    }

    if debug {
        fmt.Printf("running command for %s: %v\n", release, myargs)
    }

	cmd := exec.Command("helm", myargs...)
	cmdOutput := &bytes.Buffer{}
	cmd.Stdout = cmdOutput
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}

}

// Upgrade ...
func Upgrade(namespace string, chart string, chartPath string, valuesSet string, dryRun bool, debug bool) {

	var myargs []string
	if dryRun {
		myargs = []string{"upgrade", "--install", "--namespace", namespace, "--set", chart + ".enabled=true," + valuesSet, chart, chartPath, "--dry-run"}
	} else {
		myargs = []string{"upgrade", "--install", "--namespace", namespace, "--set", chart + ".enabled=true," + valuesSet, chart, chartPath}
	}

    if debug {
        fmt.Printf("running command: %v\n", myargs)
    }

	cmd := exec.Command("helm", myargs...)
	cmdOutput := &bytes.Buffer{}
	cmd.Stdout = cmdOutput
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

// GetHelmStatus ...
func GetHelmStatus(chart string) string {
	cmd := exec.Command("helm", "status", chart)
	cmdOutput := &bytes.Buffer{}
	cmd.Stdout = cmdOutput
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
	output := cmdOutput.Bytes()
	helmstatus := helmStatus{}
	parseOutput(output, &helmstatus)
	return helmstatus.status
}

// Fetch ...
func Fetch(chart string, version string) {
	fmt.Println("Fetching chart " + chart + " version " + version + " ...")
	cmd := exec.Command("helm", "fetch", chart, "--version", version)
	cmdOutput := &bytes.Buffer{}
	cmd.Stdout = cmdOutput
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}
