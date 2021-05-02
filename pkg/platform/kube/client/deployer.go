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

func (d *Deployer) CreateOrUpdateFunction(functionInstance *nuclioio.NuclioFunction,
	createFunctionOptions *platform.CreateFunctionOptions,
	functionStatus *functionconfig.Status) (*nuclioio.NuclioFunction, error) {

	var err error

	// boolean which indicates whether the function exists or not
	// the function will be created if it doesn't exit, otherwise will updated
	functionExists := functionInstance != nil

	createFunctionOptions.Logger.DebugWith("Creating/updating function",
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

	createFunctionOptions.Logger.DebugWith("Populated function with configuration and status",
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
			Create(functionInstance)
	} else {
		functionInstance, err = nuclioClientSet.NuclioV1beta1().
			NuclioFunctions(functionInstance.Namespace).
			Update(functionInstance)
	}

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create/update function")
	}

	return functionInstance, nil
}

func (d *Deployer) Deploy(functionInstance *nuclioio.NuclioFunction,
	createFunctionOptions *platform.CreateFunctionOptions) (*platform.CreateFunctionResult, *nuclioio.NuclioFunction, string, error) {

	// Get the logger with which we need to Deploy
	deployLogger := createFunctionOptions.Logger
	if deployLogger == nil {
		deployLogger = d.logger
	}

	// do the create / update
	// TODO: Infer timestamp from function config (consider create/update scenarios)
	functionCreateOrUpdateTimestamp := time.Now()
	if _, err := d.CreateOrUpdateFunction(functionInstance,
		createFunctionOptions,
		&functionconfig.Status{
			State: functionconfig.FunctionStateWaitingForResourceConfiguration,
		}); err != nil {
		return nil, nil, err.Error(), errors.Wrap(err, "Failed to create function")
	}

	// wait for the function to be ready
	updatedFunctionInstance, err := waitForFunctionReadiness(deployLogger,
		d.consumer,
		functionInstance.Namespace,
		functionInstance.Name,
		functionCreateOrUpdateTimestamp)
	if err != nil {
		podLogs, briefErrorsMessage := d.getFunctionPodLogsAndEvents(functionInstance.Namespace, functionInstance.Name)
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

	externalIPAddresses, err := d.platform.GetExternalIPAddresses()
	if err != nil {
		return errors.Wrap(err, "Failed to get external ip address")
	}

	// -1 because port was not assigned yet, it is just a placeholder
	functionInstance.Status.ExternalInvocationURLs = []string{fmt.Sprintf("%s:-1", externalIPAddresses[0])}
	return nil

}

func (d *Deployer) getFunctionPodLogsAndEvents(namespace string, name string) (string, string) {
	var briefErrorsMessage string
	podLogsMessage := "\nPod logs:\n"

	// list pods
	functionPods, listPodErr := d.consumer.KubeClientSet.CoreV1().
		Pods(namespace).
		List(metav1.ListOptions{
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
	} else {
		var pod v1.Pod

		// get the latest pod
		for _, currentPod := range functionPods.Items {
			if pod.ObjectMeta.CreationTimestamp.Before(&currentPod.ObjectMeta.CreationTimestamp) {
				pod = currentPod
			}
		}

		// get the pod logs
		podLogsMessage += "\n* " + pod.Name + "\n"

		maxLogLines := int64(MaxLogLines)
		logsRequest, getLogsErr := d.consumer.KubeClientSet.CoreV1().
			Pods(namespace).
			GetLogs(pod.Name, &v1.PodLogOptions{TailLines: &maxLogLines}).
			Stream()
		if getLogsErr != nil {
			podLogsMessage += "Failed to read logs: " + getLogsErr.Error() + "\n"
		} else {
			scanner := bufio.NewScanner(logsRequest)

			var formattedProcessorLogs string
			formattedProcessorLogs, briefErrorsMessage = d.platform.GetProcessorLogsAndBriefError(scanner)

			podLogsMessage += formattedProcessorLogs

			// close the stream
			logsRequest.Close() // nolint: errcheck
		}

		podWarningEvents, err := d.getFunctionPodWarningEvents(namespace, pod.Name)
		if err != nil {
			podLogsMessage += "Failed to get pod warning events: " + err.Error() + "\n"
		} else if briefErrorsMessage == "" && podWarningEvents != "" {

			// if there is no brief error message and there are warning events - add them
			podLogsMessage += "\n* Warning events:\n" + podWarningEvents
			briefErrorsMessage += podWarningEvents
		}
	}

	return podLogsMessage, briefErrorsMessage
}

func (d *Deployer) getFunctionPodWarningEvents(namespace string, podName string) (string, error) {
	eventList, err := d.consumer.KubeClientSet.CoreV1().Events(namespace).List(metav1.ListOptions{})
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

func isFunctionDeploymentFailed(consumer *Consumer,
	namespace string,
	name string,
	functionCreateOrUpdateTimestamp time.Time) (bool, error) {

	// list function pods
	pods, err := consumer.KubeClientSet.CoreV1().
		Pods(namespace).
		List(metav1.ListOptions{
			LabelSelector: common.CompileListFunctionPodsLabelSelector(name),
		})
	if err != nil {
		return false, errors.Wrap(err, "Failed to Get pods")
	}

	// infer from the pod statuses if the function deployment had failed
	// failure of one pod is enough to tell that the deployment had failed
	for _, pod := range pods.Items {

		// skip irrelevant pods (leftovers of previous function deployments)
		// (subtract 2 seconds from create/update timestamp because of ms accuracy loss of pod.creationTimestamp)
		if !pod.GetCreationTimestamp().After(functionCreateOrUpdateTimestamp.Add(-2 * time.Second)) {
			continue
		}

		for _, containerStatus := range pod.Status.ContainerStatuses {

			if pod.Status.ContainerStatuses[0].State.Waiting != nil {

				// check if the pod is on a crashLoopBackoff
				if containerStatus.State.Waiting.Reason == "CrashLoopBackOff" {

					return true, errors.Errorf("NuclioFunction pod (%s) is in a crash loop", pod.Name)
				}
			}
		}

		for _, condition := range pod.Status.Conditions {

			// check if the pod is in pending state, and the reason is that it is unschedulable
			// (meaning no k8s node can currently run it, because of insufficient resources etc..)
			if pod.Status.Phase == v1.PodPending &&
				condition.Reason == "Unschedulable" {

				return true, errors.Errorf("NuclioFunction pod (%s) is unschedulable", pod.Name)
			}
		}
	}

	return false, nil
}

func waitForFunctionReadiness(loggerInstance logger.Logger,
	consumer *Consumer,
	namespace string,
	name string,
	functionCreateOrUpdateTimestamp time.Time) (*nuclioio.NuclioFunction, error) {
	var err error
	var function *nuclioio.NuclioFunction

	// gets the function, checks if ready
	conditionFunc := func() (bool, error) {

		// get the appropriate function CR
		function, err = consumer.NuclioClientSet.NuclioV1beta1().
			NuclioFunctions(namespace).
			Get(name, metav1.GetOptions{})
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
			if !function.Spec.WaitReadinessTimeoutBeforeFailure {

				// check if function deployment had failed
				// (ignore the error if there's no concrete indication of failure, because it might still stabilize)
				if functionDeploymentFailed, err := isFunctionDeploymentFailed(consumer,
					namespace,
					name,
					functionCreateOrUpdateTimestamp); functionDeploymentFailed {

					return false, errors.Wrapf(err, "NuclioFunction deployment failed")
				}
			}

			return false, nil
		}
	}

	err = wait.PollInfinite(250*time.Millisecond, conditionFunc)
	return function, err
}
