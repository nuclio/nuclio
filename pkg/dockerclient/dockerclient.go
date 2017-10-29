/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or impliec.
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

// Client is a docker client
type Client struct {
	logger    nuclio.Logger
	cmdRunner cmdrunner.CmdRunner
}

// BuildOptions are options for building a docker image
type BuildOptions struct {
	ImageName      string
	ContextDir     string
	DockerfilePath string
	NoCache        bool
}

// RunOptions are options for running a docker image
type RunOptions struct {
	Ports         map[int]int
	ContainerName string
	NetworkType   string
	Env           map[string]string
	Labels        map[string]string
	Volumes       map[string]string
}

// GetContainerOptions are options for container search
type GetContainerOptions struct {
	Labels map[string]string
}

// NewClient creates a new docker client
func NewClient(parentLogger nuclio.Logger) (*Client, error) {
	var err error

	b := &Client{
		logger: parentLogger.GetChild("docker").(nuclio.Logger),
	}

	// set cmdrunner
	b.cmdRunner, err = cmdrunner.NewShellRunner(b.logger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create command runner")
	}

	_, err = b.cmdRunner.Run(nil, "docker version")
	if err != nil {
		return nil, errors.Wrap(err, "No docker client found")
	}

	return b, nil
}

// Build will build a docker image, given build options
func (c *Client) Build(buildOptions *BuildOptions) error {
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

	_, err := c.cmdRunner.Run(&cmdrunner.RunOptions{WorkingDir: &buildOptions.ContextDir},
		"docker build --force-rm -t %s -f %s %s .",
		buildOptions.ImageName,
		buildOptions.DockerfilePath,
		cacheOption)

	return err
}

// CopyObjectsFromImage copies objects (files, directories) from a given image to local storage. it does
// this through an intermediate container which is deleted afterwards
func (c *Client) CopyObjectsFromImage(imageName string, objectsToCopy map[string]string, allowCopyErrors bool) error {
	containerID, err := c.cmdRunner.Run(nil, "docker create %s", imageName)
	if err != nil {
		return errors.Wrapf(err, "Failed to create container from %s", imageName)
	}

	containerID = strings.TrimSpace(containerID)
	defer func() {
		c.cmdRunner.Run(nil, "docker rm -f %s", containerID)
	}()

	for objectImagePath, objectLocalPath := range objectsToCopy {
		_, err = c.cmdRunner.Run(nil, "docker cp %s:%s %s", containerID, objectImagePath, objectLocalPath)
		if err != nil && !allowCopyErrors {
			return errors.Wrapf(err, "Can't copy %s:%s -> %s", containerID, objectImagePath, objectLocalPath)
		}
	}

	return nil
}

// PushImage pushes a local image to a remote docker repository
func (c *Client) PushImage(imageName string, registryURL string) error {
	taggedImageName := registryURL + "/" + imageName

	c.logger.InfoWith("Pushing image", "from", imageName, "to", taggedImageName)

	_, err := c.cmdRunner.Run(nil, "docker tag %s %s", imageName, taggedImageName)
	if err != nil {
		return errors.Wrap(err, "Failed to tag image")
	}

	_, err = c.cmdRunner.Run(nil, "docker push %s", taggedImageName)
	if err != nil {
		return errors.Wrap(err, "Failed to push image")
	}

	return nil
}

// PullImage pulls an image from a remote docker repository
func (c *Client) PullImage(imageURL string) error {
	_, err := c.cmdRunner.Run(nil, "docker pull %s", imageURL)
	return err
}

// RemoveImage will remove (delete) a local image
func (c *Client) RemoveImage(imageName string) error {
	_, err := c.cmdRunner.Run(nil, "docker rmi -f %s", imageName)
	return err
}

// RunContainer will run a container based on an image and run options
func (c *Client) RunContainer(imageName string, runOptions *RunOptions) (string, error) {
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
			labelArgument += fmt.Sprintf("--label %s=%s ", labelName, labelValue)
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

	out, err := c.cmdRunner.Run(nil,
		"docker run -d %s %s %s %s %s %s %s",
		portsArgument,
		nameArgument,
		netArgument,
		labelArgument,
		envArgument,
		volumeArgument,
		imageName)

	if err != nil {
		c.logger.WarnWith("Failed to run container", "err", err, "out", out)

		return "", err
	}

	return strings.TrimSpace(out), err
}

// RemoveContainer removes a container given a container ID
func (c *Client) RemoveContainer(containerID string) error {
	_, err := c.cmdRunner.Run(nil, "docker rm -f %s", containerID)
	return err
}

// GetContainerLogs returns raw logs from a given container ID
func (c *Client) GetContainerLogs(containerID string) (string, error) {
	return c.cmdRunner.Run(nil, "docker logs %s", containerID)
}

// GetContainers returns a list of container IDs which match a certain criteria
func (c *Client) GetContainers(options *GetContainerOptions) ([]Container, error) {
	c.logger.DebugWith("Getting containers", "options", options)

	filterArgument := ""
	for labelName, labelValue := range options.Labels {
		filterArgument += fmt.Sprintf(`--filter "label=%s=%s" `, labelName, labelValue)
	}

	containerIDsAsString, err := c.cmdRunner.Run(nil, "docker ps -q %s", filterArgument)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get containers")
	}

	if len(containerIDsAsString) == 0 {
		return []Container{}, nil
	}

	containersInfoString, err := c.cmdRunner.Run(nil,
		"docker inspect %s",
		strings.Replace(containerIDsAsString, "\n", " ", -1))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to inspect containers")
	}

	var containersInfo []Container

	// parse the result
	if err := json.Unmarshal([]byte(containersInfoString), &containersInfo); err != nil {
		return nil, errors.Wrap(err, "Failed to parse inspect response")
	}

	return containersInfo, nil
}
