package platform

import (
	"io"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/build"

	"github.com/nuclio/nuclio-sdk"
)

// Platform defines the interface that any underlying function platform must provide for nuclio
// to run over it
type Platform interface {

	// Build will locally build a processor image and return its name (or the error)
	BuildFunction(buildOptions *BuildOptions) (string, error)

	// Deploy will deploy a processor image to the platform (optionally building it, if source is provided)
	DeployFunction(deployOptions *DeployOptions) (*DeployResult, error)

	// InvokeFunction will invoke a previously deployed function
	InvokeFunction(invokeOptions *InvokeOptions, writer io.Writer) error

	// InvokeFunction will invoke a previously deployed function
	GetFunctions(getOptions *GetOptions) ([]Function, error)

	// UpdateOptions will update a previously deployed function
	UpdateFunction(updateOptions *UpdateOptions) error

	// DeleteFunction will delete a previously deployed function
	DeleteFunction(deleteOptions *DeleteOptions) error
}

//
// Base for all platforms
//

type AbstractPlatform struct {
	Logger nuclio.Logger
	platform Platform
	invoker *invoker
}

func NewAbstractPlatform(parentLogger nuclio.Logger, platform Platform) (*AbstractPlatform, error) {
	var err error

	newAbstractPlatform := &AbstractPlatform{
		Logger: parentLogger.GetChild("platform").(nuclio.Logger),
		platform: platform,
	}

	// create invoker
	newAbstractPlatform.invoker, err = newInvoker(newAbstractPlatform.Logger, platform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create invoker")
	}

	return newAbstractPlatform, nil
}

func (ap *AbstractPlatform) BuildFunction(buildOptions *BuildOptions) (string, error) {

	// convert options
	builderOptions := build.Options{
		Verbose:         buildOptions.Common.Verbose,
		FunctionName:    buildOptions.Common.Identifier,
		FunctionPath:    buildOptions.Path,
		OutputType:      buildOptions.OutputType,
		OutputName:      buildOptions.ImageName,
		OutputVersion:   buildOptions.ImageVersion,
		NuclioSourceDir: buildOptions.NuclioSourceDir,
		NuclioSourceURL: buildOptions.NuclioSourceURL,
		PushRegistry:    buildOptions.Registry,
		Runtime:         buildOptions.Runtime,
		NoBaseImagePull: buildOptions.NoBaseImagesPull,
	}

	// if output name isn't set, use identifier
	if builderOptions.OutputName == "" {
		builderOptions.OutputName = buildOptions.Common.Identifier
	}

	// execute a build
	builder, err := build.NewBuilder(buildOptions.Common.GetLogger(ap.Logger), &builderOptions)
	if err != nil {
		return "", errors.Wrap(err, "Failed to create builder")
	}

	return builder.Build()
}

// HandleDeployFunction calls a deployer that does the platform specific deploy, but adds a lot
// of common code
func (ap *AbstractPlatform) HandleDeployFunction(deployOptions *DeployOptions,
	deployer func() (*DeployResult, error)) (*DeployResult, error) {

	ap.Logger.InfoWith("Deploying function", "name", deployOptions.Common.Identifier)

	// first, check if the function exists so that we can delete it
	functions, err := ap.platform.GetFunctions(&GetOptions{
		Common: deployOptions.Common,
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function")
	}

	// if the function exists, delete it
	if len(functions) > 0 {
		ap.Logger.InfoWith("Function already exists, deleting")

		err = ap.platform.DeleteFunction(&DeleteOptions{
			Common: deployOptions.Common,
		})

		if err != nil {
			return nil, errors.Wrap(err, "Failed to delete existing function")
		}
	}

	// if the image is not set, we need to build
	if deployOptions.ImageName == "" {
		deployOptions.ImageName, err = ap.platform.BuildFunction(&deployOptions.Build)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to build image")
		}
	}

	// call the underlying deployer
	deployResult, err := deployer()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to deploy")
	}

	ap.Logger.InfoWith("Function deploy complete", "httpPort", deployResult.Port)

	return deployResult, err
}

// InvokeFunction will invoke a previously deployed function
func (ap *AbstractPlatform) InvokeFunction(invokeOptions *InvokeOptions, writer io.Writer) error {
	return ap.invoker.invoke(invokeOptions, writer)
}
