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
	"errors"
	"fmt"
	"os"
	"bufio"
	"strconv"
	"strings"
	"time"
	"text/tabwriter"

	"github.com/gemalto/helm-spray/pkg/helm"
	"github.com/gemalto/helm-spray/pkg/kubectl"
	chartutil "k8s.io/helm/pkg/chartutil"

	"github.com/spf13/cobra"
)

type sprayCmd struct {
	chartName						string
	chartVersion					string
	targets							[]string
	namespace						string
	prefixReleases					string
	prefixReleasesWithNamespace		bool
	resetValues						bool
	reuseValues						bool
	valueFiles						[]string
	valuesSet						string
	force							bool
	timeout		 					int
	dryRun							bool
	verbose							bool
	debug							bool
}

// Dependency ...
type Dependency struct {
	Name						string
	Alias						string
	UsedName					string
	Targeted					bool
	Weight						int
	CorrespondingReleaseName	string
}

var (
	globalUsage = `
This command upgrades sub charts from an umbrella chart supporting deployment orders.

Arguments shall be a chart reference, a path to a packaged chart,
a path to an unpacked chart directory or a URL.

To override values in a chart, use either the '--values' flag and pass in a file
or use the '--set' flag and pass configuration from the command line.
To force string values in '--set', use '--set-string' instead.
In case a value is large and therefore you want not to use neither '--values' 
nor '--set', use '--set-file' to read the single large value from file.

 $ helm spray -f myvalues.yaml ./umbrella-chart
 $ helm spray --set key1=val1,key2=val2 ./umbrella-chart
 $ helm spray stable/umbrella-chart
 $ helm spray umbrella-chart-1.0.0-rc.1+build.32.tgz -f myvalues.yaml

You can specify the '--values'/'-f' flag only one time.
You can specify the '--set' flag one times, but several values comma separated.
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

To see the list of chart repositories, use 'helm repo list'. To search for
charts in a repository, use 'helm search'.
`
)

func newSprayCmd(args []string) *cobra.Command {

	p := &sprayCmd{}

	cmd := &cobra.Command{
		Use:			"helm spray [CHART]",
		Short:			`Helm plugin to upgrade subcharts from an umbrella chart`,
		Long:			globalUsage,
		SilenceUsage:	true,
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 1 {
				return errors.New("This command needs at least 1 argument: chart name")
			}

			p.chartName = args[0]

			if p.chartVersion != "" {
				if strings.HasSuffix(p.chartName, "tgz") {
					os.Stderr.WriteString("You cannot use --version together with chart archive\n")
					os.Exit(1)
				}

				if _, err := os.Stat(p.chartName); err == nil {
					os.Stderr.WriteString("You cannot use --version together with chart directory\n")
					os.Exit(1)
				}

				if (strings.HasPrefix(p.chartName, "http://") || strings.HasPrefix(p.chartName, "https://")) {
					os.Stderr.WriteString("You cannot use --version together with chart URL\n")
					os.Exit(1)
				}
			}

			if p.prefixReleasesWithNamespace == true && p.prefixReleases != "" {
				os.Stderr.WriteString("You cannot use both --prefix-releases and --prefix-releases-with-namespace together\n")
				os.Exit(1)
			}


			// If chart is specified through an url, the fetch it from the url.
			if (strings.HasPrefix(p.chartName, "http://") || strings.HasPrefix(p.chartName, "https://")) {
				log(1, "fetching chart from url \"%s\"...", p.chartName)
				p.chartName = helm.Fetch(p.chartName, "")

			// If local file (or directory) does not exist, then fetch it from a repo.
			} else if _, err := os.Stat(p.chartName); err != nil {
				if p.chartVersion != "" {
					log(1, "fetching chart \"%s\" version \"%s\" from repos...", p.chartName, p.chartVersion)
				} else {
					log(1, "fetching chart \"%s\" from repos...", p.chartName)
				}
				p.chartName = helm.Fetch(p.chartName, p.chartVersion)

			} else {
				log(1, "processing chart from local file or directory \"%s\"...", p.chartName)
			}

			return p.spray()
		},
	}

	f := cmd.Flags()
	f.StringSliceVarP(&p.valueFiles, "values", "f", []string{}, "specify values in a YAML file or a URL (can specify multiple)")
	f.StringVarP(&p.namespace, "namespace", "n", "default", "namespace to spray the chart into")
	f.StringVarP(&p.chartVersion, "version", "", "", "specify the exact chart version to install. If this is not specified, the latest version is installed")
	f.StringSliceVarP(&p.targets, "target", "t", []string{}, "specify the subchart to target (can specify multiple). If '--target' is not specified, all subcharts are targeted")
	f.StringVarP(&p.prefixReleases, "prefix-releases", "", "", "prefix the releases by the given string, resulting into releases names formats:\n    \"<prefix>-<chart name or alias>\"\nAllowed characters are a-z A-Z 0-9 and -")
	f.BoolVar(&p.prefixReleasesWithNamespace, "prefix-releases-with-namespace", false, "prefix the releases by the name of the namespace, resulting into releases names formats:\n    \"<namespace>-<chart name or alias>\"")
	f.BoolVar(&p.resetValues, "reset-values", false, "when upgrading, reset the values to the ones built into the chart")
	f.BoolVar(&p.reuseValues, "reuse-values", false, "when upgrading, reuse the last release's values and merge in any overrides from the command line via '--set' and '-f'.\nIf '--reset-values' is specified, this is ignored")
	f.StringVarP(&p.valuesSet, "set", "", "", "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.BoolVar(&p.force, "force", false, "force resource update through delete/recreate if needed")
	f.IntVar(&p.timeout, "timeout", 300, "time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks)\nand for liveness and readiness (like Deployments and regular Jobs completion)")
	f.BoolVar(&p.dryRun, "dry-run", false, "simulate a spray")
	f.BoolVar(&p.verbose, "verbose", false, "enable spray verbose output")
	f.BoolVar(&p.debug, "debug", false, "enable helm debug output (also include spray verbose output)")
	f.Parse(args)

	// When called through helm, debug mode is transmitted through the HELM_DEBUG envvar
	if !p.debug {
		if "1" == os.Getenv("HELM_DEBUG") {
			p.debug = true
		}
	}
	if p.debug {
		p.verbose = true
	}

	return cmd

}

func (p *sprayCmd) spray() error {

	// Load and valide the umbrella chart...
	chart, err := chartutil.Load(p.chartName)
	if err != nil {
		panic(fmt.Errorf("%s", err))
	}

	// Load and valid the requirements file...
	reqs, err := chartutil.LoadRequirements(chart)
	if err != nil {
		panic(fmt.Errorf("%s", err))
	}

	// Load default values...
	values, err := chartutil.CoalesceValues(chart, chart.GetValues())
	if err != nil {
		panic(fmt.Errorf("%s", err))
	}

	// Build the list of all rependencies, and their key attributes
	dependencies := make([]Dependency, len(reqs.Dependencies))
	for i, req := range reqs.Dependencies {
		// Dependency name and alias
		dependencies[i].Name = req.Name
		dependencies[i].Alias = req.Alias
		if req.Alias == "" {
			dependencies[i].UsedName = dependencies[i].Name
		} else {
			dependencies[i].UsedName = dependencies[i].Alias
		}

		// Is dependency targeted? If --target is specificed, it should match the name of the current dependency; 
		// if no --target is specified, then all dependencies are targeted
		if len(p.targets) > 0 {
			dependencies[i].Targeted = false
			for j := range p.targets {
				if p.targets[j] == dependencies[i].UsedName {
					dependencies[i].Targeted = true
				}
			}
		} else {
			dependencies[i].Targeted = true
		}

		// Get weight of the dependency. If no weight is specified, setting it to 0
		dependencies[i].Weight = 0
		depi, err := values.Table(dependencies[i].UsedName)
		if (err == nil && depi["weight"] != nil) {
			w64 := depi["weight"].(float64)
			w, err := strconv.Atoi(strconv.FormatFloat(w64, 'f', 0, 64))
			if err != nil {
				panic(fmt.Errorf("%s", err))
			}
			dependencies[i].Weight = w
		}

		// Compute the corresponding release name
		if p.prefixReleasesWithNamespace == true {
			dependencies[i].CorrespondingReleaseName = p.namespace + "-" + dependencies[i].UsedName
		} else if p.prefixReleases != "" {
			dependencies[i].CorrespondingReleaseName = p.prefixReleases + "-" + dependencies[i].UsedName
		} else {
			dependencies[i].CorrespondingReleaseName = dependencies[i].UsedName
		}
	}


	// Starting the processing...
	if p.prefixReleasesWithNamespace == true {
		log(1, "deploying solution chart \"%s\" in namespace \"%s\", with releases prefix \"%s-\"", p.chartName, p.namespace, p.namespace)
	} else if p.prefixReleases != "" {
		log(1, "deploying solution chart \"%s\" in namespace \"%s\", with releases prefix \"%s-\"", p.chartName, p.namespace, p.prefixReleases)
	} else {
		log(1, "deploying solution chart \"%s\" in namespace \"%s\"", p.chartName, p.namespace)
	}

	helmReleases := helm.List(p.namespace)

	if p.verbose {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.Debug)
		fmt.Fprintln(w, "[spray]  \t subchart\t is alias of\t targeted\t weight\t| corresponding release\t revision\t status\t")
		fmt.Fprintln(w, "[spray]  \t --------\t -----------\t --------\t ------\t| ---------------------\t --------\t ------\t")

		for _, dependency := range dependencies {
			currentRevision := "None"
			currentStatus := "Not deployed"
			if release, ok := helmReleases[dependency.CorrespondingReleaseName]; ok {
				currentRevision = strconv.Itoa(release.Revision)
				currentStatus = release.Status
			}

			if dependency.Alias == "" {
				fmt.Fprintln(w, fmt.Sprintf ("[spray]  \t %s\t %s\t %t\t %d\t| %s\t %s\t %s\t", dependency.Name, "-", dependency.Targeted, dependency.Weight, dependency.CorrespondingReleaseName, currentRevision, currentStatus))

			} else {
				fmt.Fprintln(w, fmt.Sprintf ("[spray]  \t %s\t %s\t %t\t %d\t| %s\t %s\t %s\t", dependency.Alias, dependency.Name, dependency.Targeted, dependency.Weight, dependency.CorrespondingReleaseName, currentRevision, currentStatus))
			}
		}
w.Flush()
	}

	// Loop on the increasing weight
	for i := 0; i <= getMaxWeight(dependencies); i++ {
		shouldWait := false
		firstInWeight := true
		helmstatusses := make([]helm.HelmStatus, 0)

		// Upgrade the targeted Deployments corresponding the the current weight
		for _, dependency := range dependencies {
			if dependency.Targeted {
				if dependency.Weight == i {
					if firstInWeight {
						log(1, "processing sub-charts of weight %d", dependency.Weight)
						firstInWeight = false
					}

					if release, ok := helmReleases[dependency.CorrespondingReleaseName]; ok {
						log(2, "upgrading release \"%s\": going from revision %d (status %s) to %d...", dependency.CorrespondingReleaseName, release.Revision, release.Status, release.Revision+1)

					} else {
						log(2, "upgrading release \"%s\": deploying first revision...", dependency.CorrespondingReleaseName)
					}

					shouldWait = true

					// Add the "<dependency>.enabled" flags to ensure that only the current chart is to be executed
					valuesSet := ""
					for _, dep := range dependencies {
						if dep.UsedName == dependency.UsedName {
							valuesSet = valuesSet + dep.UsedName + ".enabled=true,"
						} else {
							valuesSet = valuesSet + dep.UsedName + ".enabled=false,"
						}
					}
					valuesSet = valuesSet + p.valuesSet

					// Upgrade the Deployment
					helmstatus := helm.UpgradeWithValues(p.namespace, dependency.CorrespondingReleaseName, p.chartName, p.resetValues, p.reuseValues, p.valueFiles, valuesSet, p.force, p.timeout, p.dryRun, p.debug)
					helmstatusses = append(helmstatusses, helmstatus)

					log(3, "release: \"%s\" upgraded", dependency.CorrespondingReleaseName)
					if p.verbose {
						log(3, "helm status: %s", helmstatus.Status)
					}

					if p.verbose {
						log(3, "helm resources:")
						var scanner = bufio.NewScanner(strings.NewReader(helmstatus.Resources))
						for scanner.Scan() {
							if len (scanner.Text()) > 0 {
								log(4, scanner.Text())
							}
						}
					}

					if helmstatus.Status == "" {
						log(2, "Warning: no status returned by helm.")
					} else if helmstatus.Status != "DEPLOYED" {
						log(2, "Error: status returned by helm differs from \"DEPLOYED\". Cannot continue spray processing.")
						os.Exit(1)
					}
				}
			}
		}

		// Wait availability of the just upgraded Releases
		if shouldWait {
			log(2, "waiting for Liveness and Readiness...")

			if !p.dryRun {
				for _, status := range helmstatusses {
					sleep_time := 5

					// Wait for completion of the Deployments update
					for _, dep := range status.Deployments {
						done := false

						for i := 0; i < p.timeout; {
							if p.verbose {
								log(3, "waiting for Deployment \"%s\"", dep)
							}
							if kubectl.IsDeploymentUpToDate(dep, p.namespace) {
								done = true
								break
							}
							time.Sleep(time.Duration(sleep_time) * time.Second)
							i = i + sleep_time
						}

						if !done {
							os.Stderr.WriteString("Error: UPGRADE FAILED: timed out waiting for the condition\n")
							os.Stderr.WriteString("==> Error: exit status 1\n")
							os.Exit(1)
						}
					}

					// Wait for completion of the StatefulSets update
					for _, ss := range status.StatefulSets {
						done := false

						for i := 0; i < p.timeout; {
							if p.verbose {
								log(3, "waiting for StatefulSet \"%s\"", ss)
							}
							if kubectl.IsStatefulSetUpToDate(ss, p.namespace) {
								done = true
								break
							}
							time.Sleep(time.Duration(sleep_time) * time.Second)
							i = i + sleep_time
						}

						if !done {
							os.Stderr.WriteString("Error: UPGRADE FAILED: timed out waiting for the condition\n")
							os.Stderr.WriteString("==> Error: exit status 1\n")
							os.Exit(1)
						}
					}

					// Wait for completion of the Jobs
					for _, job := range status.Jobs {
						done := false

						for i := 0; i < p.timeout; {
							if p.verbose {
								log(3, "waiting for Job \"%s\"", job)
							}
							if kubectl.IsJobCompleted(job, p.namespace) {
								done = true
								break
							}
							time.Sleep(time.Duration(sleep_time) * time.Second)
							i = i + sleep_time
						}

						if !done {
							os.Stderr.WriteString("Error: UPGRADE FAILED: timed out waiting for the condition\n")
							os.Stderr.WriteString("==> Error: exit status 1\n")
							os.Exit(1)
						}
					}
				}
			}
		}
	}

	log(1, "upgrade of solution chart \"%s\" completed", p.chartName)

	return nil
}

// Retrieve the highest chart.weight in values.yaml
func getMaxWeight(v []Dependency) (m int) {
	if len(v) > 0 {
		m = v[0].Weight
	}
	for i := 1; i < len(v); i++ {
		if v[i].Weight > m {
			m = v[i].Weight
		}
	}
	return m
}

// Log spray messages
func log(level int, str string, params ...interface{}) {
	var logStr = "[spray] "

	if level == 2 {
		logStr = logStr + "  > "
	} else if level == 3 {
		logStr = logStr + "    o "
	} else if level == 4 {
		logStr = logStr + "      - "
	} else if level >= 5 {
		logStr = logStr + "        . "
	}

	fmt.Println(logStr + fmt.Sprintf(str, params...))
}

func main() {
	cmd := newSprayCmd(os.Args[1:])
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
