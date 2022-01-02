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

package client

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const MaxLogLines = 100

type Deployer struct {
	logger   logger.Logger
	consumer *Consumer
	platform platform.Platform
}

func NewDeployer(parentLogger logger.Logger, consumer *Consumer, platform platform.Platform) (*Deployer, error) {
	newDeployer := &Deployer{
		logger:   parentLogger.GetChild("deployer"),
		platform: platform,
		consumer: consumer,
	}

	return newDeployer, nil
}

func (d *Deployer) CreateOrUpdateFunction(ctx context.Context,
	functionInstance *nuclioio.NuclioFunction,
	createFunctionOptions *platform.CreateFunctionOptions,
	functionStatus *functionconfig.Status) (*nuclioio.NuclioFunction, error) {

	var err error

	// boolean which indicates whether the function exists or not
	// the function will be created if it doesn't exit, otherwise will updated
	functionExists := functionInstance != nil

	createFunctionOptions.Logger.DebugWithCtx(ctx,
		"Creating/updating function",
		"functionExists", functionExists,
		"functionInstance", functionInstance)

	if !functionExists {
		functionInstance = &nuclioio.NuclioFunction{}
		functionInstance.Status.State = functionconfig.FunctionStateWaitingForResourceConfiguration
	} else {
		functionStatus.InternalInvocationURLs = functionInstance.Status.InternalInvocationURLs
		functionStatus.ExternalInvocationURLs = functionInstance.Status.ExternalInvocationURLs
		functionStatus.HTTPPort = functionInstance.Status.HTTPPort
	}

	// convert config, status -> function
	if err := d.populateFunction(&createFunctionOptions.FunctionConfig,
		functionStatus,
		functionInstance,
		functionExists); err != nil {
		return nil, errors.Wrap(err, "Failed to populate function")
	}

	createFunctionOptions.Logger.DebugWithCtx(ctx,
		"Populated function with configuration and status",
		"function", functionInstance,
		"functionExists", functionExists)

	// get clientset
	nuclioClientSet, err := d.consumer.getNuclioClientSet(createFunctionOptions.AuthConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get nuclio clientset")
	}

	// if function didn't exist, create. otherwise update
	if !functionExists {
		functionInstance, err = nuclioClientSet.NuclioV1beta1().
			NuclioFunctions(functionInstance.Namespace).
			Create(ctx, functionInstance, metav1.CreateOptions{})
	} else {
		functionInstance, err = nuclioClientSet.NuclioV1beta1().
			NuclioFunctions(functionInstance.Namespace).
			Update(ctx, functionInstance, metav1.UpdateOptions{})
	}
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create/update function")
	}

	return functionInstance, nil
}

func (d *Deployer) Deploy(ctx context.Context,
	functionInstance *nuclioio.NuclioFunction,
	createFunctionOptions *platform.CreateFunctionOptions) (*platform.CreateFunctionResult, *nuclioio.NuclioFunction, string, error) {

	// do the create / update
	// TODO: Infer timestamp from function config (consider create/update scenarios)
	if _, err := d.CreateOrUpdateFunction(ctx,
		functionInstance,
		createFunctionOptions,
		&functionconfig.Status{
			State: functionconfig.FunctionStateWaitingForResourceConfiguration,
		}); err != nil {
		return nil, nil, err.Error(), errors.Wrap(err, "Failed to create function")
	}

	// wait for the function to be ready
	updatedFunctionInstance, err := waitForFunctionReadiness(ctx,
		d.consumer,
		functionInstance.Namespace,
		functionInstance.Name)
	if err != nil {
		podLogs, briefErrorsMessage := d.getFunctionPodLogsAndEvents(ctx, functionInstance.Namespace, functionInstance.Name)
		return nil, updatedFunctionInstance, briefErrorsMessage, errors.Wrapf(err, "Failed to wait for function readiness.\n%s", podLogs)
	}

	return &platform.CreateFunctionResult{
		Port:           updatedFunctionInstance.Status.HTTPPort,
		FunctionStatus: updatedFunctionInstance.Status,
	}, updatedFunctionInstance, "", nil
}

func (d *Deployer) populateFunction(functionConfig *functionconfig.Config,
	functionStatus *functionconfig.Status,
	functionInstance *nuclioio.NuclioFunction,
	functionExisted bool) error {

	functionInstance.Spec = functionConfig.Spec

	// set meta
	functionInstance.Name = functionConfig.Meta.Name
	functionInstance.Namespace = functionConfig.Meta.Namespace
	functionInstance.Annotations = functionConfig.Meta.Annotations

	// set labels only on function creation (never on update)
	if !functionExisted {
		functionInstance.Labels = functionConfig.Meta.Labels
	}

	// set alias as "latest" for now
	functionInstance.Spec.Alias = "latest"

	// there are two cases here:
	// 1. user specified --run-image: in this case, we will get here with a full URL in the image field (e.g.
	//    localhost:5000/foo:latest)
	// 2. user didn't specify --run-image and a build was performed. in such a case, image is set to the image
	//    name:tag (e.g. foo:latest) and we need to prepend run registry

	// if, for some reason, the run registry is specified, prepend that
	if functionConfig.Spec.RunRegistry != "" {

		// check if the run registry is part of the image already first
		if !strings.HasPrefix(functionInstance.Spec.Image, fmt.Sprintf("%s/", functionConfig.Spec.RunRegistry)) {
			functionInstance.Spec.Image = fmt.Sprintf("%s/%s", functionConfig.Spec.RunRegistry, functionInstance.Spec.Image)
		}
	}

	// update the spec with a new image hash to trigger pod restart. in the future this can be removed,
	// assuming the processor can reload configuration
	functionConfig.Spec.ImageHash = strconv.Itoa(int(time.Now().UnixNano()))

	// update status
	functionInstance.Status = *functionStatus
	return nil

}

func (d *Deployer) getFunctionPodLogsAndEvents(ctx context.Context, namespace string, name string) (string, string) {
	var briefErrorsMessage string
	podLogsMessage := "\nPod logs:\n"

	// list pods
	functionPods, listPodErr := d.consumer.KubeClientSet.CoreV1().
		Pods(namespace).
		List(ctx, metav1.ListOptions{
			LabelSelector: common.CompileListFunctionPodsLabelSelector(name),
		})

	if listPodErr != nil {
		podLogsMessage += fmt.Sprintf("Failed to list pods: %s\n", listPodErr.Error())
		return podLogsMessage, ""
	}

	if len(functionPods.Items) == 0 {
		podLogsMessage += fmt.Sprintf("No pods found for %s:%s, is replicas set to 0?",
			namespace,
			name)

		return podLogsMessage, briefErrorsMessage

	}

	// extract logs from the last created pod
	pod := d.getLastCreatedPod(functionPods.Items)

	// get the pod logs
	podLogsMessage += "\n* " + pod.Name + "\n"

	maxLogLines := int64(MaxLogLines)
	if logsRequest, getLogsErr := d.consumer.KubeClientSet.CoreV1().
		Pods(namespace).
		GetLogs(pod.Name, &v1.PodLogOptions{TailLines: &maxLogLines}).
		Stream(ctx); getLogsErr != nil {
		podLogsMessage += "Failed to read logs: " + getLogsErr.Error() + "\n"
	} else {
		scanner := bufio.NewScanner(logsRequest)

		var formattedProcessorLogs string

		// close the stream
		defer logsRequest.Close() // nolint: errcheck

		formattedProcessorLogs, briefErrorsMessage = d.platform.GetProcessorLogsAndBriefError(scanner)

		podLogsMessage += formattedProcessorLogs
	}

	podWarningEvents, err := d.getFunctionPodWarningEvents(ctx, namespace, pod.Name)
	if err != nil {
		podLogsMessage += "Failed to get pod warning events: " + err.Error() + "\n"
	} else if briefErrorsMessage == "" && podWarningEvents != "" {

		// if there is no brief error message and there are warning events - add them
		podLogsMessage += "\n* Warning events:\n" + podWarningEvents
		briefErrorsMessage += podWarningEvents
	}

	return podLogsMessage, briefErrorsMessage
}

func (d *Deployer) getFunctionPodWarningEvents(ctx context.Context, namespace string, podName string) (string, error) {
	eventList, err := d.consumer.KubeClientSet.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	var podWarningEvents []string
	for _, event := range eventList.Items {
		if event.InvolvedObject.Name == podName && event.Type == "Warning" {
			if !common.StringInSlice(event.Message, podWarningEvents) {
				podWarningEvents = append(podWarningEvents, event.Message)
			}
		}
	}

	return fmt.Sprintf("%s\n", strings.Join(podWarningEvents, "\n")), nil
}

func (d *Deployer) getLastCreatedPod(pods []v1.Pod) v1.Pod {
	var latestPod v1.Pod

	// get the latest pod
	for _, pod := range pods {
		if latestPod.ObjectMeta.CreationTimestamp.Before(&pod.ObjectMeta.CreationTimestamp) {
			latestPod = pod
		}
	}

	return latestPod
}

func waitForFunctionReadiness(ctx context.Context,
	consumer *Consumer,
	namespace string,
	name string) (*nuclioio.NuclioFunction, error) {
	var err error
	var function *nuclioio.NuclioFunction

	// gets the function, checks if ready
	conditionFunc := func() (bool, error) {

		// get the appropriate function CR
		function, err = consumer.NuclioClientSet.NuclioV1beta1().
			NuclioFunctions(namespace).
			Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}

		switch function.Status.State {
		case functionconfig.FunctionStateReady:
			return true, nil
		case functionconfig.FunctionStateError, functionconfig.FunctionStateUnhealthy:
			return false, errors.Errorf("NuclioFunction in %s state:\n%s",
				function.Status.State,
				function.Status.Message)
		default:

			// keep waiting
			return false, nil
		}
	}

	err = wait.PollInfinite(250*time.Millisecond, conditionFunc)
	return function, err
}
