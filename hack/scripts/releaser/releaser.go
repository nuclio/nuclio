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
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"

	"github.com/coreos/go-semver/semver"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"gopkg.in/yaml.v2"
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
	githubToken             string
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

	githubWorkflowID string
	helmChartConfig  helmChart
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
	if err := r.validateGithubCredentials(); err != nil {
		return errors.Wrap(err, "Failed to validate github credentials")
	}

	// do the cloning, fetch tags, etc
	if err := r.prepareRepository(); err != nil {
		return errors.Wrap(err, "Failed to ensure repository")
	}

	// merge development branch onto the release branch (e.g. development -> master)
	if err := r.mergeAndPush(r.releaseBranch, r.developmentBranch); err != nil {
		return errors.Wrap(err, "Failed to sync release and development branches")
	}

	// read the helm chart and populate its values for target version determination
	if err := r.populateHelmChartConfig(); err != nil {
		return errors.Wrap(err, "Failed to populate helm chart config")
	}

	// set current and target versions
	if err := r.populateCurrentAndTargetVersions(); err != nil {
		return errors.Wrap(err, "Failed to populate current and target versions")
	}

	// nuclio services release (github release, images, binaries, etc)
	if !r.skipCreateRelease {
		if err := r.runAndRetrySkipIfFailed(r.createRelease,
			"Waiting for release creation has failed"); err != nil {
			return errors.Wrap(err, "Failed to wait for release creation")
		}

		if err := r.runAndRetrySkipIfFailed(r.waitForReleaseCompleteness,
			"Waiting for release completeness has failed"); err != nil {
			return errors.Wrap(err, "Failed to wait for release completion")
		}
	} else {
		r.logger.Info("Skipping release creation")
	}

	// helm chart release
	if !r.skipBumpHelmChart {
		if err := r.bumpHelmChartVersion(); err != nil {
			return errors.Wrap(err, "Failed to bump helm chart version")
		}
	} else {
		r.logger.Info("Skipping bump helm chart")
	}

	r.logger.Info("Release is now done")
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

		if _, err = r.cmdRunner.Run(&cmdrunner.RunOptions{
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
		if _, err := r.cmdRunner.Run(runOptions, `git checkout %s`, branchName); err != nil {
			return errors.Wrap(err, "Failed to ensure branch exists")
		}
	}

	// get all tags
	if _, err := r.cmdRunner.Run(runOptions, `git fetch --tags`); err != nil {
		return errors.Wrap(err, "Failed to fetch tags")
	}

	return nil
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

	r.logger.InfoWith("Successfully populated versions",
		"currentVersion", r.currentVersion,
		"targetVersion", r.targetVersion,
		"helmChartsTargetVersion", r.helmChartsTargetVersion)
	return nil
}

func (r *Release) compileReleaseNotes() (string, error) {
	results, err := r.cmdRunner.Run(&cmdrunner.RunOptions{
		WorkingDir: &r.repositoryDirPath,
	},
		`git log --pretty=format:'%%h %%s' %s...%s`,
		r.releaseBranch,
		r.currentVersion)
	if err != nil {
		return "", errors.Wrap(err, "Failed to describe tags")
	}
	return strings.TrimSpace(results.Output), nil
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
	if _, err := r.cmdRunner.Run(runOptions, `git checkout %s`, branch); err != nil {
		return errors.Wrapf(err, "Failed to checkout to branch %s", branch)
	}

	if _, err := r.cmdRunner.Run(runOptions, `git merge %s`, branchToMerge); err != nil {
		return errors.Wrapf(err, "Failed to merge branch %s", branchToMerge)
	}

	if _, err := r.cmdRunner.Run(runOptions, `git push`); err != nil {
		return errors.Wrapf(err, "Failed to push")
	}

	return nil
}

func (r *Release) getGithubWorkflowsReleaseStatus() (string, error) {
	if err := r.populateReleaseWorkflowID(); err != nil {
		return "", errors.Wrap(err, "Failed to get release workflow id")
	}

	workflowReleaseRunsURL := fmt.Sprintf("%s/actions/workflows/%s/runs?event=release&branch=%s",
		r.compileGithubAPIURL(),
		r.githubWorkflowID,
		r.targetVersion)

	// getting workflow status body
	r.logger.DebugWith("Getting workflow id", "workflowReleaseRunsURL", workflowReleaseRunsURL)
	request, err := r.resolveGithubActionAPIRequest(http.MethodGet, workflowReleaseRunsURL, nil)
	if err != nil {
		return "", err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", errors.Wrap(err, "Failed to perform request")
	}

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", errors.Wrap(err, "Failed to read all response body")
	}

	var workflowRunsResponse struct {
		WorkflowRuns []struct {
			Status     string `json:"status,omitempty"`
			Conclusion string `json:"conclusion,omitempty"`
		} `json:"workflow_runs,omitempty"`
	}
	if err := json.Unmarshal(responseBody, &workflowRunsResponse); err != nil {
		return "", errors.Wrap(err, "Failed to unmarshal workflow runs response")
	}
	if len(workflowRunsResponse.WorkflowRuns) == 0 {
		r.logger.WarnWith("No workflow runs were found",
			"responseBody", responseBody)
		return "", nil
	}

	workflowRun := workflowRunsResponse.WorkflowRuns[0]
	r.logger.DebugWith("Received workflow run",
		"workflowRun", workflowRun)

	status := workflowRun.Status
	conclusion := workflowRun.Conclusion

	// https://developer.github.com/v3/actions/workflow-runs/#parameters-1
	// conclusion is null until status become completed
	// and then it holds whether it completed successfully or not.
	if status == "completed" {
		return conclusion, nil
	}
	return status, nil
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

	type Build struct {
		Branch string `json:"branch,omitempty"`
		State  string `json:"state,omitempty"`
	}
	var BuildsResponse []Build
	if err := json.Unmarshal(responseBody, &BuildsResponse); err != nil {
		return "", errors.Wrap(err, "Failed to unmarshal builds response")
	}
	releaseBuildState := ""
	for _, buildResponse := range BuildsResponse {
		if buildResponse.Branch == r.targetVersion.String() {
			releaseBuildState = buildResponse.State
			break
		}
	}
	return releaseBuildState, nil
}

func (r *Release) compileGithubAPIURL() string {
	return fmt.Sprintf("%s/repos/%s/nuclio", githubAPIURL, r.repositoryOwnerName)
}

func (r *Release) validateGithubCredentials() error {
	if r.githubToken == "" {
		r.logger.Debug("No github token was given")
		return nil
	}

	r.logger.DebugWith("Validating github credentials")

	// get workflows
	workflowsURL := fmt.Sprintf("%s/actions/workflows", r.compileGithubAPIURL())

	// prepare request
	request, err := r.resolveGithubActionAPIRequest(http.MethodGet, workflowsURL, nil)
	if err != nil {
		return err
	}

	// make call
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return errors.Wrap(err, "Failed to make a GET request")
	}
	if response.StatusCode >= 400 {
		return errors.Errorf("Unexpected status '%s'", response.Status)
	}

	r.logger.Info("Github credentials are valid")
	return nil
}

func (r *Release) populateReleaseWorkflowID() error {
	if r.githubWorkflowID != "" {
		return nil
	}

	workflowName := "Release"
	workflowID := ""
	workflowsURL := fmt.Sprintf("%s/actions/workflows", r.compileGithubAPIURL())

	r.logger.DebugWith("Populating release workflow id",
		"workflowName", workflowName,
		"workflowsURL", workflowsURL)

	// prepare request
	request, err := r.resolveGithubActionAPIRequest(http.MethodGet, workflowsURL, nil)
	if err != nil {
		return err
	}

	// make call
	response, err := http.DefaultClient.Do(request)
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
		r.logger.WarnWith("No workflow were found",
			"responseBody", responseBody)
		return errors.New("Failed to find workflow ID")
	}

	r.logger.InfoWith("Found workflow ID", "workflowID", workflowID)
	r.githubWorkflowID = workflowID
	return nil
}

func (r *Release) writeToClipboard(s string) error {
	var err error
	pbcopy := exec.Command("pbcopy")
	stdin, err := pbcopy.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "Failed to open stdin to clipboard")
	}

	if err = pbcopy.Start(); err != nil {
		return errors.Wrap(err, "Failed to start pbcopy")
	}

	if _, err = stdin.Write([]byte(s)); err != nil {
		return errors.Wrap(err, "Failed to write to clipboard")
	}

	if err = stdin.Close(); err != nil {
		return errors.Wrap(err, "Failed to close stdin pipe")
	}
	return pbcopy.Wait()
}

func (r *Release) createRelease() error {
	releaseNotes, err := r.compileReleaseNotes()
	if err != nil {
		return errors.Wrap(err, "Failed to compile release notes")
	}

	switch runtimeName := runtime.GOOS; runtimeName {
	case "darwin":
		if len(releaseNotes) > 2000 {
			r.logger.InfoWith("Release notes is too long, trying to copy to clipboard")
			if err := r.writeToClipboard(releaseNotes); err == nil {
				r.logger.InfoWith(`Successfully copied to clipboard. Paste it to the release notes body on the opened window`)
			} else {
				r.logger.Warn(`Failed to copy to clipboard, printing to log. Manually parse its \\n and copy to opened window)`,
					"releaseNotes", releaseNotes,
					"err", err)
			}

			// empty out
			releaseNotes = ""
		}
		if _, err = r.cmdRunner.Run(nil,
			`open "%s/releases/new?target=%s&tag=%s&title=%s&body=%s"`,
			r.compileRepositoryURL("https"),
			url.QueryEscape(r.releaseBranch),
			url.QueryEscape(r.targetVersion.String()),
			url.QueryEscape(r.targetVersion.String()),
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
			status, err := r.getGithubWorkflowsReleaseStatus()
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
			status, err := r.getGithubWorkflowsReleaseStatus()
			if err != nil {
				r.logger.DebugWith("Get release status returned with an error", "err", err)
				return false
			}

			r.logger.DebugWith("Waiting for release completeness", "status", status)
			if status == "failure" {
				r.logger.Warn(`Release job has failed, checkout its job status from 
https://github.com/nuclio/nuclio/actions?query=workflow%3ARelease
Once re-run, it will catch up here.`)
			}

			// TODO: handle failure/cancelled from here? or let it run as suggested above
			return status == "success"
		})
}

func (r *Release) bumpHelmChartVersion() error {
	runOptions := &cmdrunner.RunOptions{
		WorkingDir: &r.repositoryDirPath,
	}

	// bump & release helm charts is being done from the release branch
	if _, err := r.cmdRunner.Run(runOptions,
		`git checkout %s`,
		r.releaseBranch); err != nil {
		return errors.Wrap(err, "Failed to checkout to release branch")
	}
	for _, chartDir := range r.resolveSupportedChartDirs() {
		if _, err := r.cmdRunner.Run(runOptions,
			`git grep -lF "%s" %s | grep yaml | xargs sed -i '' -e "s/%s/%s/g"`,
			r.helmChartConfig.AppVersion,
			path.Join("hack", chartDir),
			r.helmChartConfig.AppVersion,
			r.targetVersion); err != nil {
			return errors.Wrap(err, "Failed to update target versions")
		}
	}

	// explicitly bump the app version
	if _, err := r.cmdRunner.Run(runOptions,
		`sed -i '' -e "s/^\(appVersion: \).*$/\1%s/g" %s`,
		r.targetVersion,
		r.resolveHelmChartFullPath()); err != nil {
		return errors.Wrap(err, "Failed to write helm chart target version")
	}

	if _, err := r.cmdRunner.Run(runOptions,
		`sed -i '' -e "s/^\(version: \).*$/\1%s/g" %s`,
		r.helmChartsTargetVersion,
		r.resolveHelmChartFullPath()); err != nil {
		return errors.Wrap(err, "Failed to write helm chart target version")
	}

	// commit & push changes
	commitMessage := fmt.Sprintf("Bump to %s", r.targetVersion)
	if _, err := r.cmdRunner.Run(runOptions, `git commit -am "%s"`, commitMessage); err != nil {
		return errors.Wrap(err, "Failed to checkout to release branch")
	}

	if _, err := r.cmdRunner.Run(runOptions, `git push`); err != nil {
		return errors.Wrap(err, "Failed to checkout to release branch")
	}

	if r.releaseBranch != r.developmentBranch {
		if err := r.mergeAndPush(r.developmentBranch, r.releaseBranch); err != nil {
			return errors.Wrap(err, "Failed to sync development and release branches")
		}
	}

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

func (r *Release) runAndRetrySkipIfFailed(funcToRetry func() error, errorMessage string) error {
	for {
		if err := funcToRetry(); err != nil {

			if r.promptForYesNo(fmt.Sprintf("%s. Retry?", errorMessage)) {

				// retry
				r.logger.Debug("Retrying")
				continue
			}

			if r.promptForYesNo(fmt.Sprintf("%s. Skip?", errorMessage)) {

				// do not retry
				r.logger.Debug("Skipping")
				break
			}

			// failure
			return err
		}

		// success
		return nil
	}
	return nil
}

func (r *Release) promptForYesNo(promptMessage string) bool {
	fmt.Printf("%s ([y] Yes [n] No): ", promptMessage)

	userInput, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		panic("Failed to read input from stdin")
	}

	switch normalizedResponse := strings.ToLower(strings.TrimSpace(userInput)); normalizedResponse {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	default:

		parsed, err := strconv.ParseBool(normalizedResponse)
		if err != nil {
			fmt.Printf("Invalid input '%s', retry again", normalizedResponse)
			return r.promptForYesNo(promptMessage)
		}
		return parsed
	}
}

func (r *Release) resolveGithubActionAPIRequest(method, url string, body io.Reader) (*http.Request, error) {
	request, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create new request")
	}
	request.Header.Set("Accept", "application/vnd.github.v3+json")
	if r.githubToken != "" {
		request.Header.Set("Authorization", fmt.Sprintf("token %s", r.githubToken))
	}

	return request, nil
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

func (r *Release) resolveSupportedChartDirs() []string {
	return []string{
		"k8s",
		"gke",
		"aks",
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
	flag.StringVar(&release.githubToken, "github-token", common.GetEnvOrDefaultString("NUCLIO_RELEASER_GITHUB_TOKEN", ""), "A scope-less Github token header to avoid API rate limit")
	flag.StringVar(&release.repositoryOwnerName, "repository-owner-name", "nuclio", "Repository owner name to clone nuclio from (Default: nuclio)")
	flag.StringVar(&release.repositoryScheme, "repository-scheme", "https", "Scheme to use when cloning nuclio repository")
	flag.StringVar(&release.developmentBranch, "development-branch", "development", "Development branch (e.g.: development, 1.3.x")
	flag.StringVar(&release.releaseBranch, "release-branch", "master", "Release branch (e.g.: master, 1.3.x, ...)")
	flag.BoolVar(&release.skipCreateRelease, "skip-create-release", false, "Skip build & release flow (useful when publishing helm charts only)")
	flag.BoolVar(&release.skipBumpHelmChart, "skip-bump-helm-chart", false, "Skip bump helm chart")
	flag.BoolVar(&release.skipPublishHelmCharts, "skip-publish-helm-charts", false, "Whether to skip publishing helm charts")
	flag.BoolVar(&release.bumpPatch, "bump-patch", false, "Resolve chart version and bump both Nuclio and Chart patch version")
	flag.BoolVar(&release.bumpMinor, "bump-minor", false, "Resolve chart version and bump both Nuclio and Chart minor version")
	flag.BoolVar(&release.bumpMajor, "bump-major", false, "Resolve chart version and bump both Nuclio and Chart major version")
	flag.Parse()

	// ensure github token value is redacted
	loggerInstance.GetRedactor().AddRedactions([]string{release.githubToken})

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
