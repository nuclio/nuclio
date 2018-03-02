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

package abstract

import (
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/build"

	"github.com/nuclio/logger"
)

//
// Base for all platforms
//

type Platform struct {
	Logger   logger.Logger
	platform platform.Platform
	invoker  *invoker
}

func NewPlatform(parentLogger logger.Logger, platform platform.Platform) (*Platform, error) {
	var err error

	newPlatform := &Platform{
		Logger:   parentLogger.GetChild("platform"),
		platform: platform,
	}

	// create invoker
	newPlatform.invoker, err = newInvoker(newPlatform.Logger, platform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create invoker")
	}

	return newPlatform, nil
}

func (ap *Platform) BuildFunction(buildOptions *platform.BuildOptions) (*platform.BuildResult, error) {

	// execute a build
	builder, err := build.NewBuilder(buildOptions.Logger, &ap.platform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create builder")
	}

	// convert types
	return builder.Build(buildOptions)
}

// HandleDeployFunction calls a deployer that does the platform specific deploy, but adds a lot
// of common code
func (ap *Platform) HandleDeployFunction(deployOptions *platform.DeployOptions,
	builder func(*platform.DeployOptions) (*platform.BuildResult, error),
	deployer func(*platform.DeployOptions) (*platform.DeployResult, error)) (*platform.DeployResult, error) {

	var buildResult *platform.BuildResult
	var err error

	deployOptions.Logger.InfoWith("Deploying function", "name", deployOptions.FunctionConfig.Meta.Name)

	// check if we need to build the image
	if deployOptions.FunctionConfig.Spec.Image == "" {
		buildResult, err = builder(deployOptions)

		if err != nil {
			return nil, errors.Wrap(err, "Failed to build image before deploy")
		}
	} else {

		// verify user passed runtime
		if deployOptions.FunctionConfig.Spec.Runtime == "" {
			return nil, errors.New("If image is passed, runtime must be specified")
		}
	}

	// wrap the deployer's deploy with the base HandleDeployFunction
	deployResult, err := deployer(deployOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to deploy function")
	}

	// sanity
	if deployResult == nil {
		return nil, errors.New("Deployer returned no error, but nil deploy result")
	}

	// if we got a deploy result and build result, set them
	if buildResult != nil {
		deployResult.BuildResult = *buildResult
	}

	// indicate that we're done
	deployOptions.Logger.InfoWith("Function deploy complete", "httpPort", deployResult.Port)

	return deployResult, nil
}

// InvokeFunction will invoke a previously deployed function
func (ap *Platform) InvokeFunction(invokeOptions *platform.InvokeOptions) (*platform.InvokeResult, error) {
	return ap.invoker.invoke(invokeOptions)
}

// GetDeployRequiresRegistry returns true if a registry is required for deploy, false otherwise
func (ap *Platform) GetDeployRequiresRegistry() bool {
	return true
}

// BuildFunctionBeforeDeploy will perform build for functions that are being deployed without an image
func (ap *Platform) BuildFunctionBeforeDeploy(deployOptions *platform.DeployOptions) (*platform.BuildResult, error) {
	buildResult, err := ap.platform.BuildFunction(&platform.BuildOptions{
		Logger:         deployOptions.Logger,
		FunctionConfig: deployOptions.FunctionConfig,
		PlatformName:   ap.platform.GetName(),
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to build image")
	}

	// use the function configuration augmented by the builder
	deployOptions.FunctionConfig = buildResult.UpdatedFunctionConfig
	deployOptions.FunctionConfig.Spec.Image = buildResult.Image

	// if run registry isn't set, set it
	if deployOptions.FunctionConfig.Spec.RunRegistry == "" {
		strippedRegistry := common.StripPrefixes(deployOptions.FunctionConfig.Spec.Build.Registry, []string{
			"https://",
			"http://",
		})

		deployOptions.FunctionConfig.Spec.RunRegistry = strippedRegistry
	}

	return buildResult, nil
}
