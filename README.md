# Helm Spray

[![Build Status](https://api.travis-ci.com/gemalto/helm-spray.svg?branch=master)](https://travis-ci.com/gemalto/helm-spray)

![helm-spray](https://gemalto.github.io/helm-spray/logo/helm-spray_150x150.png)

## What is Helm Spray?

This is a Helm plugin to install or upgrade sub-charts one by one from an umbrella chart.

It works like `helm upgrade --install`, except that it upgrades or installs each sub-charts according to a weight (>=0) set on each sub-chart. All sub-charts of weight 0 are processed first, then sub-charts of weight 1, etc.
Chart weight shall be specified using the `<chart name>.weight` value.

Each sub-chart is deployed under a specific Release named `<chart name or alias>`, enabling a later individual upgrade targeting this sub-chart only. All global or individual upgrade should still be done on the umbrella chart.


## Continuous Integration & Delivery

Helm Spray is building and delivering under Travis.

[![Build Status](https://api.travis-ci.com/gemalto/helm-spray.svg?branch=master)](https://travis-ci.com/gemalto/helm-spray)


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

A "values" file shall also be set with the weight ito be applied to each individual sub-chart. This weight shall be set in the `<chart name or alias>.weight` element. A good practice is that thei weigths are statically set in the default `values.yaml` file of the umbrella chart (and not in a yaml file provided using the `-f` option), as sub-chart's weight is not likely to change over time.
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

Helm Spray creates one helm Release per sub-chart. Releases are individually upgraded when running the helm spray process, in particular when using the `--target` option.
The name and version of the umbrella chart is set as the Chart name for all the Revisions.
```
NAME            REVISION        UPDATED                         STATUS          CHART           APP VERSION     NAMESPACE
micro-service-1 12              Wed Jan 30 17:19:15 2019        DEPLOYED        solution-0.1    0.1             default
micro-service-2 21              Wed Jan 30 17:18:55 2019        DEPLOYED        solution-0.1    0.1             default
ms3             7               Wed Jan 30 17:18:45 2019        DEPLOYED        solution-0.1    0.1             default
```

Note: if an alias is set for a sub-chart, then this is this alias that should be used with the `--target` optioni, not the sub-chart name.

### Flags:

```
      --debug              enable verbose output
      --dry-run            simulate a spray
      --force              force resource update through delete/recreate if needed
  -h, --help               help for helm
  -n, --namespace string   namespace to spray the chart into. (default "default")
      --reset-values       when upgrading, reset the values to the ones built into the chart
      --reuse-values       when upgrading, reuse the last release's values and merge in any overrides from the command line via --set and -f. If '--reset-values' is specified, this is ignored.
      --set string         set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
  -t, --target strings     specify the subchart to target (can specify multiple). If --target is not specified, all subcharts are targeted
  -f, --values strings     specify values in a YAML file or a URL (can specify multiple)
      --version string     specify the exact chart version to install. If this is not specified, the latest version is installed
```

## Install

```
  $ helm plugin install https://github.com/gemalto/helm-spray
```

The above will fetch the latest binary release of `helm spray` and install it.

## Developer (From Source) Install

If you would like to handle the build yourself, instead of fetching a binary,
this is how recommend doing it.

First, set up your environment:

- You need to have [Go](http://golang.org) installed. Make sure to set `$GOPATH`
- If you don't have [Glide](http://glide.sh) installed, this will install it into
  `$GOPATH/bin` for you.

Clone this repo into your `$GOPATH`. You can use `go get -d github.com/gemalto/helm-spray`
for that.

```
$ cd $GOPATH/src/github.com/gemalto/helm-spray
$ make bootstrap build
$ SKIP_BIN_INSTALL=1 helm plugin install $GOPATH/src/github.com/gemalto/helm-spray
```

That last command will skip fetching the binary install and use the one you
built.







