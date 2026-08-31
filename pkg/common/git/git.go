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

package git

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	cryptossh "golang.org/x/crypto/ssh"
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
	var err error

	// resolve full git reference name
	referenceName, err = ResolveReference(repositoryURL, attributes)
	if err != nil {
		return errors.Wrap(err, "Failed to resolve git reference")
	}

	// resolve the auth method (SSH public key or HTTP basic auth) when credentials are given
	gitAuth, err := agc.resolveAuthMethod(repositoryURL, attributes)
	if err != nil {
		return errors.Wrap(err, "Failed to resolve git auth method")
	}

	// HACK: if it's Azure Devops repo - clone differently (the normal go-git client doesn't support it yet)
	// TODO: remove when the issue is resolved - https://github.com/go-git/go-git/issues/64
	// Azure Devops cloning shells out to the git CLI and only supports HTTP basic auth
	if isAzureDevopsRepositoryURL(repositoryURL) && !IsSSHRepositoryURL(repositoryURL) {
		basicAuth, ok := gitAuth.(*githttp.BasicAuth)
		if gitAuth != nil && !ok {
			return errors.New("Unexpected git auth method for Azure DevOps repository")
		}
		return agc.cloneFromAzureDevops(outputDir, repositoryURL, referenceName, basicAuth, agc.cmdRunner)
	}

	return agc.clone(outputDir, repositoryURL, referenceName, gitAuth, attributes)
}

func (agc *AbstractClient) clone(outputDir string,
	repositoryURL string,
	referenceName string,
	gitAuth transport.AuthMethod,
	attributes *Attributes) error {

	agc.logger.DebugWith("Cloning",
		"outputDir", outputDir,
		"referenceName", referenceName,
		"repositoryURL", repositoryURL)

	if _, err := git.PlainClone(outputDir, false, agc.buildCloneOptions(repositoryURL, referenceName, gitAuth, attributes)); err != nil {
		return errors.Wrap(err, "Failed to clone git repository")
	}

	agc.logCurrentCommitSHA(outputDir, repositoryURL, referenceName)

	return nil
}

// buildCloneOptions is split out from clone() so the mapping from Attributes onto
// git.CloneOptions -- in particular the mutual-TLS fields -- can be unit-tested without
// actually cloning a repository.
func (agc *AbstractClient) buildCloneOptions(repositoryURL string,
	referenceName string,
	gitAuth transport.AuthMethod,
	attributes *Attributes) *git.CloneOptions {

	return &git.CloneOptions{
		URL:           repositoryURL,
		ReferenceName: plumbing.ReferenceName(referenceName),
		Depth:         1,
		Auth:          gitAuth,

		// mutual TLS: a client cert/key to present to the server, and/or an additional CA
		// bundle to verify the server's certificate against (together with the system pool).
		ClientCert: []byte(attributes.ClientCert),
		ClientKey:  []byte(attributes.ClientKey),
		CABundle:   []byte(attributes.CABundle),
	}
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
		prefix := splitFunctionPath[0]
		projectPath := splitFunctionPath[1]

		// when getting a git URL from azure, the project name might appear in the URL, so we need to remove it
		// as we comprise the URL with the credentials instead.
		if strings.Contains(splitFunctionPath[1], "@") {
			splitProjectPath := strings.Split(splitFunctionPath[1], "@")
			projectPath = splitProjectPath[1]
		}

		repositoryURL = fmt.Sprintf("%s://%s:%s@%s",
			prefix,
			gitAuth.Username,
			gitAuth.Password,
			projectPath)

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

// resolveAuthMethod picks the git auth method based on the given attributes and repository URL.
// SSH public key auth is used when the repository URL is an SSH URL; otherwise HTTP basic auth is
// used (username/password or a personal access token). SSH credentials with an HTTP URL are rejected
// rather than silently selecting an auth method that cannot work with that URL.
func (agc *AbstractClient) resolveAuthMethod(repositoryURL string, attributes *Attributes) (transport.AuthMethod, error) {

	// SSH auth - the repository URL determines the transport.
	if IsSSHRepositoryURL(repositoryURL) {
		return agc.parseSSHAuth(attributes, sshUserFromRepositoryURL(repositoryURL))
	}

	if attributes.SSHPrivateKey != "" || attributes.SSHPassphrase != "" || attributes.SSHKnownHosts != "" {
		return nil, nuclio.NewErrBadRequest("SSH authentication fields require an SSH repository URL")
	}

	// HTTP basic auth - returns nil when no credentials are given (e.g. public repositories)
	if basicAuth := agc.parseCredentials(attributes); basicAuth != nil {
		return basicAuth, nil
	}

	return nil, nil
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

// parseSSHAuth builds an SSH public key auth method from the given attributes and configures strict
// host key verification from the supplied known hosts.
func (agc *AbstractClient) parseSSHAuth(attributes *Attributes, username string) (transport.AuthMethod, error) {
	if attributes.SSHPrivateKey == "" {
		return nil, nuclio.NewErrBadRequest("An SSH private key must be provided for SSH-based git authentication")
	}

	publicKeys, err := gitssh.NewPublicKeys(username, []byte(attributes.SSHPrivateKey), attributes.SSHPassphrase)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create SSH public key auth method")
	}

	// resolve host key verification
	hostKeyCallback, err := agc.resolveHostKeyCallback(attributes)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to resolve SSH host key callback")
	}
	publicKeys.HostKeyCallback = hostKeyCallback

	return publicKeys, nil
}

// resolveHostKeyCallback returns a host key verification callback using the caller-provided known
// hosts. SSH clones require known hosts so that public-key authentication is not exposed to a
// man-in-the-middle server.
func (agc *AbstractClient) resolveHostKeyCallback(attributes *Attributes) (cryptossh.HostKeyCallback, error) {
	if attributes.SSHKnownHosts == "" {
		return nil, nuclio.NewErrBadRequest("SSH known hosts must be provided for SSH-based git authentication")
	}

	knownHostsPath, err := writeTempKnownHostsFile(attributes.SSHKnownHosts)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to write known hosts file")
	}

	hostKeyCallback, err := gitssh.NewKnownHostsCallback(knownHostsPath)
	if removeErr := os.Remove(knownHostsPath); removeErr != nil {
		agc.logger.WarnWith("Failed to remove temporary known hosts file", "path", knownHostsPath, "err", removeErr)
	}
	if err != nil {
		return nil, err
	}

	return hostKeyCallback, nil
}

// writeTempKnownHostsFile writes the given known_hosts contents to a temporary file and returns its
// path. go-git's known hosts callback reads from a file, so the in-memory contents must be persisted.
func writeTempKnownHostsFile(knownHostsContents string) (string, error) {
	tempFile, err := os.CreateTemp("", "nuclio-git-known-hosts-*")
	if err != nil {
		return "", errors.Wrap(err, "Failed to create temp known hosts file")
	}
	defer tempFile.Close() // nolint: errcheck

	if _, err := tempFile.WriteString(knownHostsContents); err != nil {
		return "", errors.Wrap(err, "Failed to write known hosts contents")
	}

	return tempFile.Name(), nil
}

// IsSSHRepositoryURL reports whether the repository URL uses SSH, either via an ssh:// scheme or the
// scp-like syntax (e.g. git@github.com:org/repo.git).
func IsSSHRepositoryURL(repositoryURL string) bool {
	repositoryURL = strings.TrimSpace(repositoryURL)
	if strings.HasPrefix(strings.ToLower(repositoryURL), "ssh://") {
		return true
	}

	// scp-like syntax: user@host:path, with no explicit scheme
	return !strings.Contains(repositoryURL, "://") &&
		strings.Contains(repositoryURL, "@") &&
		strings.Contains(repositoryURL, ":")
}

// sshUserFromRepositoryURL returns the SSH user from the repository URL. Git hosting providers
// conventionally use "git", which is also the fallback for URLs without an explicit user.
func sshUserFromRepositoryURL(repositoryURL string) string {
	repositoryURL = strings.TrimSpace(repositoryURL)
	if strings.HasPrefix(strings.ToLower(repositoryURL), "ssh://") {
		if parsedURL, err := url.Parse(repositoryURL); err == nil && parsedURL.User != nil && parsedURL.User.Username() != "" {
			return parsedURL.User.Username()
		}
	}

	if atIndex := strings.IndexByte(repositoryURL, '@'); atIndex > 0 {
		return repositoryURL[:atIndex]
	}

	return "git"
}

func ResolveReference(repositoryURL string, attributes *Attributes) (string, error) {
	addReferencePrefix := !isAzureDevopsRepositoryURL(repositoryURL) || IsSSHRepositoryURL(repositoryURL)

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
