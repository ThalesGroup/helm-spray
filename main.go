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
	"io/ioutil"
	"strconv"
	"strings"
	"regexp"
	"time"
	"text/tabwriter"

	"github.com/gemalto/helm-spray/pkg/helm"
	"github.com/gemalto/helm-spray/pkg/kubectl"

	chartutil "k8s.io/helm/pkg/chartutil"
	chartHapi "k8s.io/helm/pkg/proto/hapi/chart"

	"github.com/spf13/cobra"
	"github.com/ghodss/yaml"
)

type sprayCmd struct {
	chartName						string
	chartVersion					string
	targets							[]string
	excludes						[]string
	namespace						string
	prefixReleases					string
	prefixReleasesWithNamespace		bool
	resetValues						bool
	reuseValues						bool
	valueFiles						[]string
	valuesSet						[]string
	valuesSetString					[]string
	valuesSetFile					[]string
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
	AppVersion					string
	Targeted					bool
	Weight						int
	CorrespondingReleaseName	string
	HasTags						bool
	AllowedByTags				bool
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

func newSprayCmd(args []string) *cobra.Command {

	p := &sprayCmd{}

	cmd := &cobra.Command{
		Use:			"helm spray [CHART]",
		Short:			`Helm plugin to upgrade subcharts from an umbrella chart`,
		Long:			globalUsage,
		SilenceUsage:	true,
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) == 0 {
				return errors.New("This command needs at least 1 argument: chart name")
			} else if len(args) > 1 {
				return errors.New("This command accepts only 1 argument: chart name")
			}

			p.chartName = args[0]

			if p.chartVersion != "" {
				if strings.HasSuffix(p.chartName, "tgz") {
					logErrorAndExit("You cannot use --version together with chart archive")
				}

				if _, err := os.Stat(p.chartName); err == nil {
					logErrorAndExit("You cannot use --version together with chart directory")
				}

				if (strings.HasPrefix(p.chartName, "http://") || strings.HasPrefix(p.chartName, "https://")) {
					logErrorAndExit("You cannot use --version together with chart URL")
				}
			}

			if p.prefixReleasesWithNamespace == true && p.prefixReleases != "" {
				logErrorAndExit("You cannot use both --prefix-releases and --prefix-releases-with-namespace together")
			}

			if len(p.targets) > 0 && len(p.excludes) > 0 {
				logErrorAndExit("You cannot use both --target and --exclude together")
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
	f.StringVarP(&p.namespace, "namespace", "n", "default", "namespace to spray the chart into")
	f.StringVarP(&p.chartVersion, "version", "", "", "specify the exact chart version to install. If this is not specified, the latest version is installed")
	f.StringSliceVarP(&p.targets, "target", "t", []string{}, "specify the subchart to target (can specify multiple). If '--target' is not specified, all subcharts are targeted")
	f.StringSliceVarP(&p.excludes, "exclude", "x", []string{}, "specify the subchart to exclude (can specify multiple): process all subcharts except the ones specified in '--exclude'")
	f.StringVarP(&p.prefixReleases, "prefix-releases", "", "", "prefix the releases by the given string, resulting into releases names formats:\n    \"<prefix>-<chart name or alias>\"\nAllowed characters are a-z A-Z 0-9 and -")
	f.BoolVar(&p.prefixReleasesWithNamespace, "prefix-releases-with-namespace", false, "prefix the releases by the name of the namespace, resulting into releases names formats:\n    \"<namespace>-<chart name or alias>\"")
	f.BoolVar(&p.resetValues, "reset-values", false, "when upgrading, reset the values to the ones built into the chart")
	f.BoolVar(&p.reuseValues, "reuse-values", false, "when upgrading, reuse the last release's values and merge in any overrides from the command line via '--set' and '-f'.\nIf '--reset-values' is specified, this is ignored")
	f.StringSliceVarP(&p.valueFiles, "values", "f", []string{}, "specify values in a YAML file or a URL (can specify multiple)")
	f.StringSliceVarP(&p.valuesSet, "set", "", []string{}, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringSliceVarP(&p.valuesSetString, "set-string", "", []string{}, "set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringSliceVarP(&p.valuesSetFile, "set-file", "", []string{}, "set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)")
	f.BoolVar(&p.force, "force", false, "force resource update through delete/recreate if needed")
	f.IntVar(&p.timeout, "timeout", 300, "time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks)\nand for liveness and readiness (like Deployments and regular Jobs completion)")
	f.BoolVar(&p.dryRun, "dry-run", false, "simulate a spray")
	f.BoolVar(&p.verbose, "verbose", false, "enable spray verbose output")
	f.BoolVar(&p.debug, "debug", false, "enable helm debug output (also include spray verbose output)")

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

// Running Spray command
func (p *sprayCmd) spray() error {

	// Load and valide the umbrella chart...
	chart, err := chartutil.Load(p.chartName)
	if err != nil {
		logErrorAndExit("Error loading chart \"%s\": %s", p.chartName, err)
	}

	// Load and valid the requirements file...
	reqs, err := chartutil.LoadRequirements(chart)
	if err != nil {
		logErrorAndExit("Error reading \"requirements.yaml\" file: %s", err)
	}

	// Get the default values file of the umbrella chart and process the '#! .File.Get' directives that might be specified in it
	// Only in case '--reuseValues' has not been set
	var values chartutil.Values
	if p.reuseValues == false {
		updatedDefaultValues := processIncludeInValuesFile(chart, p.verbose)

		// Load default values...
		values, err = chartutil.CoalesceValues(chart, &chartHapi.Config{Raw: string(updatedDefaultValues)})
		if err != nil {
			logErrorAndExit("Error processing default values for umbrella chart: %s", err)
		}

		// Write default values to a temporary file and add it to the list of values files, 
		// for later usage during the calls to helm
		tempDir, err := ioutil.TempDir("", "spray-")
		if err != nil {
			logErrorAndExit("Error creating temporary directory to write updated default values file for umbrella chart: %s", err)
		}
		defer os.RemoveAll(tempDir)

		tempFile, err := ioutil.TempFile(tempDir, "updatedDefaultValues-*.yaml")
		if err != nil {
			logErrorAndExit("Error creating temporary file to write updated default values file for umbrella chart: %s", err)
		}
		defer os.Remove(tempFile.Name())

		if _, err = tempFile.Write([]byte(updatedDefaultValues)); err != nil {
			logErrorAndExit("Error writing updated default values file for umbrella chart into temporary file: %s", err)
		}
		p.valueFiles = append(p.valueFiles, tempFile.Name())

	} else {
		values, err = chartutil.CoalesceValues(chart, chart.GetValues())
		if err != nil {
			logErrorAndExit("Error processing default values for umbrella chart: %s", err)
		}
	}


	// Get the list of "tags" specified in the values...
	// (locally-provided values only; values coming from server are not considered)
	allDisabled := ""
	for _, req := range reqs.Dependencies {
		if req.Alias == "" {
			allDisabled = allDisabled + req.Name + ".enabled=false,"
		} else {
			allDisabled = allDisabled + req.Alias + ".enabled=false,"
		}
	}
	newValuesSet := append(p.valuesSet, allDisabled)

	localValues := helm.GetLocalValues(p.chartName, p.valueFiles, newValuesSet, p.valuesSetString, p.valuesSetFile)

	localValuesAsMap := map[string]interface{}{}
	if err := yaml.Unmarshal([]byte (localValues), &localValuesAsMap); err != nil {
		logErrorAndExit("Error parsing values to get 'tags' content")
	}
	var providedTags map[string]interface{}
    if localValuesAsMap["tags"] != nil {
        providedTags = localValuesAsMap["tags"].(map[string]interface{})
    }

	if p.verbose {
		log(1, "looking for \"tags\" in values provided through \"--values/-f\", \"--set\", \"--set-string\", and \"--set-file\"...")
		for k, v := range providedTags {
			log(2, fmt.Sprintf("found tag \"%s: %s\"", k, fmt.Sprint(v)))
		}
	}

	// Build the list of all dependencies, and their key attributes
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

		// Is dependency targeted? If --target or --excludes are specificed, it should match the name of the current dependency; 
		// if neither --target nor --exclude are specified, then all dependencies are targeted
		if len(p.targets) > 0 {
			dependencies[i].Targeted = false
			for j := range p.targets {
				if p.targets[j] == dependencies[i].UsedName {
					dependencies[i].Targeted = true
				}
			}

		} else if len(p.excludes) > 0 {
			dependencies[i].Targeted = true
			for j := range p.excludes {
				if p.excludes[j] == dependencies[i].UsedName {
					dependencies[i].Targeted = false
				}
			}

		} else {
			dependencies[i].Targeted = true
		}


		// Loop on the tags associated to the dependency and check with the tags provided in the values
		dependencies[i].AllowedByTags = false
		if len (req.Tags) == 0 {
			dependencies[i].HasTags = false
			dependencies[i].AllowedByTags = true

		} else {
			dependencies[i].HasTags = true

			for _, tag := range req.Tags {
				for k, v := range providedTags {
					if k == tag && v == true {
						dependencies[i].AllowedByTags = true
					}
				}
			}
		}


		// Get weight of the dependency. If no weight is specified, setting it to 0
		dependencies[i].Weight = 0
		depi, err := values.Table(dependencies[i].UsedName)
		if (err == nil && depi["weight"] != nil) {
			w64 := depi["weight"].(float64)
			w, err := strconv.Atoi(strconv.FormatFloat(w64, 'f', 0, 64))
			if err != nil {
				logErrorAndExit("Error computing weight value for sub-chart \"%s\": %s", dependencies[i].UsedName, err)
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

		// Get the AppVersion that is contained in the Chart.yaml file of the dependency sub-chart
		for _, subChart := range chart.GetDependencies() {
			if subChart.GetMetadata().GetName() == dependencies[i].Name {
				dependencies[i].AppVersion = subChart.GetMetadata().GetAppVersion()
				break
			}
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

			name := dependency.Name
			alias := "-"
			if dependency.Alias != "" {
				name = dependency.Alias
				alias = dependency.Name
			}

			targeted := fmt.Sprint (dependency.Targeted)
			if dependency.Targeted && dependency.HasTags && (dependency.AllowedByTags == true) {
				targeted = "true (tag match)"
			} else if dependency.Targeted && dependency.HasTags && (dependency.AllowedByTags == false) {
				targeted = "false (no tag match)"
			}

			fmt.Fprintln(w, fmt.Sprintf ("[spray]  \t %s\t %s\t %s\t %d\t| %s\t %s\t %s\t", name, alias, targeted, dependency.Weight, dependency.CorrespondingReleaseName, currentRevision, currentStatus))
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
			if dependency.Targeted  && dependency.AllowedByTags == true {
				if dependency.Weight == i {
					if firstInWeight {
						log(1, "processing sub-charts of weight %d", dependency.Weight)
						firstInWeight = false
					}

					if release, ok := helmReleases[dependency.CorrespondingReleaseName]; ok {
						log(2, "upgrading release \"%s\": going from revision %d (status %s) to %d (appVersion %s)...", dependency.CorrespondingReleaseName, release.Revision, release.Status, release.Revision+1, dependency.AppVersion)

					} else {
						log(2, "upgrading release \"%s\": deploying first revision (appVersion %s)...", dependency.CorrespondingReleaseName, dependency.AppVersion)
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
					p.valuesSet = append(p.valuesSet, valuesSet)

					// Upgrade the Deployment
					helmstatus := helm.UpgradeWithValues(p.namespace, dependency.CorrespondingReleaseName, p.chartName, p.resetValues, p.reuseValues, p.valueFiles, p.valuesSet, p.valuesSetString, p.valuesSetFile, p.force, p.timeout, p.dryRun, p.debug)
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
						logErrorAndExit("Error: status returned by helm differs from \"DEPLOYED\". Cannot continue spray processing.")
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
							logErrorAndExit("Error: UPGRADE FAILED: timed out waiting for the condition\n==> Error: exit status 1")
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
							logErrorAndExit("Error: UPGRADE FAILED: timed out waiting for the condition\n==> Error: exit status 1")
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
							logErrorAndExit("Error: UPGRADE FAILED: timed out waiting for the condition\n==> Error: exit status 1")
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

// Search the "include" clauses in the default value file of the chart and replace them by the content
// of the corresponding file.
// Allows:
//   - Includeing a file:
//       #! {{ .File.Get myfile.yaml }}
//   - Including a sub-part of a file, picking a specific tag. Tags can target a Yaml element (aka table) or a
//	   leaf value, but tags cannot target a list item.
//       #! {{ pick (.File.Get myfile.yaml) tag }}
//   - Indenting the include content:
//       #! {{ .File.Get myfile.yaml | indent 2 }}
//   - All combined...:
//       #! {{ pick (.File.Get "myfile.yaml") "tag.subTag" | indent 4 }}
//
func processIncludeInValuesFile(chart *chartHapi.Chart, verbose bool) string {
	defaultValues := string(chart.GetValues().GetRaw())

	regularExpressions := []string {
		// Expression #0: Process file inclusion ".File.Get" with optional "| indent"
		`#!\s*\{\{\s*pick\s*\(\s*\.File\.Get\s+([a-zA-Z0-9_"\\\/\.\-\(\):]+)\s*\)\s*([a-zA-Z0-9_"\.\-]+)\s*(\|\s*indent\s*(\d+))?\s*\}\}\s*(\n|\z)`,
		// Expression #1: Process file inclusion ".File.Get", picking a specific element of the file content "pick (.File.Get <file>) <tag>", with an optional "| indent"
		`#!\s*\{\{\s*\.File\.Get\s+([a-zA-Z0-9_"\\\/\.\-\(\):]+)\s*(\|\s*indent\s*(\d+))?\s*\}\}\s*(\n|\z)`}

	expressionNumber := 1
	includeFileNameExp := regexp.MustCompile(regularExpressions[expressionNumber-1])
	match := includeFileNameExp.FindStringSubmatch(defaultValues)

	if verbose {
		log(1, "looking for \"#! .File.Get\" clauses into the values file of the umbrella chart...")
	}

	for ; len(match) != 0; {
		var fullMatch, includeFileName, subValuePath, indent string
		if expressionNumber == 1 {
			fullMatch = match[0]
			includeFileName = strings.Trim (match[1], `"`)
			subValuePath = strings.Trim (match[2], `"`)
			indent = match[4]
		} else if expressionNumber == 2 {
			fullMatch = match[0]
			includeFileName = strings.Trim (match[1], `"`)
			subValuePath = ""
			indent = match[3]
		}

		replaced := false

		for _, f := range chart.GetFiles() {
			if f.GetTypeUrl() == strings.Trim(strings.TrimSpace(includeFileName), "\"") {
				if verbose {
					if subValuePath == "" {
						if indent == "" {
							log(2, "found reference to values file \"%s\"", includeFileName)
						} else {
							log(2, "found reference to values file \"%s\" (with indent of \"%s\")", includeFileName, indent)
						}
					} else {
						if indent == "" {
							log(2, "found reference to values file \"%s\" (with yaml sub-path \"%s\")", includeFileName, subValuePath)
						} else {
							log(2, "found reference to values file \"%s\" (with yaml sub-path \"%s\" and indent of \"%s\")", includeFileName, subValuePath, indent)
						}
					}
				}

				dataToAdd := string(f.GetValue())
				if subValuePath != "" {
					data, err := chartutil.ReadValues(f.GetValue())
					if err != nil {
						logErrorAndExit("Unable to read values from file \"%s\": %s", includeFileName, err)
					}

					// Suppose that the element at the path is an element (list items are not supported)
					if subData, err := data.Table(subValuePath); err == nil {
						if dataToAdd, err = subData.YAML(); err != nil {
							logErrorAndExit("Unable to generate a valid YAML file from values at path \"%s\" in values file \"%s\": %s", subValuePath, includeFileName, err)
						}

					// If it is not an element, then maybe it is directly a value
					} else {
						if val, err2 := data.PathValue(subValuePath); err2 == nil {
							var ok bool
							if dataToAdd, ok = val.(string); ok == false {
								logErrorAndExit("Unable to find values matching path \"%s\" in values file \"%s\": %s\n%s", subValuePath, includeFileName, err, "Targeted item is most propably a list: not supported. Only elements (aka Yaml table) and leaf values are supported.")
							}

						} else {
							logErrorAndExit("Unable to find values matching path \"%s\" in values file \"%s\": %s", subValuePath, includeFileName, err)
						}
					}
				}

				if indent == "" {
					defaultValues = strings.Replace(defaultValues, fullMatch, dataToAdd + "\n", -1)
				} else {
					nbrOfSpaces, err := strconv.Atoi(indent)
					if err != nil {
						logErrorAndExit("Error computing indentation value in \"#! .File.Get\" clause: %s", err)
					}

					toAdd := strings.Replace(dataToAdd, "\n", "\n" + strings.Repeat (" ", nbrOfSpaces), -1)
					defaultValues = strings.Replace(defaultValues, fullMatch, strings.Repeat (" ", nbrOfSpaces) + toAdd + "\n", -1)
				}

				replaced = true
			}
		}

		if !replaced {
			logErrorAndExit("Unable to find file \"%s\" referenced in the \"%s\" clause of the default values file of the umbrella chart", match[1], strings.TrimRight(match[0], "\n"))
		}

		match = includeFileNameExp.FindStringSubmatch(defaultValues)

		if len(match) == 0 && expressionNumber < len(regularExpressions) {
			expressionNumber++
			includeFileNameExp = regexp.MustCompile(regularExpressions[expressionNumber-1])
			match = includeFileNameExp.FindStringSubmatch(defaultValues)
		}
	}

	return defaultValues
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

// Log error and exit
func logErrorAndExit(str string, params ...interface{}) {
	os.Stderr.WriteString(fmt.Sprintf(str + "\n", params...))
	os.Exit(1)
}


func main() {
	cmd := newSprayCmd(os.Args[1:])
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
