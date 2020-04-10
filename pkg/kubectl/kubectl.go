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
package kubectl

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func AreDeploymentsReady(names []string, namespace string, debug bool) (bool, error) {
	return areWorkloadsReady("deployment", names, namespace, debug)
}

func AreStatefulSetsReady(names []string, namespace string, debug bool) (bool, error) {
	return areWorkloadsReady("statefulset", names, namespace, debug)
}

func AreJobsReady(names []string, namespace string, debug bool) (bool, error) {
	if len(names) == 0 {
		return true, nil
	}
	template := generateTemplate(names, "{{if .status.succeeded}}{{if lt .status.succeeded 1}}{{printf \"%s \" .metadata.name}}{{end}}{{end}}")
	if debug {
		fmt.Printf("kubectl template: %s\n", template)
	}
	cmd := exec.Command("kubectl", "--namespace", namespace, "get", "jobs", "-o", "go-template="+template+"")
	cmd.Stderr = os.Stderr
	result, err := cmd.Output()
	if err != nil {
		// Cannot make the difference between an error when calling kubectl and no corresponding resource found. Return "" in any case.
		return false, err
	}
	strResult := string(result)
	if debug {
		fmt.Printf("kubectl output: %s\n", strResult)
	}
	if len(strResult) > 0 {
		return false, nil
	}
	return true, nil
}

func areWorkloadsReady(k8sObjectType string, names []string, namespace string, debug bool) (bool, error) {
	if len(names) == 0 {
		return true, nil
	}
	template := generateTemplate(names, "{{if .status.readyReplicas}}{{if lt .status.readyReplicas .spec.replicas}}{{printf \"%s \" .metadata.name}}{{end}}{{else}}{{printf \"%s \" .metadata.name}}{{end}}")
	if debug {
		fmt.Printf("kubectl template: %s\n", template)
	}
	cmd := exec.Command("kubectl", "--namespace", namespace, "get", k8sObjectType, "-o", "go-template="+template+"")
	cmd.Stderr = os.Stderr
	result, err := cmd.Output()
	if err != nil {
		// Cannot make the difference between an error when calling kubectl and no corresponding resource found. Return "" in any case.
		return false, err
	}
	strResult := string(result)
	if debug {
		fmt.Printf("kubectl output: %s\n", strResult)
	}
	if len(strResult) > 0 {
		return false, nil
	}
	return true, nil
}

func generateTemplate(names []string, test string) string {
	var sb strings.Builder
	sb.WriteString("{{range .items}}")
	sb.WriteString("{{if ")
	if len(names) == 1 {
		sb.WriteString("eq \"")
		sb.WriteString(names[0])
		sb.WriteString("\" .metadata.name")
	} else {
		sb.WriteString("or ")
		for _, object := range names {
			sb.WriteString("(")
			sb.WriteString("eq \"")
			sb.WriteString(object)
			sb.WriteString("\" .metadata.name")
			sb.WriteString(") ")
		}
	}
	sb.WriteString("}}")
	sb.WriteString(test)
	sb.WriteString("{{end}}")
	sb.WriteString("{{end}}")
	return sb.String()
}
