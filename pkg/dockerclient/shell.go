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

package dockerclient

import (
	"fmt"
	"path"
	"strings"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/nuclio/nuclio-sdk"
	"k8s.io/apimachinery/pkg/util/json"
)

// ShellClient is a docker client that uses the shell to communicate with docker
type ShellClient struct {
	logger    nuclio.Logger
	cmdRunner cmdrunner.CmdRunner
}

// NewShellClient creates a new docker client
func NewShellClient(parentLogger nuclio.Logger, runner cmdrunner.CmdRunner) (*ShellClient, error) {
	var err error

	newClient := &ShellClient{
		logger:    parentLogger.GetChild("docker"),
		cmdRunner: runner,
	}

	// set cmdrunner
	if newClient.cmdRunner == nil {
		newClient.cmdRunner, err = cmdrunner.NewShellRunner(newClient.logger)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create command runner")
		}
	}

	// verify
	_, err = newClient.cmdRunner.Run(nil, "docker version")
	if err != nil {
		return nil, errors.Wrap(err, "No docker client found")
	}

	return newClient, nil
}

// Build will build a docker image, given build options
func (c *ShellClient) Build(buildOptions *BuildOptions) error {
	c.logger.DebugWith("Building image", "image", buildOptions.ImageName)

	// if context dir is not passed, use the dir containing the dockerfile
	if buildOptions.ContextDir == "" && buildOptions.DockerfilePath != "" {
		buildOptions.ContextDir = path.Dir(buildOptions.DockerfilePath)
	}

	// user can only specify context directory
	if buildOptions.DockerfilePath == "" && buildOptions.ContextDir != "" {
		buildOptions.DockerfilePath = path.Join(buildOptions.ContextDir, "Dockerfile")
	}

	cacheOption := ""
	if buildOptions.NoCache {
		cacheOption = "--no-cache"
	}

	_, err := c.runCommand(&cmdrunner.RunOptions{WorkingDir: &buildOptions.ContextDir},
		"docker build --force-rm -t %s -f %s %s .",
		buildOptions.ImageName,
		buildOptions.DockerfilePath,
		cacheOption)

	return err
}

// CopyObjectsFromImage copies objects (files, directories) from a given image to local storage. it does
// this through an intermediate container which is deleted afterwards
func (c *ShellClient) CopyObjectsFromImage(imageName string, objectsToCopy map[string]string, allowCopyErrors bool) error {
	runResult, err := c.runCommand(nil, "docker create %s", imageName)
	if err != nil {
		return errors.Wrapf(err, "Failed to create container from %s", imageName)
	}

	containerID := runResult.Output
	containerID = strings.TrimSpace(containerID)
	defer func() {
		c.runCommand(nil, "docker rm -f %s", containerID)
	}()

	for objectImagePath, objectLocalPath := range objectsToCopy {
		_, err = c.runCommand(nil, "docker cp %s:%s %s", containerID, objectImagePath, objectLocalPath)
		if err != nil && !allowCopyErrors {
			return errors.Wrapf(err, "Can't copy %s:%s -> %s", containerID, objectImagePath, objectLocalPath)
		}
	}

	return nil
}

// PushImage pushes a local image to a remote docker repository
func (c *ShellClient) PushImage(imageName string, registryURL string) error {
	taggedImageName := registryURL + "/" + imageName

	c.logger.InfoWith("Pushing image", "from", imageName, "to", taggedImageName)

	_, err := c.runCommand(nil, "docker tag %s %s", imageName, taggedImageName)
	if err != nil {
		return errors.Wrap(err, "Failed to tag image")
	}

	_, err = c.runCommand(nil, "docker push %s", taggedImageName)
	if err != nil {
		return errors.Wrap(err, "Failed to push image")
	}

	return nil
}

// PullImage pulls an image from a remote docker repository
func (c *ShellClient) PullImage(imageURL string) error {
	c.logger.InfoWith("Pulling base image", "name", imageURL)

	_, err := c.runCommand(nil, "docker pull %s", imageURL)
	return err
}

// RemoveImage will remove (delete) a local image
func (c *ShellClient) RemoveImage(imageName string) error {
	_, err := c.runCommand(nil, "docker rmi -f %s", imageName)
	return err
}

// RunContainer will run a container based on an image and run options
func (c *ShellClient) RunContainer(imageName string, runOptions *RunOptions) (string, error) {
	portsArgument := ""

	for localPort, dockerPort := range runOptions.Ports {
		portsArgument += fmt.Sprintf("-p %d:%d ", localPort, dockerPort)
	}

	nameArgument := ""
	if runOptions.ContainerName != "" {
		nameArgument = fmt.Sprintf("--name %s", runOptions.ContainerName)
	}

	netArgument := ""
	if runOptions.NetworkType != "" {
		nameArgument = fmt.Sprintf("--net %s", runOptions.NetworkType)
	}

	labelArgument := ""
	if runOptions.Labels != nil {
		for labelName, labelValue := range runOptions.Labels {
			labelArgument += fmt.Sprintf("--label %s='%s' ", labelName, labelValue)
		}
	}

	envArgument := ""
	if runOptions.Env != nil {
		for envName, envValue := range runOptions.Env {
			labelArgument += fmt.Sprintf("--env %s=%s ", envName, envValue)
		}
	}

	volumeArgument := ""
	if runOptions.Volumes != nil {
		for volumeHostPath, volumeContainerPath := range runOptions.Volumes {
			volumeArgument += fmt.Sprintf("--volume %s:%s ", volumeHostPath, volumeContainerPath)
		}
	}

	runResult, err := c.cmdRunner.Run(nil,
		"docker run -d %s %s %s %s %s %s %s",
		portsArgument,
		nameArgument,
		netArgument,
		labelArgument,
		envArgument,
		volumeArgument,
		imageName)

	if err != nil {
		c.logger.WarnWith("Failed to run container",
			"err", err,
			"stdout", runResult.Output,
			"stderr", runResult.Stderr)

		return "", err
	}

	stdoutLines := strings.Split(runResult.Output, "\n")
	lastStdoutLine := c.getLastNonEmptyLine(stdoutLines)

	// make sure there are no spaces in the ID
	if strings.Contains(lastStdoutLine, " ") {
		return "", fmt.Errorf("Output from docker command includes more than just ID: %s", lastStdoutLine)
	}

	return lastStdoutLine, err
}

// RemoveContainer removes a container given a container ID
func (c *ShellClient) RemoveContainer(containerID string) error {
	_, err := c.runCommand(nil, "docker rm -f %s", containerID)
	return err
}

// GetContainerLogs returns raw logs from a given container ID
// Concatenating stdout and stderr since there's no way to re-interlace them
func (c *ShellClient) GetContainerLogs(containerID string) (string, error) {
	runResult, err := c.runCommand(nil, "docker logs %s", containerID)
	return runResult.Output, err
}

// GetContainers returns a list of container IDs which match a certain criteria
func (c *ShellClient) GetContainers(options *GetContainerOptions) ([]Container, error) {
	c.logger.DebugWith("Getting containers", "options", options)

	filterArgument := ""
	for labelName, labelValue := range options.Labels {
		filterArgument += fmt.Sprintf(`--filter "label=%s=%s" `, labelName, labelValue)
	}

	runResult, err := c.runCommand(nil, "docker ps -q %s", filterArgument)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get containers")
	}

	containerIDsAsString := runResult.Output
	if len(containerIDsAsString) == 0 {
		return []Container{}, nil
	}

	runResult, err = c.runCommand(nil,
		"docker inspect %s",
		strings.Replace(containerIDsAsString, "\n", " ", -1))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to inspect containers")
	}

	containersInfoString := runResult.Output

	var containersInfo []Container

	// parse the result
	if err := json.Unmarshal([]byte(containersInfoString), &containersInfo); err != nil {
		return nil, errors.Wrap(err, "Failed to parse inspect response")
	}

	return containersInfo, nil
}

// LogIn allows docker client to access secured registries
func (c *ShellClient) LogIn(options *LogInOptions) error {
	_, err := c.runCommand(nil, `docker login -u %s -p '%s' %s`,
		options.Username,
		options.Password,
		options.URL)

	return err
}

func (c *ShellClient) runCommand(runOptions *cmdrunner.RunOptions, format string, vars ...interface{}) (cmdrunner.RunResult, error) {

	// if user
	if runOptions == nil {
		runOptions = &cmdrunner.RunOptions{}
	}

	// make sure output mode is that stdout and stderr are two different streams (don't combine)
	runOptions.CaptureOutputMode = cmdrunner.CaptureOutputModeStdout

	runResult, err := c.cmdRunner.Run(runOptions, format, vars...)

	if runResult.Stderr != "" {
		c.logger.WarnWith("Docker command outputted to stderr - this may result in errors",
			"cmd", fmt.Sprintf(format, vars),
			"stderr", runResult.Stderr)
	}

	return runResult, err
}

func (c *ShellClient) getLastNonEmptyLine(lines []string) string {

	// iterate backwards over the libes
	for idx := len(lines) - 1; idx >= 0; idx-- {
		if lines[idx] != "" {
			return lines[idx]
		}
	}

	return ""
}
