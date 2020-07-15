# Helm Spray

[![Build Status](https://travis-ci.org/ThalesGroup/helm-spray.svg?branch=master)](https://travis-ci.org/ThalesGroup/helm-spray)

![helm-spray](https://thalesgroup.github.io/helm-spray/logo/helm-spray_150x150.png)

## What is Helm Spray?

This is a Helm plugin to install or upgrade sub-charts one by one from an umbrella chart.

It works like `helm upgrade --install`, except that it upgrades or installs each sub-charts according to a weight (>=0) set on each sub-chart. All sub-charts of weight 0 are processed first, then sub-charts of weight 1, etc.
Chart weight shall be specified using the `<chart name>.weight` value.

Each sub-chart is deployed under a specific Release named `<chart name or alias>`, enabling a later individual upgrade targeting this sub-chart only. All global or individual upgrade should still be done on the umbrella chart.

## Compatibility with helm
- helm-spray v3.x releases are only compatible with helm v2.x
- helm-spray v4.x releases are only compatible with helm v3.x

## Continuous Integration & Delivery

Helm Spray is building and delivering under Travis.

[![Build Status](https://travis-ci.org/ThalesGroup/helm-spray.svg?branch=master)](https://travis-ci.org/ThalesGroup/helm-spray)

## How to install (starting from v4)

```
-bash-4.2$ helm plugin install https://github.com/ThalesGroup/helm-spray
Downloading and installing spray v4.0.0...
Installed plugin: spray
-bash-4.2$ helm plugin list
NAME    VERSION DESCRIPTION
spray   4.0.0   Helm plugin for upgrading sub-charts from umbrella chart with dependency orders
```

Please note the helm plugin install command requires git

## Pre-requisites

Helm Spray is using kubectl to communicate with Kubernetes cluster.
Please follow this [page](https://kubernetes.io/docs/tasks/tools/install-kubectl) to install it

## Usage

```
  $ helm spray [flags] CHART
```

Helm Spray shall always be called on the umbrella chart, whatever it is for upgrading the full set of charts, or for upgrading individual sub-charts (using the `--target` option).
For a proper usage of helm spray, the umbrella chart shall have a `requirement.yaml` file listing all the sub-charts to be deployed (under the `dependencies` element). Sub-charts may have an `alias` element and the `condition` element shall be set to the value `<chart name or alias>.enabled`.
Here is an example of `requirement.yaml` file for an umbrella chart having three sub-charts, one of them having an alias:
```
dependencies:
- name: micro-service-1
  version: ~1.2
  repository: http://chart-museum/charts
  condition: micro-service-1.enabled
- name: micro-service-2
  version: ~2.3
  repository: http://chart-museum/charts
  condition: micro-service-2.enabled
- name: micro-service-3
  alias: ms3
  version: ~1.1
  repository: http://chart-museum/charts
  condition: ms3.enabled
```

A "values" file shall also be set with the weight it be applied to each individual sub-chart. This weight shall be set in the `<chart name or alias>.weight` element. A good practice is that thei weigths are statically set in the default `values.yaml` file of the umbrella chart (and not in a yaml file provided using the `-f` option), as sub-chart's weight is not likely to change over time.
As an example corresponding to the above `requirement.yaml` file, the `values.yaml` file of the umbrella chart might be:
```
micro-service-1:
  weight: 0

micro-service-2:
  weight: 1

ms3:
  weight: 2
```
Several sub-charts may have the same weight, meaning that they will be upgraded together.
Upgrade of sub-charts of weight n+1 will only be triggered when upgrade of sub-charts of weight n is completed.
Note also that while weights should primarilly be set in the `values.yaml` file of the umbrella chart, it is also possible to set them using the `--values/-f` or `--set` flags of the command line, for example to temporarilly overwrite a weight value. If so, take care that weight values provided through the command line are not taken into account for the next calls to Helm Spray, including if the `--reuse-values` flag is used: they would have to be provided again at each call.


Helm Spray creates one helm Release per sub-chart. Releases are individually upgraded when running the helm spray process, in particular when using the `--target` option.
The name and version of the umbrella chart is set as the Chart name for all the Revisions.
```
NAME            REVISION        UPDATED                         STATUS          CHART           APP VERSION     NAMESPACE
micro-service-1 12              Wed Jan 30 17:19:15 2019        DEPLOYED        solution-0.1    0.1             default
micro-service-2 21              Wed Jan 30 17:18:55 2019        DEPLOYED        solution-0.1    0.1             default
ms3             7               Wed Jan 30 17:18:45 2019        DEPLOYED        solution-0.1    0.1             default
```

Note: if an alias is set for a sub-chart, then this is this alias that should be used with the `--target` option, not the sub-chart name.

### Values:

The umbrella chart gathers several components or micro-services into a single solution. Values can then be set at many different places:
- At micro-service level, inside the `values.yaml` file of each micro-service chart: these are common defaults values set by the micro-service developer, independently from the deployment context and location of the micro-service
- At the solution level, inside the `values.yaml` file of the umbrella chart: these are values complementing or overwriting default values of the micro-services sub-charts, usually formalizing the deployment topology of the solution and giving the standard configuration of the underlying micro-services for any deployments of cwthis specific solution
- At deployment time, using the `--values/-f`, `--set`, `--set-string`, or `--set-file` flags: this is the placeholder for giving the deployment-dependent values, specifying for example the exact database url for this deployment, the exact password value for this deployment, the targeted remote server url for this deployment, etc. These values usually change from one deployment of the solution to another.

Within the micro-services paradigm, decoupling between micro-services is one of the most important criteria to respect. While values con be provided in a per-micro-service basis for the first and last places mentioned above, Helm only allows one single `values.yaml` file in the umbrella chart. All solution-level values should then be gathered into a single file, while it would have been better to provide values in several files, on a one-file-per-micro-service basis (to ensure decoupling of the micro-services configuration, even at solution level).
Helm Spray is consequently adding this capability to have several values file in the umbrella chart and to include them into the single `values.yaml` file using the `#! {{ .Files.Get <file name> }}` directive.
- The file to be included shall be a valid yaml file.
- It is possible to only include a sub-part of the yaml content by picking an element of the `Files.Get`, specifying the path to be extracted and included: `#! {{ pick (.Files.Get <file name>) for.bar }}`. Only paths targeting a Yaml element or a leaf value can be provided. Paths targeted lists are not supported.
- It is possible to indent the included content using the `indent` directive: `#! {{ .Files.Get <file name> | indent 2 }}`, `#! {{ pick (.Files.Get <file name>) for.bar | indent 4 }}

Note: The `{{ .Files.Get ... }}` directive shall be prefixed by `#!` as the `values.yaml` file is parsed both with and without the included content. When parsed without the included content, it shall still be a valid yaml file, thus mandating the usage of a comment to specify the `{{ .Files.Get ... }}` clause that is by default supported by neither yaml nor Helm in default values files of charts. Usage of `#!` (with a bang '!') allows differentiating the include clauses from regular comments.
Note also that when Helm is parsing the `values.yaml` file without the included content, some warning may be raised by helm if yaml elements are nil or empty (while they are not with the included content). A typical warning could be: 'Warning: Merging destination map for chart 'my-solution'. The destination item 'bar' is a table and ignoring the source 'bar' as it has a non-table value of: <nil>'

Example of `values.yaml`:
```
micro-service-1:
  weight: 0
#! {{ .Files.Get ms1.yaml }}

micro-service-2:
  weight: 1
#! {{ pick (.Files.Get ms2.yaml) foo | indent 2 }}

ms3:
  weight: 2
  bar:
#! {{ pick (.Files.Get ms3.yaml) bar.baz | indent 4 }}
# To prevent from having a warning when the file is processed by Helm, a fake content may be set here.
# Format of the added dummy elements fully depends on the application's values structure
    dummy:
      dummy: "just here to prevent from a warning"

```

### Tags and Conditions:

As Helm Spray internally uses the Helm Conditions for its own purpose, it is not possible to specify other Conditions that the ones required by Helm Spray itself (`<chart name or alias>.enabled`). Such extra Conditions will be ignored.

However, Helm Spray is compatible with Tags set in the `requirements.yaml` file, as displayed in the following example:
```
dependencies:
- name: micro-service-1
  version: ~1.2
  repository: http://chart-museum/charts
  condition: micro-service-1.enabled
  tags:
  - common
  - front-end
- name: micro-service-2
  version: ~2.3
  repository: http://chart-museum/charts
  condition: micro-service-2.enabled
  tags:
  - common
  - back-end
- name: micro-service-3
  alias: ms3
  version: ~1.1
  repository: http://chart-museum/charts
  condition: ms3.enabled
```
With such a configuration, if Helm Spray is called with the `--set tags.front-end=true` argument, the `micro-service-1` will be deployed (because it has a tag that matches one of those given in the command line) and `micro-service-3` as well (because it has no tag, so no restriction applies), while `micro-service-2` will not be deployed (because it has tags and none of them is matching one of those given in the command line).

Note that tags shall be provided through the `--values`/`-f`, `--set`, `--set-string`, or `--set-file` flags: values coming from the server/Tiller (for example when using the `--reuse-values` flag) are not considered.
Tags values can also not be templated (e.g. `tags.front-end` set to `{{ .Values.x.y.z }}` will not be processed).

### Flags:

```
      --debug                            enable helm debug output (also include spray verbose output)
      --dry-run                          simulate a spray
  -x, --exclude strings                  specify the subchart to exclude (can specify multiple): process all subcharts except the ones specified in '--exclude'
      --force                            force resource update through delete/recreate if needed
  -h, --help                             help for helm
  -n, --namespace string                 namespace to spray the chart into (default "default")
      --prefix-releases string           prefix the releases by the given string, resulting into releases names formats:
                                             "<prefix>-<chart name or alias>"
                                         Allowed characters are a-z A-Z 0-9 and -
      --prefix-releases-with-namespace   prefix the releases by the name of the namespace, resulting into releases names formats:
                                             "<namespace>-<chart name or alias>"
      --reset-values                     when upgrading, reset the values to the ones built into the chart
      --reuse-values                     when upgrading, reuse the last release's values and merge in any overrides from the command line via '--set' and '-f'.
                                         If '--reset-values' is specified, this is ignored
      --set strings                      set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
      --set-file strings                 set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)
      --set-string strings               set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
  -t, --target strings                   specify the subchart to target (can specify multiple). If '--target' is not specified, all subcharts are targeted
      --timeout int                      time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks)
                                         and for liveness and readiness (like Deployments and regular Jobs completion) (default 300)
  -f, --values strings                   specify values in a YAML file or a URL (can specify multiple)
      --verbose                          enable spray verbose output
      --version string                   specify the exact chart version to install. If this is not specified, the latest version is installed
```

## Developer (From Source) Install

If you would like to handle the build yourself, instead of fetching a binary,
this is how recommend doing it.

First, set up your environment:

- You need to have [Go](http://golang.org) installed. Make sure to set `$GOPATH`
- If you don't have [Glide](http://glide.sh) installed, this will install it into
  `$GOPATH/bin` for you.

Clone this repo into your `$GOPATH`. You can use `go get -d github.com/ThalesGroup/helm-spray`
for that.

```
$ cd $GOPATH/src/github.com/ThalesGroup/helm-spray
$ make dist_linux
$ helm plugin install .
```
