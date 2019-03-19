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

	"github.com/gemalto/helm-spray/pkg/helm"
	"github.com/gemalto/helm-spray/pkg/kubectl"
	chartutil "k8s.io/helm/pkg/chartutil"

	"github.com/spf13/cobra"
)

type sprayCmd struct {
	chartName		string
	chartVersion	string
	targets			[]string
	namespace		string
	resetValues		bool
	reuseValues		bool
	valueFiles		[]string
	valuesSet		string
	force			bool
	dryRun			bool
	verbose			bool
	debug			bool
}

// Dependency ...
type Dependency struct {
	Name		string
	Alias		string
	UsedName	string
	Targeted	bool
	Weight		int
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

 1. By chart reference: helm spray stable/umbrella-chart
 2. By path to a packaged chart: helm spray umbrella-chart-1.0.0-rc.1+build.32.tgz
 3. By path to an unpacked chart directory: helm spray ./umbrella-chart
 4. By absolute URL: helm spray https://example.com/charts/umbrella-chart-1.0.0-rc.1+build.32.tgz

It will install the latest version of that chart unless you also supply a version number with the
'--version' flag.

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

			// TODO: check format for chart name (directory, url, tgz...)
			p.chartName = args[0]

			if p.chartVersion != "" {
				if strings.Contains(p.chartName, "tgz") {
					fmt.Println("You cannot use --version together with chart archive")
					os.Exit(1)
				}
				if _, err := os.Stat(p.chartName); err == nil {
					fmt.Println("You cannot use --version together with chart directory")
					os.Exit(1)
				}

				log(1, "fetching chart \"%s\" version %s...", p.chartName, p.chartVersion)
				helm.Fetch(p.chartName, p.chartVersion)
			}

			return p.spray()
		},
	}

	f := cmd.Flags()
	f.StringSliceVarP(&p.valueFiles, "values", "f", []string{}, "specify values in a YAML file or a URL (can specify multiple)")
	f.StringVarP(&p.namespace, "namespace", "n", "default", "namespace to spray the chart into.")
	f.StringVarP(&p.chartVersion, "version", "", "", "specify the exact chart version to install. If this is not specified, the latest version is installed")
	f.StringSliceVarP(&p.targets, "target", "t", []string{}, "specify the subchart to target (can specify multiple). If --target is not specified, all subcharts are targeted")
	f.BoolVar(&p.resetValues, "reset-values", false, "when upgrading, reset the values to the ones built into the chart")
	f.BoolVar(&p.reuseValues, "reuse-values", false, "when upgrading, reuse the last release's values and merge in any overrides from the command line via --set and -f. If '--reset-values' is specified, this is ignored.")
	f.StringVarP(&p.valuesSet, "set", "", "", "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.BoolVar(&p.force, "force", false, "force resource update through delete/recreate if needed")
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
	}

	log(1, "deploying solution chart \"%s\" in namespace \"%s\"", p.chartName, p.namespace)
	helmReleases := helm.ListAll() //p.namespace)

	if p.verbose {
		log(1, "subcharts:")

		for _, dependency := range dependencies {
			currentRevision := "None"
			currentStatus := "Not deployed"
			if release, ok := helmReleases[dependency.UsedName]; ok {
				currentRevision = strconv.Itoa(release.Revision)
				currentStatus = release.Status
			}

			if dependency.Alias == "" {
				log(2, "\"%s\" | targeted: %t | weight: %d | current revision: %s | current status: %s", dependency.Name, dependency.Targeted, dependency.Weight, currentRevision, currentStatus)

			} else {
				log(2, "\"%s\" (is alias of \"%s\") | targeted: %t | weight: %d | current revision: %s | current status: %s", dependency.Alias, dependency.Name, dependency.Targeted, dependency.Weight, currentRevision, currentStatus)
			}
		}
	}

	for i := 0; i <= getMaxWeight(dependencies); i++ {
		shouldWait := false
		firstInWeight := true
		helmstatusses := make([]helm.HelmStatus, 0)

		// Upgrade the current (targeted) Deployments, following the increasing weight
		for _, dependency := range dependencies {
			if dependency.Targeted {
				if dependency.Weight == i {
					if firstInWeight {
						log(1, "processing sub-charts of weight %d", dependency.Weight)
						firstInWeight = false
					}

					if release, ok := helmReleases[dependency.UsedName]; ok {
						log(2, "upgrading release \"%s\": going from revision %d (status %s) to %d...", dependency.UsedName, release.Revision, release.Status, release.Revision+1)
					} else {
						log(2, "upgrading release \"%s\": deploying first revision...", dependency.UsedName)
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
					helmstatus := helm.UpgradeWithValues(p.namespace, dependency.UsedName, dependency.UsedName, p.chartName, p.resetValues, p.reuseValues, p.valueFiles, valuesSet, p.force, p.dryRun, p.debug)

					helmstatusses = append(helmstatusses, helmstatus)

					log(3, "release: \"%s\" upgraded", dependency.UsedName)
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

		// Wait availability of the Deployment just upgraded
		if shouldWait {
			log(2, "waiting for Liveness and Readiness...")

			if !p.dryRun {
				for _, status := range helmstatusses {
					// Wait for completion of the Deployments update
					for _, dep := range status.Deployments {
						for {
							if p.verbose {
								log(3, "waiting for Deployment \"%s\"", dep)
							}
							if kubectl.IsDeploymentUpToDate(dep, p.namespace) {
								break
							}
							time.Sleep(5 * time.Second)
						}
					}

					// Wait for completion of the StatefulSets update
					for _, ss := range status.StatefulSets {
						for {
							if p.verbose {
								log(3, "waiting for StatefulSet \"%s\"", ss)
							}
							if kubectl.IsStatefulSetUpToDate(ss, p.namespace) {
								break
							}
							time.Sleep(5 * time.Second)
						}
					}

					// Wait for completion of the Jobs
					for _, job := range status.Jobs {
						for {
							if p.verbose {
								log(3, "waiting for Job \"%s\"", job)
							}
							if kubectl.IsJobCompleted(job, p.namespace) {
								break
							}
							time.Sleep(5 * time.Second)
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
