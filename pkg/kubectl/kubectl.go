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
	"log"
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

// IsDeploymentUpToDate ...
func IsDeploymentUpToDate(deployment string, namespace string) bool {
	desired := getDesired("deployment", namespace, deployment)
	updated := getUpdated("deployment", namespace, deployment)
	ready := getReady("deployment", namespace, deployment)
	if desired != ready {
		return false
	} else {
		if desired == updated {
			return true
		} else {
			return false
		}
	}
}

// IsStatefulSetUpToDate ...
func IsStatefulSetUpToDate(deployment string, namespace string) bool {
	desired := getDesired("statefulset", namespace, deployment)
	ready := getReady("statefulset", namespace, deployment)
	if desired == ready {
		return true
	} else {
		return false
	}
}

func getDesired(k8stype string, namespace string, deployment string) string {
	desired, err := exec.Command("kubectl", "--namespace", namespace, "get", k8stype, deployment, "-o=jsonpath='{.spec.replicas}'").Output()
	if err != nil {
		log.Fatal(err)
	}
	return string(desired)
}

func getReady(k8stype string, namespace string, deployment string) string {
	ready, err := exec.Command("kubectl", "--namespace", namespace, "get", k8stype, deployment, "-o=jsonpath='{.status.readyReplicas}'").Output()
	if err != nil {
		log.Fatal(err)
	}
	return string(ready)
}

func getUpdated(k8stype string, namespace string, deployment string) string {
	updated, err := exec.Command("kubectl", "--namespace", namespace, "get", k8stype, deployment, "-o=jsonpath='{.status.updatedReplicas}'").Output()
	if err != nil {
		log.Fatal(err)
	}
	return string(updated)
}
