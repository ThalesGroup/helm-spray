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
package main

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/gemalto/helm-spray/internal/dependencies"
	"github.com/gemalto/helm-spray/internal/log"
	"github.com/gemalto/helm-spray/internal/values"
	"github.com/gemalto/helm-spray/pkg/helm"
	"github.com/gemalto/helm-spray/pkg/kubectl"
	"helm.sh/helm/v3/pkg/chart/loader"
	cliValues "helm.sh/helm/v3/pkg/cli/values"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

type sprayCmd struct {
	chartName                   string
	chartVersion                string
	targets                     []string
	excludes                    []string
	namespace                   string
	prefixReleases              string
	prefixReleasesWithNamespace bool
	resetValues                 bool
	reuseValues                 bool
	valuesOpts                  cliValues.Options
	force                       bool
	timeout                     int
	dryRun                      bool
	verbose                     bool
	debug                       bool
}

var (
	globalUsage = `
This command upgrades sub charts from an umbrella chart supporting deployment orders.
A release is created for each subchart

Arguments shall be a chart reference, a path to a packaged chart,
a path to an unpacked chart directory or a URL.

To override values in a chart, use either the '--values'/'-f' flag and pass in a file name
or use the '--set' flag and pass configuration from the command line.
To force string values in '--set', use '--set-string' instead.
In case a value is large and therefore you want not to use neither '--values' 
nor '--set', use '--set-file' to read the single large value from file.

 $ helm spray -f myvalues.yaml ./umbrella-chart
 $ helm spray --set key1=val1,key2=val2 ./umbrella-chart
 $ helm spray stable/umbrella-chart
 $ helm spray umbrella-chart-1.0.0-rc.1+build.32.tgz -f myvalues.yaml

You can specify the '--values'/'-f' flag several times or provide a single comma separated value.
You can specify the '--set' flag several times or provide a single comma separated value.
Helm Spray does not support Helm Conditions, but supports Helm Tags, with some restrictions.

To check the generated manifests of a release without installing the chart,
the '--debug' and '--dry-run' flags can be combined. This will still require a
round-trip to the Tiller server.

There are four different ways you can express the chart you want to install:

 1. By chart reference within a repo: helm spray stable/umbrella-chart
 2. By path to a packaged chart: helm spray umbrella-chart-1.0.0-rc.1+build.32.tgz
 3. By path to an unpacked chart directory: helm spray ./umbrella-chart
 4. By absolute URL: helm spray https://example.com/charts/umbrella-chart-1.0.0-rc.1+build.32.tgz

When specifying a chart reference or a chart URL, it installs the latest version
of that chart unless you also supply a version number with the '--version' flag.

To see the list of installed releases, use 'helm list'.

To see the list of chart repositories, use 'helm repo list'. To search for
charts in a repository, use 'helm search'.
`
)

var version = "SNAPSHOT"

func newSprayCmd() *cobra.Command {

	p := &sprayCmd{}

	cmd := &cobra.Command{
		Use:          "helm spray [CHART]",
		Short:        fmt.Sprintf("upgrade subcharts from an umbrella chart (helm-spray %s)", version),
		Long:         globalUsage,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) == 0 {
				return errors.New("this command needs at least 1 argument: chart name")
			} else if len(args) > 1 {
				return errors.New("this command accepts only 1 argument: chart name")
			}

			p.chartName = args[0]

			if p.chartVersion != "" {
				if strings.HasSuffix(p.chartName, "tgz") {
					return errors.New("cannot use --version together with chart archive")
				}

				if _, err := os.Stat(p.chartName); err == nil {
					return errors.New("cannot use --version together with chart directory")
				}

				if strings.HasPrefix(p.chartName, "http://") || strings.HasPrefix(p.chartName, "https://") {
					return errors.New("cannot use --version together with chart URL")
				}
			}

			if p.prefixReleasesWithNamespace == true && p.prefixReleases != "" {
				return errors.New("cannot use both --prefix-releases and --prefix-releases-with-namespace together")
			}

			if len(p.targets) > 0 && len(p.excludes) > 0 {
				return errors.New("cannot use both --target and --exclude together")
			}

			// If chart is specified through an url, the fetch it from the url.
			if strings.HasPrefix(p.chartName, "http://") || strings.HasPrefix(p.chartName, "https://") {
				log.Info(1, "fetching chart from url \"%s\"...", p.chartName)
				var err error
				p.chartName, err = helm.Fetch(p.chartName, "")
				if err != nil {
					return fmt.Errorf("fetching chart %s: %w", p.chartName, err)
				}
			} else if _, err := os.Stat(p.chartName); err != nil {
				// If local file (or directory) does not exist, then fetch it from a repo.
				if p.chartVersion != "" {
					log.Info(1, "fetching chart \"%s\" version \"%s\" from repos...", p.chartName, p.chartVersion)
				} else {
					log.Info(1, "fetching chart \"%s\" from repos...", p.chartName)
				}
				var err error
				p.chartName, err = helm.Fetch(p.chartName, p.chartVersion)
				if err != nil {
					return fmt.Errorf("fetching chart %s with version %s: %w", p.chartName, p.chartVersion, err)
				}
			} else {
				log.Info(1, "processing chart from local file or directory \"%s\"...", p.chartName)
			}

			return p.spray()
		},
	}

	f := cmd.Flags()
	f.StringVarP(&p.chartVersion, "version", "", "", "specify the exact chart version to install. If this is not specified, the latest version is installed")
	f.StringSliceVarP(&p.targets, "target", "t", []string{}, "specify the subchart to target (can specify multiple). If '--target' is not specified, all subcharts are targeted")
	f.StringSliceVarP(&p.excludes, "exclude", "x", []string{}, "specify the subchart to exclude (can specify multiple): process all subcharts except the ones specified in '--exclude'")
	f.StringVarP(&p.prefixReleases, "prefix-releases", "", "", "prefix the releases by the given string, resulting into releases names formats:\n    \"<prefix>-<chart name or alias>\"\nAllowed characters are a-z A-Z 0-9 and -")
	f.BoolVar(&p.prefixReleasesWithNamespace, "prefix-releases-with-namespace", false, "prefix the releases by the name of the namespace, resulting into releases names formats:\n    \"<namespace>-<chart name or alias>\"")
	f.BoolVar(&p.resetValues, "reset-values", false, "when upgrading, reset the values to the ones built into the chart")
	f.BoolVar(&p.reuseValues, "reuse-values", false, "when upgrading, reuse the last release's values and merge in any overrides from the command line via '--set' and '-f'.\nIf '--reset-values' is specified, this is ignored")
	f.StringSliceVarP(&p.valuesOpts.ValueFiles, "values", "f", []string{}, "specify values in a YAML file or a URL (can specify multiple)")
	f.StringArrayVar(&p.valuesOpts.Values, "set", []string{}, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&p.valuesOpts.StringValues, "set-string", []string{}, "set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&p.valuesOpts.FileValues, "set-file", []string{}, "set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)")
	f.BoolVar(&p.force, "force", false, "force resource update through delete/recreate if needed")
	f.IntVar(&p.timeout, "timeout", 300, "time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks)\nand for liveness and readiness (like Deployments and regular Jobs completion)")
	f.BoolVar(&p.dryRun, "dry-run", false, "simulate a spray")
	f.BoolVar(&p.verbose, "verbose", false, "enable spray verbose output")
	f.BoolVar(&p.debug, "debug", false, "enable helm debug output (also include spray verbose output)")

	// When called through helm, debug mode is transmitted through the HELM_DEBUG envvar
	helmDebug := os.Getenv("HELM_DEBUG")
	if helmDebug == "1" || strings.EqualFold(helmDebug, "true") || strings.EqualFold(helmDebug, "on") {
		p.debug = true
	}
	if p.debug {
		p.verbose = true
	}

	// When called through helm, namespace is transmitted through the HELM_NAMESPACE envvar
	namespace := os.Getenv("HELM_NAMESPACE")
	if len(namespace) > 0 {
		p.namespace = namespace
	} else {
		p.namespace = "default"
	}

	return cmd
}

// Running Spray command
func (p *sprayCmd) spray() error {

	if p.debug {
		log.Info(1, "starting spray with flags: %+v\n", p)
	}

	// Load and validate the umbrella chart...
	chart, err := loader.Load(p.chartName)
	if err != nil {
		return fmt.Errorf("loading chart \"%s\": %w", p.chartName, err)
	}

	mergedValues, updatedChartValuesAsString, err := values.Merge(chart, p.reuseValues, &p.valuesOpts, p.verbose)
	if err != nil {
		return fmt.Errorf("merging values: %w", err)
	}
	if len(updatedChartValuesAsString) > 0 {
		// Write default values to a temporary file and add it to the list of values files,
		// for later usage during the calls to helm
		tempDir, err := ioutil.TempDir("", "spray-")
		if err != nil {
			return fmt.Errorf("creating temporary directory to write updated default values file for umbrella chart: %w", err)
		}
		defer removeTempDir(tempDir)
		tempFile, err := ioutil.TempFile(tempDir, "updatedDefaultValues-*.yaml")
		if err != nil {
			return fmt.Errorf("creating temporary file to write updated default values file for umbrella chart: %w", err)
		}
		defer removeTempFile(tempFile.Name())
		if _, err = tempFile.Write([]byte(updatedChartValuesAsString)); err != nil {
			return fmt.Errorf("writing updated default values file for umbrella chart into temporary file: %w", err)
		}
		err = tempFile.Close()
		if err != nil {
			return fmt.Errorf("closing temporary file to write updated default values file for umbrella chart: %w", err)
		}
		prependArray := []string{tempFile.Name()}
		p.valuesOpts.ValueFiles = append(prependArray, p.valuesOpts.ValueFiles...)
	}

	releasePrefix := ""
	if p.prefixReleasesWithNamespace && len(p.namespace) > 0 {
		releasePrefix = p.namespace + "-"
	} else if len(p.prefixReleases) > 0 {
		releasePrefix = p.prefixReleases + "-"
	}
	deps, err := dependencies.Get(chart, &mergedValues, p.targets, p.excludes, releasePrefix, p.verbose)
	if err != nil {
		return fmt.Errorf("analyzing dependencies: %w", err)
	}

	// Starting the processing...
	if len(releasePrefix) > 0 {
		log.Info(1, "deploying solution chart \"%s\" in namespace \"%s\", with releases releasePrefix \"%s-\"", p.chartName, p.namespace, releasePrefix)
	} else {
		log.Info(1, "deploying solution chart \"%s\" in namespace \"%s\"", p.chartName, p.namespace)
	}

	releases, err := helm.List(p.namespace)
	if err != nil {
		return fmt.Errorf("listing releases: %w", err)
	}

	if p.verbose {
		logRelease(releases, deps)
	}

	err = checkTargetsAndExcludes(deps, p.targets, p.excludes)
	if err != nil {
		return fmt.Errorf("checking targets and excludes: %w", err)
	}

	// Loop on the increasing weight
	for i := 0; i <= maxWeight(deps); i++ {
		statuses := make([]helm.Status, 0)
		shouldWait, err := upgrade(statuses, releases, deps, i, p)
		if err != nil {
			return err
		}
		// Wait availability of the just upgraded Releases
		if shouldWait && !p.dryRun {
			err = wait(statuses, p)
			if err != nil {
				return err
			}
		}
	}

	log.Info(1, "upgrade of solution chart \"%s\" completed", p.chartName)

	return nil
}

func upgrade(statuses []helm.Status, releases map[string]helm.Release, deps []dependencies.Dependency, currentWeight int, p *sprayCmd) (bool, error) {
	shouldWait := false
	firstInWeight := true
	// Upgrade the targeted Deployments corresponding the the current weight
	for _, dependency := range deps {
		if dependency.Targeted && dependency.AllowedByTags == true {
			if dependency.Weight == currentWeight {
				if firstInWeight {
					log.Info(1, "processing sub-charts of weight %d", dependency.Weight)
					firstInWeight = false
				}

				if release, ok := releases[dependency.CorrespondingReleaseName]; ok {
					oldRevision, _ := strconv.Atoi(release.Revision)
					log.Info(2, "upgrading release \"%s\": going from revision %d (status %s) to %d (appVersion %s)...", dependency.CorrespondingReleaseName, oldRevision, release.Status, oldRevision+1, dependency.AppVersion)

				} else {
					log.Info(2, "upgrading release \"%s\": deploying first revision (appVersion %s)...", dependency.CorrespondingReleaseName, dependency.AppVersion)
				}

				shouldWait = true

				// Add the "<dependency>.enabled" flags to ensure that only the current chart is to be executed
				depValuesSet := ""
				for _, dep := range deps {
					if dep.UsedName == dependency.UsedName {
						depValuesSet = depValuesSet + dep.UsedName + ".enabled=true,"
					} else {
						depValuesSet = depValuesSet + dep.UsedName + ".enabled=false,"
					}
				}
				var valuesSet []string
				valuesSet = append(valuesSet, p.valuesOpts.Values...)
				valuesSet = append(valuesSet, depValuesSet)

				// Upgrade the Deployment
				helmstatus, err := helm.UpgradeWithValues(
					p.namespace,
					dependency.CorrespondingReleaseName,
					p.chartName,
					p.resetValues,
					p.reuseValues,
					p.valuesOpts.ValueFiles,
					valuesSet,
					p.valuesOpts.StringValues,
					p.valuesOpts.FileValues,
					p.force,
					p.timeout,
					p.dryRun,
					p.debug,
				)
				if err != nil {
					return false, fmt.Errorf("calling helm upgrade: %w", err)
				}
				statuses = append(statuses, helmstatus)

				log.Info(3, "release: \"%s\" upgraded", dependency.CorrespondingReleaseName)

				if p.verbose {
					log.Info(3, "helm status: %s", helmstatus.Status)
					log.Info(3, "helm resources:")
					var scanner = bufio.NewScanner(strings.NewReader(helmstatus.Resources))
					for scanner.Scan() {
						if len(scanner.Text()) > 0 {
							log.Info(4, scanner.Text())
						}
					}
				}

				if !p.dryRun && helmstatus.Status != "deployed" {
					return false, errors.New("status returned by helm differs from \"deployed\", spray interrupted")
				}
			}
		}
	}
	return shouldWait, nil
}

func wait(statuses []helm.Status, p *sprayCmd) error {
	log.Info(2, "waiting for liveness and readiness...")

	sleepTime := 5
	doneDeployments := false
	doneStatefulSets := false
	doneJobs := false

	// Wait for completion of the Deployments/StatefulSets/Jobs
	for i := 0; i < p.timeout; {
		deployments := deployments(statuses)
		statefulSets := statefulSets(statuses)
		jobs := jobs(statuses)
		if len(deployments) > 0 && !doneDeployments {
			if p.verbose {
				log.Info(3, "waiting for Deployments %v", deployments)
			}
			doneDeployments, _ = kubectl.AreDeploymentsReady(deployments, p.namespace, p.debug)
		} else {
			doneDeployments = true
		}
		if len(statefulSets) > 0 && !doneStatefulSets {
			if p.verbose {
				log.Info(3, "waiting for StatefulSets %v", statefulSets)
			}
			doneStatefulSets, _ = kubectl.AreStatefulSetsReady(statefulSets, p.namespace, p.debug)
		} else {
			doneStatefulSets = true
		}
		if len(jobs) > 0 && !doneJobs {
			if p.verbose {
				log.Info(3, "waiting for Jobs %v", jobs)
			}
			doneJobs, _ = kubectl.AreJobsReady(jobs, p.namespace, p.debug)
		} else {
			doneJobs = true
		}
		if doneDeployments && doneStatefulSets && doneJobs {
			break
		}
		time.Sleep(time.Duration(sleepTime) * time.Second)
		i = i + sleepTime
	}

	if !doneDeployments || !doneStatefulSets || !doneJobs {
		return errors.New("timed out waiting for liveness and readiness")
	}
	return nil
}

func deployments(helmStatuses []helm.Status) []string {
	deployments := make([]string, 0)
	for _, status := range helmStatuses {
		deployments = append(deployments, status.Deployments...)
	}
	return deployments
}

func statefulSets(helmStatuses []helm.Status) []string {
	statefulSets := make([]string, 0)
	for _, status := range helmStatuses {
		statefulSets = append(statefulSets, status.StatefulSets...)
	}
	return statefulSets
}

func jobs(helmStatuses []helm.Status) []string {
	jobs := make([]string, 0)
	for _, status := range helmStatuses {
		jobs = append(jobs, status.Jobs...)
	}
	return jobs
}

// Retrieve the highest chart.weight in values.yaml
func maxWeight(deps []dependencies.Dependency) (m int) {
	if len(deps) > 0 {
		m = deps[0].Weight
	}
	for i := 1; i < len(deps); i++ {
		if deps[i].Weight > m {
			m = deps[i].Weight
		}
	}
	return m
}

func checkTargetsAndExcludes(deps []dependencies.Dependency, targets []string, excludes []string) error {
	// Check that the provided target(s) or exclude(s) correspond to valid sub-chart names or alias
	if len(targets) > 0 {
		for i := range targets {
			found := false
			for _, dependency := range deps {
				if targets[i] == dependency.UsedName {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("invalid targetted sub-chart name/alias \"%s\"", targets[i])
			}
		}
	} else if len(excludes) > 0 {
		for i := range excludes {
			found := false
			for _, dependency := range deps {
				if excludes[i] == dependency.UsedName {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("invalid excluded sub-chart name/alias \"%s\"", excludes[i])
			}
		}
	}
	return nil
}

func logRelease(releases map[string]helm.Release, deps []dependencies.Dependency) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.Debug)
	_, _ = fmt.Fprintln(w, "[spray]  \t subchart\t is alias of\t targeted\t weight\t| corresponding release\t revision\t status\t")
	_, _ = fmt.Fprintln(w, "[spray]  \t --------\t -----------\t --------\t ------\t| ---------------------\t --------\t ------\t")

	for _, dependency := range deps {
		currentRevision := "None"
		currentStatus := "Not deployed"
		if release, ok := releases[dependency.CorrespondingReleaseName]; ok {
			currentRevision = release.Revision
			currentStatus = release.Status
		}

		name := dependency.Name
		alias := "-"
		if dependency.Alias != "" {
			name = dependency.Alias
			alias = dependency.Name
		}

		targeted := fmt.Sprint(dependency.Targeted)
		if dependency.Targeted && dependency.HasTags && (dependency.AllowedByTags == true) {
			targeted = "true (tag match)"
		} else if dependency.Targeted && dependency.HasTags && (dependency.AllowedByTags == false) {
			targeted = "false (no tag match)"
		}

		_, _ = fmt.Fprintln(w, fmt.Sprintf("[spray]  \t %s\t %s\t %s\t %d\t| %s\t %s\t %s\t", name, alias, targeted, dependency.Weight, dependency.CorrespondingReleaseName, currentRevision, currentStatus))
	}
	_ = w.Flush()
}

func removeTempDir(tempDir string) {
	if err := os.RemoveAll(tempDir); err != nil {
		log.Error("Error: removing temporary directory: %s", err)
	}
}

func removeTempFile(tempFile string) {
	if err := os.Remove(tempFile); err != nil {
		log.Error("Error: removing temporary file: %s", err)
	}
}

func main() {
	cmd := newSprayCmd()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
