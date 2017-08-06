package deleter

import (
	"github.com/nuclio/nuclio/pkg/nuclio-cli"

	"github.com/nuclio/nuclio-sdk"
	"github.com/pkg/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FunctionDeleter struct {
	nucliocli.KubeConsumer
	logger  nuclio.Logger
	options *Options
}

func NewFunctionDeleter(parentLogger nuclio.Logger, options *Options) (*FunctionDeleter, error) {
	var err error

	newFunctionDeleter := &FunctionDeleter{
		logger:  parentLogger.GetChild("deleter").(nuclio.Logger),
		options: options,
	}

	// get kube stuff
	_, err = newFunctionDeleter.GetClients(newFunctionDeleter.logger, options.Common.KubeconfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get clients")
	}

	return newFunctionDeleter, nil
}

func (fd *FunctionDeleter) Execute() error {
	var err error

	resourceName, _, err := nucliocli.ParseResourceIdentifier(fd.options.Common.Identifier)
	if err != nil {
		return errors.Wrap(err, "Failed to parse resource identifier")
	}

	// get specific function CR
	return fd.FunctioncrClient.Delete(fd.options.Common.Namespace, resourceName, &meta_v1.DeleteOptions{})
}
