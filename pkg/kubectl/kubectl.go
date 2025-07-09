package kubectl

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

import (
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gemalto/helm-spray/v4/internal/log"
)

func GetDeployments(namespace string) ([]string, error) {
	return getWorkloads("deployments", namespace)
}

func GetStatefulSets(namespace string) ([]string, error) {
	return getWorkloads("statefulsets", namespace)
}

func GetJobs(namespace string) ([]string, error) {
	return getWorkloads("jobs", namespace)
}

func AreDeploymentsReady(names []string, namespace string, debug bool) (bool, error) {
	return areWorkloadsReady("deployment", names, namespace, debug)
}

func AreStatefulSetsReady(names []string, namespace string, debug bool) (bool, error) {
	return areWorkloadsReady("statefulset", names, namespace, debug)
}

func AreJobsReady(names []string, namespace string, debug bool) (bool, error) {
	for _, name := range names {
		cmd := exec.Command("kubectl", "--namespace", namespace, "get", "job", name, "--output=jsonpath={.status.succeeded}")
		cmd.Stderr = os.Stderr
		result, err := cmd.Output()
		if err != nil {
			// Cannot make the difference between an error when calling kubectl and no corresponding resource found. Return "" in any case.
			return false, err
		}
		strResult := string(result)
		if debug {
			log.Info(3, "kubectl output: %s", strResult)
		}
		succeeded, _ := strconv.Atoi(strResult)
		if succeeded < 1 {
			if debug {
				log.Info(3, "job %s is not completed", name)
			}
			return false, nil
		}
	}
	return true, nil
}

func getWorkloads(k8sObjectType string, namespace string) ([]string, error) {
	cmd := exec.Command("kubectl", "--namespace", namespace, "get", k8sObjectType, "--output=jsonpath={.items..metadata.name}")
	cmd.Stderr = os.Stderr
	result, err := cmd.Output()
	if err != nil {
		// Cannot make the difference between an error when calling kubectl and no corresponding resource found. Return "" in any case.
		return nil, err
	}
	return strings.Split(string(result), " "), nil
}

func areWorkloadsReady(k8sObjectType string, names []string, namespace string, debug bool) (bool, error) {
	if len(names) == 0 {
		return true, nil
	}
	if debug {
		template := generateTemplate(names, "{{$ready := 0}}{{if .status.readyReplicas}}{{$ready = .status.readyReplicas}}{{end}}{{$current := .spec.replicas}}{{if .status.currentReplicas}}{{$current = .status.currentReplicas}}{{end}}{{$updated := 0}}{{if .status.updatedReplicas}}{{$updated = .status.updatedReplicas}}{{end}}{{printf \"{name: %s, ready: %d, current: %d, updated: %d}\" .metadata.name $ready $current $updated}}")
		log.Info(3, "kubectl template: %s", template)
		cmd := exec.Command("kubectl", "--namespace", namespace, "get", k8sObjectType, "-o", "go-template="+template)
		cmd.Stderr = os.Stderr
		result, err := cmd.Output()
		if err != nil {
			// Activating debug logs should not generate additional errors so let's only warn the user and go further
			// If there is a real error linked to kubectl execution, it will pop up just after
			log.Info(3, "warning: cannot get kubectl output because of an error (%s)", err)
		} else {
			log.Info(3, "kubectl output: %s", string(result))
		}
	}
	template := generateTemplate(names, "{{$ready := 0}}{{if .status.readyReplicas}}{{$ready = .status.readyReplicas}}{{end}}{{$current := .spec.replicas}}{{if .status.currentReplicas}}{{$current = .status.currentReplicas}}{{end}}{{$updated := 0}}{{if .status.updatedReplicas}}{{$updated = .status.updatedReplicas}}{{end}}{{if or (lt $ready .spec.replicas) (lt $current .spec.replicas) (lt $updated .spec.replicas)}}{{printf \"%s \" .metadata.name}}{{end}}")
	if debug {
		log.Info(3, "kubectl template: %s", template)
	}
	cmd := exec.Command("kubectl", "--namespace", namespace, "get", k8sObjectType, "-o", "go-template="+template)
	cmd.Stderr = os.Stderr
	result, err := cmd.Output()
	if err != nil {
		// Cannot make the difference between an error when calling kubectl and no corresponding resource found. Return false in any case.
		return false, err
	}
	strResult := string(result)
	if debug {
		log.Info(3, "kubectl output: %s", strResult)
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
