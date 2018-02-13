/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the Licensd.
You may obtain a copy of the License at

    http://www.apachd.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dotnetcore

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
	"github.com/nuclio/nuclio/pkg/processor/build/util"
	"github.com/nuclio/nuclio/pkg/version"
)

const (
	handlerBuilderImageName = "nuclio/handler-builder-dotnetcore"
)

type dotnetcore struct {
	*runtime.AbstractRuntime
}

// GetProcessorBaseImageName returns the image name of the default processor base image
func (d *dotnetcore) GetProcessorBaseImageName() (string, error) {
	return "microsoft/dotnet:2-runtime", nil
}

// DetectFunctionHandlers returns a list of all the handlers
// in that directory given a path holding a function (or functions)
func (d *dotnetcore) DetectFunctionHandlers(functionPath string) ([]string, error) {
	return []string{d.getFunctionHandler()}, nil
}

func (d *dotnetcore) getFunctionHandler() string {

	// use the function path: /some/path/func.py -> func
	functionFileName := path.Base(d.FunctionConfig.Spec.Build.Path)
	functionFileName = functionFileName[:len(functionFileName)-len(path.Ext(functionFileName))]

	// take that file name without extension and add a default "handler"
	return fmt.Sprintf("%s:%s", functionFileName, "handler")
}

// GetProcessorImageObjectPaths returns a map of objects the runtime needs to copy into the processor image
// the key can be a dir, a file or a url of a file
// the value is an absolute path into the docker image
func (d *dotnetcore) GetProcessorImageObjectPaths() map[string]string {
	return map[string]string{
		path.Join(d.StagingDir, "processor"):             "/usr/local/bin/processor",
		path.Join(d.StagingDir, "wrapper"):               "/opt/nuclio/wrapper",
		path.Join(d.StagingDir, "nuclio-sdk-dotnetcore"): "/opt/nuclio/nuclio-sdk-dotnetcore",
		path.Join(d.StagingDir, "handler"):               "/opt/nuclio/handler",
	}
}

// OnAfterStagingDirCreated prepares anything it may need in that directory
// towards building a functioning processor,
func (d *dotnetcore) OnAfterStagingDirCreated(stagingDir string) error {

	// copy the function source into the appropriate location
	if err := d.createUserFunctionPath(stagingDir); err != nil {
		return errors.Wrap(err, "Failed to create user function path")
	}

	// build the handler plugin. if successful, we'll have the processor binary and handler plugin
	// in the staging directory
	if err := d.buildHandlerPlugin(stagingDir); err != nil {
		return errors.Wrap(err, "Failed to build handler plugin")
	}

	return nil
}

// GetName returns the name of the runtime, including version if applicable
func (d *dotnetcore) GetName() string {
	return "dotnetcore"
}

func (d *dotnetcore) createUserFunctionPath(stagingDir string) error {
	userFunctionPathInStaging := filepath.Join(stagingDir, "/function")
	d.Logger.DebugWith("Creating user function path", "path", userFunctionPathInStaging)

	if err := os.MkdirAll(userFunctionPathInStaging, 0755); err != nil {
		return errors.Wrapf(err, "Failed to create user function path in staging at %s", userFunctionPathInStaging)
	}

	copyFrom := d.GetFunctionDir()
	d.Logger.DebugWith("Copying user function", "from", copyFrom, "to", userFunctionPathInStaging)

	_, err := util.CopyDir(copyFrom, userFunctionPathInStaging)
	if err != nil {
		return errors.Wrapf(err, "Error copying from %s to %s", copyFrom, userFunctionPathInStaging)
	}

	return nil
}

func (d *dotnetcore) parseGitURL(url string) (string, *string) {
	urlAndRef := strings.Split(url, "#")
	if len(urlAndRef) == 2 {
		return urlAndRef[0], &urlAndRef[1]
	}

	return url, nil
}

func (d *dotnetcore) buildHandlerPlugin(stagingDir string) error {

	// build the image that builds the handler. it will contain the handler when it's done
	// and/or a handler_build.log
	if err := d.buildHandlerBuilderImage(stagingDir); err != nil {
		return errors.Wrap(err, "Failed to build handler builder image")
	}

	// delete the image when we're done
	defer d.DockerClient.RemoveImage(handlerBuilderImageName)

	// the staging paths of the files we want to copy
	handlerBinaryPathInStaging := path.Join(stagingDir, "handler")
	sdkPathInStaging := path.Join(stagingDir, "nuclio-sdk-dotnetcore")
	wrapperBinaryPathInStaging := path.Join(stagingDir, "wrapper")
	processorBinaryPathInStaging := path.Join(stagingDir, "processor")
	handlerBuildLogPathInStaging := path.Join(stagingDir, "handler_build.log")

	// copy artifacts from the image we build - these directories are defined
	// in the onbuild dockerfile. we allow copy errors because processor may not
	// exist
	objectsToCopy := map[string]string{
		"/usr/local/bin/processor":          processorBinaryPathInStaging,
		"/opt/nuclio/handler":               handlerBinaryPathInStaging,
		"/opt/nuclio/wrapper":               wrapperBinaryPathInStaging,
		"/opt/nuclio/nuclio-sdk-dotnetcore": sdkPathInStaging,
	}

	if err := d.DockerClient.CopyObjectsFromImage(handlerBuilderImageName, objectsToCopy, false); err != nil {
		return errors.Wrap(err, "Failed to copy objects from image")
	}

	// if handler doesn't exist, return why the build failed
	if !common.FileExists(handlerBinaryPathInStaging) {

		// read the build log
		handlerBuildLogContents, err := ioutil.ReadFile(handlerBuildLogPathInStaging)
		if err != nil {
			return errors.Wrap(err, "Failed to read build log contents")
		}

		return errors.Errorf("Failed to build function:\n%s", string(handlerBuildLogContents))
	}

	d.Logger.DebugWith("Successfully built and copied handler plugin", "path", handlerBinaryPathInStaging)

	return nil
}

func (d *dotnetcore) buildHandlerBuilderImage(stagingDir string) error {

	versionInfo, err := version.Get()
	if err != nil {
		return errors.Wrap(err, "Failed to get version info")
	}

	handlerBuilderOnBuildImageName := fmt.Sprintf("nuclio/handler-builder-dotnetcore-onbuild:%s-%s",
		versionInfo.Label,
		versionInfo.Arch)
	d.Logger.Info("Building from %s", handlerBuilderOnBuildImageName)
	if !d.FunctionConfig.Spec.Build.NoBaseImagesPull {

		// pull the onbuild image we need to build the processor builder
		if err := d.DockerClient.PullImage(handlerBuilderOnBuildImageName); err != nil {
			return errors.Wrap(err, "Failed to pull onbuild image for dotnetcore")
		}
	}

	// write a file indicating where exactly the handler package resides
	handlerPkgPathFilePath := path.Join(stagingDir, "handler-pkg-path.txt")
	if err := ioutil.WriteFile(handlerPkgPathFilePath, []byte("handler"), 0644); err != nil {
		return err
	}

	handlerBuilderDockerfilePath := path.Join(stagingDir, "Dockerfile.handler-builder-dotnetcore")
	handlerBuilderDockerfileContents := fmt.Sprintf("FROM %s", handlerBuilderOnBuildImageName)
	if err := ioutil.WriteFile(handlerBuilderDockerfilePath,
		[]byte(handlerBuilderDockerfileContents),
		0644); err != nil {
		return err
	}

	d.Logger.Info("Building handler Dotnetcore plugin")

	// build the handler
	if err := d.DockerClient.Build(&dockerclient.BuildOptions{
		ImageName:      handlerBuilderImageName,
		DockerfilePath: handlerBuilderDockerfilePath,
	}); err != nil {
		return errors.Wrap(err, "Failed to build handler")
	}

	return nil
}
