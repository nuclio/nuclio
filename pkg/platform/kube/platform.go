package kube

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/nuclio-sdk"
	"io"
)

type Platform struct {
	*platform.AbstractPlatform
	deployer       *deployer
	invoker        *invoker
	getter         *getter
	kubeconfigPath string
	consumer       *consumer
}

// NewPlatform instantiates a new kubernetes platform
func NewPlatform(parentLogger nuclio.Logger, kubeconfigPath string) (*Platform, error) {

	// create base
	newAbstractPlatform, err := platform.NewAbstractPlatform(parentLogger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract platform")
	}

	// create platform
	newPlatform := &Platform{
		AbstractPlatform: newAbstractPlatform,
		kubeconfigPath:   kubeconfigPath,
	}

	// create consumer
	newPlatform.consumer, err = newConsumer(newPlatform.Logger, kubeconfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create consumer")
	}

	// create deployer
	newPlatform.deployer, err = newDeployer(newAbstractPlatform.Logger, newPlatform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create platform")
	}

	// create invoker
	newPlatform.invoker, err = newInvoker(newAbstractPlatform.Logger, newPlatform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create platform")
	}

	// create getter
	newPlatform.getter, err = newGetter(newAbstractPlatform.Logger, newPlatform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create platform")
	}

	return newPlatform, nil
}

// Deploy will deploy a processor image to the platform (optionally building it, if source is provided)
func (p *Platform) DeployFunction(deployOptions *platform.DeployOptions) (*platform.DeployResult, error) {
	return p.deployer.deploy(p.consumer, deployOptions)
}

// InvokeFunction will invoke a previously deployed function
func (p *Platform) InvokeFunction(invokeOptions *platform.InvokeOptions, writer io.Writer) error {
	return p.invoker.invoke(p.consumer, invokeOptions, writer)
}

// GetFunctions will return deployed functions
func (p *Platform) GetFunctions(getOptions *platform.GetOptions, writer io.Writer) error {
	return p.getter.get(p.consumer, getOptions, writer)
}
