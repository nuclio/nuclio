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

package git

import (
	"fmt"
	"strings"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

type Client interface {
	Clone(outputDir, repositoryURL string, attributes *Attributes) error
}

type AbstractClient struct {
	Client

	logger    logger.Logger
	cmdRunner cmdrunner.CmdRunner
}

func NewClient(parentLogger logger.Logger) (Client, error) {
	var err error

	abstractClient := AbstractClient{logger: parentLogger.GetChild("git-client")}

	// create cmd runner
	abstractClient.cmdRunner, err = cmdrunner.NewShellRunner(parentLogger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create cmd runner")
	}

	return &abstractClient, nil
}

func (agc *AbstractClient) Clone(outputDir, repositoryURL string, attributes *Attributes) error {
	var referenceName string
	var gitAuth *githttp.BasicAuth
	var err error

	// resolve full git reference name
	referenceName, err = ResolveReference(repositoryURL, attributes)
	if err != nil {
		return errors.Wrap(err, "Failed to resolve git reference")
	}

	// resolve git credentials when given
	gitAuth = agc.parseCredentials(attributes)

	// HACK: if it's Azure Devops repo - clone differently (the normal go-git client doesn't support it yet)
	// TODO: remove when the issue is resolved - https://github.com/go-git/go-git/issues/64
	if isAzureDevopsRepositoryURL(repositoryURL) {
		return agc.cloneFromAzureDevops(outputDir, repositoryURL, referenceName, gitAuth, agc.cmdRunner)
	}

	return agc.clone(outputDir, repositoryURL, referenceName, gitAuth)
}

func (agc *AbstractClient) clone(outputDir string,
	repositoryURL string,
	referenceName string,
	gitAuth transport.AuthMethod) error {

	agc.logger.DebugWith("Cloning",
		"outputDir", outputDir,
		"referenceName", referenceName,
		"repositoryURL", repositoryURL)

	if _, err := git.PlainClone(outputDir, false, &git.CloneOptions{
		URL:           repositoryURL,
		ReferenceName: plumbing.ReferenceName(referenceName),
		Depth:         1,
		Auth:          gitAuth,
	}); err != nil {
		return errors.Wrap(err, "Failed to clone git repository")
	}

	agc.logCurrentCommitSHA(outputDir, repositoryURL, referenceName)

	return nil
}

func (agc *AbstractClient) cloneFromAzureDevops(outputDir string,
	repositoryURL string,
	referenceName string,
	gitAuth *githttp.BasicAuth,
	cmdRunner cmdrunner.CmdRunner) error {

	agc.logger.DebugWith("Cloning from azure devops",
		"outputDir", outputDir,
		"referenceName", referenceName,
		"repositoryURL", repositoryURL)

	var runOptions *cmdrunner.RunOptions

	// compile repository URL with git auth credentials
	if gitAuth != nil {
		splitFunctionPath := strings.Split(repositoryURL, "://")
		repositoryURL = fmt.Sprintf("%s://%s:%s@%s",
			splitFunctionPath[0],
			gitAuth.Username,
			gitAuth.Password,
			splitFunctionPath[1])

		// redact username and password (so it won't be logged)
		runOptions = &cmdrunner.RunOptions{
			LogRedactions: []string{gitAuth.Username, gitAuth.Password},
		}
	}

	// generate a git clone command
	cloneCommand := fmt.Sprintf("git clone %s --depth 1 -q %s",
		common.Quote(repositoryURL),
		common.Quote(outputDir))

	// attach git reference name when given (use -b as it works both for branch/tag)
	if referenceName != "" {
		cloneCommand = fmt.Sprintf("%s -b %s", cloneCommand, common.Quote(referenceName))
	}

	// run the above git clone command
	res, err := cmdRunner.Run(runOptions, cloneCommand)
	if err != nil {
		return errors.Wrap(err, "Failed to run clone command on azure repository")
	}

	if res.ExitCode != 0 {
		return errors.Errorf("Failed to clone azure devops git repository. Reason: %s", res.Output)
	}

	agc.logCurrentCommitSHA(outputDir, repositoryURL, referenceName)
	return nil
}

func (agc *AbstractClient) logCurrentCommitSHA(gitDir, repositoryURL, referenceName string) {
	res, err := agc.cmdRunner.Run(nil, fmt.Sprintf("cd %s;git rev-parse HEAD", common.Quote(gitDir)))
	if err != nil || res.ExitCode != 0 {
		agc.logger.WarnWith("Failed to get commit SHA", "err", err)
		return
	}
	if res.ExitCode != 0 {
		agc.logger.WarnWith("Failed to get commit SHA (non-zero exit code)", "output", res.Output)
		return
	}

	// remove automatic new line from end of res.Output
	commitSHA := strings.TrimSuffix(res.Output, "\n")

	agc.logger.DebugWith("Current commit SHA",
		"repositoryURL", repositoryURL,
		"referenceName", referenceName,
		"commitSHA", commitSHA)
}

func (agc *AbstractClient) parseCredentials(attributes *Attributes) *githttp.BasicAuth {
	username := attributes.Username
	password := attributes.Password

	if username != "" || password != "" {

		// username must not be empty when password is given (doesn't matter what's the user as long as it's not empty)
		if username == "" {
			username = "defaultuser"
		}

		return &githttp.BasicAuth{
			Username: username,
			Password: password,
		}
	}

	return nil
}

func ResolveReference(repositoryURL string, attributes *Attributes) (string, error) {
	addReferencePrefix := !isAzureDevopsRepositoryURL(repositoryURL)

	// branch
	if ref := attributes.Branch; ref != "" {
		if addReferencePrefix {
			ref = fmt.Sprintf("refs/heads/%s", ref)
		}
		return ref, nil
	}

	// tag
	if ref := attributes.Tag; ref != "" {
		if addReferencePrefix {
			ref = fmt.Sprintf("refs/tags/%s", ref)
		}
		return ref, nil
	}

	// reference
	if ref := attributes.Reference; ref != "" {
		return ref, nil
	}

	return "", nuclio.NewErrBadRequest("No git reference was specified. (must specify branch/tag/reference)")
}

func isAzureDevopsRepositoryURL(repositoryURL string) bool {
	return strings.Contains(repositoryURL, "dev.azure.com")
}
