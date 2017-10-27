package abstract

import (
	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/build"
	"io"
)

//
// Base for all platforms
//

type AbstractPlatform struct {
	Logger   nuclio.Logger
	platform platform.Platform
	invoker  *invoker
}

func NewAbstractPlatform(parentLogger nuclio.Logger, platform platform.Platform) (*AbstractPlatform, error) {
	var err error

	newAbstractPlatform := &AbstractPlatform{
		Logger:   parentLogger.GetChild("platform").(nuclio.Logger),
		platform: platform,
	}

	// create invoker
	newAbstractPlatform.invoker, err = newInvoker(newAbstractPlatform.Logger, platform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create invoker")
	}

	return newAbstractPlatform, nil
}

func (ap *AbstractPlatform) BuildFunction(buildOptions *platform.BuildOptions) (*platform.BuildResult, error) {

	// execute a build
	builder, err := build.NewBuilder(buildOptions.GetLogger(ap.Logger))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create builder")
	}

	// convert types
	result, err := builder.Build(buildOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to build")
	}

	return &platform.BuildResult{
		ImageName:              result.ImageName,
		Runtime:                result.Runtime,
		Handler:                result.Handler,
		FunctionConfigContents: result.FunctionConfigContents,
	}, nil
}

// HandleDeployFunction calls a deployer that does the platform specific deploy, but adds a lot
// of common code
func (ap *AbstractPlatform) HandleDeployFunction(deployOptions *platform.DeployOptions,
	deployer func() (*platform.DeployResult, error)) (*platform.DeployResult, error) {

	var buildResult *platform.BuildResult
	var err error

	// get the logger we need to deploy with
	logger := deployOptions.GetLogger(ap.Logger)

	logger.InfoWith("Deploying function", "name", deployOptions.Identifier)

	// first, check if the function exists so that we can delete it
	functions, err := ap.platform.GetFunctions(platform.NewGetOptions(deployOptions.CommonOptions))

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function")
	}

	// if the function exists, delete it
	if len(functions) > 0 {
		logger.InfoWith("Function already exists, deleting")

		err = ap.platform.DeleteFunction(platform.NewDeleteOptions(deployOptions.CommonOptions))

		if err != nil {
			return nil, errors.Wrap(err, "Failed to delete existing function")
		}
	}

	// if the image is not set, we need to build
	if deployOptions.ImageName == "" {
		buildResult, err = ap.platform.BuildFunction(&deployOptions.Build)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to build image")
		}

		deployOptions.ImageName = buildResult.ImageName
		deployOptions.Build.Runtime = buildResult.Runtime
		deployOptions.Build.Handler = buildResult.Handler
	}

	// call the underlying deployer
	deployResult, err := deployer()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to deploy")
	}

	// update deploy result with build result
	deployResult.BuildResult = *buildResult

	logger.InfoWith("Function deploy complete", "httpPort", deployResult.Port)

	return deployResult, err
}

// InvokeFunction will invoke a previously deployed function
func (ap *AbstractPlatform) InvokeFunction(invokeOptions *platform.InvokeOptions, writer io.Writer) error {
	return ap.invoker.invoke(invokeOptions, writer)
}

// GetDeployRequiresRegistry returns true if a registry is required for deploy, false otherwise
func (ap *AbstractPlatform) GetDeployRequiresRegistry() bool {
	return true
}
