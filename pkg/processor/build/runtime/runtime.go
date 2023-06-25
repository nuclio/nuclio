/*
Copyright 2023 The Nuclio Authors.

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

package runtime

import (
	"fmt"
	"path"
	"strings"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/build/runtimeconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/v3io/version-go"
)

type ProcessorDockerfileInfo struct {
	BaseImage          string
	ImageArtifactPaths map[string]string
	OnbuildArtifacts   []Artifact
	Directives         map[string][]functionconfig.Directive
	DockerfileContents string
	DockerfilePath     string
	BuildArgs          map[string]string
}

type Artifact struct {
	Name          string
	Image         string
	Paths         map[string]string
	ExternalImage bool
}

type Runtime interface {
	// DetectFunctionHandlers returns a list of all the handlers
	// in that directory given a path holding a function (or functions)
	DetectFunctionHandlers(functionPath string) ([]string, error)

	// OnAfterStagingDirCreated prepares anything it may need in that directory
	// towards building a functioning processor,
	OnAfterStagingDirCreated(runtimeConfig *runtimeconfig.Config, stagingDir string) error

	// GetProcessorDockerfileInfo returns information required to build the processor Dockerfile
	GetProcessorDockerfileInfo(runtimeConfig *runtimeconfig.Config, onbuildImageRegistry string) (*ProcessorDockerfileInfo, error)

	// GetName returns the name of the runtime, including version if applicable
	GetName() string

	// GetHandlerDirObjectPaths returns the paths of all objects that should reside in the handler
	// directory
	GetHandlerDirObjectPaths() []string

	// GetOverrideImageRegistryFromMap returns an override image for the runtime from the given map
	GetOverrideImageRegistryFromMap(map[string]string) string

	// GetRuntimeBuildArgs returns building arguments
	GetRuntimeBuildArgs(runtimeConfig *runtimeconfig.Config) map[string]string
}

type Factory interface {
	Create(logger.Logger, string, string, *functionconfig.Config) (Runtime, error)
}

type AbstractRuntime struct {
	Logger         logger.Logger
	StagingDir     string
	FunctionConfig *functionconfig.Config
	DockerClient   dockerclient.Client
	CmdRunner      cmdrunner.CmdRunner
	VersionInfo    *version.Info
}

func NewAbstractRuntime(logger logger.Logger,
	containerBuilderKind string,
	stagingDir string,
	functionConfig *functionconfig.Config) (*AbstractRuntime, error) {
	var err error

	newRuntime := &AbstractRuntime{
		Logger:         logger,
		StagingDir:     stagingDir,
		FunctionConfig: functionConfig,
		VersionInfo:    version.Get(),
	}

	newRuntime.CmdRunner, err = cmdrunner.NewShellRunner(newRuntime.Logger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create command runner")
	}

	// create a docker client
	if containerBuilderKind == "docker" {
		newRuntime.DockerClient, err = dockerclient.NewShellClient(newRuntime.Logger, newRuntime.CmdRunner)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create docker client")
		}
	}

	return newRuntime, nil
}

func (ar *AbstractRuntime) OnAfterStagingDirCreated(runtimeConfig *runtimeconfig.Config, stagingDir string) error {
	return nil
}

// GetProcessorImageObjectPaths return a map of objects the runtime needs to copy into the processor image
// the key can be a dir, a file or a url of a file
// the value is an absolute path into the docker image
func (ar *AbstractRuntime) GetProcessorImageObjectPaths() map[string]string {
	return nil
}

func (ar *AbstractRuntime) GetFunctionDir() string {

	// if the function directory was passed, just return that. if the function path was passed, return the directory
	// the function is in
	if common.IsDir(ar.FunctionConfig.Spec.Build.Path) {
		return ar.FunctionConfig.Spec.Build.Path
	}

	return path.Dir(ar.FunctionConfig.Spec.Build.Path)
}

// GetHandlerDirObjectPaths returns the paths of all objects that should reside in the handler
// directory
func (ar *AbstractRuntime) GetHandlerDirObjectPaths() []string {

	// by default, just return the build path
	return []string{ar.FunctionConfig.Spec.Build.Path}
}

// DetectFunctionHandlers returns a list of all the handlers
// in that directory given a path holding a function (or functions)
func (ar *AbstractRuntime) DetectFunctionHandlers(functionPath string) ([]string, error) {

	// use the function path: /some/path/func.py -> func
	functionBuildPath := ar.FunctionConfig.Spec.Build.Path
	functionFileName := strings.TrimSuffix(path.Base(functionBuildPath), path.Ext(functionBuildPath))

	return []string{fmt.Sprintf("%s:%s", functionFileName, "handler")}, nil
}

func (ar *AbstractRuntime) GetOverrideImageRegistryFromMap(imagesOverrideMap map[string]string) string {
	runtimeName, runtimeVersion := common.GetRuntimeNameAndVersion(ar.FunctionConfig.Spec.Runtime)

	// supports both overrides per runtimeName and per runtimeName + runtimeVersion
	if runtimeVersion != "" {
		key := runtimeName + ":" + runtimeVersion
		if imageOverride, ok := imagesOverrideMap[key]; ok {
			return imageOverride
		}
	}

	// no version-specific override, or no known version for our runtime, try for runtimeName only
	if imageOverride, ok := imagesOverrideMap[runtimeName]; ok {
		return imageOverride
	}

	// no override found
	return ""
}

func (ar *AbstractRuntime) GetRuntimeBuildArgs(runtimeConfig *runtimeconfig.Config) map[string]string {
	if runtimeConfig != nil && runtimeConfig.Common != nil {
		return runtimeConfig.Common.BuildArgs
	}
	return map[string]string{}
}
