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
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime/golang/eventhandlerparser"
	"github.com/nuclio/nuclio/pkg/processor/build/util"
)

const (
	processorBuilderDockerfileName = "Dockerfile.processor-builder-golang"
	handlerBuilderDockerfileName   = "Dockerfile.handler-builder-golang"
	processorBuilderImageName      = "nuclio/processor-builder-golang:latest"
	handlerOnBuildImageName        = "nuclio/processor-builder-golang-onbuild"
	handlerBuilderImageName        = "nuclio/handler-builder-golang:latest"
	processorImageName             = "nuclio/processor-golang:latest"

	processorConfigTemplate = `
function:
  kind: "golang"
  name: %s
  version: "latest"
  handler: %s`
)

type golang struct {
	*runtime.AbstractRuntime
	handlerName string
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

	g.handlerName = handlers[0] // Keep it for GetProcessorConfigFileContents
	return []string{handlers[0]}, nil
}

func (g *golang) GetProcessorImageObjectPaths() map[string]string {
	return map[string]string{
		path.Join(g.Configuration.GetStagingDir(), "processor"):  "/usr/local/bin/processor",
		path.Join(g.Configuration.GetStagingDir(), "handler.so"): "/opt/nuclio/handler.so",
	}
}

// given a staging directory, prepares anything it may need in that directory
// towards building a functioning processor
func (g *golang) OnAfterStagingDirCreated(stagingDir string) error {

	// copy the function source into the appropriate location
	if err := g.createUserFunctionPath(stagingDir); err != nil {
		return errors.Wrap(err, "Failed to create user function path")
	}

	// build the handler plugin
	if err := g.buildHandlerPlugin(stagingDir); err != nil {
		return errors.Wrap(err, "Failed to build handler plugin")
	}

	// get processor binary
	if err := g.getProcessorBinary(stagingDir); err != nil {
		return errors.Wrap(err, "Failed to get processor binary")
	}

	return nil
}

func (g *golang) GetExtension() string {
	return "go"
}

// get the string that signifies a comment if appears at the beginning of the line
func (g *golang) GetCommentPattern() string {
	return "//"
}

func (g *golang) createUserFunctionPath(stagingDir string) error {
	userFunctionPathInStaging := filepath.Join(stagingDir, "handler")
	g.Logger.DebugWith("Creating user function path", "path", userFunctionPathInStaging)

	if err := os.MkdirAll(userFunctionPathInStaging, 0755); err != nil {
		return errors.Wrapf(err, "Failed to create user function path in staging at %s", userFunctionPathInStaging)
	}

	copyFrom := g.Configuration.GetFunctionDir()
	g.Logger.DebugWith("Copying user function", "from", copyFrom, "to", userFunctionPathInStaging)
	_, err := util.CopyDir(copyFrom, userFunctionPathInStaging)
	if err != nil {
		return errors.Wrapf(err, "Error copying from %s to %s", copyFrom, userFunctionPathInStaging)
	}

	return nil
}

func (g *golang) parseGitURL(url string) (string, *string) {
	urlAndRef := strings.Split(url, "#")
	if len(urlAndRef) == 2 {
		return urlAndRef[0], &urlAndRef[1]
	}

	return url, nil
}

func (g *golang) buildHandlerPlugin(stagingDir string) error {
	g.Logger.InfoWith("Building handler binary (dockerized)")

	if !g.Configuration.GetNoBaseImagePull() {
		// pull the onbuild image we need to build the processor builder
		if err := g.DockerClient.PullImage(handlerOnBuildImageName); err != nil {
			return errors.Wrap(err, "Failed to pull onbuild image for golang")
		}
	}

	handlerPkgPathFilePath := path.Join(stagingDir, "handler-pkg-path.txt")
	// TODO: Add build option for handler path in $GOPATH
	if err := ioutil.WriteFile(handlerPkgPathFilePath, []byte("handler"), 0644); err != nil {
		return err
	}

	dockerfilePath := path.Join(stagingDir, handlerBuilderDockerfileName)
	dockerText := fmt.Sprintf("FROM %s", handlerOnBuildImageName)
	if err := ioutil.WriteFile(dockerfilePath, []byte(dockerText), 0644); err != nil {
		return err
	}

	// build the handler
	if err := g.DockerClient.Build(&dockerclient.BuildOptions{
		ImageName:      handlerBuilderImageName,
		DockerfilePath: dockerfilePath,
	}); err != nil {
		return errors.Wrap(err, "Failed to build handler")
	}

	// delete the image when we're done
	defer g.DockerClient.RemoveImage(handlerBuilderImageName)

	// the staging paths of the files we want to copy
	handlerBinaryPathInStaging := path.Join(stagingDir, "handler.so")
	handlerBuildLogPathInStaging := path.Join(stagingDir, "handler_build.log")

	// copy artifacts from the image we build - these directories are defined
	// in the onbuild dockerfile we allow copy errors because processor may not
	// exist
	objectsToCopy := map[string]string{
		"/handler.so":        handlerBinaryPathInStaging,
		"/handler_build.log": handlerBuildLogPathInStaging,
	}

	if err := g.DockerClient.CopyObjectsFromImage(handlerBuilderImageName, objectsToCopy, true); err != nil {
		return errors.Wrap(err, "Failed to copy objects from image")
	}

	// if handler doesn't exist, return why the build failed
	if !common.FileExists(handlerBinaryPathInStaging) {
		// read the build log
		handlerBuildLogContents, err := ioutil.ReadFile(handlerBuildLogPathInStaging)
		if err != nil {
			return errors.Wrap(err, "Failed to read build log contents")
		}

		// log the error
		g.Logger.ErrorWith("Failed to build function", "error", string(handlerBuildLogContents))

		return errors.Errorf("Failed to build function:\n%s", string(handlerBuildLogContents))
	}

	g.Logger.DebugWith("Successfully built and copied handler plugin", "path", handlerBinaryPathInStaging)

	return nil
}

func (g *golang) getProcessorBinary(stagingDir string) error {
	if !g.Configuration.GetNoBaseImagePull() {
		// pull the onbuild image we need to build the processor builder
		if err := g.DockerClient.PullImage(processorImageName); err != nil {
			return errors.Wrap(err, "Failed to pull processor image for golang")
		}
	}

	processorBinaryPathInStaging := path.Join(stagingDir, "processor")

	objectsToCopy := map[string]string{
		"/usr/local/bin/processor": processorBinaryPathInStaging,
	}

	return g.DockerClient.CopyObjectsFromImage(processorImageName, objectsToCopy, false)
}

func (g *golang) GetProcessorConfigFileContents() string {
	return fmt.Sprintf(processorConfigTemplate, g.handlerName, g.handlerName)
}
