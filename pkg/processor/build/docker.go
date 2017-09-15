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

package build

import (
	"path"
	"strings"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/util/cmdrunner"

	"github.com/pkg/errors"
)

type dockerClient struct {
	logger    nuclio.Logger
	cmdRunner *cmdrunner.CmdRunner
}

type buildOptions struct {
	imageName      string
	contextDir     string
	dockerfilePath string
}

func newDockerClient(parentLogger nuclio.Logger) (*dockerClient, error) {
	var err error

	b := &dockerClient{
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

func (d *dockerClient) build(buildOptions *buildOptions) error {
	d.logger.DebugWith("Building image", "image", buildOptions.imageName)

	// if context dir is not passed, use the dir containing the dockerfile
	if buildOptions.contextDir == "" && buildOptions.dockerfilePath != "" {
		buildOptions.contextDir = path.Dir(buildOptions.dockerfilePath)
	}

	// user can only specify context directory
	if buildOptions.dockerfilePath == "" && buildOptions.contextDir != "" {
		buildOptions.dockerfilePath = path.Join(buildOptions.contextDir, "Dockerfile")
	}

	_, err := d.cmdRunner.Run(&cmdrunner.RunOptions{WorkingDir: &buildOptions.contextDir},
		"docker build --force-rm -t %s -f %s .",
		buildOptions.imageName,
		buildOptions.dockerfilePath)

	return err
}

func (d *dockerClient) copyFromImage(imageName string, paths ...string) error {
	if len(paths)%2 != 0 {
		return errors.New("paths must be an even number")
	}

	out, err := d.cmdRunner.Run(nil, "docker create %s", imageName)
	if err != nil {
		return errors.Wrapf(err, "Failed to create container from %s", imageName)
	}

	containerID := strings.TrimSpace(out)
	defer func() {
		d.cmdRunner.Run(nil, "docker rm %s", containerID)
	}()

	for i := 0; i < len(paths); i += 2 {
		srcPath := paths[i]
		destPath := paths[i+1]
		_, err = d.cmdRunner.Run(nil, "docker cp %s:%s %s", containerID, srcPath, destPath)
		if err != nil {
			return errors.Wrapf(err, "Can't copy %s:%s -> %s", containerID, srcPath, destPath)
		}
	}

	return nil
}

func (d *dockerClient) pushImage(imageName, registryURL string) error {
	taggedImageName := registryURL + "/" + imageName

	d.logger.InfoWith("Pushing image", "from", imageName, "to", taggedImageName)

	_, err := d.cmdRunner.Run(nil, "docker tag %s %s", imageName, taggedImageName)
	if err != nil {
		return errors.Wrap(err, "Failed to tag image")
	}

	_, err = d.cmdRunner.Run(nil, "docker push %s", taggedImageName)
	if err != nil {
		return errors.Wrap(err, "Failed to push image")
	}

	return nil
}
