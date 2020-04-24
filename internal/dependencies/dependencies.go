package dependencies

import (
	"encoding/json"
	"fmt"
	"github.com/gemalto/helm-spray/internal/log"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"reflect"
)

// Dependency ...
type Dependency struct {
	Name                     string
	Alias                    string
	UsedName                 string
	AppVersion               string
	Targeted                 bool
	Weight                   int
	CorrespondingReleaseName string
	HasTags                  bool
	AllowedByTags            bool
}

func Get(chart *chart.Chart, values *chartutil.Values, targets []string, excludes []string, releasePrefix string, verbose bool) []Dependency {
	// Compute tags
	providedTags := tags(values, verbose)

	// Build the list of all dependencies, and their key attributes
	dependencies := make([]Dependency, len(chart.Metadata.Dependencies))
	for i, req := range chart.Metadata.Dependencies {
		// Dependency name and alias
		dependencies[i].Name = req.Name
		dependencies[i].Alias = req.Alias
		if req.Alias == "" {
			dependencies[i].UsedName = dependencies[i].Name
		} else {
			dependencies[i].UsedName = dependencies[i].Alias
		}

		// Is dependency targeted?
		// If --target or --excludes are specified, it should match the name of the current dependency;
		// If neither --target nor --exclude are specified, then all dependencies are targeted
		if len(targets) > 0 {
			dependencies[i].Targeted = false
			for j := range targets {
				if targets[j] == dependencies[i].UsedName {
					dependencies[i].Targeted = true
				}
			}

		} else if len(excludes) > 0 {
			dependencies[i].Targeted = true
			for j := range excludes {
				if excludes[j] == dependencies[i].UsedName {
					dependencies[i].Targeted = false
				}
			}

		} else {
			dependencies[i].Targeted = true
		}

		// Loop on the tags associated to the dependency and check with the tags provided in the values
		dependencies[i].AllowedByTags = false
		if len(req.Tags) == 0 {
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
		weightJson, err := values.PathValue(dependencies[i].UsedName + ".weight")
		if err != nil {
			log.ErrorAndExit("Error computing weight value for sub-chart \"%s\": %s", dependencies[i].UsedName, err)
		}

		weightInteger := 0
		// Depending on the configuration of the json parser, integer can be returned either as Float64 or json.Number
		if reflect.TypeOf(weightJson).String() == "json.Number" {
			w, err := weightJson.(json.Number).Int64()
			if err != nil {
				log.ErrorAndExit("Error computing weight value for sub-chart \"%s\": value shall be an integer", dependencies[i].UsedName)
			}
			weightInteger = int(w)

		} else if reflect.TypeOf(weightJson).String() == "float64" {
			weightInteger = int(weightJson.(float64))

		} else {
			log.ErrorAndExit("Error computing weight value for sub-chart \"%s\": value shall be an integer", dependencies[i].UsedName)
		}

		if weightInteger < 0 {
			log.ErrorAndExit("Error computing weight value for sub-chart \"%s\": value shall be positive or equal to zero", dependencies[i].UsedName)
		}
		dependencies[i].Weight = weightInteger
		dependencies[i].CorrespondingReleaseName = releasePrefix + dependencies[i].UsedName

		// Get the AppVersion that is contained in the Chart.yaml file of the dependency sub-chart
		for _, subChart := range chart.Dependencies() {
			if subChart.Metadata.Name == dependencies[i].Name {
				dependencies[i].AppVersion = subChart.Metadata.AppVersion
				break
			}
		}
	}
	return dependencies
}

func tags(values *chartutil.Values, verbose bool) map[string]interface{} {
	// Get the list of "tags" specified in the values...
	// (locally-provided values only; values coming from server are not considered)
	if verbose {
		log.Info(1, "looking for \"tags\" in values provided through \"--values/-f\", \"--set\", \"--set-string\", and \"--set-file\"...")
	}
	var providedTags map[string]interface{}
	tags, err := values.Table("tags")
	if err == nil {
		providedTags = tags.AsMap()
	}
	if verbose {
		for k, v := range providedTags {
			log.Info(2, fmt.Sprintf("found tag \"%s: %s\"", k, fmt.Sprint(v)))
		}
	}
	return providedTags
}
