package dependencies

import (
	"encoding/json"
	"fmt"
	"github.com/gemalto/helm-spray/v4/internal/log"
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

func Get(chart *chart.Chart, values *chartutil.Values, targets []string, excludes []string, releasePrefix string, verbose bool) ([]Dependency, error) {
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

		dependencies[i].Targeted = computeTargeted(dependencies[i].UsedName, targets, excludes)
		dependencies[i].HasTags, dependencies[i].AllowedByTags = computeAllowedByTags(req.Tags, providedTags)

		weight, err := computeWeight(values, dependencies[i].UsedName)
		if err != nil {
			return nil, err
		}
		dependencies[i].Weight = weight
		dependencies[i].CorrespondingReleaseName = releasePrefix + dependencies[i].UsedName

		// Get the AppVersion that is contained in the Chart.yaml file of the dependency sub-chart
		for _, subChart := range chart.Dependencies() {
			if subChart.Metadata.Name == dependencies[i].Name {
				dependencies[i].AppVersion = subChart.Metadata.AppVersion
				break
			}
		}
	}
	return dependencies, nil
}

// computeTargeted determines whether a dependency is targeted.
// If --target or --excludes are specified, it should match the name of the current dependency;
// if neither --target nor --exclude are specified, then all dependencies are targeted.
func computeTargeted(usedName string, targets []string, excludes []string) bool {
	if len(targets) > 0 {
		for j := range targets {
			if targets[j] == usedName {
				return true
			}
		}
		return false
	}
	if len(excludes) > 0 {
		for j := range excludes {
			if excludes[j] == usedName {
				return false
			}
		}
		return true
	}
	return true
}

// computeAllowedByTags checks the tags associated to the dependency against the tags provided in the values.
func computeAllowedByTags(reqTags []string, providedTags map[string]interface{}) (hasTags bool, allowed bool) {
	if len(reqTags) == 0 {
		return false, true
	}
	for _, tag := range reqTags {
		for k, v := range providedTags {
			if k == tag && v == true {
				return true, true
			}
		}
	}
	return true, false
}

// computeWeight extracts the weight of the dependency. If no weight is specified, it is 0.
func computeWeight(values *chartutil.Values, usedName string) (int, error) {
	weightJson, err := values.PathValue(usedName + ".weight")
	if err != nil {
		return 0, fmt.Errorf("computing weight value for sub-chart \"%s\": %w", usedName, err)
	}

	weightInteger := 0
	// Depending on the configuration of the json parser, integer can be returned either as Float64 or json.Number
	if reflect.TypeOf(weightJson).String() == "json.Number" {
		w, err := weightJson.(json.Number).Int64()
		if err != nil {
			return 0, fmt.Errorf("computing weight value for sub-chart \"%s\": %w", usedName, err)
		}
		weightInteger = int(w)

	} else if reflect.TypeOf(weightJson).String() == "float64" {
		weightInteger = int(weightJson.(float64))

	} else {
		return 0, fmt.Errorf("computing weight value for sub-chart \"%s\", value shall be an integer", usedName)
	}

	if weightInteger < 0 {
		return 0, fmt.Errorf("computing weight value for sub-chart \"%s\", value shall be positive or equal to zero", usedName)
	}
	return weightInteger, nil
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
			log.Info(2, "found tag \"%s: %s\"", k, fmt.Sprint(v))
		}
	}
	return providedTags
}
