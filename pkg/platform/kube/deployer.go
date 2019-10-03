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
	"bufio"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"

	"github.com/nuclio/logger"
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const MaxLogLines = 100

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

func (d *deployer) createOrUpdateFunction(functionInstance *nuclioio.NuclioFunction,
	createFunctionOptions *platform.CreateFunctionOptions,
	functionStatus *functionconfig.Status) (*nuclioio.NuclioFunction, error) {

	var err error

	// boolean which indicates whether the function existed or not
	functionExisted := functionInstance != nil

	createFunctionOptions.Logger.DebugWith("Creating/updating function",
		"existed", functionExisted)

	if !functionExisted {
		functionInstance = &nuclioio.NuclioFunction{}
		functionInstance.Status.State = functionconfig.FunctionStateWaitingForResourceConfiguration
	} else {
		functionStatus.HTTPPort = functionInstance.Status.HTTPPort
	}

	// convert config, status -> function
	d.populateFunction(&createFunctionOptions.FunctionConfig, functionStatus, functionInstance, functionExisted)

	createFunctionOptions.Logger.DebugWith("Populated function with configuration and status",
		"function", functionInstance)

	// get clientset
	nuclioClientSet, err := d.consumer.getNuclioClientSet(createFunctionOptions.AuthConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get nuclio clientset")
	}

	// if function didn't exist, create. otherwise update
	if !functionExisted {
		functionInstance, err = nuclioClientSet.NuclioV1beta1().NuclioFunctions(functionInstance.Namespace).Create(functionInstance)
	} else {
		functionInstance, err = nuclioClientSet.NuclioV1beta1().NuclioFunctions(functionInstance.Namespace).Update(functionInstance)
	}

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create/update function")
	}

	return functionInstance, nil
}

func (d *deployer) populateFunction(functionConfig *functionconfig.Config,
	functionStatus *functionconfig.Status,
	functionInstance *nuclioio.NuclioFunction,
	functionExisted bool) {

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
}

func (d *deployer) deploy(functionInstance *nuclioio.NuclioFunction,
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
	_, err = waitForFunctionReadiness(deployLogger,
		d.consumer,
		functionInstance.Namespace,
		functionInstance.Name)
	if err != nil {
		errMessage := d.getFunctionPodLogs(functionInstance.Namespace, functionInstance.Name)
		return nil, errors.Wrapf(err, "Failed to wait for function readiness.\n%s", errMessage)
	}

	return &platform.CreateFunctionResult{
		Port: functionInstance.Status.HTTPPort,
	}, nil
}

func waitForFunctionReadiness(loggerInstance logger.Logger,
	consumer *consumer,
	namespace string,
	name string) (*nuclioio.NuclioFunction, error) {
	var err error
	var function *nuclioio.NuclioFunction

	// gets the function, checks if ready
	conditionFunc := func() (bool, error) {

		// get the appropriate function CR
		function, err = consumer.nuclioClientSet.NuclioV1beta1().NuclioFunctions(namespace).Get(name, meta_v1.GetOptions{})
		if err != nil {
			return true, err
		}

		switch function.Status.State {
		case functionconfig.FunctionStateReady:
			return true, nil
		case functionconfig.FunctionStateError:
			return false, errors.Errorf("NuclioFunction in error state (%s)", function.Status.Message)
		default:
			return false, nil
		}
	}

	err = wait.PollInfinite(250*time.Millisecond, conditionFunc)
	return function, err
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
		logsRequest, getLogsErr := d.consumer.kubeClientSet.CoreV1().Pods(namespace).GetLogs(pod.Name, &v1.PodLogOptions{TailLines: &maxLogLines}).Stream()
		if getLogsErr != nil {
			podLogsMessage += "Failed to read logs: " + getLogsErr.Error() + "\n"
		} else {

			scanner := bufio.NewScanner(logsRequest)

			// get the last MaxLogLines logs
			for scanner.Scan() {
				currentLogLine, err := d.prettifyPodLogLine(scanner.Bytes())
				if err != nil {

					// when it is unstructured just add the log as a text
					podLogsMessage += scanner.Text() + "\n"
					continue
				}

				// when it is a processor log line
				podLogsMessage += currentLogLine + "\n"
			}

			// close the stream
			logsRequest.Close() // nolint: errcheck
		}
	}

	return common.FixEscapeChars(podLogsMessage)
}

func (d *deployer) prettifyPodLogLine(log []byte) (string, error) {
	logStruct := struct {
		Time    *string `json:"time"`
		Level   *string `json:"level"`
		Message *string `json:"message"`
		More    *string `json:"more,omitempty"`
	}{}

	if len(log) > 0 && log[0] == 'l' {

		// when it is a wrapper log line
		wrapperLogStruct := struct {
			Datetime *string           `json:"datetime"`
			Level    *string           `json:"level"`
			Message  *string           `json:"message"`
			With     map[string]string `json:"with,omitempty"`
		}{}

		if err := json.Unmarshal(log[1:], &wrapperLogStruct); err != nil {
			return "", err
		}

		// manipulate the time format so it can be parsed later
		unparsedTime := *wrapperLogStruct.Datetime + "Z"
		unparsedTime = strings.Replace(unparsedTime, " ", "T", 1)
		unparsedTime = strings.Replace(unparsedTime, ",", ".", 1)

		logStruct.Time = &unparsedTime
		logStruct.Level = wrapperLogStruct.Level
		logStruct.Message = wrapperLogStruct.Message

		more := common.CreateKeyValuePairs(wrapperLogStruct.With)
		logStruct.More = &more

	} else {

		// when it is a processor log line
		if err := json.Unmarshal(log, &logStruct); err != nil {
			return "", err
		}
	}

	// check required fields existence
	if logStruct.Time == nil || logStruct.Level == nil || logStruct.Message == nil {
		return "", errors.New("Missing required fields in pod log line")
	}

	parsedTime, err := time.Parse(time.RFC3339, *logStruct.Time)
	if err != nil {
		return "", err
	}

	res := fmt.Sprintf("[%s] (%c) %s [%s]",
		parsedTime.Format("15:04:05.000"),
		strings.ToUpper(*logStruct.Level)[0],
		*logStruct.Message,
		*logStruct.More)

	return res, nil
}
