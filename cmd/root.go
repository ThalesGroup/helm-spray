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
package cmd

import (
	"errors"
	"fmt"
	"github.com/gemalto/helm-spray/v4/internal/log"
	"github.com/gemalto/helm-spray/v4/pkg/helm"
	"github.com/gemalto/helm-spray/v4/pkg/helmspray"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

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

func NewRootCmd() *cobra.Command {

	s := &helmspray.Spray{}

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

			s.ChartName = args[0]

			if s.ChartVersion != "" {
				if strings.HasSuffix(s.ChartName, "tgz") {
					return errors.New("cannot use --version together with chart archive")
				}

				if _, err := os.Stat(s.ChartName); err == nil {
					return errors.New("cannot use --version together with chart directory")
				}

				if strings.HasPrefix(s.ChartName, "http://") || strings.HasPrefix(s.ChartName, "https://") || strings.HasPrefix(s.ChartName, "oci://") {
					return errors.New("cannot use --version together with chart URL")
				}
			}

			if s.PrefixReleasesWithNamespace == true && s.PrefixReleases != "" {
				return errors.New("cannot use both --prefix-releases and --prefix-releases-with-namespace together")
			}

			if len(s.Targets) > 0 && len(s.Excludes) > 0 {
				return errors.New("cannot use both --target and --exclude together")
			}

			// If chart is specified through an url, the fetch it from the url.
			if strings.HasPrefix(s.ChartName, "http://") || strings.HasPrefix(s.ChartName, "https://") || strings.HasPrefix(s.ChartName, "oci://") {
				log.Info(1, "fetching chart from url \"%s\"...", s.ChartName)
				var err error
				s.ChartName, err = helm.Fetch(s.ChartName, "")
				if err != nil {
					return fmt.Errorf("fetching chart %s: %w", s.ChartName, err)
				}
			} else if _, err := os.Stat(s.ChartName); err != nil {
				// If local file (or directory) does not exist, then fetch it from a repo.
				if s.ChartVersion != "" {
					log.Info(1, "fetching chart \"%s\" version \"%s\" from repos...", s.ChartName, s.ChartVersion)
				} else {
					log.Info(1, "fetching chart \"%s\" from repos...", s.ChartName)
				}
				var err error
				s.ChartName, err = helm.Fetch(s.ChartName, s.ChartVersion)
				if err != nil {
					return fmt.Errorf("fetching chart %s with version %s: %w", s.ChartName, s.ChartVersion, err)
				}
			} else {
				log.Info(1, "processing chart from local file or directory \"%s\"...", s.ChartName)
			}

			return s.Spray()
		},
	}

	f := cmd.Flags()
	f.StringVarP(&s.ChartVersion, "version", "", "", "specify the exact chart version to install. If this is not specified, the latest version is installed")
	f.StringSliceVarP(&s.Targets, "target", "t", []string{}, "specify the subchart to target (can specify multiple). If '--target' is not specified, all subcharts are targeted")
	f.StringSliceVarP(&s.Excludes, "exclude", "x", []string{}, "specify the subchart to exclude (can specify multiple): process all subcharts except the ones specified in '--exclude'")
	f.StringVarP(&s.PrefixReleases, "prefix-releases", "", "", "prefix the releases by the given string, resulting into releases names formats:\n    \"<prefix>-<chart name or alias>\"\nAllowed characters are a-z A-Z 0-9 and -")
	f.BoolVar(&s.PrefixReleasesWithNamespace, "prefix-releases-with-namespace", false, "prefix the releases by the name of the namespace, resulting into releases names formats:\n    \"<namespace>-<chart name or alias>\"")
	f.BoolVar(&s.CreateNamespace, "create-namespace", false, "automatically create the namespace if necessary")
	f.BoolVar(&s.ResetValues, "reset-values", false, "when upgrading, reset the values to the ones built into the chart")
	f.BoolVar(&s.ReuseValues, "reuse-values", false, "when upgrading, reuse the last release's values and merge in any overrides from the command line via '--set' and '-f'.\nIf '--reset-values' is specified, this is ignored")
	f.StringSliceVarP(&s.ValuesOpts.ValueFiles, "values", "f", []string{}, "specify values in a YAML file or a URL (can specify multiple)")
	f.StringArrayVar(&s.ValuesOpts.Values, "set", []string{}, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&s.ValuesOpts.StringValues, "set-string", []string{}, "set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&s.ValuesOpts.FileValues, "set-file", []string{}, "set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)")
	f.BoolVar(&s.Force, "force", false, "force resource update through delete/recreate if needed")
	f.IntVar(&s.Timeout, "timeout", 300, "time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks)\nand for liveness and readiness (like Deployments and regular Jobs completion)")
	f.BoolVar(&s.DryRun, "dry-run", false, "simulate a spray")
	f.BoolVarP(&s.Verbose, "verbose", "v", false, "enable spray verbose output")
	f.BoolVar(&s.Debug, "debug", false, "enable helm debug output (also include spray verbose output)")

	// When called through helm, debug mode is transmitted through the HELM_DEBUG envvar
	helmDebug := os.Getenv("HELM_DEBUG")
	if helmDebug == "1" || strings.EqualFold(helmDebug, "true") || strings.EqualFold(helmDebug, "on") {
		s.Debug = true
	}
	if s.Debug {
		s.Verbose = true
	}

	// When called through helm, namespace is transmitted through the HELM_NAMESPACE envvar
	namespace := os.Getenv("HELM_NAMESPACE")
	if len(namespace) > 0 {
		s.Namespace = namespace
	} else {
		s.Namespace = "default"
	}

	return cmd
}
