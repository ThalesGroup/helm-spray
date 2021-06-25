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
	"os"
	"strconv"
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
	deployments                 map[string]struct{}
	statefulsets                map[string]struct{}
	jobs                        map[string]struct{}
}

// Spray ...
func (s *Spray) Spray() error {

	if s.Debug {
		log.Info(1, "starting spray with flags: %+v\n", s)
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

	releases, err := helm.List(s.Namespace)
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

	s.deployments = map[string]struct{}{}
	s.statefulsets = map[string]struct{}{}
	s.jobs = map[string]struct{}{}

	allDeployments, err := kubectl.GetDeployments(s.Namespace)
	if err != nil {
		return errors.New("cannot list deployments")
	}
	allStatefulSets, err := kubectl.GetStatefulSets(s.Namespace)
	if err != nil {
		return errors.New("cannot list statefulsets")
	}
	allJobs, err := kubectl.GetJobs(s.Namespace)
	if err != nil {
		return errors.New("cannot list jobs")
	}

	for _, deployment := range allDeployments {
		s.deployments[deployment] = struct{}{}
	}
	for _, statefulSet := range allStatefulSets {
		s.statefulsets[statefulSet] = struct{}{}
	}
	for _, job := range allJobs {
		s.jobs[job] = struct{}{}
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
				helmstatus, err := helm.UpgradeWithValues(
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
					log.Info(3, "helm status: %s", helmstatus.Status)
				}

				if !s.DryRun && helmstatus.Status != "deployed" {
					return false, errors.New("status returned by helm differs from \"deployed\", spray interrupted")
				}
			}
		}
	}
	return shouldWait, nil
}

func (s *Spray) wait() error {
	log.Info(2, "waiting for liveness and readiness...")

	allDeployments, err := kubectl.GetDeployments(s.Namespace)
	if err != nil {
		return errors.New("cannot list deployments")
	}
	allStatefulSets, err := kubectl.GetStatefulSets(s.Namespace)
	if err != nil {
		return errors.New("cannot list statefulsets")
	}
	allJobs, err := kubectl.GetJobs(s.Namespace)
	if err != nil {
		return errors.New("cannot list jobs")
	}

	deployments := make([]string, 0)
	for _, deployment := range allDeployments {
		if _, ok := s.deployments[deployment]; !ok {
			deployments = append(deployments, deployment)
		}
	}
	statefulSets := make([]string, 0)
	for _, statefulset := range allStatefulSets {
		if _, ok := s.statefulsets[statefulset]; !ok {
			statefulSets = append(statefulSets, statefulset)
		}
	}
	jobs := make([]string, 0)
	for _, job := range allJobs {
		if _, ok := s.jobs[job]; !ok {
			jobs = append(jobs, job)
		}
	}

	sleepTime := 5
	doneDeployments := false
	doneStatefulSets := false
	doneJobs := false

	// Wait for completion of the Deployments/StatefulSets/Jobs
	for i := 0; i < s.Timeout; {
		if len(deployments) > 0 && !doneDeployments {
			if s.Verbose {
				log.Info(3, "waiting for Deployments %v", deployments)
			}
			doneDeployments, _ = kubectl.AreDeploymentsReady(deployments, s.Namespace, s.Debug)
		} else {
			doneDeployments = true
		}
		if len(statefulSets) > 0 && !doneStatefulSets {
			if s.Verbose {
				log.Info(3, "waiting for StatefulSets %v", statefulSets)
			}
			doneStatefulSets, _ = kubectl.AreStatefulSetsReady(statefulSets, s.Namespace, s.Debug)
		} else {
			doneStatefulSets = true
		}
		if len(jobs) > 0 && !doneJobs {
			if s.Verbose {
				log.Info(3, "waiting for Jobs %v", jobs)
			}
			doneJobs, _ = kubectl.AreJobsReady(jobs, s.Namespace, s.Debug)
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

	for _, deployment := range deployments {
		s.deployments[deployment] = struct{}{}
	}
	for _, statefulSet := range statefulSets {
		s.statefulsets[statefulSet] = struct{}{}
	}
	for _, job := range jobs {
		s.jobs[job] = struct{}{}
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
