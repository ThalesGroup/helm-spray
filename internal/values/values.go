package values

import (
	"fmt"
	"github.com/gemalto/helm-spray/internal/log"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"regexp"
	"strconv"
	"strings"
)

var httpProvider = getter.Provider{
	Schemes: []string{"http", "https"},
	New:     getter.NewHTTPGetter,
}

func Merge(chart *chart.Chart, reuseValues bool, valueOpts *values.Options, verbose bool) (chartutil.Values, string, error) {
	var chartValues chartutil.Values
	var updatedChartValuesAsString string
	var err error

	// Get the default values file of the umbrella chart and process the '#! .Files.Get' directives that might be specified in it
	// Only in case '--reuseValues' has not been set
	if reuseValues == false {
		updatedChartValuesAsString, err = processIncludeInValuesFile(chart, verbose)
		if err != nil {
			return nil, "", fmt.Errorf("processing includes: %w", err)
		}
		updatedChartValues, err := chartutil.ReadValues([]byte(updatedChartValuesAsString))
		if err != nil {
			return nil, "", fmt.Errorf("generating updated values after processing of include(s): %w", err)
		}
		// Merge the new values (including the ones coming from chart dependencies)
		chartValues, err = chartutil.CoalesceValues(chart, updatedChartValues)
		if err != nil {
			if verbose {
				log.WithNumberedLines(1, updatedChartValuesAsString)
			}
			return nil, "", fmt.Errorf("merging updated values with umbrella chart: %w", err)
		}
	} else {
		chartValues, err = chartutil.CoalesceValues(chart, chart.Values)
		if err != nil {
			return nil, "", fmt.Errorf("merging values with umbrella chart: %w", err)
		}
	}

	providedValues, err := valueOpts.MergeValues(getter.Providers{httpProvider})
	if err != nil {
		return nil, "", fmt.Errorf("merging values from CLI flags: %w", err)
	}

	return mergeMaps(chartValues, providedValues), updatedChartValuesAsString, nil
}

// Search the "include" clauses in the default value file of the chart and replace them by the content
// of the corresponding file.
// Allows:
//   - Includeing a file:
//       #! {{ .Files.Get myfile.yaml }}
//   - Including a sub-part of a file, picking a specific tag. Tags can target a Yaml element (aka table) or a
//	   leaf value, but tags cannot target a list item.
//       #! {{ pick (.Files.Get myfile.yaml) tag }}
//   - Indenting the include content:
//       #! {{ .Files.Get myfile.yaml | indent 2 }}
//   - All combined...:
//       #! {{ pick (.Files.Get "myfile.yaml") "tag.subTag" | indent 4 }}
//
func processIncludeInValuesFile(chart *chart.Chart, verbose bool) (string, error) {

	regularExpressions := []string{
		// Expression #0: Process file inclusion ".Files.Get" with optional "| indent"
		// Note: for backward compatibility, ".File.Get" is also allowed
		`#!\s*\{\{\s*pick\s*\(\s*\.Files?\.Get\s+([a-zA-Z0-9_"\\\/\.\-\(\):]+)\s*\)\s*([a-zA-Z0-9_"\.\-]+)\s*(\|\s*indent\s*(\d+))?\s*\}\}\s*(\n|\z)`,
		// Expression #1: Process file inclusion ".Files.Get", picking a specific element of the file content "pick (.Files.Get <file>) <tag>", with an optional "| indent"
		`#!\s*\{\{\s*\.Files?\.Get\s+([a-zA-Z0-9_"\\\/\.\-\(\):]+)\s*(\|\s*indent\s*(\d+))?\s*\}\}\s*(\n|\z)`}

	var chartValues string
	for _, f := range chart.Raw {
		if f.Name == chartutil.ValuesfileName {
			chartValues = string(f.Data)
		}
	}

	if verbose {
		log.Info(1, "looking for \"#! .Files.Get\" clauses into the values file of the umbrella chart...")
	}

	for expressionNumber := 0; expressionNumber < len(regularExpressions); expressionNumber++ {
		includeFileNameExp := regexp.MustCompile(regularExpressions[expressionNumber])

		for match := includeFileNameExp.FindStringSubmatch(chartValues); len(match) != 0; {
			var fullMatch, includeFileName, subValuePath, indent string
			if expressionNumber == 0 {
				fullMatch = match[0]
				includeFileName = strings.Trim(match[1], `"`)
				subValuePath = strings.Trim(match[2], `"`)
				indent = match[4]
			} else if expressionNumber == 1 {
				fullMatch = match[0]
				includeFileName = strings.Trim(match[1], `"`)
				subValuePath = ""
				indent = match[3]
			}

			replaced := false

			for _, f := range chart.Files {
				if f.Name == strings.Trim(strings.TrimSpace(includeFileName), "\"") {
					if verbose {
						if subValuePath == "" {
							if indent == "" {
								log.Info(2, "found reference to values file \"%s\"", includeFileName)
							} else {
								log.Info(2, "found reference to values file \"%s\" (with indent of \"%s\")", includeFileName, indent)
							}
						} else {
							if indent == "" {
								log.Info(2, "found reference to values file \"%s\" (with yaml sub-path \"%s\")", includeFileName, subValuePath)
							} else {
								log.Info(2, "found reference to values file \"%s\" (with yaml sub-path \"%s\" and indent of \"%s\")", includeFileName, subValuePath, indent)
							}
						}
					}

					dataToAdd := string(f.Data)
					if subValuePath != "" {
						data, err := chartutil.ReadValues(f.Data)
						if err != nil {
							return "", fmt.Errorf("reading values from file \"%s\": %w", includeFileName, err)
						}

						// Suppose that the element at the path is an element (list items are not supported)
						if subData, err := data.Table(subValuePath); err == nil {
							if dataToAdd, err = subData.YAML(); err != nil {
								return "", fmt.Errorf("generating a valid YAML file from values at path \"%s\" in values file \"%s\": %w", subValuePath, includeFileName, err)
							}
						} else {
							// If it is not an element, then maybe it is directly a value
							if val, err2 := data.PathValue(subValuePath); err2 == nil {
								var ok bool
								if dataToAdd, ok = val.(string); ok == false {
									return "", fmt.Errorf("finding values matching path \"%s\" in values file \"%s\": %w", subValuePath, includeFileName, err)
								}
							} else {
								return "", fmt.Errorf("finding values matching path \"%s\" in values file \"%s\": %w", subValuePath, includeFileName, err)
							}
						}
					}

					if indent == "" {
						chartValues = strings.Replace(chartValues, fullMatch, dataToAdd+"\n", -1)
					} else {
						nbrOfSpaces, err := strconv.Atoi(indent)
						if err != nil {
							return "", fmt.Errorf("computing indentation value in \"#! .Files.Get\" clause: %w", err)
						}
						dataToAdd := strings.Replace(dataToAdd, "\n", "\n"+strings.Repeat(" ", nbrOfSpaces), -1)
						chartValues = strings.Replace(chartValues, fullMatch, strings.Repeat(" ", nbrOfSpaces)+dataToAdd+"\n", -1)
					}
					replaced = true
				}
			}

			if !replaced {
				return "", fmt.Errorf("finding file \"%s\" referenced in the \"%s\" clause of the default values file of the umbrella chart", match[1], strings.TrimRight(match[0], "\n"))
			}
		}
	}

	return chartValues, nil
}

func mergeMaps(a, b map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(a))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if v, ok := v.(map[string]interface{}); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]interface{}); ok {
					out[k] = mergeMaps(bv, v)
					continue
				}
			}
		}
		out[k] = v
	}
	return out
}
