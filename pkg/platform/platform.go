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
}

func NewAbstractPlatform(parentLogger nuclio.Logger) (*AbstractPlatform, error) {
	return &AbstractPlatform{
		Logger: parentLogger.GetChild("platform").(nuclio.Logger),
	}, nil
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
