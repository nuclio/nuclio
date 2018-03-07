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
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
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

func (ap *Platform) CreateFunctionBuild(createFunctionBuildOptions *platform.CreateFunctionBuildOptions) (*platform.CreateFunctionBuildResult, error) {

	// execute a build
	builder, err := build.NewBuilder(createFunctionBuildOptions.Logger, &ap.platform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create builder")
	}

	// convert types
	return builder.Build(createFunctionBuildOptions)
}

// HandleDeployFunction calls a deployer that does the platform specific deploy, but adds a lot
// of common code
func (ap *Platform) HandleDeployFunction(createFunctionOptions *platform.CreateFunctionOptions,
	onAfterConfigUpdated func(*functionconfig.Config) error,
	onAfterBuild func(*platform.CreateFunctionBuildResult, error) (*platform.CreateFunctionResult, error)) (*platform.CreateFunctionResult, error) {

	createFunctionOptions.Logger.InfoWith("Deploying function", "name", createFunctionOptions.FunctionConfig.Meta.Name)

	var buildResult *platform.CreateFunctionBuildResult
	var buildErr error

	// when the config is updated, save to deploy options and call underlying hook
	onAfterConfigUpdatedWrapper := func(updatedFunctionConfig *functionconfig.Config) error {
		createFunctionOptions.FunctionConfig = *updatedFunctionConfig

		return onAfterConfigUpdated(updatedFunctionConfig)
	}

	// check if we need to build the image
	if createFunctionOptions.FunctionConfig.Spec.Image == "" {
		buildResult, buildErr = ap.platform.CreateFunctionBuild(&platform.CreateFunctionBuildOptions{
			Logger:              createFunctionOptions.Logger,
			FunctionConfig:      createFunctionOptions.FunctionConfig,
			PlatformName:        ap.platform.GetName(),
			OnAfterConfigUpdate: onAfterConfigUpdatedWrapper,
		})

		if buildErr != nil {
			return nil, errors.Wrap(buildErr, "Failed to build image")
		}

		// use the function configuration augmented by the builder
		createFunctionOptions.FunctionConfig.Spec.Image = buildResult.Image

		// if run registry isn't set, set it to that of the build
		if createFunctionOptions.FunctionConfig.Spec.RunRegistry == "" {
			createFunctionOptions.FunctionConfig.Spec.RunRegistry = createFunctionOptions.FunctionConfig.Spec.Build.Registry
		}
	} else {

		// verify user passed runtime
		if createFunctionOptions.FunctionConfig.Spec.Runtime == "" {
			return nil, errors.New("If image is passed, runtime must be specified")
		}

		// trigger the on after config update ourselves
		if err := onAfterConfigUpdatedWrapper(&createFunctionOptions.FunctionConfig); err != nil {
			return nil, errors.Wrap(err, "Failed to trigger on after config update")
		}
	}

	// wrap the deployer's deploy with the base HandleDeployFunction
	deployResult, err := onAfterBuild(buildResult, buildErr)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to deploy function")
	}

	// sanity
	if deployResult == nil {
		return nil, errors.New("Deployer returned no error, but nil deploy result")
	}

	// if we got a deploy result and build result, set them
	if buildResult != nil {
		deployResult.CreateFunctionBuildResult = *buildResult
	}

	// indicate that we're done
	createFunctionOptions.Logger.InfoWith("Function deploy complete", "httpPort", deployResult.Port)

	return deployResult, nil
}

// CreateFunctionInvocation will invoke a previously deployed function
func (ap *Platform) CreateFunctionInvocation(createFunctionInvocationOptions *platform.CreateFunctionInvocationOptions) (*platform.CreateFunctionInvocationResult, error) {
	return ap.invoker.invoke(createFunctionInvocationOptions)
}

// GetDeployRequiresRegistry returns true if a registry is required for deploy, false otherwise
func (ap *Platform) GetDeployRequiresRegistry() bool {
	return true
}
