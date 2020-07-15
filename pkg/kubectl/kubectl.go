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
	"strings"
	"os"
	"os/exec"
)

// Version ...
func Version() {
	fmt.Println("kubectl version: ")
	cmd := exec.Command("kubectl", "version")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

// GetPods ...
func GetPods(namespace string) {
	cmd := exec.Command("kubectl", "get", "pods", "-n", namespace)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

// GetStatefulSet ...
func GetStatefulSet(namespace string) {
	cmd := exec.Command("kubectl", "get", "statefulset", "--namespace", namespace)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

// GetDeployment ...
func GetDeployment(namespace string) {
	cmd := exec.Command("kubectl", "get", "deployment", "--namespace", namespace)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

// GetJob ...
func GetJob(namespace string) {
	cmd := exec.Command("kubectl", "get", "jobs", "--namespace", namespace)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}


// IsDeploymentUpToDate ...
func IsDeploymentUpToDate(deployment string, namespace string) bool {
	desired := getDesired("deployment", deployment, namespace)
	current := getCurrent("deployment", deployment, namespace)
	updated := getUpdated("deployment", deployment, namespace)
	ready := getReady("deployment", deployment, namespace)
	if desired != ready {
		return false
	} else {
		if (desired == updated) && (desired == current) {
			return true
		} else {
			return false
		}
	}
}

// IsStatefulSetUpToDate ...
func IsStatefulSetUpToDate(statefulset string, namespace string) bool {
	desired := getDesired("statefulset", statefulset, namespace)
	ready := getReady("statefulset", statefulset, namespace)
	current := getStatefulsetCurrent(statefulset, namespace)
	if desired != ready {
		return false
	} else {
		if (desired == current) {
			return true
		} else {
			return false
		}
	}
}

// IsJobCompleted ...
func IsJobCompleted(job string, namespace string) bool {
	succeeded := getSucceeded("job", job, namespace)
	if succeeded == "'1'" {
		return true
	} else {
		return false
	}
}

// GetStatefulSetStrategy
func GetStatefulSetStrategy(statefulset string, namespace string) string {
	s := getObjectDescriptionItem("statefulset", statefulset, namespace, ".spec.updateStrategy.type")
//	if len(s) >= 2 {
//		if s[0] == `'` && s[len(s)-1] == `'` {
//			return s[1 : len(s)-1]
//		}
//	}
	return strings.Trim(s, "'")
}

// Utility functions to get informations extracted from a 'kubectl get' command result
func getObjectDescriptionItem(k8sObjectType string, objectName string, namespace string, itemJsonPath string) string {
	item, err := exec.Command("kubectl", "--namespace", namespace, "get", k8sObjectType, objectName, "-o=jsonpath='{" + itemJsonPath + "}'").Output()
	if err != nil {
		// Cannot make the difference between an error when calling kubectl and no corresponding resource found. Return "" in any case.
		return ""
	}
	return string(item)
}

func getDesired(k8sObjectType string, objectName string, namespace string) string {
	return getObjectDescriptionItem(k8sObjectType, objectName, namespace, ".spec.replicas")
}

func getCurrent(k8sObjectType string, objectName string, namespace string) string {
	return getObjectDescriptionItem(k8sObjectType, objectName, namespace, ".status.replicas")
}

func getStatefulsetCurrent(objectName string, namespace string) string {
	return getObjectDescriptionItem("statefulset", objectName, namespace, ".status.currentReplicas")
}

func getReady(k8sObjectType string, objectName string, namespace string) string {
	return getObjectDescriptionItem(k8sObjectType, objectName, namespace, ".status.readyReplicas")
}

func getUpdated(k8sObjectType string, objectName string, namespace string) string {
	return getObjectDescriptionItem(k8sObjectType, objectName, namespace, ".status.updatedReplicas")
}

func getSucceeded(k8sObjectType string, objectName string, namespace string) string {
	return getObjectDescriptionItem(k8sObjectType, objectName, namespace, ".status.succeeded")
}

