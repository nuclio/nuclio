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
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"

	"github.com/docker/distribution/reference"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"k8s.io/apimachinery/pkg/util/json"
)

// RestrictedNameChars collects the characters allowed to represent a network or endpoint name.
const restrictedNameChars = `[a-zA-Z0-9][a-zA-Z0-9_.-]`

// RestrictedNamePattern is a regular expression to validate names against the collection of restricted characters.
// taken from moby and used to validate names (network, container, labels, endpoints)
var restrictedNameRegex = regexp.MustCompile(`^/?` + restrictedNameChars + `+$`)

var containerIDRegex = regexp.MustCompile(`^[\w+-\.]+$`)

// loose regexes, today just prohibit whitespaces
var restrictedBuildArgRegex = regexp.MustCompile(`^[\S]+$`)
var volumeNameRegex = regexp.MustCompile(`^[\S]+$`)

// this is an open issue https://github.com/kubernetes/kubernetes/issues/53201#issuecomment-534647130
// taking the loose approach,
var envVarNameRegex = regexp.MustCompile(`^[^=]+$`)

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

	// set cmd runner
	if newClient.cmdRunner == nil {
		newClient.cmdRunner, err = cmdrunner.NewShellRunner(newClient.logger)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create command runner")
		}
	}

	// verify
	if _, err := newClient.GetVersion(false); err != nil {
		return nil, errors.Wrap(err, "No docker client found")
	}

	return newClient, nil
}

// Build will build a docker image, given build options
func (c *ShellClient) Build(buildOptions *BuildOptions) error {
	c.logger.DebugWith("Building image", "buildOptions", buildOptions)

	if err := c.validateBuildOptions(buildOptions); err != nil {
		return errors.Wrap(err, "Invalid build options passed")
	}

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

	if err := c.build(buildOptions, buildArgs); err != nil {
		return errors.Wrap(err, "Failed to build")
	}

	c.logger.DebugWith("Successfully built image", "image", buildOptions.Image)
	return nil
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

	if _, err := reference.Parse(imageName); err != nil {
		return errors.Wrap(err, "Invalid image name to tag/push")
	}

	if _, err := reference.Parse(taggedImage); err != nil {
		return errors.Wrap(err, "Invalid tagged image name to tag/push")
	}

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

	if _, err := reference.Parse(imageURL); err != nil {
		return errors.Wrap(err, "Invalid image URL to pull")
	}

	_, err := c.runCommand(nil, "docker pull %s", imageURL)
	return err
}

// RemoveImage will remove (delete) a local image
func (c *ShellClient) RemoveImage(imageName string) error {
	c.logger.DebugWith("Removing image", "imageName", imageName)

	if _, err := reference.Parse(imageName); err != nil {
		return errors.Wrap(err, "Invalid image name to remove")
	}

	_, err := c.runCommand(nil, "docker rmi -f %s", imageName)
	return err
}

// RunContainer will run a container based on an image and run options
func (c *ShellClient) RunContainer(imageName string, runOptions *RunOptions) (string, error) {
	c.logger.DebugWith("Running container", "imageName", imageName, "runOptions", runOptions)

	// validate the given run options against malicious contents
	if err := c.validateRunOptions(imageName, runOptions); err != nil {
		return "", errors.Wrap(err, "Invalid run options passed")
	}

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
	c.logger.DebugWith("Executing in container", "containerID", containerID, "execOptions", execOptions)

	// validate the given run options against malicious contents
	if err := c.validateExecOptions(containerID, execOptions); err != nil {
		return errors.Wrap(err, "Invalid exec options passed")
	}

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
	c.logger.DebugWith("Removing container", "containerID", containerID)

	// containerID is ID or name
	if !containerIDRegex.MatchString(containerID) && !restrictedNameRegex.MatchString(containerID) {
		return errors.New("Invalid container ID name in remove container")
	}

	_, err := c.runCommand(nil, "docker rm -f %s", containerID)
	return err
}

// StopContainer stops a container given a container ID
func (c *ShellClient) StopContainer(containerID string) error {
	c.logger.DebugWith("Stopping container", "containerID", containerID)

	// containerID is ID or name
	if !containerIDRegex.MatchString(containerID) && !restrictedNameRegex.MatchString(containerID) {
		return errors.New("Invalid container ID to stop")
	}

	_, err := c.runCommand(nil, "docker stop %s", containerID)
	return err
}

// StartContainer stops a container given a container ID
func (c *ShellClient) StartContainer(containerID string) error {
	c.logger.DebugWith("Starting container", "containerID", containerID)

	// containerID is ID or name
	if !containerIDRegex.MatchString(containerID) && !restrictedNameRegex.MatchString(containerID) {
		return errors.New("Invalid container ID to start")
	}

	_, err := c.runCommand(nil, "docker start %s", containerID)
	return err
}

func (c *ShellClient) GetContainerPort(container *Container, boundPort int) (int, error) {
	functionHostPort := Port(fmt.Sprintf("%d/tcp", boundPort))

	portBindings := container.HostConfig.PortBindings[functionHostPort]
	ports := container.NetworkSettings.Ports[functionHostPort]
	if len(portBindings) == 0 && len(ports) == 0 {
		return 0, nil
	}

	// by default take the port binding, as if the user requested
	if len(portBindings) != 0 &&
		portBindings[0].HostPort != "" && // docker version < 20.10
		portBindings[0].HostPort != "0" { // on docker version >= 20.10, the host port would by 0 and not empty string.
		return strconv.Atoi(portBindings[0].HostPort)
	}

	// port was not explicit by user, take port assigned by docker daemon
	if len(ports) != 0 && ports[0].HostPort != "" {
		return strconv.Atoi(ports[0].HostPort)
	}

	// function might failed during deploying and did not assign a port
	return 0, nil

}

// GetContainerLogs returns raw logs from a given container ID
// Concatenating stdout and stderr since there's no way to re-interlace them
func (c *ShellClient) GetContainerLogs(containerID string) (string, error) {
	c.logger.DebugWith("Getting container logs", "containerID", containerID)

	// containerID is ID or name
	if !containerIDRegex.MatchString(containerID) && !restrictedNameRegex.MatchString(containerID) {
		return "", errors.New("Invalid container ID to get logs from")
	}

	runOptions := &cmdrunner.RunOptions{
		CaptureOutputMode: cmdrunner.CaptureOutputModeCombined,
	}

	runResult, err := c.runCommand(runOptions, "docker logs %s", containerID)
	return runResult.Output, err
}

// AwaitContainerHealth blocks until the given container is healthy or the timeout passes
func (c *ShellClient) AwaitContainerHealth(containerID string, timeout *time.Duration) error {
	c.logger.DebugWith("Awaiting container health", "containerID", containerID, "timeout", timeout)

	if !containerIDRegex.MatchString(containerID) && !restrictedNameRegex.MatchString(containerID) {
		return errors.New("Invalid container ID to await health for")
	}

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

	if err := c.validateGetContainerOptions(options); err != nil {
		return nil, errors.Wrap(err, "Invalid get container options passed")
	}

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
		strings.ReplaceAll(containerIDsAsString, "\n", " "))
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
	c.logger.DebugWith("Getting container events",
		"containerName", containerName,
		"since", since,
		"until", until)

	if !restrictedNameRegex.MatchString(containerName) {
		return nil, errors.New("Invalid container name to get events for")
	}

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

	// TODO: validate login URL
	c.logger.DebugWith("Performing docker login", "URL", options.URL)

	c.redactedValues = append(c.redactedValues, options.Password)

	_, err := c.runCommand(nil, `docker login -u %s -p '%s' %s`,
		options.Username,
		options.Password,
		options.URL)

	return err
}

// CreateNetwork creates a docker network
func (c *ShellClient) CreateNetwork(options *CreateNetworkOptions) error {
	c.logger.DebugWith("Creating docker network", "options", options)

	// validate the given create network options against malicious contents
	if err := c.validateCreateNetworkOptions(options); err != nil {
		return errors.Wrap(err, "Invalid network creation options passed")
	}

	_, err := c.runCommand(nil, `docker network create %s`, options.Name)

	return err
}

// DeleteNetwork deletes a docker network
func (c *ShellClient) DeleteNetwork(networkName string) error {
	c.logger.DebugWith("Deleting docker network", "networkName", networkName)
	if !restrictedNameRegex.MatchString(networkName) {
		return errors.New("Invalid network name to delete")
	}

	_, err := c.runCommand(nil, `docker network rm %s`, networkName)

	return err
}

// CreateVolume creates a docker volume
func (c *ShellClient) CreateVolume(options *CreateVolumeOptions) error {
	c.logger.DebugWith("Creating docker volume", "options", options)

	// validate the given create network options against malicious contents
	if err := c.validateCreateVolumeOptions(options); err != nil {
		return errors.Wrap(err, "Invalid volume creation options passed")
	}

	_, err := c.runCommand(nil, `docker volume create %s`, options.Name)

	return err
}

// DeleteVolume deletes a docker volume
func (c *ShellClient) DeleteVolume(volumeName string) error {
	c.logger.DebugWith("Deleting docker volume", "volumeName", volumeName)
	if !volumeNameRegex.MatchString(volumeName) {
		return errors.New("Invalid volume name to delete")
	}

	_, err := c.runCommand(nil, `docker volume rm --force %s`, volumeName)
	return err
}

func (c *ShellClient) Save(imageName string, outPath string) error {
	c.logger.DebugWith("Docker saving to path", "outPath", outPath, "imageName", imageName)
	if _, err := reference.Parse(imageName); err != nil {
		return errors.Wrap(err, "Invalid image name to save")
	}

	_, err := c.runCommand(nil, `docker save --output %s %s`, outPath, imageName)

	return err
}

func (c *ShellClient) Load(inPath string) error {
	c.logger.DebugWith("Docker loading from path", "inPath", inPath)
	_, err := c.runCommand(nil, `docker load --input %s`, inPath)
	return err
}

func (c *ShellClient) GetVersion(quiet bool) (string, error) {
	runOptions := &cmdrunner.RunOptions{
		LogOnlyOnFailure: quiet,
	}
	output, err := c.runCommand(runOptions, `docker version --format "{{json .}}"`)
	if err != nil {
		return "", errors.Wrap(err, "Failed to get docker version")
	}
	return output.Output, nil
}

func (c *ShellClient) GetContainerIPAddresses(containerID string) ([]string, error) {
	c.logger.DebugWith("Getting container IP addresses", "containerID", containerID)
	runResults, err := c.runCommand(nil, `docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' %s`, containerID)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get container ip addresses")
	}

	return strings.Split(strings.TrimSpace(runResults.Output), "\n"), nil
}

func (c *ShellClient) GetContainerLogStream(ctx context.Context,
	containerID string,
	logOptions *ContainerLogsOptions) (io.ReadCloser, error) {

	if logOptions == nil {
		logOptions = &ContainerLogsOptions{
			Follow: true,
		}
	}

	var cmdArgs []string
	if logOptions.Since != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--since %s", common.Quote(logOptions.Since)))
	}
	if logOptions.Tail != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--tail %s", common.Quote(logOptions.Tail)))
	}

	if logOptions.Follow {
		return c.streamCommand(ctx,
			nil,
			"docker logs %s --follow %s", strings.Join(cmdArgs, " "), containerID)
	}

	output, err := c.runCommand(&cmdrunner.RunOptions{
		CaptureOutputMode: cmdrunner.CaptureOutputModeCombined,
	}, "docker logs %s %s", strings.Join(cmdArgs, " "), containerID)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get container log stream")
	}

	return ioutil.NopCloser(strings.NewReader(output.Output)), nil
}

func (c *ShellClient) runCommand(runOptions *cmdrunner.RunOptions,
	format string,
	vars ...interface{}) (cmdrunner.RunResult, error) {

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
			"cmd", cmdrunner.Redact(runOptions.LogRedactions, fmt.Sprintf(format, vars...)),
			"stderr", runResult.Stderr)
	}

	return runResult, err
}

func (c *ShellClient) streamCommand(ctx context.Context,
	runOptions *cmdrunner.RunOptions,
	format string,
	vars ...interface{}) (io.ReadCloser, error) {
	return c.cmdRunner.Stream(ctx, runOptions, format, vars...)
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
	return strings.ReplaceAll(input, "'", "’")
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

func (c *ShellClient) build(buildOptions *BuildOptions, buildArgs string) error {
	var lastBuildErr error

	cacheOption := ""
	if buildOptions.NoCache {
		cacheOption = "--no-cache"
	}

	pullOption := ""
	if buildOptions.Pull {
		pullOption = "--pull"
	}

	buildCommand := fmt.Sprintf("docker build %s --force-rm -t %s -f %s %s %s %s .",
		c.resolveDockerBuildNetwork(),
		buildOptions.Image,
		buildOptions.DockerfilePath,
		cacheOption,
		pullOption,
		buildArgs)

	retryOnErrorMessages := []string{

		// when one of the underlying image is gone (from cache)
		"^No such image: sha256:",
		"^unknown parent image ID sha256:",
		"^failed to set parent sha256:",
		"^failed to export image:",

		// when overlay image is gone (from disk)
		"^failed to get digest sha256:",

		// when trying to reuse a missing nuclio-onbuild between functions
		"^Unable to find image 'nuclio-onbuild-.*' locally",
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
		func() string {
			runResults, err := c.runCommand(runOptions, buildCommand)

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

	if _, err := reference.Parse(imageName); err != nil {
		return "", errors.Wrap(err, "Invalid image name to create container from")
	}

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
func (c *ShellClient) validateBuildOptions(buildOptions *BuildOptions) error {
	if _, err := reference.Parse(buildOptions.Image); err != nil {
		return errors.Wrap(err, "Invalid image name in build options")
	}

	for buildArgName, buildArgValue := range buildOptions.BuildArgs {
		if !restrictedBuildArgRegex.MatchString(buildArgName) {
			message := "Invalid build arg name supplied"
			c.logger.WarnWith(message, "buildArgName", buildArgName)
			return errors.New(message)
		}
		if !restrictedBuildArgRegex.MatchString(buildArgValue) {
			message := "Invalid build arg value supplied"
			c.logger.WarnWith(message, "buildArgValue", buildArgValue)
			return errors.New(message)
		}
	}
	return nil
}

func (c *ShellClient) validateRunOptions(imageName string, runOptions *RunOptions) error {
	if _, err := reference.Parse(imageName); err != nil {
		return errors.Wrap(err, "Invalid image name passed to run command")
	}

	// container name can't be empty
	if runOptions.ContainerName != "" && !restrictedNameRegex.MatchString(runOptions.ContainerName) {
		return errors.New("Invalid container name in build options")
	}

	for envVarName := range runOptions.Env {
		if !envVarNameRegex.MatchString(envVarName) {
			return errors.New("Invalid env var name in run options")
		}
	}

	for volumeHostPath, volumeContainerPath := range runOptions.Volumes {
		if !volumeNameRegex.MatchString(volumeHostPath) {
			return errors.New("Invalid volume host path in run options")
		}
		if !volumeNameRegex.MatchString(volumeContainerPath) {
			return errors.New("Invalid volume container path in run options")
		}
	}

	if runOptions.Network != "" && !restrictedNameRegex.MatchString(runOptions.Network) {
		return errors.New("Invalid network name in run options")
	}

	return nil
}

func (c *ShellClient) validateExecOptions(containerID string, execOptions *ExecOptions) error {

	// containerID is ID or name
	if !containerIDRegex.MatchString(containerID) && !restrictedNameRegex.MatchString(containerID) {
		return errors.New("Invalid container ID name in container exec")
	}

	for envVarName := range execOptions.Env {
		if !envVarNameRegex.MatchString(envVarName) {
			return errors.New("Invalid env var name in exec options")
		}
	}
	return nil
}

func (c *ShellClient) validateCreateNetworkOptions(options *CreateNetworkOptions) error {
	if !restrictedNameRegex.MatchString(options.Name) {
		return errors.New("Invalid network name in network creation options")
	}
	return nil
}

func (c *ShellClient) validateCreateVolumeOptions(options *CreateVolumeOptions) error {
	if !restrictedNameRegex.MatchString(options.Name) {
		return errors.New("Invalid volume name in volume creation options")
	}
	return nil
}

func (c *ShellClient) validateGetContainerOptions(options *GetContainerOptions) error {
	if options.Name != "" && !restrictedNameRegex.MatchString(options.Name) {
		return errors.New("Invalid container name in get container options")
	}

	if options.ID != "" && !containerIDRegex.MatchString(options.ID) {
		return errors.New("Invalid container ID in get container options")
	}

	return nil
}
