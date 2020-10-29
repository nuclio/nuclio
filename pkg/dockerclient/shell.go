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
	"os"
	"path"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"k8s.io/apimachinery/pkg/util/json"
)

// ShellClient is a docker client that uses the shell to communicate with docker
type ShellClient struct {
	logger             logger.Logger
	cmdRunner          cmdrunner.CmdRunner
	redactedValues     []string
	buildTimeout       time.Duration
	buildRetryInterval time.Duration
}

// NewShellClient creates a new docker client
func NewShellClient(parentLogger logger.Logger, runner cmdrunner.CmdRunner) (*ShellClient, error) {
	var err error

	newClient := &ShellClient{
		logger:             parentLogger.GetChild("docker"),
		cmdRunner:          runner,
		buildTimeout:       1 * time.Hour,
		buildRetryInterval: 3 * time.Second,
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
	c.logger.DebugWith("Building image", "buildOptions", buildOptions)

	// if context dir is not passed, use the dir containing the dockerfile
	if buildOptions.ContextDir == "" && buildOptions.DockerfilePath != "" {
		buildOptions.ContextDir = path.Dir(buildOptions.DockerfilePath)
	}

	// user can only specify context directory
	if buildOptions.DockerfilePath == "" && buildOptions.ContextDir != "" {
		buildOptions.DockerfilePath = path.Join(buildOptions.ContextDir, "Dockerfile")
	}

	buildArgs := ""
	for buildArgName, buildArgValue := range buildOptions.BuildArgs {
		buildArgs += fmt.Sprintf("--build-arg %s=%s ", buildArgName, buildArgValue)
	}

	cacheOption := ""
	if buildOptions.NoCache {
		cacheOption = "--no-cache"
	}

	return c.build(buildOptions, buildArgs, cacheOption)
}

// CopyObjectsFromImage copies objects (files, directories) from a given image to local storage. it does
// this through an intermediate container which is deleted afterwards
func (c *ShellClient) CopyObjectsFromImage(imageName string,
	objectsToCopy map[string]string,
	allowCopyErrors bool) error {

	// create container from image
	containerID, err := c.createContainer(imageName)
	if err != nil {
		return errors.Wrapf(err, "Failed to create container from %s", imageName)
	}

	// delete once done copying objects
	defer c.runCommand(nil, "docker rm -f %s", containerID) // nolint: errcheck

	// copy objects
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
	taggedImage := common.CompileImageName(registryURL, imageName)

	c.logger.InfoWith("Pushing image", "from", imageName, "to", taggedImage)

	_, err := c.runCommand(nil, "docker tag %s %s", imageName, taggedImage)
	if err != nil {
		return errors.Wrap(err, "Failed to tag image")
	}

	_, err = c.runCommand(nil, "docker push %s", taggedImage)
	if err != nil {
		return errors.Wrap(err, "Failed to push image")
	}

	return nil
}

// PullImage pulls an image from a remote docker repository
func (c *ShellClient) PullImage(imageURL string) error {
	c.logger.InfoWith("Pulling image", "imageName", imageURL)

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
	var dockerArguments []string

	for localPort, dockerPort := range runOptions.Ports {
		if localPort == RunOptionsNoPort {
			dockerArguments = append(dockerArguments, fmt.Sprintf("-p %d", dockerPort))
		} else {
			dockerArguments = append(dockerArguments, fmt.Sprintf("-p %d:%d", localPort, dockerPort))
		}
	}

	if runOptions.RestartPolicy != nil && runOptions.RestartPolicy.Name != RestartPolicyNameNo {

		// sanity check
		// https://docs.docker.com/engine/reference/run/#restart-policies---restart
		// combining --restart (restart policy) with the --rm (clean up) flag results in an error.
		if runOptions.Remove {
			return "", errors.Errorf("Cannot combine restart policy with container removal")
		}
		restartMaxRetries := runOptions.RestartPolicy.MaximumRetryCount
		restartPolicy := fmt.Sprintf("--restart %s", runOptions.RestartPolicy.Name)
		if runOptions.RestartPolicy.Name == RestartPolicyNameOnFailure && restartMaxRetries >= 0 {
			restartPolicy += fmt.Sprintf(":%d", restartMaxRetries)
		}
		dockerArguments = append(dockerArguments, restartPolicy)
	}

	if !runOptions.Attach {
		dockerArguments = append(dockerArguments, "-d")
	}

	if runOptions.GPUs != "" {
		dockerArguments = append(dockerArguments, fmt.Sprintf("--gpus %s", runOptions.GPUs))
	}

	if runOptions.Remove {
		dockerArguments = append(dockerArguments, "--rm")
	}

	if runOptions.ContainerName != "" {
		dockerArguments = append(dockerArguments, fmt.Sprintf("--name %s", runOptions.ContainerName))
	}

	if runOptions.Network != "" {
		dockerArguments = append(dockerArguments, fmt.Sprintf("--net %s", runOptions.Network))
	}

	if runOptions.Labels != nil {
		for labelName, labelValue := range runOptions.Labels {
			dockerArguments = append(dockerArguments,
				fmt.Sprintf("--label %s='%s'", labelName, c.replaceSingleQuotes(labelValue)))
		}
	}

	if runOptions.Env != nil {
		for envName, envValue := range runOptions.Env {
			dockerArguments = append(dockerArguments, fmt.Sprintf("--env %s='%s'", envName, envValue))
		}
	}

	if runOptions.Volumes != nil {
		for volumeHostPath, volumeContainerPath := range runOptions.Volumes {
			dockerArguments = append(dockerArguments,
				fmt.Sprintf("--volume %s:%s ", volumeHostPath, volumeContainerPath))
		}
	}

	if len(runOptions.MountPoints) > 0 {
		for _, mountPoint := range runOptions.MountPoints {
			readonly := ""
			if !mountPoint.RW {
				readonly = ",readonly"
			}
			dockerArguments = append(dockerArguments,
				fmt.Sprintf("--mount source=%s,destination=%s%s",
					mountPoint.Source,
					mountPoint.Destination,
					readonly))
		}
	}

	if runOptions.RunAsUser != nil || runOptions.RunAsGroup != nil {
		userStr := ""
		if runOptions.RunAsUser != nil {
			userStr += fmt.Sprintf("%d", *runOptions.RunAsUser)
		}
		if runOptions.RunAsGroup != nil {
			userStr += fmt.Sprintf(":%d", *runOptions.RunAsGroup)
		}

		dockerArguments = append(dockerArguments, fmt.Sprintf("--user %s", userStr))
	}

	if runOptions.FSGroup != nil {
		dockerArguments = append(dockerArguments, fmt.Sprintf("--group-add %d", *runOptions.FSGroup))
	}

	runResult, err := c.cmdRunner.Run(
		&cmdrunner.RunOptions{LogRedactions: c.redactedValues},
		"docker run %s %s %s",
		strings.Join(dockerArguments, " "),
		imageName,
		runOptions.Command)

	if err != nil {
		c.logger.WarnWith("Failed to run container",
			"err", err,
			"stdout", runResult.Output,
			"stderr", runResult.Stderr)

		return "", err
	}

	// if user requested, set stdout / stderr
	if runOptions.Stdout != nil {
		*runOptions.Stdout = runResult.Output
	}

	if runOptions.Stderr != nil {
		*runOptions.Stderr = runResult.Stderr
	}

	stdoutLines := strings.Split(runResult.Output, "\n")
	lastStdoutLine := c.getLastNonEmptyLine(stdoutLines, 0)

	// make sure there are no spaces in the ID, as normally we expect this command to only produce container ID
	if strings.Contains(lastStdoutLine, " ") {

		// if the image didn't exist prior to calling RunContainer, it will be pulled implicitly which will
		// cause additional information to be outputted. if runOptions.ImageMayNotExist is false,
		// this will result in an error.
		if !runOptions.ImageMayNotExist {
			return "", fmt.Errorf("Output from docker command includes more than just ID: %s", lastStdoutLine)
		}

		// if the implicit image pull was allowed and actually happened, the container ID will appear in the
		// second to last line ¯\_(ツ)_/¯
		lastStdoutLine = c.getLastNonEmptyLine(stdoutLines, 1)
	}

	return lastStdoutLine, err
}

// ExecInContainer will run a command in a container
func (c *ShellClient) ExecInContainer(containerID string, execOptions *ExecOptions) error {

	envArgument := ""
	if execOptions.Env != nil {
		for envName, envValue := range execOptions.Env {
			envArgument += fmt.Sprintf("--env %s='%s' ", envName, envValue)
		}
	}

	runResult, err := c.cmdRunner.Run(
		&cmdrunner.RunOptions{LogRedactions: c.redactedValues},
		"docker exec %s %s %s",
		envArgument,
		containerID,
		execOptions.Command)

	if err != nil {
		c.logger.DebugWith("Failed to execute command in container",
			"err", err,
			"stdout", runResult.Output,
			"stderr", runResult.Stderr)

		return err
	}

	// if user requested, set stdout / stderr
	if execOptions.Stdout != nil {
		*execOptions.Stdout = runResult.Output
	}

	if execOptions.Stderr != nil {
		*execOptions.Stderr = runResult.Stderr
	}

	return nil
}

// RemoveContainer removes a container given a container ID
func (c *ShellClient) RemoveContainer(containerID string) error {
	_, err := c.runCommand(nil, "docker rm -f %s", containerID)
	return err
}

// StopContainer stops a container given a container ID
func (c *ShellClient) StopContainer(containerID string) error {
	_, err := c.runCommand(nil, "docker stop %s", containerID)
	return err
}

// StartContainer stops a container given a container ID
func (c *ShellClient) StartContainer(containerID string) error {
	_, err := c.runCommand(nil, "docker start %s", containerID)
	return err
}

// GetContainerLogs returns raw logs from a given container ID
// Concatenating stdout and stderr since there's no way to re-interlace them
func (c *ShellClient) GetContainerLogs(containerID string) (string, error) {
	runOptions := &cmdrunner.RunOptions{
		CaptureOutputMode: cmdrunner.CaptureOutputModeCombined,
	}

	runResult, err := c.runCommand(runOptions, "docker logs %s", containerID)
	return runResult.Output, err
}

// AwaitContainerHealth blocks until the given container is healthy or the timeout passes
func (c *ShellClient) AwaitContainerHealth(containerID string, timeout *time.Duration) error {
	timedOut := false

	containerHealthy := make(chan error, 1)
	var timeoutChan <-chan time.Time

	// if no timeout is given, create a channel that we'll never send on
	if timeout == nil {
		timeoutChan = make(<-chan time.Time, 1)
	} else {
		timeoutChan = time.After(*timeout)
	}

	go func() {

		// start with a small interval between health checks, increasing it gradually
		inspectInterval := 100 * time.Millisecond

		for !timedOut {
			containers, err := c.GetContainers(&GetContainerOptions{
				ID:      containerID,
				Stopped: true,
			})
			if err == nil && len(containers) > 0 {
				container := containers[0]

				// container is healthy
				if container.State.Health.Status == "healthy" {
					containerHealthy <- nil
					return
				}

				// container exited, bail out
				if container.State.Status == "exited" {
					containerHealthy <- errors.Errorf("Container exited with status: %d", container.State.ExitCode)
					return
				}

				// container is dead, bail out
				// https://docs.docker.com/engine/reference/commandline/ps/#filtering
				if container.State.Status == "dead" {
					containerHealthy <- errors.New("Container seems to be dead")
					return
				}

				// wait a bit before retrying
				c.logger.DebugWith("Container not healthy yet, retrying soon",
					"timeout", timeout,
					"containerID", containerID,
					"containerState", container.State,
					"nextCheckIn", inspectInterval)
			}

			time.Sleep(inspectInterval)

			// increase the interval up to a cap
			if inspectInterval < 800*time.Millisecond {
				inspectInterval *= 2
			}
		}
	}()

	// wait for either the container to be healthy or the timeout
	select {
	case err := <-containerHealthy:
		if err != nil {
			return errors.Wrapf(err, "Container %s is not healthy", containerID)
		}
		c.logger.DebugWith("Container is healthy", "containerID", containerID)
	case <-timeoutChan:
		timedOut = true

		containerLogs, err := c.GetContainerLogs(containerID)
		if err != nil {
			c.logger.ErrorWith("Container wasn't healthy within timeout (failed to get logs)",
				"containerID", containerID,
				"timeout", timeout,
				"err", err)
		} else {
			c.logger.WarnWith("Container wasn't healthy within timeout",
				"containerID", containerID,
				"timeout", timeout,
				"logs", containerLogs)
		}

		return errors.New("Container wasn't healthy in time")
	}

	return nil
}

// GetContainers returns a list of container IDs which match a certain criteria
func (c *ShellClient) GetContainers(options *GetContainerOptions) ([]Container, error) {
	c.logger.DebugWith("Getting containers", "options", options)

	stoppedContainersArgument := ""
	if options.Stopped {
		stoppedContainersArgument = "--all "
	}

	nameFilterArgument := ""
	if options.Name != "" {
		nameFilterArgument = fmt.Sprintf(`--filter "name=^/%s$" `, options.Name)
	}

	idFilterArgument := ""
	if options.ID != "" {
		idFilterArgument = fmt.Sprintf(`--filter "id=%s"`, options.ID)
	}

	labelFilterArgument := ""
	for labelName, labelValue := range options.Labels {
		labelFilterArgument += fmt.Sprintf(`--filter "label=%s=%s" `,
			labelName,
			labelValue)
	}

	runResult, err := c.runCommand(nil,
		"docker ps --quiet %s %s %s %s",
		stoppedContainersArgument,
		idFilterArgument,
		nameFilterArgument,
		labelFilterArgument)

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

// GetContainerEvents returns a list of container events which occurred within a time range
func (c *ShellClient) GetContainerEvents(containerName string, since string, until string) ([]string, error) {
	runResults, err := c.runCommand(nil, "docker events --filter container=%s --since %s --until %s",
		containerName,
		since,
		until)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get container events")
	}
	return strings.Split(strings.TrimSpace(runResults.Output), "\n"), nil
}

// LogIn allows docker client to access secured registries
func (c *ShellClient) LogIn(options *LogInOptions) error {
	c.redactedValues = append(c.redactedValues, options.Password)

	_, err := c.runCommand(nil, `docker login -u %s -p '%s' %s`,
		options.Username,
		options.Password,
		options.URL)

	return err
}

// CreateNetwork creates a docker network
func (c *ShellClient) CreateNetwork(options *CreateNetworkOptions) error {
	_, err := c.runCommand(nil, `docker network create %s`, options.Name)

	return err
}

// DeleteNetwork deletes a docker network
func (c *ShellClient) DeleteNetwork(networkName string) error {
	_, err := c.runCommand(nil, `docker network rm %s`, networkName)

	return err
}

// CreateVolume creates a docker volume
func (c *ShellClient) CreateVolume(options *CreateVolumeOptions) error {
	_, err := c.runCommand(nil, `docker volume create %s`, options.Name)

	return err
}

// DeleteVolume deletes a docker volume
func (c *ShellClient) DeleteVolume(volumeName string) error {
	_, err := c.runCommand(nil, `docker volume rm %s`, volumeName)

	return err
}

func (c *ShellClient) Save(imageName string, outPath string) error {
	_, err := c.runCommand(nil, `docker save --output %s %s`, outPath, imageName)

	return err
}

func (c *ShellClient) Load(inPath string) error {
	_, err := c.runCommand(nil, `docker load --input %s`, inPath)

	return err
}

func (c *ShellClient) runCommand(runOptions *cmdrunner.RunOptions, format string, vars ...interface{}) (cmdrunner.RunResult, error) {

	// if user
	if runOptions == nil {
		runOptions = &cmdrunner.RunOptions{
			CaptureOutputMode: cmdrunner.CaptureOutputModeStdout,
		}
	}

	runOptions.LogRedactions = append(runOptions.LogRedactions, c.redactedValues...)

	runResult, err := c.cmdRunner.Run(runOptions, format, vars...)

	if runOptions.CaptureOutputMode == cmdrunner.CaptureOutputModeStdout && runResult.Stderr != "" {
		c.logger.WarnWith("Docker command outputted to stderr - this may result in errors",
			"workingDir", runOptions.WorkingDir,
			"cmd", common.Redact(runOptions.LogRedactions, fmt.Sprintf(format, vars...)),
			"stderr", runResult.Stderr)
	}

	return runResult, err
}

func (c *ShellClient) getLastNonEmptyLine(lines []string, offset int) string {

	numLines := len(lines)

	// protect ourselves from overflows
	if offset >= numLines {
		offset = numLines - 1
	} else if offset < 0 {
		offset = 0
	}

	// iterate backwards over the lines
	for idx := numLines - 1 - offset; idx >= 0; idx-- {
		if lines[idx] != "" {
			return lines[idx]
		}
	}

	return ""
}

func (c *ShellClient) replaceSingleQuotes(input string) string {
	return strings.Replace(input, "'", "’", -1)
}

func (c *ShellClient) resolveDockerBuildNetwork() string {

	// may contain none as a value
	networkInterface := os.Getenv("NUCLIO_DOCKER_BUILD_NETWORK")
	if networkInterface == "" {
		networkInterface = common.GetEnvOrDefaultString("NUCLIO_BUILD_USE_HOST_NET", "host")
	}
	switch networkInterface {
	case "host":
		fallthrough
	case "default":
		fallthrough
	case "none":
		return fmt.Sprintf("--network %s", networkInterface)
	default:
		return ""
	}
}

func (c *ShellClient) build(buildOptions *BuildOptions, buildArgs string, cacheOption string) error {
	var lastBuildErr error
	retryOnErrorMessages := []string{

		// when one of the underlying image is gone (from cache)
		"^No such image: sha256:",
		"^unknown parent image ID sha256:",

		// when overlay image is gone (from disk)
		"^failed to get digest sha256:",
	}

	runOptions := &cmdrunner.RunOptions{
		CaptureOutputMode: cmdrunner.CaptureOutputModeStdout,
		WorkingDir:        &buildOptions.ContextDir,
	}

	// retry build on predefined errors that occur during race condition and collisions between
	// shared onbuild layers
	common.RetryUntilSuccessfulOnErrorPatterns(c.buildTimeout, // nolint: errcheck
		c.buildRetryInterval,
		retryOnErrorMessages,
		func() string { // nolint: errcheck
			runResults, err := c.runCommand(runOptions,
				"docker build %s --force-rm -t %s -f %s %s %s .",
				c.resolveDockerBuildNetwork(),
				buildOptions.Image,
				buildOptions.DockerfilePath,
				cacheOption,
				buildArgs)

			// preserve error
			lastBuildErr = err

			if err != nil {
				return runResults.Stderr
			}
			return ""
		})
	return lastBuildErr
}

func (c *ShellClient) createContainer(imageName string) (string, error) {
	var lastCreateContainerError error
	var containerID string
	retryOnErrorMessages := []string{

		// sometimes, creating the container fails on not finding the image because
		// docker was on high load and did not get to update its cache
		fmt.Sprintf("^Unable to find image '%s.*' locally", imageName),
	}

	// retry in case docker daemon is under high load
	// e.g.: between build and create, docker would need to update its cached manifest of built images
	common.RetryUntilSuccessfulOnErrorPatterns(10*time.Second, // nolint: errcheck
		2*time.Second,
		retryOnErrorMessages,
		func() string {

			// create container from image
			runResults, err := c.runCommand(nil, "docker create %s /bin/sh", imageName)

			// preserve error
			lastCreateContainerError = err

			if err != nil {
				return runResults.Stderr
			}
			containerID = runResults.Output
			containerID = strings.TrimSpace(containerID)
			return ""
		})

	return containerID, lastCreateContainerError
}
