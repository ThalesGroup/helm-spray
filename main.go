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
	"strconv"
	"strings"
	"time"

	"github.com/gemalto/helm-spray/pkg/helm"
	"github.com/gemalto/helm-spray/pkg/kubectl"
	chartutil "k8s.io/helm/pkg/chartutil"

	"github.com/spf13/cobra"
)

type sprayCmd struct {
	chartName    string
	chartVersion string
	targets      []string
	namespace    string
	valueFiles   []string
	valuesSet    string
	dryRun       bool
	debug        bool
}

// Dependency ...
type Dependency struct {
	Name        string
	Targetable  bool
	Weight      int
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
		Use:          "helm spray [CHART]",
		Short:        `Helm plugin to upgrade subcharts from an umbrella chart`,
		Long:         globalUsage,
		SilenceUsage: true,
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
	f.StringVarP(&p.valuesSet, "set", "", "", "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.BoolVar(&p.dryRun, "dry-run", false, "simulate a spray")
	f.BoolVar(&p.debug, "debug", false, "enable verbose output")
	f.Parse(args)

    // When called through helm, debug mode is transmitted through the HELM_DEBUG envvar
    if !p.debug {
        if "1" == os.Getenv("HELM_DEBUG") {
            p.debug = true
        }
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
        // Dependency name
		dependencies[i].Name = req.Name

        // Is dependency targetable? If --target is specificed, it should match the name of the current dependency; 
        // if no --target is specified, then all dependencies are targetable
        if len(p.targets) > 0 {
            dependencies[i].Targetable = false
            for j := range p.targets {
                if p.targets[j] == dependencies[i].Name {
                    dependencies[i].Targetable = true
                }
            }
        } else {
            dependencies[i].Targetable = true
        }

        // Get weight of the dependency
		depi, err := values.Table(req.Name)
		if err != nil {
			panic(fmt.Errorf("%s", err))
		}
		if depi["weight"] != nil {
			w64 := depi["weight"].(float64)
			w, err := strconv.Atoi(strconv.FormatFloat(w64, 'f', 0, 64))
			if err != nil {
				panic(fmt.Errorf("%s", err))
			}
			dependencies[i].Weight = w
		}
	}

	// For debug...
    if p.debug {
		for _, dependency := range dependencies {
			fmt.Printf("[spray] subchart: \"%s\" | targetable: %t | weight: %d\n", dependency.Name, dependency.Targetable, dependency.Weight)
		}
    }

	for i := 0; i <= getMaxWeight(dependencies); i++ {
        shouldWait := false

        // Upgrade the (targetable) Deployments, following the increasing weight
		for _, dependency := range dependencies {
            if dependency.Targetable {
	    		if dependency.Weight == i {
	    			fmt.Println("[spray] upgrading release: \"" + dependency.Name + "\"...")
                    shouldWait = true

                    // Add the "<dependency>.enabled" flags to ensure that only the current chart is to be executed
                    valuesSet := ""
                    for _, dep := range dependencies {
                        if dep.Name == dependency.Name {
                            valuesSet = valuesSet + dep.Name + ".enabled=true,"
                        } else {
                            valuesSet = valuesSet + dep.Name + ".enabled=false,"
                        }
                    }
                    valuesSet = valuesSet + p.valuesSet

                    // Upgrade the Deployment
	    			helm.UpgradeWithValues(p.namespace, dependency.Name, dependency.Name, p.chartName, p.valueFiles, valuesSet, p.dryRun, p.debug)

                    if !p.dryRun {
        				status := helm.GetHelmStatus(dependency.Name)
	    			    if status != "DEPLOYED" {
		    	    		os.Exit(1)
                        }
		    		}

	    			fmt.Println("[spray] release: \"" + dependency.Name + "\" upgraded")
    			}
            }
		}

        // Wait availability of the Deployment just upgraded
        if shouldWait {
    		fmt.Println("[spray] waiting for Liveness and Readiness...")

            if !p.dryRun {
    	    	for _, dependency := range dependencies {
	    	    	if i > 0 && dependency.Weight == i {
		    	    	for {
			    		    if kubectl.IsDeploymentUpToDate(dependency.Name, p.namespace) {
				        		break
			    	    	}
		    			    time.Sleep(5 * time.Second)
	        			}
        			}
        }
            }
		}
	}

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

func main() {
	cmd := newSprayCmd(os.Args[1:])
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
