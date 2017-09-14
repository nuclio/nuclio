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
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/util/cmdrunner"

	"github.com/pkg/errors"
)

const (
	onBuildImageName       = "nuclio/nuclio:onbuild"
	binaryBuilderImageName = "nuclio/builder-output"
	goHandlerImageName     = "nuclio/go-handler"
	processorBuilderImage  = "nuclio/processor-build:latest"
)

type dockerHelper struct {
	logger    nuclio.Logger
	cmdRunner *cmdrunner.CmdRunner
}

type buildOptions struct {
	Tag        string
	Dockerfile string
}

func newDockerHelper(parentLogger nuclio.Logger) (*dockerHelper, error) {
	var err error

	b := &dockerHelper{
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

func (d *dockerHelper) doBuild(image string, buildContext string, opts *buildOptions) error {
	d.logger.DebugWith("Building image", "image", image)

	var err error
	runOpts := &cmdrunner.RunOptions{WorkingDir: &buildContext}
	if opts == nil {
		_, err = d.cmdRunner.Run(runOpts, "docker build --force-rm -t %s .", image)
	} else {
		_, err = d.cmdRunner.Run(runOpts, "docker build --force-rm -t %s -f %s .", opts.Tag, opts.Dockerfile)
	}

	if err != nil {
		return errors.Wrap(err, "Cannot build")
	}

	return nil
}

func (d *dockerHelper) buildGoHandler(workDirPath, buildCommand string) error {
	dockerfileTemplate, err := template.New("").Parse(goHandlerDockerfileTemplateText)
	if err != nil {
		return err
	}

	dockerfilePath := filepath.Join(workDirPath, "Dockerfile.go-handler")
	dockerFile, err := os.Create(dockerfilePath)
	if err != nil {
		return err
	}

	defer dockerFile.Close()
	// TODO: In build.yaml?
	params := map[string]interface{}{
		"Image":   processorBuilderImage,
		"SrcDir":  "/go/src/handler",
		"Command": buildCommand,
	}

	if err := dockerfileTemplate.Execute(dockerFile, params); err != nil {
		return err
	}

	opts := &buildOptions{
		Tag:        goHandlerImageName,
		Dockerfile: dockerfilePath,
	}

	return d.doBuild("", workDirPath, opts)
}

func (d *dockerHelper) copyFromImage(imageName string, paths ...string) error {
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

func (d *dockerHelper) pushImage(imageName, registryURL string) error {
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
