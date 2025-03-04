/*
Copyright 2023 The Nuclio Authors.

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
	"bufio"
	"flag"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"

	"github.com/coreos/go-semver/semver"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"gopkg.in/yaml.v3"
)

const (
	helmChartFilePath = "hack/k8s/helm/nuclio/Chart.yaml"
	githubAPIURL      = "https://api.github.com"
	travisAPIURL      = "https://api.travis-ci.org"
)

type helmChart struct {
	Version    semver.Version `yaml:"version,omitempty"`
	AppVersion semver.Version `yaml:"appVersion,omitempty"`
}

type Release struct {
	currentVersion          *semver.Version
	targetVersion           *semver.Version
	helmChartsTargetVersion *semver.Version
	releaseInfoPath         string
	repositoryDirPath       string
	repositoryOwnerName     string
	repositoryScheme        string
	developmentBranch       string
	releaseBranch           string
	skipPublishHelmCharts   bool
	skipBumpHelmChart       bool
	skipCreateRelease       bool
	bumpPatch               bool
	bumpMinor               bool
	bumpMajor               bool

	logger    logger.Logger
	cmdRunner cmdrunner.CmdRunner

	helmChartConfig helmChart
}

func NewRelease(cmdRunner cmdrunner.CmdRunner, logger logger.Logger) *Release {
	return &Release{
		logger:    logger,
		cmdRunner: cmdRunner,

		// initialize with empty versions
		currentVersion:          &semver.Version{},
		targetVersion:           &semver.Version{},
		helmChartsTargetVersion: &semver.Version{},
	}
}

func (r *Release) Run() error {

	// read the helm chart and populate its values for target version determination
	if err := r.populateHelmChartConfig(); err != nil {
		return errors.Wrap(err, "Failed to populate helm chart config")
	}

	// set current and target versions
	if err := r.populateCurrentAndTargetVersions(); err != nil {
		return errors.Wrap(err, "Failed to populate current and target versions")
	}

	// helm chart release
	if !r.skipBumpHelmChart {
		if err := r.bumpHelmChartVersion(); err != nil {
			return errors.Wrap(err, "Failed to bump helm chart version")
		}
	} else {
		r.logger.Info("Skipping bump helm chart")
	}
	return nil
}

func (r *Release) populateHelmChartConfig() error {

	// read
	yamlFile, err := os.ReadFile(r.resolveHelmChartFullPath())
	if err != nil {
		return errors.Wrap(err, "Failed to read chart file")
	}

	// populate
	if err := yaml.Unmarshal(yamlFile, &r.helmChartConfig); err != nil {
		return errors.Wrap(err, "Failed to unmarshal chart config")
	}
	return nil
}

func (r *Release) resolveHelmChartFullPath() string {
	return helmChartFilePath
}

func (r *Release) populateCurrentAndTargetVersions() error {
	runOptions := &cmdrunner.RunOptions{
		WorkingDir: &r.repositoryDirPath,
	}

	// try populate bumped versions
	if err := r.populateBumpedVersions(); err != nil {
		return errors.Wrap(err, "Failed to resolve desired patch version")
	}

	// current version is empty, infer from tags
	if r.currentVersion.Equal(semver.Version{}) {

		// nuclio binaries & images version
		results, err := r.cmdRunner.Run(runOptions, `git describe --abbrev=0 --tags`)
		if err != nil {
			return errors.Wrap(err, "Failed to describe tags")
		}
		r.currentVersion = semver.New(strings.TrimSpace(results.Output))
	}

	// target version is empty, prompt user for an input
	if r.targetVersion.Equal(semver.Version{}) {

		// we do not intend to create a release and hence preserving the current target version
		if r.skipCreateRelease {
			r.targetVersion = r.currentVersion
		} else {

			// read as input
			reader := bufio.NewReader(os.Stdin)
			fmt.Printf("Target version (Current version: %s, Press enter to continue): ",
				r.currentVersion)
			targetVersion, err := reader.ReadString('\n')
			if err != nil {
				return errors.Wrap(err, "Failed to read target version from stdin")
			}
			r.targetVersion = semver.New(strings.TrimSpace(targetVersion))
		}
	}

	// helm charts target version is empty, prompt user for an input
	if r.helmChartsTargetVersion.Equal(semver.Version{}) {
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("Helm chart target version (Current version: %s, Press enter to continue): ",
			r.helmChartConfig.Version)
		helmChartsTargetVersionStr, err := reader.ReadString('\n')
		if err != nil {
			return errors.Wrap(err, "Failed to read helm chart target version from stdin")
		}
		r.helmChartsTargetVersion = semver.New(strings.TrimSpace(helmChartsTargetVersionStr))
	}

	// sanity
	for _, version := range []*semver.Version{
		r.currentVersion,
		r.targetVersion,
		r.helmChartsTargetVersion,
	} {
		if version == nil || version.Equal(semver.Version{}) {
			return errors.New("Found an empty version, bailing")
		}
	}

	if err := r.saveReleaseInfo(); err != nil {
		return errors.Wrap(err, "Failed to save release info")
	}

	r.logger.InfoWith("Successfully populated versions",
		"currentVersion", r.currentVersion,
		"targetVersion", r.targetVersion,
		"helmChartsTargetVersion", r.helmChartsTargetVersion)
	return nil
}

func (r *Release) saveReleaseInfo() error {
	// Define the file where the release info will be saved
	if r.releaseInfoPath == "" {
		return nil
	}

	// Create or open the file for writing
	file, err := os.Create(r.releaseInfoPath)
	if err != nil {
		return errors.Wrap(err, "Failed to create file")
	}
	defer file.Close()

	// Write the version information with specific labels to the file
	_, err = file.WriteString(fmt.Sprintf("CURRENT_VERSION: %s\n", r.currentVersion.String()))
	if err != nil {
		return errors.Wrap(err, "Failed to write current version")
	}

	_, err = file.WriteString(fmt.Sprintf("TARGET_VERSION: %s\n", r.targetVersion.String()))
	if err != nil {
		return errors.Wrap(err, "Failed to write target version")
	}

	_, err = file.WriteString(fmt.Sprintf("HELM_CHARTS_TARGET_VERSION: %s\n", r.helmChartsTargetVersion.String()))
	if err != nil {
		return errors.Wrap(err, "Failed to write helm charts target version")
	}

	return nil
}

func (r *Release) bumpHelmChartVersion() error {
	r.logger.DebugWith("Bumping helm chart version",
		"targetVersion", r.targetVersion,
		"helmChartsTargetVersion", r.helmChartsTargetVersion)

	runOptions := &cmdrunner.RunOptions{
		WorkingDir: &r.repositoryDirPath,
	}

	// bump & release helm charts is being done from the release branch
	if _, err := r.cmdRunner.Run(runOptions,
		`git checkout %s`,
		r.releaseBranch); err != nil {
		return errors.Wrap(err, "Failed to checkout to release branch")
	}
	if _, err := r.cmdRunner.Run(runOptions,
		`git grep -lF "%s" %s | grep yaml | xargs sed -i -e "s/%s/%s/g"`,
		r.helmChartConfig.AppVersion,
		path.Join("hack", "k8s"),
		r.helmChartConfig.AppVersion,
		r.targetVersion); err != nil {
		return errors.Wrap(err, "Failed to update target version")
	}

	// explicitly bump the app version
	if _, err := r.cmdRunner.Run(runOptions,
		`sed -i -e "s/^\(appVersion: \).*$/\1%s/g" %s`,
		r.targetVersion,
		r.resolveHelmChartFullPath()); err != nil {
		return errors.Wrap(err, "Failed to write helm chart target version")
	}

	if _, err := r.cmdRunner.Run(runOptions,
		`sed -i  -e "s/^\(version: \).*$/\1%s/g" %s`,
		r.helmChartsTargetVersion,
		r.resolveHelmChartFullPath()); err != nil {
		return errors.Wrap(err, "Failed to write helm chart target version")
	}

	// commit & push changes
	//commitMessage := fmt.Sprintf("Bump to %s", r.targetVersion)

	if !r.skipPublishHelmCharts {
		r.logger.Debug("Publishing helm charts")
		if _, err := r.cmdRunner.Run(runOptions,
			`git checkout %s`,
			r.releaseBranch); err != nil {
			return errors.Wrap(err, "Failed to checkout to release branch")
		}
		if _, err := r.cmdRunner.Run(&cmdrunner.RunOptions{
			WorkingDir: &r.repositoryDirPath,
		}, `make helm-publish`); err != nil {
			return errors.Wrap(err, "Failed to publish helm charts")
		}
	}
	return nil
}

func (r *Release) populateBumpedVersions() error {
	if !(r.bumpPatch || r.bumpMinor || r.bumpMajor) {
		return nil
	}

	var bumpAppVersion bool
	var bumpChartVersion bool

	// if not set, fill target version from helm chart app version
	// NOTE: this can be overridden via CLI
	if r.targetVersion.Equal(semver.Version{}) {
		if err := r.targetVersion.Set(r.helmChartConfig.AppVersion.String()); err != nil {
			return errors.Wrap(err, "Failed to set target version")
		}
		bumpAppVersion = true
	}

	// if not set, fill helm chart target version from helm chart
	// NOTE: this can be overridden via CLI
	if r.helmChartsTargetVersion.Equal(semver.Version{}) {
		if err := r.helmChartsTargetVersion.Set(r.helmChartConfig.Version.String()); err != nil {
			return errors.Wrap(err, "Failed to set helm charts target version")
		}
		bumpChartVersion = true
	}

	// fill current version from helm chart
	if r.currentVersion.Equal(semver.Version{}) {
		if err := r.currentVersion.Set(r.helmChartConfig.AppVersion.String()); err != nil {
			return errors.Wrap(err, "Failed to set current version")
		}
	}

	if bumpAppVersion {
		r.logger.DebugWith("Bumping app version", "version", r.targetVersion)
		r.bumpVersion(r.targetVersion)
	}

	if bumpChartVersion {
		r.logger.DebugWith("Bumping chart version", "version", r.helmChartsTargetVersion)
		r.bumpVersion(r.helmChartsTargetVersion)
	}

	r.logger.DebugWith("Successfully populated bumped version",
		"currentVersion", r.currentVersion,
		"currentHelmChartsVersion", r.helmChartConfig.Version,
		"targetVersion", r.targetVersion,
		"helmChartsTargetVersion", r.helmChartsTargetVersion)
	return nil
}

func (r *Release) bumpVersion(version *semver.Version) {
	switch {
	case r.bumpPatch:
		r.logger.Info("Bumping patch version")
		version.BumpPatch()
	case r.bumpMinor:
		r.logger.Info("Bumping minor version")
		version.BumpMinor()
	case r.bumpMajor:
		r.logger.Info("Bumping major version")
		version.BumpMajor()
	}
}

func run() error {
	loggerInstance, err := nucliozap.NewNuclioZapCmd("releaser",
		nucliozap.DebugLevel,
		common.GetRedactorInstance(os.Stdout))
	if err != nil {
		return errors.Wrap(err, "Failed to create logger")
	}

	shellRunner, err := cmdrunner.NewShellRunner(loggerInstance)
	if err != nil {
		return errors.Wrap(err, "Failed to create command runner")
	}

	release := NewRelease(shellRunner, loggerInstance)
	flag.Var(release.targetVersion, "target-version", "Release target version")
	flag.Var(release.currentVersion, "current-version", "Current version")
	flag.Var(release.helmChartsTargetVersion, "helm-charts-release-version", "Helm charts release target version")
	flag.StringVar(&release.releaseInfoPath, "release-info-path", "", "Path to save release info")
	flag.BoolVar(&release.skipBumpHelmChart, "skip-bump-helm-chart", false, "Skip bump helm chart")
	flag.BoolVar(&release.skipPublishHelmCharts, "skip-publish-helm-charts", false, "Whether to skip publishing helm charts")
	flag.BoolVar(&release.bumpPatch, "bump-patch", false, "Resolve chart version and bump both Nuclio and Chart patch version")
	flag.BoolVar(&release.bumpMinor, "bump-minor", false, "Resolve chart version and bump both Nuclio and Chart minor version")
	flag.BoolVar(&release.bumpMajor, "bump-major", false, "Resolve chart version and bump both Nuclio and Chart major version")
	flag.Parse()

	release.logger.InfoWith("Running release",
		"targetVersion", release.targetVersion,
		"currentVersion", release.currentVersion,
		"repositoryOwnerName", release.repositoryOwnerName,
		"repositoryScheme", release.repositoryScheme,
		"developmentBranch", release.developmentBranch,
		"releaseBranch", release.releaseBranch,
		"helmChartsTargetVersion", release.helmChartsTargetVersion,
		"skipPublishHelmCharts", release.skipPublishHelmCharts,
		"skipCreateRelease", release.skipCreateRelease)
	return release.Run()
}

func main() {
	if err := run(); err != nil {
		errors.PrintErrorStack(os.Stderr, err, 10)
		os.Exit(1)
	}
	os.Exit(0)
}
