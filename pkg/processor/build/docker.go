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
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/processor/build/util"
	"github.com/nuclio/nuclio/pkg/util/cmdrunner"
	"github.com/nuclio/nuclio/pkg/util/common"

	"github.com/pkg/errors"
)

const (
	onBuildImageName       = "nuclio/nuclio:onbuild"
	binaryBuilderImageName = "nuclio/builder-output"
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
		_, err = d.cmdRunner.Run(runOpts, "docker build --force-rm -t %s .", image)
	} else {
		_, err = d.cmdRunner.Run(runOpts, "docker build --force-rm -t %s -f %s .", opts.Tag, opts.Dockerfile)
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

func (d *dockerHelper) buildBinaryBuilder() error {
	buildContext := d.env.getNuclioDir()
	options := buildOptions{
		Tag:        binaryBuilderImageName,
		Dockerfile: filepath.Join("hack", "processor", "build", "builder", "Dockerfile"),
	}
	return d.doBuild(binaryBuilderImageName, buildContext, &options)
}

func (d *dockerHelper) createProcessorBinary() error {

	// create an image containing the processor binary so that we may extract it. if the compilation fails,
	// the build should still succeed, we simply shouldn't be able to extract the processor binary because
	// it doesn't exist
	if err := d.buildBinaryBuilder(); err != nil {
		return err
	}

	// whatever happens, clear the builder image
	defer d.removeBinaryBuilderImage()

	// create a stopped container from the image so that we may extract processor binary
	binaryBuilderContainerID, err := d.createBinaryBuilderContainer()
	if err != nil {
		return err
	}

	// whatever happens, clear the builder image
	defer d.removeBinaryBuilderContainer(binaryBuilderContainerID)

	destDir := d.env.getWorkDir()

	// try to copy the processor from the container
	err = d.copyFileFromContainer(binaryBuilderContainerID, "/go/bin/processor", destDir)

	// if we failed, try to get the processor build log
	if err != nil {
		d.logger.Warn("Failed to find processor binary after build")

		err = d.copyFileFromContainer(binaryBuilderContainerID, "/processor_build.log", destDir)
		if err != nil {
			return errors.Wrap(err, "Failed to extract processor build log from container")
		}

		// read the build log
		buildLogContents, err := ioutil.ReadFile(path.Join(destDir, "processor_build.log"))
		if err != nil {
			return errors.Wrap(err, "Failed to read build log contents")
		}

		// log the error
		d.logger.ErrorWith("Failed to build function", "error", string(buildLogContents))

		return fmt.Errorf("Failed to build function:\n%s", string(buildLogContents))
	}

	return nil
}

func (d *dockerHelper) createBinaryBuilderContainer() (string, error) {
	d.logger.DebugWith("Creating container for image", "name", binaryBuilderImageName)

	out, err := d.cmdRunner.Run(nil, "docker create %s", binaryBuilderImageName)

	if err != nil {
		return "", errors.Wrap(err, "Unable to create builder container.")
	}

	return strings.TrimSpace(out), nil
}

func (d *dockerHelper) removeBinaryBuilderImage() error {
	_, err := d.cmdRunner.Run(nil, "docker rmi -f %s", binaryBuilderImageName)
	return err
}

func (d *dockerHelper) removeBinaryBuilderContainer(containerID string) error {
	_, err := d.cmdRunner.Run(nil, "docker rm -f %s", containerID)
	return err
}

func (d *dockerHelper) copyFileFromContainer(containerID string, containerPath string, destPath string) error {
	d.logger.DebugWith("Copying file from container",
		"container", containerID,
		"containerPath", containerPath,
		"destPath", destPath)

	_, err := d.cmdRunner.Run(nil, "docker cp %s:%s %s", containerID, containerPath, destPath)
	if err != nil {
		return errors.Wrapf(err, "Failed to copy %s from container %s", containerPath, containerID)
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
	var err error
	var entries []os.FileInfo

	if common.IsDir(src) {
		entries, err = ioutil.ReadDir(src)
		if err != nil {
			return errors.Wrapf(err, "Can't read %q content", src)
		}
	} else {
		fileInfo, err := os.Stat(src)
		if err != nil {
			return errors.Wrapf(err, "Failed to stat %s", src)
		}

		entries = []os.FileInfo{fileInfo}

		// update src to hold the base dir
		src = path.Dir(src)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			d.logger.DebugWith("Skipping directory copy", "src", src, "name", entry.Name())
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

func (d *dockerHelper) createProcessorDockerfile() (string, error) {
	baseTemplateName := "Dockerfile.tmpl"
	templateFilePath := filepath.Join(d.env.getNuclioDir(), "hack", "processor", "build", baseTemplateName)

	funcMap := template.FuncMap{
		"basename":        path.Base,
		"isDir":           common.IsDir,
		"configFilePaths": d.env.ExternalConfigFilePaths,
	}

	dockerfileTemplate, err := template.New("").Funcs(funcMap).ParseFiles(templateFilePath)
	if err != nil {
		return "", errors.Wrapf(err, "Can't parse template at %q", templateFilePath)
	}

	dockerfilePath := filepath.Join(d.env.getNuclioDir(), "Dockerfile.processor")
	dockerfile, err := os.Create(dockerfilePath)
	if err != nil {
		return "", errors.Wrap(err, "Can't create processor docker file")
	}

	d.logger.DebugWith("Creating Dockerfile from template",
		"template",
		templateFilePath,
		"dest",
		dockerfilePath)

	if err = dockerfileTemplate.ExecuteTemplate(dockerfile, baseTemplateName, d.env.config.Build); err != nil {
		return "", errors.Wrapf(err, "Can't execute template with %#v", d.env.config.Build)
	}

	return dockerfilePath, nil
}

func (d *dockerHelper) createProcessorImage() error {
	if err := os.MkdirAll(filepath.Join(d.env.getNuclioDir(), "bin"), 0755); err != nil {
		return errors.Wrapf(err, "Unable to mkdir for bin output")
	}

	processorOutputPath := filepath.Join(d.env.getNuclioDir(), "bin", "processor")

	if err := util.CopyFile(d.env.getBinaryPath(), processorOutputPath); err != nil {
		return errors.Wrapf(err, "Unable to copy file %s to %s", d.env.getBinaryPath(), processorOutputPath)
	}

	d.env.config.Build.Copy = map[string]string{}

	// get the path to the user function
	handlerPath := filepath.Join(d.env.userFunctionPath, d.env.config.Name)
	relativeUserFunctionPath := handlerPath[len(d.env.nuclioDestDir)+1:]

	// copy processor.yaml for all runtumes
	d.env.config.Build.Copy[filepath.Join(relativeUserFunctionPath, processorConfigFileName)] = "/etc/nuclio"

	// if we're python, copy in the wrapper script and handler script to /opt/nuclio
	if true {
		d.env.config.Build.Copy[filepath.Join(relativeUserFunctionPath, "my_handler.py")] = "/opt/nuclio/"
		d.env.config.Build.Copy[filepath.Join("pkg", "processor", "runtime", "python", "wrapper.py")] = "/opt/nuclio/"
	}

	buildContext := d.env.getNuclioDir()
	//if err := d.copyFiles(handlerPath, buildContext); err != nil {
	//	return errors.Wrapf(err, "Can't copy files from %q to %q", handlerPath, buildContext)
	//}

	dockerfile, err := d.createProcessorDockerfile()
	if err != nil {
		return errors.Wrap(err, "Can't create Dockerfile")
	}

	options := buildOptions{
		Tag:        d.env.outputName,
		Dockerfile: dockerfile,
	}

	if err := d.doBuild(d.env.outputName, buildContext, &options); err != nil {
		return errors.Wrap(err, "Failed to build image")
	}

	if d.env.options.PushRegistry != "" {
		return d.pushImage(d.env.outputName, d.env.options.PushRegistry)
	}

	return nil
}

func (d *dockerHelper) close() {
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
