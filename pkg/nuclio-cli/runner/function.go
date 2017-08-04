package runner

import (
	"fmt"
	"strconv"
	"time"

	"github.com/nuclio/nuclio/pkg/functioncr"
	"github.com/nuclio/nuclio/pkg/nuclio-cli"
	"github.com/nuclio/nuclio/pkg/nuclio-cli/builder"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/util/common"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type FunctionRunner struct {
	nucliocli.KubeConsumer
	logger           nuclio.Logger
	options          *Options
	functioncrClient *functioncr.Client
	clientset        *kubernetes.Clientset
}

func NewFunctionRunner(parentLogger nuclio.Logger, options *Options) (*FunctionRunner, error) {
	var err error

	newFunctionRunner := &FunctionRunner{
		logger:  parentLogger.GetChild("runner").(nuclio.Logger),
		options: options,
	}

	// get kube stuff
	_, newFunctionRunner.clientset,
		newFunctionRunner.functioncrClient,
		err = newFunctionRunner.GetClients(newFunctionRunner.logger, options.Common.KubeconfigPath)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get clients")
	}

	return newFunctionRunner, nil
}

func (fr *FunctionRunner) Execute() error {

	// create a function, set default values and try to update from file
	functioncrInstance := functioncr.Function{}
	functioncrInstance.SetDefaults()
	functioncrInstance.Name = fr.options.Build.Name

	if fr.options.SpecPath != "" {
		err := functioncrInstance.FromSpecFile(fr.options.SpecPath)
		if err != nil {
			return errors.Wrap(err, "Failed to read function spec file")
		}
	}

	// override with options
	if err := fr.updateFunctioncrWithOptions(fr.options, &functioncrInstance); err != nil {
		return errors.Wrap(err, "Failed to update function with options")
	}

	// create a builder
	builder, err := builder.NewFunctionBuilder(fr.logger, &fr.options.Build)
	if err != nil {
		return errors.Wrap(err, "Failed to create builder")
	}

	// execute the build
	err = builder.Execute()
	if err != nil {
		return err
	}

	// deploy the function
	return fr.deployFunction(&functioncrInstance)
}

func (fr *FunctionRunner) deployFunction(functioncrToCreate *functioncr.Function) error {
	createdFunctioncr, err := fr.functioncrClient.Create(functioncrToCreate)
	if err != nil {
		return err
	}

	// wait until function is processed
	return fr.functioncrClient.WaitUntil(createdFunctioncr.Namespace,
		createdFunctioncr.Name,
		func(functioncrInstance *functioncr.Function) (bool, error) {

			// TODO: maybe possible that error existed before and our new post wasnt yet updated to status created ("")
			if functioncrInstance.Status.State != functioncr.FunctionStateCreated {
				if functioncrInstance.Status.State == functioncr.FunctionStateError {
					return true, fmt.Errorf("Function in error state (%s)", functioncrInstance.Status.Message)
				}

				return true, nil
			}

			return false, nil
		},
		10*time.Second,
	)
}

func (fr *FunctionRunner) updateFunctioncrWithOptions(options *Options, functioncrInstance *functioncr.Function) error {

	if options.Description != "" {
		functioncrInstance.Spec.Description = options.Description
	}

	// update replicas if scale was specified
	if options.Scale != "" {

		// TODO: handle/Set Min/Max replicas (used only with Auto mode)
		if options.Scale == "auto" {
			functioncrInstance.Spec.Replicas = 0
		} else {
			i, err := strconv.Atoi(options.Scale)
			if err != nil {
				return fmt.Errorf(`Invalid function scale, must be "auto" or an integer value`)
			} else {
				functioncrInstance.Spec.Replicas = int32(i)
			}
		}
	}

	// Set specified labels, is label = "" remove it (if exists)
	labels := common.StringToStringMap(options.Labels)
	for labelName, labelValue := range labels {
		if labelName != "name" && labelName != "version" && labelName != "alias" {
			if labelValue == "" {
				delete(functioncrInstance.Labels, labelName)
			} else {
				functioncrInstance.Labels[labelName] = labelValue
			}
		}
	}

	envmap := common.StringToStringMap(options.Env)
	newenv := []v1.EnvVar{}

	// merge new Environment var: update existing then add new
	for _, e := range functioncrInstance.Spec.Env {
		if v, ok := envmap[e.Name]; ok {
			if v != "" {
				newenv = append(newenv, v1.EnvVar{Name: e.Name, Value: v})
			}
			delete(envmap, e.Name)
		} else {
			newenv = append(newenv, e)
		}
	}

	for k, v := range envmap {
		newenv = append(newenv, v1.EnvVar{Name: k, Value: v})
	}

	functioncrInstance.Spec.Env = newenv

	// TODO: update events and data

	if options.HTTPPort != 0 {
		functioncrInstance.Spec.HTTPPort = options.HTTPPort
	}

	if options.Publish {
		functioncrInstance.Spec.Publish = options.Publish
	}

	if options.Disabled {
		functioncrInstance.Spec.Disabled = options.Disabled // TODO: use string to detect if noop/true/false
	}

	// TODO: real image
	if options.Image == "" {
		functioncrInstance.Spec.Image = "localhost:5000/" + options.Build.Name
	} else {
		functioncrInstance.Spec.Image = options.Image
	}

	return nil
}
