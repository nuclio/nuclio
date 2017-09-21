/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the Licensg.
You may obtain a copy of the License at

    http://www.apachg.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the Licensg.
*/

package golang

import (
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime/golang/eventhandlerparser"
	"github.com/nuclio/nuclio/pkg/processor/build/util"
	"github.com/nuclio/nuclio/pkg/util/cmdrunner"
	"github.com/nuclio/nuclio/pkg/util/common"
)

const (
	processorBuilderDockerfileName = "Dockerfile.processor-builder-golang"
	processorBuilderImageName      = "nuclio/processor-builder-golang:latest"
)

type golang struct {
	*runtime.AbstractRuntime
	functionPackage string
}

// returns the image name of the default processor base image
func (g *golang) GetDefaultProcessorBaseImageName() string {
	return "alpine"
}

// given a path holding a function (or functions) returns a list of all the handlers
// in that directory
func (g *golang) DetectFunctionHandlers(functionPath string) ([]string, error) {
	parser := eventhandlerparser.NewEventHandlerParser(g.Logger)

	packages, handlers, err := parser.ParseEventHandlers(functionPath)
	if err != nil {
		return nil, errors.Wrapf(err, "Can't find handlers in %q", functionPath)
	}

	g.Logger.DebugWith("Parsed event handlers", "packages", packages, "handlers", handlers)

	if len(handlers) != 1 {
		return nil, errors.Wrapf(err, "Expected one handler, found %d", len(handlers))
	}

	if len(packages) != 1 {
		return nil, errors.Wrapf(err, "Expected one package, found %d", len(packages))
	}

	// set the package of the function
	g.functionPackage = packages[0]

	return []string{handlers[0]}, nil
}

func (g *golang) GetProcessorImageObjectPaths() map[string]string {

	// return the processor binary we generated as something we want in the image
	return map[string]string{
		path.Join(g.Configuration.GetStagingDir(), "processor"): path.Join("usr", "local", "bin", "processor"),
	}
}

// given a staging directory, prepares anything it may need in that directory
// towards building a functioning processor
func (g *golang) OnAfterStagingDirCreated(stagingDir string) error {

	// get nuclio source code to staging
	if err := g.getNuclioSource(stagingDir); err != nil {
		return errors.Wrap(err, "Failed to get nuclio source")
	}

	// copy the function source into the appropriate location in the staged nuclio source
	if _, err := g.createUserFunctionPath(stagingDir); err != nil {
		return errors.Wrap(err, "Failed to create user function path")
	}

	// build the processor binary into staging
	return g.buildProcessorBinary(stagingDir)
}

func (g *golang) GetExtension() string {
	return "go"
}

// get the string that signifies a comment if appears at the beginning of the line
func (g *golang) GetCommentPattern() string {
	return "//"
}

func (g *golang) createUserFunctionPath(stagingDir string) (string, error) {
	nuclioSourceDirInStaging := g.getNuclioSourceDirInStaging(stagingDir)

	userFunctionPathInStaging := filepath.Join(nuclioSourceDirInStaging, "cmd", "processor", "user_functions")
	g.Logger.DebugWith("Creating user function path", "path", userFunctionPathInStaging)

	// shell out to mkdir
	if _, err := g.CmdRunner.Run(nil, "mkdir -p %s", userFunctionPathInStaging); err != nil {
		return "", errors.Wrapf(err, "Failed to create user function path in staging at %s", userFunctionPathInStaging)
	}

	// copy from the directory the source is in to the user_functions directory in staging, under a directory
	// named after the function
	copyFrom := g.Configuration.GetFunctionDir()
	copyTo := filepath.Join(userFunctionPathInStaging, g.Configuration.GetFunctionName())

	g.Logger.DebugWith("Copying user function", "from", copyFrom, "to", copyTo)
	_, err := util.CopyDir(copyFrom, copyTo)
	if err != nil {
		return "", errors.Wrapf(err, "Error copying from %s to %s", copyFrom, copyTo)
	}

	// create a registry file
	return copyTo, g.createRegistryFile(filepath.Join(nuclioSourceDirInStaging, "cmd", "processor"))
}

func (g *golang) getNuclioSource(stagingDir string) error {
	nuclioSourceDirInStaging := g.getNuclioSourceDirInStaging(stagingDir)

	if g.Configuration.GetNuclioSourceDir() == "" {
		url, ref := g.parseGitUrl(g.Configuration.GetNuclioSourceURL())

		_, err := g.CmdRunner.Run(nil, "git clone %s %s", url, nuclioSourceDirInStaging)
		if err != nil {
			return errors.Wrap(err, "Unable to clone nuclio")
		}

		if ref != nil {
			_, err := g.CmdRunner.Run(&cmdrunner.RunOptions{WorkingDir: &nuclioSourceDirInStaging}, "git checkout %s", *ref)
			if err != nil {
				return errors.Wrapf(err, "Unable to checkout nuclio ref %s", *ref)
			}
		}
	} else {
		_, err := g.CmdRunner.Run(nil, "cp -R %s %s", g.Configuration.GetNuclioSourceDir(), nuclioSourceDirInStaging)
		if err != nil {
			return errors.Wrap(err, "Unable to copy nuclio from local directory")
		}
	}

	g.Logger.Debug("Completed getting nuclio source")

	return nil
}

func (g *golang) parseGitUrl(url string) (string, *string) {
	urlAndRef := strings.Split(url, "#")
	if len(urlAndRef) == 2 {
		return urlAndRef[0], &urlAndRef[1]
	}

	return url, nil
}

func (g *golang) createRegistryFile(path string) error {
	registryFileTemplateFuncs := template.FuncMap{
		"functionName":    g.Configuration.GetFunctionName,
		"functionPackage": func() string { return g.functionPackage },
		"functionHandler": g.Configuration.GetFunctionHandler,
	}

	t, err := template.New("registry").Funcs(registryFileTemplateFuncs).Parse(registryFileTemplate)

	if err != nil {
		return errors.Wrap(err, "Unable to create registry template")
	}

	registryFilePath := filepath.Join(path, "nuclio_user_functions__"+strings.ToLower(g.Configuration.GetFunctionName())+".go")
	g.Logger.DebugWith("Writing registry file", "path", registryFilePath)

	var buffer bytes.Buffer
	if err := t.Execute(&buffer, nil); err != nil {
		return err
	}

	return ioutil.WriteFile(registryFilePath, buffer.Bytes(), 0644)
}

func (g *golang) getNuclioSourceDirInStaging(stagingDir string) string {
	return path.Join(stagingDir, "nuclio")
}

func (g *golang) buildProcessorBinary(stagingDir string) error {
	g.Logger.InfoWith("Building processor binary (dockerized)")

	// make sure the image exists. don't pull if instructed not to
	if !g.Configuration.GetNoBaseImagePull() {

		// pull the onbuild image we need to build the processor builder
		if err := g.DockerClient.PullImage("nuclio/processor-builder-golang-onbuild"); err != nil {
			return errors.Wrap(err, "Failed to pull onbuild image for golang")
		}
	}

	// processor builder is an image that simply triggers the onbuild - copies the source from the
	// staged nuclio source dir and builds that
	processorBuilderDockerfilePath := path.Join(stagingDir, processorBuilderDockerfileName)

	// create the processor builder dockerfile @ the staging dir
	if err := ioutil.WriteFile(processorBuilderDockerfilePath,
		[]byte(processorBuilderDockerfileTemplate),
		os.FileMode(0644)); err != nil {
		return errors.Wrap(err, "Failed to create processor builder dockerfile")
	}

	// build the processor builder image
	if err := g.DockerClient.Build(&dockerclient.BuildOptions{
		ImageName:      processorBuilderImageName,
		DockerfilePath: processorBuilderDockerfilePath,
	}); err != nil {
		return errors.Wrap(err, "Failed to build processor builder")
	}

	// delete the processor builder image when we're done
	defer g.DockerClient.RemoveImage(processorBuilderImageName)

	// the staging paths of the files we want to copy
	processorBinaryPathInStaging := path.Join(stagingDir, "processor")
	processorBuildLogPathInStaging := path.Join(stagingDir, "processor_build.log")

	// copy artifacts from the image we build - these directories are defined in the onbuild dockerfile
	// we allow copy errors because processor may not exist
	if err := g.DockerClient.CopyObjectsFromImage(processorBuilderImageName, map[string]string{
		path.Join("go", "bin", "processor"): processorBinaryPathInStaging,
		"processor_build.log":               processorBuildLogPathInStaging,
	}, true); err != nil {
		return errors.Wrap(err, "Failed to copy objects from image")
	}

	// if processor doesn't exist, return why it failed compiling
	if !common.FileExists(processorBinaryPathInStaging) {

		// read the build log
		processorBuildLogContents, err := ioutil.ReadFile(path.Join(processorBuildLogPathInStaging))
		if err != nil {
			return errors.Wrap(err, "Failed to read build log contents")
		}

		// log the error
		g.Logger.ErrorWith("Failed to build function", "error", string(processorBuildLogContents))

		return errors.Errorf("Failed to build function:\n%s", string(processorBuildLogContents))
	}

	g.Logger.DebugWith("Successfully built and copied processor binary", "path", processorBinaryPathInStaging)

	return nil
}
