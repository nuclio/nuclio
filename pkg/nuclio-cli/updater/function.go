package updater

import (
	"time"

	"github.com/nuclio/nuclio/pkg/functioncr"
	"github.com/nuclio/nuclio/pkg/nuclio-cli"
	"github.com/nuclio/nuclio/pkg/nuclio-cli/runner"

	"github.com/nuclio/nuclio-sdk"
	"github.com/pkg/errors"
)

type FunctionUpdater struct {
	nucliocli.KubeConsumer
	logger  nuclio.Logger
	options *Options
}

func NewFunctionUpdater(parentLogger nuclio.Logger, options *Options) (*FunctionUpdater, error) {
	var err error

	newFunctionUpdater := &FunctionUpdater{
		logger:  parentLogger.GetChild("updater").(nuclio.Logger),
		options: options,
	}

	// get kube stuff
	_, err = newFunctionUpdater.GetClients(newFunctionUpdater.logger, options.Common.KubeconfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get clients")
	}

	return newFunctionUpdater, nil
}

func (fu *FunctionUpdater) Execute() error {

	resourceName, _, err := nucliocli.ParseResourceIdentifier(fu.options.Common.Identifier)
	if err != nil {
		return errors.Wrap(err, "Failed to parse resource identifier")
	}

	// get specific function CR
	functioncrInstance, err := fu.FunctioncrClient.Get(fu.options.Common.Namespace, resourceName)
	if err != nil {
		return errors.Wrap(err, "Failed to get function")
	}

	// if we're updating the "latest" function
	if functioncrInstance.Spec.Alias == "latest" {

		// if we need to publish - make sure alias is unset
		if fu.options.Run.Publish {
			fu.options.Alias = ""
		} else {

			// if the function's current alias is "latest" and alias wasn't set, set it to latest
			if fu.options.Alias == "" {
				fu.options.Alias = "latest"
			}
		}
	}

	// update it with the run options
	err = runner.UpdateFunctioncrWithOptions(&fu.options.Run, functioncrInstance)
	if err != nil {
		return errors.Wrap(err, "Failed to update function")
	}

	// trigger an update
	createdFunctioncr, err := fu.FunctioncrClient.Update(functioncrInstance)

	// wait until function is processed
	// TODO: this is not proper. We need to wait until the resource version changes or something as well since
	// the function might already be processed and we will unblock immediately
	return fu.FunctioncrClient.WaitUntilCondition(createdFunctioncr.Namespace,
		createdFunctioncr.Name,
		functioncr.WaitConditionProcessed,
		10*time.Second,
	)
}
