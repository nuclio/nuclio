package platform

import (
	"io"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/build"

	"github.com/nuclio/nuclio-sdk"
)

// BuildResult holds information detected/generated as a result of a build process
type BuildResult struct {
	ImageName string
	Runtime   string
	Handler   string
}

// Platform defines the interface that any underlying function platform must provide for nuclio
// to run over it
type Platform interface {

	// Build will locally build a processor image and return its name (or the error)
	BuildFunction(buildOptions *BuildOptions) (*BuildResult, error)

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

	// GetDeployRequiresRegistry returns true if a registry is required for deploy, false otherwise
	GetDeployRequiresRegistry() bool

	// GetName returns the platform name
	GetName() string
}

//
// Base for all platforms
//

type AbstractPlatform struct {
	Logger   nuclio.Logger
	platform Platform
	invoker  *invoker
}

func NewAbstractPlatform(parentLogger nuclio.Logger, platform Platform) (*AbstractPlatform, error) {
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

func (ap *AbstractPlatform) BuildFunction(buildOptions *BuildOptions) (*BuildResult, error) {

	// convert options
	builderOptions := build.Options{
		Verbose:          buildOptions.Common.Verbose,
		FunctionName:     buildOptions.Common.Identifier,
		FunctionPath:     buildOptions.Path,
		OutputType:       buildOptions.OutputType,
		OutputName:       buildOptions.ImageName,
		OutputVersion:    buildOptions.ImageVersion,
		NuclioSourceDir:  buildOptions.NuclioSourceDir,
		NuclioSourceURL:  buildOptions.NuclioSourceURL,
		PushRegistry:     buildOptions.Registry,
		Runtime:          buildOptions.Runtime,
		Handler:          buildOptions.Handler,
		NoBaseImagePull:  buildOptions.NoBaseImagesPull,
		BaseImageName:    buildOptions.BaseImageName,
		Commands:         buildOptions.Commands,
		ScriptPaths:      buildOptions.ScriptPaths,
		AddedObjectPaths: buildOptions.AddedObjectPaths,
	}

	// if output name isn't set, use identifier
	if builderOptions.OutputName == "" {
		builderOptions.OutputName = buildOptions.Common.Identifier
	}

	// execute a build
	builder, err := build.NewBuilder(buildOptions.Common.GetLogger(ap.Logger), &builderOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create builder")
	}

	// convert types
	result, err := builder.Build()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to build")
	}

	return &BuildResult{
		ImageName: result.ImageName,
		Runtime:   result.Runtime,
		Handler:   result.Handler,
	}, nil
}

// HandleDeployFunction calls a deployer that does the platform specific deploy, but adds a lot
// of common code
func (ap *AbstractPlatform) HandleDeployFunction(deployOptions *DeployOptions,
	deployer func() (*DeployResult, error)) (*DeployResult, error) {

	// get the logger we need to deploy with
	logger := deployOptions.Common.GetLogger(ap.Logger)

	logger.InfoWith("Deploying function", "name", deployOptions.Common.Identifier)

	// first, check if the function exists so that we can delete it
	functions, err := ap.platform.GetFunctions(&GetOptions{
		Common: deployOptions.Common,
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function")
	}

	// if the function exists, delete it
	if len(functions) > 0 {
		logger.InfoWith("Function already exists, deleting")

		err = ap.platform.DeleteFunction(&DeleteOptions{
			Common: deployOptions.Common,
		})

		if err != nil {
			return nil, errors.Wrap(err, "Failed to delete existing function")
		}
	}

	// if the image is not set, we need to build
	if deployOptions.ImageName == "" {
		buildResult, err := ap.platform.BuildFunction(&deployOptions.Build)
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

	logger.InfoWith("Function deploy complete", "httpPort", deployResult.Port)

	return deployResult, err
}

// InvokeFunction will invoke a previously deployed function
func (ap *AbstractPlatform) InvokeFunction(invokeOptions *InvokeOptions, writer io.Writer) error {
	return ap.invoker.invoke(invokeOptions, writer)
}

// GetDeployRequiresRegistry returns true if a registry is required for deploy, false otherwise
func (ap *AbstractPlatform) GetDeployRequiresRegistry() bool {
	return true
}
