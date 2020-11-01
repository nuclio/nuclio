/*
Copyright 2017 The Nuclio Authors.

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
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"gopkg.in/yaml.v2"
)

const helmChartFilePath = "hack/k8s/helm/nuclio/Chart.yaml"
const githubAPIURL = "https://api.github.com"
const travisAPIURL = "https://api.travis-ci.org"

type helmChart struct {
	Version    string `yaml:"version,omitempty"`
	AppVersion string `yaml:"appVersion,omitempty"`
}

type Release struct {
	currentVersion          string
	targetVersion           string
	helmChartsTargetVersion string
	repositoryDirPath       string
	repositoryOwnerName     string
	repositoryScheme        string
	developmentBranch       string
	releaseBranch           string
	publishHelmCharts       bool
	skipCreateRelease       bool

	logger      logger.Logger
	shellRunner *cmdrunner.ShellRunner

	githubWorkflowID string
	helmChartConfig  helmChart
}

func NewRelease() (*Release, error) {
	var err error
	release := &Release{}
	release.logger, err = nucliozap.NewNuclioZapCmd("releaser", nucliozap.DebugLevel)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create logger")
	}
	release.shellRunner, err = cmdrunner.NewShellRunner(release.logger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create command runner")
	}
	return release, nil
}

func (r *Release) Run() error {
	if err := r.prepareRepository(); err != nil {
		return errors.Wrap(err, "Failed to ensure repository")
	}

	if err := r.mergeAndPush(r.releaseBranch, r.developmentBranch); err != nil {
		return errors.Wrap(err, "Failed to sync release and development branches")
	}

	if err := r.populateCurrentAndTargetVersions(); err != nil {
		return errors.Wrap(err, "Failed to populate current and target versions")
	}

	if !r.skipCreateRelease {
		if err := r.createRelease(); err != nil {
			return errors.Wrap(err, "Failed to create release")
		}

		if err := r.waitForReleaseCompleteness(); err != nil {
			return errors.Wrap(err, "Failed to wait for release")
		}
	} else {
		r.logger.Info("Skipping release creation")
	}

	if err := r.bumpHelmChartVersion(); err != nil {
		return errors.Wrap(err, "Failed to bump helm chart version")
	}

	return nil
}

func (r *Release) compileRepositoryURL(scheme string) string {
	return fmt.Sprintf("%s://github.com/%s/nuclio", scheme, r.repositoryOwnerName)
}

func (r *Release) prepareRepository() error {
	if r.repositoryDirPath == "" {
		r.logger.Debug("Creating a temp dir")

		// create a temp dir & clone to it
		workDir, err := ioutil.TempDir("", "nuclio-releaser-*")
		if err != nil {
			return errors.Wrap(err, "Failed to create work dir")
		}

		repositoryURL := r.compileRepositoryURL(r.repositoryScheme)

		r.logger.DebugWith("Work dir created, cloning...",
			"workDir", workDir,
			"repositoryOwnerName", r.repositoryOwnerName,
			"repositoryURL", repositoryURL)

		if _, err = r.shellRunner.Run(&cmdrunner.RunOptions{
			WorkingDir: &workDir,
		},
			`git clone %s.git .`,
			repositoryURL); err != nil {
			return errors.Wrap(err, "Failed to clone repository")
		}

		r.repositoryDirPath = workDir
		r.logger.DebugWith("Successfully cloned repository",
			"workDir", workDir,
			"repositoryOwnerName", r.repositoryOwnerName,
			"repositoryURL", repositoryURL)
	}

	if !common.IsDir(r.repositoryDirPath) {
		return errors.Errorf("Repository dir path %s is not a directory", r.repositoryDirPath)
	}

	runOptions := &cmdrunner.RunOptions{
		WorkingDir: &r.repositoryDirPath,
	}

	// ensure both development and release branches exists
	for _, branchName := range []string{
		r.developmentBranch,

		// should be last, we release the version from it
		r.releaseBranch,
	} {
		if _, err := r.shellRunner.Run(runOptions, `git checkout %s`, branchName); err != nil {
			return errors.Wrap(err, "Failed to ensure branch exists")
		}
	}

	// get all tags
	if _, err := r.shellRunner.Run(runOptions, `git fetch --tags`); err != nil {
		return errors.Wrap(err, "Failed to fetch tags")
	}

	return r.populateHelmChartConfig()
}

func (r *Release) populateHelmChartConfig() error {

	// read
	yamlFile, err := ioutil.ReadFile(r.resolveHelmChartFullPath())
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
	return r.repositoryDirPath + "/" + helmChartFilePath
}

func (r *Release) populateCurrentAndTargetVersions() error {
	var err error
	runOptions := &cmdrunner.RunOptions{
		WorkingDir: &r.repositoryDirPath,
	}

	if r.currentVersion == "" {

		// nuclio binaries & images version
		results, err := r.shellRunner.Run(runOptions, `git describe --abbrev=0 --tags`)
		if err != nil {
			return errors.Wrap(err, "Failed to describe tags")
		}
		r.currentVersion = strings.TrimSpace(results.Output)
	}

	if r.targetVersion == "" {
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("Target version (Current version: %s, Press enter to continue): ",
			r.currentVersion)
		r.targetVersion, err = reader.ReadString('\n')
		if err != nil {
			return errors.Wrap(err, "Failed to read target version from stdin")
		}
	}
	r.targetVersion = strings.TrimSpace(r.targetVersion)

	if r.helmChartsTargetVersion == "" {
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("Helm chart target version (Current version: %s, Press enter to continue): ",
			r.helmChartConfig.Version)
		r.helmChartsTargetVersion, err = reader.ReadString('\n')
		if err != nil {
			return errors.Wrap(err, "Failed to read helm chart target version from stdin")
		}
	}
	r.helmChartsTargetVersion = strings.TrimSpace(r.helmChartsTargetVersion)

	// sanity
	for _, version := range []string{
		r.currentVersion,
		r.targetVersion,
		r.helmChartsTargetVersion,
	} {
		if version == "" {
			return errors.New("Found an empty version, bailing")
		}
	}
	return nil
}

func (r *Release) compileReleaseNotes() (string, error) {
	results, err := r.shellRunner.Run(&cmdrunner.RunOptions{
		WorkingDir: &r.repositoryDirPath,
	},
		`git log --pretty=format:'%%h %%s' %s...%s`,
		r.releaseBranch,
		r.currentVersion)
	if err != nil {
		return "", errors.Wrap(err, "Failed to describe tags")
	}
	releaseNotes := strings.TrimSpace(results.Output)
	return releaseNotes, nil
}

func (r *Release) mergeAndPush(branch string, branchToMerge string) error {
	if branch == branchToMerge {
		r.logger.InfoWith("Nothing to merge and push when branches are equal",
			"branchToMerge", branchToMerge,
			"branch", branch)
		return nil
	}
	runOptions := &cmdrunner.RunOptions{
		WorkingDir: &r.repositoryDirPath,
	}
	_, err := r.shellRunner.Run(runOptions, `git checkout %s`, branch)
	if err != nil {
		return errors.Wrapf(err, "Failed to checkout to branch %s", branch)
	}

	_, err = r.shellRunner.Run(runOptions, `git merge %s`, branchToMerge)
	if err != nil {
		return errors.Wrapf(err, "Failed to merge branch %s", branchToMerge)
	}

	_, err = r.shellRunner.Run(runOptions, `git push`)
	if err != nil {
		return errors.Wrapf(err, "Failed to push")
	}
	return nil
}

func (r *Release) getReleaseStatus() (string, error) {
	switch r.releaseBranch {
	case "1.1.x", "1.3.x":
		return r.getTravisReleaseStatus()
	default:
		return r.getGithubWorkflowsReleaseStatus()
	}
}

func (r *Release) getGithubWorkflowsReleaseStatus() (string, error) {
	if err := r.populateReleaseWorkflowID(); err != nil {
		return "", errors.Wrap(err, "Failed to get release workflow id")
	}

	workflowReleaseRunsURL := fmt.Sprintf("%s/actions/workflows/%s/runs?event=release&branch=%s",
		r.compileGithubAPIURL(),
		r.githubWorkflowID,
		r.targetVersion)

	// getting workflow id
	response, err := http.Get(workflowReleaseRunsURL)
	if err != nil {
		return "", errors.Wrap(err, "Failed to get workflow runs")
	}
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", errors.Wrap(err, "Failed to read all response body")
	}
	var workflowRunsResponse struct {
		WorkflowRuns []struct {
			Status string `json:"status,omitempty"`
		} `json:"workflow_runs,omitempty"`
	}
	if err := json.Unmarshal(responseBody, &workflowRunsResponse); err != nil {
		return "", errors.Wrap(err, "Failed to unmarshal workflow runs response")
	}
	if len(workflowRunsResponse.WorkflowRuns) == 0 {
		return "", nil
	}
	return workflowRunsResponse.WorkflowRuns[0].Status, nil
}

func (r *Release) getTravisReleaseStatus() (string, error) {
	travisBuildsURL := fmt.Sprintf("%s/repos/nuclio/%s/builds", travisAPIURL, r.repositoryOwnerName)
	request, err := http.NewRequest(http.MethodGet, travisBuildsURL, nil)
	if err != nil {
		return "", errors.Wrap(err, "Failed to create new request")
	}
	request.Header.Set("Content-Type", "application/vnd.travis-ci.2.1+json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", errors.Wrap(err, "Failed to perform request")
	}
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", errors.Wrap(err, "Failed to read all response body")
	}

	var BuildsResponse []struct {
		Build struct {
			Branch string `json:"branch,omitempty"`
			Status string `json:"status,omitempty"`
		} `json:"build,omitempty"`
	}
	if err := json.Unmarshal(responseBody, &BuildsResponse); err != nil {
		return "", errors.Wrap(err, "Failed to unmarshal builds response")
	}
	releaseBuildState := ""
	for _, buildResponse := range BuildsResponse {
		if buildResponse.Build.Branch == r.targetVersion {
			releaseBuildState = buildResponse.Build.Status
		}
	}
	return releaseBuildState, nil
}

func (r *Release) compileGithubAPIURL() string {
	return fmt.Sprintf("%s/repos/%s/nuclio", githubAPIURL, r.repositoryOwnerName)
}

func (r *Release) populateReleaseWorkflowID() error {
	if r.githubWorkflowID != "" {
		return nil
	}
	workflowName := "Release"
	workflowID := ""

	response, err := http.Get(fmt.Sprintf("%s/actions/workflows", r.compileGithubAPIURL()))
	if err != nil {
		return errors.Wrap(err, "Failed to make a GET request")
	}
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return errors.Wrap(err, "Failed to read all response body")
	}

	workflowsResponse := struct {
		Workflows []struct {
			ID   int    `json:"id,omitempty"`
			Name string `json:"name,omitempty"`
		} `json:"workflows,omitempty"`
	}{}
	if err := json.Unmarshal(responseBody, &workflowsResponse); err != nil {
		return errors.Wrap(err, "Failed to unmarshal workflow response")
	}
	for _, workflow := range workflowsResponse.Workflows {
		if workflow.Name == workflowName {
			workflowID = strconv.Itoa(workflow.ID)
			break
		}
	}
	if workflowID == "" {
		return errors.New("Failed to find workflow ID")
	}

	r.githubWorkflowID = workflowID
	return nil
}

func (r *Release) createRelease() error {
	releaseNotes, err := r.compileReleaseNotes()
	if err != nil {
		return errors.Wrap(err, "Failed to compile release notes")
	}

	switch runtimeName := runtime.GOOS; runtimeName {
	case "darwin":
		if len(releaseNotes) > 2000 {
			releaseNotes = releaseNotes[:2000]
		}
		if _, err = r.shellRunner.Run(nil,
			`open "%s/releases/new?target=%s&tag=%s&title=%s&body=%s"`,
			r.compileRepositoryURL("https"),
			url.QueryEscape(r.releaseBranch),
			url.QueryEscape(r.targetVersion),
			url.QueryEscape(r.targetVersion),
			url.QueryEscape(releaseNotes)); err != nil {
			return errors.Wrap(err, "Failed to open release in browser")
		}

	default:

		// TODO: post to github API to create a release in case of linux
		return errors.Errorf("Not supported runtime %s", runtimeName)
	}

	// wait for release job to start
	return common.RetryUntilSuccessful(time.Minute*5,
		time.Second*5,
		func() bool {
			status, err := r.getReleaseStatus()
			if err != nil {
				r.logger.DebugWith("Get release status returned with an error", "err", err)
				return false
			}
			r.logger.DebugWith("Received status", "status", status)
			return status != ""
		})
}

func (r *Release) waitForReleaseCompleteness() error {
	return common.RetryUntilSuccessful(time.Minute*60,
		time.Minute*1,
		func() bool {
			status, err := r.getReleaseStatus()
			if err != nil {
				r.logger.DebugWith("Get release status returned with an error", "err", err)
				return false
			}

			r.logger.DebugWith("Waiting for release completeness", "status", status)
			switch r.releaseBranch {
			case "1.1.x", "1.3.x":
				return status == "finished"
			default:
				return status == "completed"
			}
		})
}

func (r *Release) bumpHelmChartVersion() error {
	runOptions := &cmdrunner.RunOptions{
		WorkingDir: &r.repositoryDirPath,
	}

	// bump & release helm charts is being done from the release branch
	if _, err := r.shellRunner.Run(runOptions,
		`git checkout %s`,
		r.releaseBranch); err != nil {
		return errors.Wrap(err, "Failed to checkout to release branch")
	}
	ChartDirs := []string{
		"k8s",
		"gke",
		"aks",
	}
	for _, chartDir := range ChartDirs {
		if _, err := r.shellRunner.Run(runOptions,
			`git grep -lF "%s" %s | grep yaml | xargs sed -i '' -e "s/%s/%s/g"`,
			r.helmChartConfig.AppVersion,
			path.Join("hack", chartDir),
			r.helmChartConfig.AppVersion,
			r.targetVersion); err != nil {
			return errors.Wrap(err, "Failed to update target versions")
		}
	}

	// explicitly bump the app version
	if _, err := r.shellRunner.Run(runOptions,
		`sed -i '' -e "s/^\(appVersion: \).*$/\1%s/g" %s`,
		r.targetVersion,
		r.resolveHelmChartFullPath()); err != nil {
		return errors.Wrap(err, "Failed to write helm chart target version")
	}

	if _, err := r.shellRunner.Run(runOptions,
		`sed -i '' -e "s/^\(version: \).*$/\1%s/g" %s`,
		r.helmChartsTargetVersion,
		r.resolveHelmChartFullPath()); err != nil {
		return errors.Wrap(err, "Failed to write helm chart target version")
	}

	// commit & push changes
	commitMessage := fmt.Sprintf("Bump to %s", r.targetVersion)
	if _, err := r.shellRunner.Run(runOptions, `git commit -am "%s"`, commitMessage); err != nil {
		return errors.Wrap(err, "Failed to checkout to release branch")
	}

	if _, err := r.shellRunner.Run(runOptions, `git push`); err != nil {
		return errors.Wrap(err, "Failed to checkout to release branch")
	}

	if r.releaseBranch != r.developmentBranch {
		if err := r.mergeAndPush(r.developmentBranch, r.releaseBranch); err != nil {
			return errors.Wrap(err, "Failed to sync development and release branches")
		}
	}

	if r.publishHelmCharts {
		if _, err := r.shellRunner.Run(runOptions,
			`git checkout %s`,
			r.releaseBranch); err != nil {
			return errors.Wrap(err, "Failed to checkout to release branch")
		}
		if _, err := r.shellRunner.Run(&cmdrunner.RunOptions{
			WorkingDir: &r.repositoryDirPath,
		}, `make helm-publish`); err != nil {
			return errors.Wrap(err, "Failed to publish helm charts")
		}
	}
	return nil
}

func run() error {
	release, err := NewRelease()
	if err != nil {
		return errors.Wrap(err, "Failed to create new release")
	}

	flag.StringVar(&release.targetVersion, "target-version", "", "Release target version")
	flag.StringVar(&release.currentVersion, "current-version", "", "Current version")
	flag.StringVar(&release.repositoryOwnerName, "repository-owner-name", "nuclio", "Repository owner name to clone nuclio from (Default: nuclio)")
	flag.StringVar(&release.repositoryScheme, "repository-scheme", "https", "Scheme to use when cloning nuclio repository")
	flag.StringVar(&release.developmentBranch, "development-branch", "development", "Development branch (e.g.: development, 1.3.x")
	flag.StringVar(&release.releaseBranch, "release-branch", "master", "Release branch (e.g.: master, 1.3.x, ...)")
	flag.BoolVar(&release.skipCreateRelease, "skip-create-release", false, "Skip build & release flow (useful when publishing helm charts only)")
	flag.StringVar(&release.helmChartsTargetVersion, "helm-charts-release-version", "", "Helm charts release target version")
	flag.BoolVar(&release.publishHelmCharts, "publish-helm-charts", true, "Whether to publish helm charts")
	flag.Parse()

	release.logger.InfoWith("Running release",
		"targetVersion", release.targetVersion,
		"currentVersion", release.currentVersion,
		"repositoryOwnerName", release.repositoryOwnerName,
		"repositoryScheme", release.repositoryScheme,
		"developmentBranch", release.developmentBranch,
		"releaseBranch", release.releaseBranch,
		"helmChartsTargetVersion", release.helmChartsTargetVersion,
		"publishHelmCharts", release.publishHelmCharts,
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
