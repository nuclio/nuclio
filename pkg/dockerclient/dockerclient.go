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
	"path"
	"strings"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/util/cmdrunner"

	"github.com/pkg/errors"
	"fmt"
)

type Client struct {
	logger    nuclio.Logger
	cmdRunner *cmdrunner.CmdRunner
}

type BuildOptions struct {
	ImageName      string
	ContextDir     string
	DockerfilePath string
	NoCache        bool
}

func NewClient(parentLogger nuclio.Logger) (*Client, error) {
	var err error

	b := &Client{
		logger: parentLogger.GetChild("docker").(nuclio.Logger),
	}

	// set cmdrunner
	b.cmdRunner, err = cmdrunner.NewCmdRunner(b.logger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create command runner")
	}

	_, err = b.cmdRunner.Run(nil, "docker version")
	if err != nil {
		return nil, errors.Wrap(err, "No docker client found")
	}

	return b, nil
}

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

func (c *Client) PullImage(imageURL string) error {
	_, err := c.cmdRunner.Run(nil, "docker pull %s", imageURL)
	return err
}

func (c *Client) RemoveImage(imageName string) error {
	_, err := c.cmdRunner.Run(nil, "docker rmi -f %s", imageName)
	return err
}

func (c *Client) RunContainer(imageName string, ports map[int]int, containerName string) (string, error) {
	portsArgument := ""

	for localPort, dockerPort := range ports {
		portsArgument += fmt.Sprintf("-p %d:%d ", localPort, dockerPort)
	}

	nameArgument := ""
	if containerName != "" {
		nameArgument = fmt.Sprintf("--name %s", containerName)
	}

	out, err := c.cmdRunner.Run(nil,
		"docker run -d %s %s %s",
		portsArgument,
		nameArgument,
		imageName)

	if err != nil {
		return "", err
	}

	return strings.TrimSpace(out), err
}

func (c *Client) RemoveContainer(containerID string) error {
	_, err := c.cmdRunner.Run(nil, "docker rm -f %s", containerID)
	return err
}
