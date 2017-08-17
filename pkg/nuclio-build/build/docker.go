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
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/nuclio-build/util"
	"github.com/nuclio/nuclio/pkg/util/cmdrunner"

	"github.com/pkg/errors"
)

const (
	onBuildImageName       = "nuclio/nuclio:onbuild"
	builderOutputImageName = "nuclio/builder-output"
)

type dockerHelper struct {
	logger    nuclio.Logger
	cmdRunner *cmdrunner.CmdRunner
	env       *env
}

type buildOptions struct {
	Tag        string
	Dockerfile string
}

func newDockerHelper(parentLogger nuclio.Logger, env *env) (*dockerHelper, error) {
	var err error

	b := &dockerHelper{
		logger: parentLogger.GetChild("docker").(nuclio.Logger),
		env:    env,
	}

	// set cmdrunner
	b.cmdRunner, err = cmdrunner.NewCmdRunner(env.logger)
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
		_, err = d.cmdRunner.Run(runOpts, "docker build -t %s .", image)
	} else {
		_, err = d.cmdRunner.Run(runOpts, "docker build -t %s -f %s .", opts.Tag, opts.Dockerfile)
	}

	if err != nil {
		return errors.Wrap(err, "Cannot build")
	}

	return nil
}

func (d *dockerHelper) createOnBuildImage() error {
	buildDir := "onbuild"
	buildContext := filepath.Join(d.env.getNuclioDir(), "hack", "processor", "build", buildDir)

	return d.doBuild(onBuildImageName, buildContext, nil)
}

func (d *dockerHelper) buildBuilder() error {
	buildContext := d.env.getNuclioDir()
	options := buildOptions{
		Tag:        builderOutputImageName,
		Dockerfile: filepath.Join("hack", "processor", "build", "builder", "Dockerfile"),
	}
	return d.doBuild(builderOutputImageName, buildContext, &options)
}

func (d *dockerHelper) createBinaryContainer() (string, error) {
	d.logger.DebugWith("Creating container for image", "name", builderOutputImageName)

	out, err := d.cmdRunner.Run(nil, "docker create %s", builderOutputImageName)
	if err != nil {
		return "", errors.Wrap(err, "Unable to create builder container.")
	}

	return strings.TrimSpace(out), nil
}

func (d *dockerHelper) createBuilderImage() error {
	if err := d.buildBuilder(); err != nil {
		return err
	}

	dockerContainerID, err := d.createBinaryContainer()
	if err != nil {
		return err
	}

	binaryPath := "/go/bin/processor"
	destDir := d.env.getWorkDir()
	d.logger.DebugWith("Copying binary from container", "container", dockerContainerID, "path", binaryPath, "target", destDir)
	_, err = d.cmdRunner.Run(nil, "docker cp %s:%s %s", dockerContainerID, binaryPath, destDir)
	if err != nil {
		return errors.Wrap(err, "Can't copy from container")
	}

	return nil
}

func isLink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

// Copy *only* files from src to dest
func (d *dockerHelper) copyFiles(src, dest string) error {
	entries, err := ioutil.ReadDir(src)
	if err != nil {
		return errors.Wrapf(err, "Can't read %q content", src)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			d.logger.InfoWith("Skipping direcotry copy", "src", src, "name", entry.Name())
			continue
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			d.logger.ErrorWith("Symlink found", "path", entry.Name())
			return errors.Wrapf(err, "%q is a symlink", entry.Name())
		}
		srcPath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dest, entry.Name())
		if err := util.CopyFile(srcPath, destPath); err != nil {
			d.logger.ErrorWith("Can't copy file", "error", err, "src", srcPath, "dest", destPath)
			return errors.Wrap(err, "Can't copy")
		}
	}
	return nil
}

func (d *dockerHelper) processorDockerFile() (string, error) {
	runtime := d.env.options.Runtime

	if len(runtime) < 2 {
		return "", fmt.Errorf("Bad runtime - %q", runtime)
	}

	switch d.env.options.Runtime[:2] {
	case "go":
		if len(d.env.config.Build.Packages) > 0 {
			return "Dockerfile.jessie", nil
		}
		return "Dockerfile.alpine", nil
	case "py":
		return fmt.Sprintf("Dockerfile.%s", runtime), nil
	}

	return "", fmt.Errorf("Unknown runtime - %q", runtime)
}

func (d *dockerHelper) createProcessorImage() error {
	if err := os.MkdirAll(filepath.Join(d.env.getNuclioDir(), "bin"), 0755); err != nil {
		return errors.Wrapf(err, "Unable to mkdir for bin output")
	}

	processorOutput := filepath.Join(d.env.getNuclioDir(), "bin", "processor")

	if err := util.CopyFile(d.env.getBinaryPath(), processorOutput); err != nil {
		return errors.Wrapf(err, "Unable to copy file %s to %s", d.env.getBinaryPath(), processorOutput)
	}

	handlerPath := filepath.Join(d.env.userFunctionPath, d.env.config.Name)
	buildContext := d.env.getNuclioDir()
	if err := d.copyFiles(handlerPath, buildContext); err != nil {
		return errors.Wrapf(err, "Can't copy files from %q to %q", handlerPath, buildContext)
	}

	dockerfile, err := d.processorDockerFile()
	if err != nil {
		return err
	}

	options := buildOptions{
		Tag:        d.env.outputName,
		Dockerfile: filepath.Join("hack", "processor", "build", dockerfile),
	}
	if err := d.doBuild(d.env.outputName, buildContext, &options); err != nil {
		return errors.Wrap(err, "Failed to build image")
	}

	if d.env.options.PushRegistry != "" {
		return d.pushImage(d.env.outputName, d.env.options.PushRegistry)
	}

	return nil
}

func (d *dockerHelper) cleanupBuilder() {
	out, err := d.cmdRunner.Run(nil, "docker ps -a --filter ancestor=%s --format {{.ID}}", builderOutputImageName)
	if err != nil {
		d.logger.WarnWith("Can't list containers", "image", builderOutputImageName, "error", err)
		return
	}
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		containerID := strings.TrimSpace(scanner.Text())
		if len(containerID) == 0 {
			continue
		}
		d.logger.InfoWith("Deleting container", "id", containerID)
		if _, err := d.cmdRunner.Run(nil, "docker rm %s", containerID); err != nil {
			d.logger.WarnWith("Can't delete container", "id", containerID, "error", err)
		}
	}

	if err = scanner.Err(); err != nil {
		d.logger.WarnWith("Can't scan docker output", "error", err)
	}
}

func (d *dockerHelper) close() {
	d.cleanupBuilder()
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
