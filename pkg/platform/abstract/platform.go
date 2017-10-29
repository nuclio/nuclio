package abstract

import (
	"io"
	"os"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/build"

	"github.com/nuclio/nuclio-sdk"
)

//
// Base for all platforms
//

type Platform struct {
	Logger   nuclio.Logger
	platform platform.Platform
	invoker  *invoker
}

func NewPlatform(parentLogger nuclio.Logger, platform platform.Platform) (*Platform, error) {
	var err error

	newPlatform := &Platform{
		Logger:   parentLogger.GetChild("platform").(nuclio.Logger),
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
	builder, err := build.NewBuilder(buildOptions.GetLogger(ap.Logger))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create builder")
	}

	// convert types
	return builder.Build(buildOptions)
}

// HandleDeployFunction calls a deployer that does the platform specific deploy, but adds a lot
// of common code
func (ap *Platform) HandleDeployFunction(deployOptions *platform.DeployOptions,
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
		deployOptions.Build.FunctionConfigPath = buildResult.FunctionConfigPath
	}

	// call the underlying deployer
	deployResult, err := deployer()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to deploy")
	}

	// update deploy result with build result
	if buildResult != nil {
		deployResult.BuildResult = *buildResult
	}

	logger.InfoWith("Function deploy complete", "httpPort", deployResult.Port)

	return deployResult, err
}

// InvokeFunction will invoke a previously deployed function
func (ap *Platform) InvokeFunction(invokeOptions *platform.InvokeOptions, writer io.Writer) error {
	return ap.invoker.invoke(invokeOptions, writer)
}

// GetDeployRequiresRegistry returns true if a registry is required for deploy, false otherwise
func (ap *Platform) GetDeployRequiresRegistry() bool {
	return true
}

// FunctionConfigToDeployOptions will read the function configuration at functionConfigPath, parse it and then
// populate the deploy options
func (ap *Platform) FunctionConfigToDeployOptions(functionConfigPath string,
	deployOptions *platform.DeployOptions) error {

	var functionConfigFile *os.File
	var functionconfigReader *functionconfig.Reader

	functionConfigFile, err := os.Open(deployOptions.Build.FunctionConfigPath)
	if err != nil {
		return errors.Wrapf(err, "Failed to open function configuraition file: %s", functionConfigFile)
	}

	defer functionConfigFile.Close()

	functionconfigReader, err = functionconfig.NewReader(ap.Logger)
	if err != nil {
		return errors.Wrap(err, "Failed to create functionconfig reader")
	}

	// read the configuration
	if err = functionconfigReader.Read(functionConfigFile, "yaml"); err != nil {
		return errors.Wrap(err, "Failed to read function configuration file")
	}

	// to build options
	if err = functionconfigReader.ToDeployOptions(deployOptions); err != nil {
		return errors.Wrap(err, "Failed to get build options from function configuration")
	}

	return nil
}
