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
	"github.com/nuclio/nuclio/pkg/version"
)

const (
	handlerBuilderImageName = "nuclio/handler-builder-golang"
)

type golang struct {
	*runtime.AbstractRuntime
}

// GetProcessorBaseImageName returns the image name of the default processor base image
func (g *golang) GetProcessorBaseImageName() (string, error) {
	return "alpine", nil
}

// DetectFunctionHandlers returns a list of all the handlers
// in that directory given a path holding a function (or functions)
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

	return []string{handlers[0]}, nil
}

// GetProcessorImageObjectPaths returns a map of objects the runtime needs to copy into the processor image
// the key can be a dir, a file or a url of a file
// the value is an absolute path into the docker image
func (g *golang) GetProcessorImageObjectPaths() map[string]string {
	return map[string]string{
		path.Join(g.StagingDir, "processor"):  "/usr/local/bin/processor",
		path.Join(g.StagingDir, "handler.so"): "/opt/nuclio/handler.so",
	}
}

// OnAfterStagingDirCreated prepares anything it may need in that directory
// towards building a functioning processor,
func (g *golang) OnAfterStagingDirCreated(stagingDir string) error {

	// copy the function source into the appropriate location
	if err := g.createUserFunctionPath(stagingDir); err != nil {
		return errors.Wrap(err, "Failed to create user function path")
	}

	// build the handler plugin. if successful, we'll have the processor binary and handler plugin
	// in the staging directory
	if err := g.buildHandlerPlugin(stagingDir); err != nil {
		return errors.Wrap(err, "Failed to build handler plugin")
	}

	return nil
}

// GetExtension returns the source extension of the runtime (e.g. .go)
func (g *golang) GetExtension() string {
	return "go"
}

// GetName returns the name of the runtime, including version if applicable
func (g *golang) GetName() string {
	return "golang"
}

func (g *golang) createUserFunctionPath(stagingDir string) error {
	userFunctionPathInStaging := filepath.Join(stagingDir, "handler")
	g.Logger.DebugWith("Creating user function path", "path", userFunctionPathInStaging)

	if err := os.MkdirAll(userFunctionPathInStaging, 0755); err != nil {
		return errors.Wrapf(err, "Failed to create user function path in staging at %s", userFunctionPathInStaging)
	}

	copyFrom := g.GetFunctionDir()
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
	g.Logger.InfoWith("Building handler plugin (dockerized)")

	// build the image that builds the handler. it will contain the handler when it's done
	// and/or a handler_build.log
	if err := g.buildHandlerBuilderImage(stagingDir); err != nil {
		return errors.Wrap(err, "Failed to build handler builder image")
	}

	// delete the image when we're done
	defer g.DockerClient.RemoveImage(handlerBuilderImageName)

	// the staging paths of the files we want to copy
	handlerBinaryPathInStaging := path.Join(stagingDir, "handler.so")
	handlerBuildLogPathInStaging := path.Join(stagingDir, "handler_build.log")
	processorBinaryPathInStaging := path.Join(stagingDir, "processor")

	// copy artifacts from the image we build - these directories are defined
	// in the onbuild dockerfile. we allow copy errors because processor may not
	// exist
	objectsToCopy := map[string]string{
		"/usr/local/bin/processor": processorBinaryPathInStaging,
		"/handler.so":              handlerBinaryPathInStaging,
		"/handler_build.log":       handlerBuildLogPathInStaging,
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

func (g *golang) buildHandlerBuilderImage(stagingDir string) error {

	versionInfo, err := version.Get()
	if err != nil {
		return errors.Wrap(err, "Failed to get version info")
	}

	handlerBuilderOnBuildImageName := fmt.Sprintf("nuclio/handler-builder-golang-onbuild:%s-%s",
		versionInfo.Label,
		versionInfo.Arch)

	if !g.FunctionConfig.Spec.Build.NoBaseImagesPull {

		// pull the onbuild image we need to build the processor builder
		if err := g.DockerClient.PullImage(handlerBuilderOnBuildImageName); err != nil {
			return errors.Wrap(err, "Failed to pull onbuild image for golang")
		}
	}

	// write a file indicating where exactly the handler package resides
	handlerPkgPathFilePath := path.Join(stagingDir, "handler-pkg-path.txt")
	// TODO: Add build option for handler path in $GOPATH
	if err := ioutil.WriteFile(handlerPkgPathFilePath, []byte("handler"), 0644); err != nil {
		return err
	}

	handlerBuilderDockerfilePath := path.Join(stagingDir, "Dockerfile.handler-builder-golang")
	handlerBuilderDockerfileContents := fmt.Sprintf("FROM %s", handlerBuilderOnBuildImageName)
	if err := ioutil.WriteFile(handlerBuilderDockerfilePath,
		[]byte(handlerBuilderDockerfileContents),
		0644); err != nil {
		return err
	}

	// build the handler
	if err := g.DockerClient.Build(&dockerclient.BuildOptions{
		ImageName:      handlerBuilderImageName,
		DockerfilePath: handlerBuilderDockerfilePath,
	}); err != nil {
		return errors.Wrap(err, "Failed to build handler")
	}

	return nil
}
