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

package pypy

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	"github.com/nuclio/nuclio/pkg/processor/build/util"
)

const (
	pypyProcessorImageName = "nuclio/processor-pypy"
)

type pypy struct {
	*runtime.AbstractRuntime
}

// GetDefaultProcessorBaseImageName returns the image name of the default processor base image
func (p *pypy) GetDefaultProcessorBaseImageName() string {
	baseImageName := "nuclio/processor-pypy-onbuild"

	// make sure the image exists. don't pull if instructed not to
	if !p.Configuration.GetNoBaseImagePull() {
		p.DockerClient.PullImage(baseImageName)
	}

	return baseImageName
}

// DetectFunctionHandlers returns a list of all the handlers
// in that directory given a path holding a function (or functions)
func (p *pypy) DetectFunctionHandlers(functionPath string) ([]string, error) {
	return []string{p.getFunctionHandler()}, nil
}

// GetProcessorImageObjectPaths returns a map of objects the runtime needs to copy into the processor image
// the key can be a dir, a file or a url of a file
// the value is an absolute path into the docker image
func (p *pypy) GetProcessorImageObjectPaths() map[string]string {
	return map[string]string{}
}

// GetExtension returns the source extension of the runtime (e.g. .go)
func (p *pypy) GetExtension() string {
	return "py"
}

// GetName returns the name of the runtime, including version if applicable
func (p *pypy) GetName() string {
	return "pypy"
}

func (p *pypy) getFunctionHandler() string {
	// use the function path: /some/path/func.py -> func
	functionFileName := path.Base(p.Configuration.GetFunctionPath())
	functionFileName = functionFileName[:len(functionFileName)-len(path.Ext(functionFileName))]

	// take that file name without extension and add a default "handler"
	// TODO: parse the python sources for this
	return fmt.Sprintf("%s:%s", functionFileName, "handler")
}

// OnAfterStagingDirCreated prepares anything it may need in that directory
// towards building a functioning processor,
func (p *pypy) OnAfterStagingDirCreated(stagingDir string) error {

	// build the processor binary into staging
	if err := p.getProcessorBinary(stagingDir); err != nil {
		return err
	}

	if err := p.copyInterfaceFile(stagingDir); err != nil {
		return err
	}

	if err := p.copyHandlerToStaging(stagingDir); err != nil {
		return err
	}

	if err := p.createDockerfile(stagingDir); err != nil {
		return err
	}

	return nil
}

func (p *pypy) copyInterfaceFile(stagingDir string) error {
	interfaceFileName := "nuclio_interface.py"

	interfaceFilePath := path.Join(
		p.Configuration.GetNuclioSourceDir(),
		"pkg/processor/runtime/pypy",
		interfaceFileName,
	)

	stagingInterfaceFilePath := path.Join(stagingDir, interfaceFileName)

	if err := util.CopyFile(interfaceFilePath, stagingInterfaceFilePath); err != nil {
		return errors.Wrap(err, "Can't copy interface file")
	}

	return nil
}

func (p *pypy) getProcessorBinary(stagingDir string) error {
	p.Logger.InfoWith("Building processor binary (dockerized)")

	// make sure the image exists. don't pull if instructed not to
	if !p.Configuration.GetNoBaseImagePull() {
		if err := p.DockerClient.PullImage(pypyProcessorImageName); err != nil {
			return errors.Wrap(err, "Failed to pull processor image for pypy")
		}
	}

	objectsToCopy := map[string]string{
		"/processor": path.Join(stagingDir, "processor"),
	}

	if err := p.DockerClient.CopyObjectsFromImage(pypyProcessorImageName, objectsToCopy, false); err != nil {
		return errors.Wrap(err, "Failed to copy objects from image")
	}

	return nil
}

func (p *pypy) copyHandlerToStaging(stagingDir string) error {
	handlerDirInStaging := path.Join(stagingDir, "handler")
	functionPath := p.Configuration.GetFunctionPath()

	if common.IsFile(functionPath) {
		if err := os.MkdirAll(handlerDirInStaging, 0755); err != nil {
			return err
		}
		handlerPath := path.Join(handlerDirInStaging, path.Base(functionPath))
		if err := util.CopyFile(functionPath, handlerPath); err != nil {
			return err
		}
	} else {
		if _, err := util.CopyDir(functionPath, handlerDirInStaging); err != nil {
			return err
		}
	}

	return nil
}

func (p *pypy) createDockerfile(stagingDir string) error {
	dockerCode := []byte("FROM nuclio/processor-pypy-onbuild")
	dockerfilePath := path.Join(stagingDir, "Dockerfile")
	if err := ioutil.WriteFile(dockerfilePath, dockerCode, 0666); err != nil {
		return errors.Wrap(err, "Can't create Dockerfile")
	}

	return nil
}
