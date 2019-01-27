# helm Spray

[![Build Status](https://api.travis-ci.com/gemalto/helm-spray.svg?branch=master)](https://travis-ci.com/gemalto/helm-spray)

![helm-spray](https://gemalto.github.io/helm-spray/logo/helm-spray_150x150.png)

## What is helm Spray?

This is a Helm plugin to install or upgrade sub-charts one by one from an umbrella chart.

It works like `helm upgrade --install`, except that it upgrades or installs each sub-charts according to a weight (>=0) set on each sub-chart. All sub-charts of weight 0 are processed first, then sub-charts of weight 1, etc.
Chart weight shall be specified using the '<chart name>.weight' value.

Each sub-chart is deployed under a specific Release named '<chart name>', enabling a later individual upgrade targeting this sub-chart only. All global or individual upgrade should still be done on the umbrella chart.


## Continuous Integration & Delivery

Helm Spray is building and delivering under Travis.

[![Build Status](https://api.travis-ci.com/gemalto/helm-spray.svg?branch=master)](https://travis-ci.com/gemalto/helm-spray)


## Usage

```
  $ helm spray [flags] CHART
```

### Flags:

```
      --dry-run            simulate a spray
      --debug              enable verbose output
  -h, --help               help for helm
  -n, --namespace string   namespace to spray the chart into. (default "default")
      --set string         set values on the command line (can specify multiple or 
                           separate values with commas: key1=val1,key2=val2)
  -f, --values string      specify values in a YAML file or a URL (can specify multiple)
  -t, --target string      specify the subchart to target (can specify multiple).
                           If --target is not specified, all subcharts are targeted.
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







