/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kube

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"

	"github.com/nuclio/logger"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

type deployer struct {
	logger   logger.Logger
	consumer *consumer
	platform *Platform
}

func newDeployer(parentLogger logger.Logger, consumer *consumer, platform *Platform) (*deployer, error) {
	newdeployer := &deployer{
		logger:   parentLogger.GetChild("deployer"),
		platform: platform,
		consumer: consumer,
	}

	return newdeployer, nil
}

func (d *deployer) createOrUpdateFunction(functionInstance *nuclioio.Function,
	createFunctionOptions *platform.CreateFunctionOptions,
	functionStatus *functionconfig.Status) (*nuclioio.Function, error) {

	var err error

	// boolean which indicates whether the function existed or not
	functionExisted := functionInstance != nil

	createFunctionOptions.Logger.DebugWith("Creating/updating function",
		"existed", functionExisted)

	if functionInstance == nil {
		functionInstance = &nuclioio.Function{}
		functionInstance.Status.State = functionconfig.FunctionStateWaitingForResourceConfiguration
	}

	// convert config, status -> function
	d.populateFunction(&createFunctionOptions.FunctionConfig, functionStatus, functionInstance)

	createFunctionOptions.Logger.DebugWith("Populated function with configuration and status",
		"function", functionInstance)

	// if function didn't exist, create. otherwise update
	if !functionExisted {
		functionInstance, err = d.consumer.nuclioClientSet.NuclioV1beta1().Functions(functionInstance.Namespace).Create(functionInstance)
	} else {
		functionInstance, err = d.consumer.nuclioClientSet.NuclioV1beta1().Functions(functionInstance.Namespace).Update(functionInstance)
	}

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create/update function")
	}

	return functionInstance, nil
}

func (d *deployer) populateFunction(functionConfig *functionconfig.Config,
	functionStatus *functionconfig.Status,
	functionInstance *nuclioio.Function) {

	functionInstance.Spec = functionConfig.Spec

	// set meta
	functionInstance.Name = functionConfig.Meta.Name
	functionInstance.Namespace = functionConfig.Meta.Namespace
	functionInstance.Labels = functionConfig.Meta.Labels
	functionInstance.Annotations = functionConfig.Meta.Annotations

	// set alias as "latest" for now
	functionInstance.Spec.Alias = "latest"

	// there are two cases here:
	// 1. user specified --run-image: in this case, we will get here with a full URL in the image field (e.g.
	//    localhost:5000/foo:latest)
	// 2. user didn't specify --run-image and a build was performed. in such a case, image is set to the image
	//    name:tag (e.g. foo:latest) and we need to prepend run registry

	// if, for some reason, the run registry is specified, prepend that
	if functionConfig.Spec.RunRegistry != "" {
		functionInstance.Spec.Image = fmt.Sprintf("%s/%s", functionConfig.Spec.RunRegistry, functionInstance.Spec.Image)
	}

	// update the spec with a new image hash to trigger pod restart. in the future this can be removed,
	// assuming the processor can reload configuration
	functionConfig.Spec.ImageHash = strconv.Itoa(int(time.Now().UnixNano()))

	// update status
	functionInstance.Status = *functionStatus
}

func (d *deployer) deploy(functionInstance *nuclioio.Function,
	createFunctionOptions *platform.CreateFunctionOptions) (*platform.CreateFunctionResult, error) {

	// get the logger with which we need to deploy
	deployLogger := createFunctionOptions.Logger
	if deployLogger == nil {
		deployLogger = d.logger
	}

	// do the create / update
	_, err := d.createOrUpdateFunction(functionInstance,
		createFunctionOptions,
		&functionconfig.Status{
			State: functionconfig.FunctionStateWaitingForResourceConfiguration,
		})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to wait for function readiness")
	}

	// wait for the function to be ready
	err = waitForFunctionReadiness(deployLogger,
		d.consumer,
		functionInstance.Namespace,
		functionInstance.Name,
		createFunctionOptions.ReadinessTimeout)
	if err != nil {
		errMessage := d.getFunctionPodLogs(functionInstance.Namespace, functionInstance.Name)

		return nil, errors.Wrapf(err, "Failed to wait for function readiness.\n%s", errMessage)
	}

	// get the function service (might take a few seconds til it's created)
	service, err := d.getFunctionService(createFunctionOptions.FunctionConfig.Meta.Namespace,
		createFunctionOptions.FunctionConfig.Meta.Name)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function service")
	}

	return &platform.CreateFunctionResult{
		Port: int(service.Spec.Ports[0].NodePort),
	}, nil
}

func waitForFunctionReadiness(loggerInstance logger.Logger,
	consumer *consumer,
	namespace string,
	name string,
	timeout *time.Duration) error {

	// TODO: you can't log a nil pointer without panicing - maybe this should be a logger-wide behavior
	var logReadinessTimeout interface{}
	if timeout == nil {
		logReadinessTimeout = "nil"
	} else {
		logReadinessTimeout = timeout
	}

	loggerInstance.InfoWith("Waiting for function to be ready", "timeout", logReadinessTimeout)

	// gets the function, checks if ready
	conditionFunc := func() (bool, error) {

		// get the appropriate function CR
		function, err := consumer.nuclioClientSet.NuclioV1beta1().Functions(namespace).Get(name, meta_v1.GetOptions{})
		if err != nil {
			return true, err
		}

		switch function.Status.State {
		case functionconfig.FunctionStateReady:
			return true, nil
		case functionconfig.FunctionStateError:
			return false, errors.Errorf("Function in error state (%s)", function.Status.Message)
		default:
			return false, nil
		}
	}

	pollInterval := 250 * time.Millisecond

	if timeout == nil {
		return wait.PollInfinite(pollInterval, conditionFunc)
	}

	return wait.Poll(pollInterval, *timeout, conditionFunc)
}

func (d *deployer) getFunctionService(namespace string, name string) (service *v1.Service, err error) {
	deadline := time.Now().Add(10 * time.Second)

	for {

		// after a few seconds, give up
		if time.Now().After(deadline) {
			break
		}

		service, err = d.consumer.kubeClientSet.CoreV1().Services(namespace).Get(name, meta_v1.GetOptions{})

		// if there was an error other than the fact that the service wasn't found,
		// return now
		if !apierrors.IsNotFound(err) {
			return
		}

		// if we got a service, check that it has a node port
		if service != nil && len(service.Spec.Ports) > 0 && service.Spec.Ports[0].NodePort != 0 {
			return
		}

		// wait a bit
		time.Sleep(1 * time.Second)
	}

	return
}

func (d *deployer) getFunctionPodLogs(namespace string, name string) string {
	podLogsMessage := "\nPod logs:\n"

	// list pods
	functionPods, listPodErr := d.consumer.kubeClientSet.CoreV1().Pods(namespace).List(meta_v1.ListOptions{
		LabelSelector: fmt.Sprintf("nuclio.io/function-name=%s", name),
	})

	if listPodErr != nil {
		podLogsMessage += "Failed to list pods: " + listPodErr.Error() + "\n"
		return podLogsMessage
	}

	if len(functionPods.Items) == 0 {
		podLogsMessage += fmt.Sprintf("No pods found for %s:%s, is replicas set to 0?",
			namespace,
			name)
	} else {

		// iterate over pods and get their logs
		for _, pod := range functionPods.Items {
			podLogsMessage += "\n* " + pod.Name + "\n"

			logsRequest, getLogsErr := d.consumer.kubeClientSet.CoreV1().Pods(namespace).GetLogs(pod.Name, &v1.PodLogOptions{}).Stream()
			if getLogsErr != nil {
				podLogsMessage += "Failed to read logs: " + getLogsErr.Error() + "\n"
				continue
			}

			logsBuffer := bytes.Buffer{}
			logsBuffer.ReadFrom(logsRequest) // nolint: errcheck

			// close the stream
			logsRequest.Close() // nolint: errcheck

			// output the logs
			podLogsMessage += logsBuffer.String() + "\n"
		}
	}

	return podLogsMessage
}
