package helmspray

import (
	"errors"
	"fmt"
	"github.com/gemalto/helm-spray/v4/internal/dependencies"
	"github.com/gemalto/helm-spray/v4/internal/log"
	"github.com/gemalto/helm-spray/v4/internal/values"
	"github.com/gemalto/helm-spray/v4/pkg/helm"
	"github.com/gemalto/helm-spray/v4/pkg/kubectl"
	"github.com/gemalto/helm-spray/v4/pkg/util"
	"helm.sh/helm/v3/pkg/chart/loader"
	cliValues "helm.sh/helm/v3/pkg/cli/values"
	"io/ioutil"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

type Spray struct {
	ChartName                   string
	ChartVersion                string
	Targets                     []string
	Excludes                    []string
	Namespace                   string
	CreateNamespace             bool
	PrefixReleases              string
	PrefixReleasesWithNamespace bool
	ResetValues                 bool
	ReuseValues                 bool
	ValuesOpts                  cliValues.Options
	Force                       bool
	Timeout                     int
	DryRun                      bool
	Verbose                     bool
	Debug                       bool
	deployments                 []string
	statefulSets                []string
	jobs                        []string
}

// Spray ...
func (s *Spray) Spray() error {

	if s.Debug {
		log.Info(1, "starting spray with flags: %+v", s)
	}

	startTime := time.Now()

	// Load and validate the umbrella chart...
	chart, err := loader.Load(s.ChartName)
	if err != nil {
		return fmt.Errorf("loading chart \"%s\": %w", s.ChartName, err)
	}

	mergedValues, updatedChartValuesAsString, err := values.Merge(chart, s.ReuseValues, &s.ValuesOpts, s.Verbose)
	if err != nil {
		return fmt.Errorf("merging values: %w", err)
	}
	if len(updatedChartValuesAsString) > 0 {
		// Write default values to a temporary file and add it to the list of values files,
		// for later usage during the calls to helm
		tempDir, err := ioutil.TempDir("", "spray-")
		if err != nil {
			return fmt.Errorf("creating temporary directory to write updated default values file for umbrella chart: %w", err)
		}
		defer removeTempDir(tempDir)
		tempFile, err := ioutil.TempFile(tempDir, "updatedDefaultValues-*.yaml")
		if err != nil {
			return fmt.Errorf("creating temporary file to write updated default values file for umbrella chart: %w", err)
		}
		defer removeTempFile(tempFile.Name())
		if _, err = tempFile.Write([]byte(updatedChartValuesAsString)); err != nil {
			return fmt.Errorf("writing updated default values file for umbrella chart into temporary file: %w", err)
		}
		err = tempFile.Close()
		if err != nil {
			return fmt.Errorf("closing temporary file to write updated default values file for umbrella chart: %w", err)
		}
		prependArray := []string{tempFile.Name()}
		s.ValuesOpts.ValueFiles = append(prependArray, s.ValuesOpts.ValueFiles...)
	}

	releasePrefix := ""
	if s.PrefixReleasesWithNamespace && len(s.Namespace) > 0 {
		releasePrefix = s.Namespace + "-"
	} else if len(s.PrefixReleases) > 0 {
		releasePrefix = s.PrefixReleases + "-"
	}
	deps, err := dependencies.Get(chart, &mergedValues, s.Targets, s.Excludes, releasePrefix, s.Verbose)
	if err != nil {
		return fmt.Errorf("analyzing dependencies: %w", err)
	}

	// Starting the processing...
	if len(releasePrefix) > 0 {
		log.Info(1, "deploying solution chart \"%s\" in namespace \"%s\", with releases releasePrefix \"%s-\"", s.ChartName, s.Namespace, releasePrefix)
	} else {
		log.Info(1, "deploying solution chart \"%s\" in namespace \"%s\"", s.ChartName, s.Namespace)
	}

	releases, err := helm.List(1, s.Namespace, s.Debug)
	if err != nil {
		return fmt.Errorf("listing releases: %w", err)
	}

	if s.Verbose {
		logRelease(releases, deps)
	}

	err = checkTargetsAndExcludes(deps, s.Targets, s.Excludes)
	if err != nil {
		return fmt.Errorf("checking targets and excludes: %w", err)
	}

	// Loop on the increasing weight
	for i := 0; i <= maxWeight(deps); i++ {
		shouldWait, err := s.upgrade(releases, deps, i)
		if err != nil {
			return err
		}
		// Wait availability of the just upgraded Releases
		if shouldWait && !s.DryRun {
			err = s.wait()
			if err != nil {
				return err
			}
		}
	}

	log.Info(1, "upgrade of solution chart \"%s\" completed in %s", s.ChartName, util.Duration(time.Since(startTime)))

	return nil
}

func (s *Spray) upgrade(releases map[string]helm.Release, deps []dependencies.Dependency, currentWeight int) (bool, error) {
	shouldWait := false
	firstInWeight := true
	// Upgrade the targeted Deployments corresponding the the current weight
	for _, dependency := range deps {
		if dependency.Targeted && dependency.AllowedByTags == true {
			if dependency.Weight == currentWeight {
				if firstInWeight {
					log.Info(1, "processing sub-charts of weight %d", dependency.Weight)
					firstInWeight = false
					s.deployments = make([]string, 0)
					s.statefulSets = make([]string, 0)
					s.jobs = make([]string, 0)
				}

				if release, ok := releases[dependency.CorrespondingReleaseName]; ok {
					oldRevision, _ := strconv.Atoi(release.Revision)
					log.Info(2, "upgrading release \"%s\": going from revision %d (status %s) to %d (appVersion %s)...", dependency.CorrespondingReleaseName, oldRevision, release.Status, oldRevision+1, dependency.AppVersion)

				} else {
					log.Info(2, "upgrading release \"%s\": deploying first revision (appVersion %s)...", dependency.CorrespondingReleaseName, dependency.AppVersion)
				}

				shouldWait = true

				// Add the "<dependency>.enabled" flags to ensure that only the current chart is to be executed
				depValuesSet := ""
				for _, dep := range deps {
					if dep.UsedName == dependency.UsedName {
						depValuesSet = depValuesSet + dep.UsedName + ".enabled=true,"
					} else {
						depValuesSet = depValuesSet + dep.UsedName + ".enabled=false,"
					}
				}
				var valuesSet []string
				valuesSet = append(valuesSet, s.ValuesOpts.Values...)
				valuesSet = append(valuesSet, depValuesSet)

				// Upgrade the Deployment
				upgradedRelease, err := helm.UpgradeWithValues(3,
					s.Namespace,
					s.CreateNamespace,
					dependency.CorrespondingReleaseName,
					s.ChartName,
					s.ResetValues,
					s.ReuseValues,
					s.ValuesOpts.ValueFiles,
					valuesSet,
					s.ValuesOpts.StringValues,
					s.ValuesOpts.FileValues,
					s.Force,
					s.Timeout,
					s.DryRun,
					s.Debug,
				)
				if err != nil {
					return false, fmt.Errorf("calling helm upgrade: %w", err)
				}

				log.Info(3, "release: \"%s\" upgraded", dependency.CorrespondingReleaseName)

				if s.Verbose {
					log.Info(3, "helm status: %s", upgradedRelease.Info["status"])
				}
				if !s.DryRun && upgradedRelease.Info["status"] != "deployed" {
					return false, errors.New("status returned by helm differs from \"deployed\", spray interrupted")
				}

				for _, yaml := range strings.Split(upgradedRelease.Manifest, "---") {
					manifest, _, err := scheme.Codecs.UniversalDeserializer().Decode([]byte(yaml), nil, nil)
					if err != nil && len(yaml) > 0 {
						log.Info(3, "warning: ignored part of helm upgrade output")
						if s.Debug {
							log.Info(3, "warning: ignored '%s'", yaml)
						}
					}
					deployment, ok := manifest.(*appsv1.Deployment)
					if ok {
						s.deployments = append(s.deployments, deployment.Name)
					}
					statefulSet, ok := manifest.(*appsv1.StatefulSet)
					if ok {
						s.statefulSets = append(s.statefulSets, statefulSet.Name)
					}
					job, ok := manifest.(*batchv1.Job)
					if ok {
						s.jobs = append(s.jobs, job.Name)
					}
				}

				if s.Verbose {
					if len(s.deployments) > 0 {
						log.Info(3, "release deployments: %v", s.deployments)
					}
					if len(s.statefulSets) > 0 {
						log.Info(3, "release statefulsets: %v", s.statefulSets)
					}
					if len(s.jobs) > 0 {
						log.Info(3, "release jobs: %v", s.jobs)
					}
				}
			}
		}
	}
	return shouldWait, nil
}

func (s *Spray) wait() error {
	log.Info(2, "waiting for liveness and readiness...")

	sleepTime := 5
	doneDeployments := false
	doneStatefulSets := false
	doneJobs := false

	// Wait for completion of the Deployments/StatefulSets/Jobs
	for i := 0; i < s.Timeout; {
		if len(s.deployments) > 0 && !doneDeployments {
			if s.Verbose {
				log.Info(3, "waiting for deployments %v", s.deployments)
			}
			doneDeployments, _ = kubectl.AreDeploymentsReady(s.deployments, s.Namespace, s.Debug)
		} else {
			doneDeployments = true
		}
		if len(s.statefulSets) > 0 && !doneStatefulSets {
			if s.Verbose {
				log.Info(3, "waiting for statefulsets %v", s.statefulSets)
			}
			doneStatefulSets, _ = kubectl.AreStatefulSetsReady(s.statefulSets, s.Namespace, s.Debug)
		} else {
			doneStatefulSets = true
		}
		if len(s.jobs) > 0 && !doneJobs {
			if s.Verbose {
				log.Info(3, "waiting for jobs %v", s.jobs)
			}
			doneJobs, _ = kubectl.AreJobsReady(s.jobs, s.Namespace, s.Debug)
		} else {
			doneJobs = true
		}
		if doneDeployments && doneStatefulSets && doneJobs {
			break
		}
		time.Sleep(time.Duration(sleepTime) * time.Second)
		i = i + sleepTime
	}

	if !doneDeployments || !doneStatefulSets || !doneJobs {
		return errors.New("timed out waiting for liveness and readiness")
	}

	return nil
}

// Retrieve the highest chart.weight in values.yaml
func maxWeight(deps []dependencies.Dependency) (m int) {
	if len(deps) > 0 {
		m = deps[0].Weight
	}
	for i := 1; i < len(deps); i++ {
		if deps[i].Weight > m {
			m = deps[i].Weight
		}
	}
	return m
}

func checkTargetsAndExcludes(deps []dependencies.Dependency, targets []string, excludes []string) error {
	// Check that the provided target(s) or exclude(s) correspond to valid sub-chart names or alias
	if len(targets) > 0 {
		for i := range targets {
			found := false
			for _, dependency := range deps {
				if targets[i] == dependency.UsedName {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("invalid targetted sub-chart name/alias \"%s\"", targets[i])
			}
		}
	} else if len(excludes) > 0 {
		for i := range excludes {
			found := false
			for _, dependency := range deps {
				if excludes[i] == dependency.UsedName {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("invalid excluded sub-chart name/alias \"%s\"", excludes[i])
			}
		}
	}
	return nil
}

func logRelease(releases map[string]helm.Release, deps []dependencies.Dependency) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.Debug)
	_, _ = fmt.Fprintln(w, "[spray]  \t subchart\t is alias of\t targeted\t weight\t| corresponding release\t revision\t status\t")
	_, _ = fmt.Fprintln(w, "[spray]  \t --------\t -----------\t --------\t ------\t| ---------------------\t --------\t ------\t")

	for _, dependency := range deps {
		currentRevision := "None"
		currentStatus := "Not deployed"
		if release, ok := releases[dependency.CorrespondingReleaseName]; ok {
			currentRevision = release.Revision
			currentStatus = release.Status
		}

		name := dependency.Name
		alias := "-"
		if dependency.Alias != "" {
			name = dependency.Alias
			alias = dependency.Name
		}

		targeted := fmt.Sprint(dependency.Targeted)
		if dependency.Targeted && dependency.HasTags && (dependency.AllowedByTags == true) {
			targeted = "true (tag match)"
		} else if dependency.Targeted && dependency.HasTags && (dependency.AllowedByTags == false) {
			targeted = "false (no tag match)"
		}

		_, _ = fmt.Fprintln(w, fmt.Sprintf("[spray]  \t %s\t %s\t %s\t %d\t| %s\t %s\t %s\t", name, alias, targeted, dependency.Weight, dependency.CorrespondingReleaseName, currentRevision, currentStatus))
	}
	_ = w.Flush()
}

func removeTempDir(tempDir string) {
	if err := os.RemoveAll(tempDir); err != nil {
		log.Error("Error: removing temporary directory: %s", err)
	}
}

func removeTempFile(tempFile string) {
	if err := os.Remove(tempFile); err != nil {
		log.Error("Error: removing temporary file: %s", err)
	}
}
