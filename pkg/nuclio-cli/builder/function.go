package builder

import (
	"github.com/nuclio/nuclio/pkg/functioncr"
	"github.com/nuclio/nuclio/pkg/nuclio-build/build"
	"github.com/nuclio/nuclio/pkg/nuclio-cli"

	"github.com/nuclio/nuclio-sdk"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
)

type FunctionBuilder struct {
	nucliocli.KubeConsumer
	logger           nuclio.Logger
	options          *Options
	functioncrClient *functioncr.Client
	clientset        *kubernetes.Clientset
}

func NewFunctionBuilder(parentLogger nuclio.Logger, options *Options) (*FunctionBuilder, error) {
	var err error

	newFunctionBuilder := &FunctionBuilder{
		logger:  parentLogger.GetChild("builder").(nuclio.Logger),
		options: options,
	}

	// get kube stuff
	_, newFunctionBuilder.clientset,
		newFunctionBuilder.functioncrClient,
		err = newFunctionBuilder.GetClients(newFunctionBuilder.logger, options.Common.KubeconfigPath)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get clients")
	}

	return newFunctionBuilder, nil
}

func (fb *FunctionBuilder) Execute() error {

	// convert options
	buildOptions := build.Options{
		Verbose:         fb.options.Common.Verbose,
		FunctionPath:    fb.options.Path,
		OutputType:      fb.options.OutputType,
		OutputName:      fb.options.OutputName,
		Version:         "latest",
		NuclioSourceDir: fb.options.NuclioSourceDir,
		NuclioSourceURL: fb.options.NuclioSourceURL,
		PushRegistry:    fb.options.PushRegistry,
	}

	// execute a build
	err := build.NewBuilder(fb.logger, &buildOptions).Build()
	if err != nil {
		return errors.Wrap(err, "Failed to build")
	}

	return nil
}
