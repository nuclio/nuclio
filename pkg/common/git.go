package common

import (
	"fmt"
	"strings"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/cmdrunner"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	githttp "gopkg.in/src-d/go-git.v4/plumbing/transport/http"
)

type GitAttributes struct {
	Branch    string `json:"branch,omitempty"`
	Tag       string `json:"tag,omitempty"`
	Reference string `json:"reference,omitempty"`
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
}

type GitClient interface {
	Clone(outputDir, repositoryURL string, gitAttributes *GitAttributes) error
}

type AbstractGitClient struct {
	GitClient

	logger    logger.Logger
	cmdRunner cmdrunner.CmdRunner
}

func NewGitClient(parentLogger logger.Logger) (GitClient, error) {
	var err error

	abstractGitClient := AbstractGitClient{logger: parentLogger.GetChild("git-client")}

	// create cmd runner
	abstractGitClient.cmdRunner, err = cmdrunner.NewShellRunner(parentLogger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create cmd runner")
	}

	return abstractGitClient, nil
}

func (agc AbstractGitClient) Clone(outputDir, repositoryURL string, gitAttributes *GitAttributes) error {
	var referenceName string
	var gitAuth *githttp.BasicAuth
	var err error

	// resolve full git reference name
	referenceName, err = ResolveGitReference(repositoryURL, gitAttributes)
	if err != nil {
		return errors.Wrap(err, "Failed to resolve git reference")
	}

	// resolve git credentials when given
	gitAuth = agc.parseFunctionGitCredentials(gitAttributes)

	// if it's Azure Devops repo - clone differently (the normal go-git client doesn't support it yet)
	if isAzureDevopsRepositoryURL(repositoryURL) {
		return agc.cloneFromAzureDevops(outputDir, repositoryURL, referenceName, gitAuth, agc.cmdRunner)
	}

	if _, err = git.PlainClone(outputDir, false, &git.CloneOptions{
		URL:           repositoryURL,
		ReferenceName: plumbing.ReferenceName(referenceName),
		Depth:         1,
		Auth:          gitAuth,
	}); err != nil {
		return errors.Wrap(err, "Failed to clone git repository")
	}

	agc.printCurrentCommitSHA(outputDir, repositoryURL, referenceName)

	return nil
}

func (agc AbstractGitClient) printCurrentCommitSHA(gitDir, referenceName, repositoryURL string) {
	res, err := agc.cmdRunner.Run(nil, fmt.Sprintf("cd %s;git rev-parse HEAD", Quote(gitDir)))
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

	agc.logger.InfoWith("Printing current commit SHA",
		"repositoryURL", repositoryURL,
		"referenceName", referenceName,
		"commitSHA", commitSHA)
}

func (agc AbstractGitClient) cloneFromAzureDevops(outputDir string,
	repositoryURL string,
	referenceName string,
	gitAuth *githttp.BasicAuth,
	cmdRunner cmdrunner.CmdRunner) error {

	// if auth is passed, transplant username:password into repository URL
	if gitAuth != nil {
		splitFunctionPath := strings.Split(repositoryURL, "://")
		repositoryURL = fmt.Sprintf("%s://%s:%s@%s",
			splitFunctionPath[0],
			gitAuth.Username,
			gitAuth.Password,
			splitFunctionPath[1])
	}

	// generate a git clone command
	cloneCommand := fmt.Sprintf("git clone %s --depth 1 -q %s",
		Quote(repositoryURL),
		outputDir)

	// attach git reference name when given (use - as it works both for branch/tag)
	if referenceName != "" {
		cloneCommand = fmt.Sprintf("%s -b %s", cloneCommand, referenceName)
	}

	// run the above git clone command
	res, err := cmdRunner.Run(nil, cloneCommand)
	if err != nil {
		return errors.Wrap(err, "Failed to run clone command on azure repository")
	}

	if res.ExitCode != 0 {
		return errors.Errorf("Failed to clone azure devops git repository. Reason: %s", res.Output)
	}

	agc.printCurrentCommitSHA(outputDir, repositoryURL, referenceName)

	return nil
}

func (agc AbstractGitClient) parseFunctionGitCredentials(gitAttributes *GitAttributes) *githttp.BasicAuth {
	username := gitAttributes.Username
	password := gitAttributes.Password

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

func ResolveGitReference(repositoryURL string, gitAttributes *GitAttributes) (string, error) {
	addReferencePrefix := !isAzureDevopsRepositoryURL(repositoryURL)

	// branch
	if ref := gitAttributes.Branch; ref != "" {
		if addReferencePrefix {
			ref = fmt.Sprintf("refs/heads/%s", ref)
		}
		return ref, nil
	}

	// tag
	if ref := gitAttributes.Tag; ref != "" {
		if addReferencePrefix {
			ref = fmt.Sprintf("refs/tags/%s", ref)
		}
		return ref, nil
	}

	// reference
	if ref := gitAttributes.Reference; ref != "" {
		return ref, nil
	}

	return "", errors.New("No git reference was specified. (must specify branch/tag/reference)")
}

func isAzureDevopsRepositoryURL(repositoryURL string) bool {
	return strings.Contains(repositoryURL, "dev.azure.com")
}
