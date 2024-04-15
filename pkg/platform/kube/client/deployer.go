/*
Copyright 2023 The Nuclio Authors.

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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const MaxLogLines = 100

type Deployer struct {
	logger   logger.Logger
	consumer *Consumer
	platform platform.Platform
	scrubber *functionconfig.Scrubber
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
	// the function will be created if it doesn't exit, otherwise it will be updated
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

	// scrub the function config if enabled
	if d.platform.GetConfig().SensitiveFields.MaskSensitiveFields && !functionInstance.Spec.DisableSensitiveFieldsMasking {

		scrubbedFunctionConfig, err := d.ScrubFunctionConfig(ctx, &createFunctionOptions.FunctionConfig)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to scrub function configuration")
		}

		// replace the function config with the scrubbed one
		createFunctionOptions.FunctionConfig = *scrubbedFunctionConfig
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

func (d *Deployer) ScrubFunctionConfig(ctx context.Context,
	functionConfig *functionconfig.Config) (*functionconfig.Config, error) {
	var err error

	d.scrubber = functionconfig.NewScrubber(d.platform.GetConfig().SensitiveFields.CompileSensitiveFieldsRegex(),
		d.consumer.KubeClientSet)

	// get existing function secret
	var existingSecretMap map[string]string
	existingSecretName, err := d.getFunctionSecretName(ctx, functionConfig.Meta.Name, functionConfig.Meta.Namespace)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function secret name")
	}

	// secret exists, get its data map
	if existingSecretName != "" {
		existingSecretMap, err = d.platform.GetFunctionSecretMap(ctx, functionConfig.Meta.Name, functionConfig.Meta.Namespace)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get function secret")
		}
	}

	// scrub the function config
	d.logger.DebugWithCtx(ctx, "Scrubbing function config", "functionName", functionConfig.Meta.Name)

	scrubbedFunctionConfig, secretsMap, err := d.scrubber.Scrub(functionConfig,
		existingSecretMap,
		d.platform.GetConfig().SensitiveFields.CompileSensitiveFieldsRegex())
	if err != nil {
		return nil, errors.Wrap(err, "Failed to scrub function config")
	}

	// create flex volume secrets if needed
	if err := d.createFlexVolumeSecrets(ctx,
		functionConfig.Spec.Volumes,
		functionConfig.Meta.Name,
		functionConfig.Meta.Namespace,
		functionConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName],
		secretsMap); err != nil {
		return nil, errors.Wrap(err, "Failed to create flex volume secrets")
	}

	// encode secrets map
	encodedSecretsMap, err := d.scrubber.EncodeSecretsMap(secretsMap)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to encode secrets map")
	}

	// create or update a secret for the function
	if err := d.createOrUpdateFunctionSecret(ctx,
		encodedSecretsMap,
		existingSecretName,
		functionConfig.Meta.Name,
		functionConfig.Meta.Namespace,
		functionConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName]); err != nil {
		return nil, errors.Wrap(err, "Failed to create or update function secret")
	}

	return functionconfig.GetFunctionConfigFromInterface(scrubbedFunctionConfig), nil
}

func (d *Deployer) createOrUpdateFunctionSecret(ctx context.Context,
	encodedSecretsMap map[string]string,
	secretName string,
	name,
	namespace,
	projectName string) error {

	if secretName == "" {
		secretName = d.scrubber.GenerateObjectSecretName(name)
	}

	secretConfig := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: map[string]string{
				common.NuclioResourceLabelKeyFunctionName: name,
				common.NuclioResourceLabelKeyProjectName:  projectName,
			},
		},
		Type:       functionconfig.SecretTypeFunctionConfig,
		StringData: encodedSecretsMap,
	}

	// create or update the secret, even if the encoded secrets map is empty
	// this is to ensure that if the function will be updated with new secrets, they will be mounted properly
	d.logger.DebugWithCtx(ctx,
		"Creating/updating function secret",
		"functionName", name,
		"functionNamespace", namespace)
	if err := d.createOrUpdateSecret(ctx, namespace, secretConfig); err != nil {
		return errors.Wrap(err, "Failed to create function secret")
	}
	return nil
}

func (d *Deployer) createFlexVolumeSecrets(ctx context.Context,
	volumes []functionconfig.Volume,
	functionName,
	functionNamespace,
	projectName string,
	secretsMap map[string]string) error {

	for volumeIndex, volume := range volumes {
		if volume.Volume.FlexVolume != nil && volume.Volume.FlexVolume.Driver == functionconfig.SecretTypeV3ioFuse {

			// if the volume doesn't have an access key, skip it
			if _, exists := volume.Volume.FlexVolume.Options["accessKey"]; !exists {
				continue
			}

			if err := d.createOrUpdateFlexVolumeSecret(ctx,
				volumeIndex,
				volume.Volume.Name,
				functionName,
				functionNamespace,
				projectName,
				secretsMap); err != nil {
				return errors.Wrap(err, "Failed to create flex volume secret")
			}
		}
	}

	return nil
}

func (d *Deployer) createOrUpdateFlexVolumeSecret(ctx context.Context,
	volumeIndex int,
	volumeName,
	functionName,
	functionNamespace,
	projectName string,
	secretsMap map[string]string) error {

	var accessKey string

	// get access key value
	for secretKey, secretValue := range secretsMap {
		if strings.Contains(secretKey, "flexvolume") && strings.Contains(secretKey, fmt.Sprintf("[%d]", volumeIndex)) {
			accessKey = secretValue
			break
		}
	}

	if accessKey == "" {
		return errors.New("Failed to find access key in secrets map")
	}

	// create secret name with unique suffix
	flexVolumeSecretName := d.scrubber.GenerateFlexVolumeSecretName(functionName, volumeName)

	// check if a secret with the same access key reference already exists
	existingFlexVolumeSecrets, err := d.consumer.KubeClientSet.CoreV1().Secrets(functionNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", common.NuclioResourceLabelKeyVolumeName, volumeName),
	})
	if err != nil {
		return errors.Wrap(err, "Failed to list flex volume secrets")
	}

	// if a secret with the same access key reference exists, use it
	if len(existingFlexVolumeSecrets.Items) > 0 {
		flexVolumeSecretName = existingFlexVolumeSecrets.Items[0].Name
	}

	// create a secret for the volume
	secretConfig := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: flexVolumeSecretName,
			Labels: map[string]string{
				common.NuclioResourceLabelKeyFunctionName: functionName,
				common.NuclioResourceLabelKeyProjectName:  projectName,
				common.NuclioResourceLabelKeyVolumeName:   volumeName,
			},
		},
		Type: functionconfig.SecretTypeV3ioFuse,
		StringData: map[string]string{
			"accessKey": accessKey,
		},
	}

	d.logger.DebugWithCtx(ctx,
		"Creating/updating flex volume secret",
		"volumeName", volumeName,
		"functionName", functionName,
		"functionNamespace", functionNamespace)
	if err := d.createOrUpdateSecret(ctx, functionNamespace, secretConfig); err != nil {
		return errors.Wrap(err, "Failed to create flex volume secret")
	}

	return nil
}

func (d *Deployer) createOrUpdateSecret(ctx context.Context, namespace string, secretConfig *v1.Secret) error {

	// check if secret exists
	if _, err := d.consumer.KubeClientSet.CoreV1().Secrets(namespace).Get(ctx,
		secretConfig.Name,
		metav1.GetOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "Failed to get secret %s", secretConfig.Name)
		}
		d.logger.DebugWithCtx(ctx,
			"Creating secret",
			"secretName", secretConfig.Name,
			"namespace", namespace)

		// create secret
		if _, err := d.consumer.KubeClientSet.CoreV1().Secrets(namespace).Create(ctx,
			secretConfig,
			metav1.CreateOptions{}); err != nil {
			return errors.Wrapf(err, "Failed to create secret %s", secretConfig.Name)
		}
		return nil
	}

	// update secret
	d.logger.DebugWithCtx(ctx,
		"Updating secret",
		"secretName", secretConfig.Name,
		"namespace", namespace)
	if _, err := d.consumer.KubeClientSet.CoreV1().Secrets(namespace).Update(ctx,
		secretConfig,
		metav1.UpdateOptions{}); err != nil {
		return errors.Wrapf(err, "Failed to update secret %s", secretConfig.Name)
	}

	return nil
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

func (d *Deployer) getFunctionSecretName(ctx context.Context, name, namespace string) (string, error) {

	secrets, err := d.platform.GetFunctionSecrets(ctx, name, namespace)
	if err != nil {
		return "", errors.Wrap(err, "Failed to get function secrets")
	}

	for _, secret := range secrets {
		if !strings.HasPrefix(secret.Kubernetes.Name, functionconfig.NuclioFlexVolumeSecretNamePrefix) {
			return secret.Kubernetes.Name, nil
		}
	}

	return "", nil
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
	conditionFunc := func(conditionCtx context.Context) (bool, error) {

		// get the appropriate function CR
		function, err = consumer.NuclioClientSet.NuclioV1beta1().
			NuclioFunctions(namespace).
			Get(conditionCtx, name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}

		switch function.Status.State {
		case functionconfig.FunctionStateScaledToZero:
			return true, nil
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

	return function, wait.PollUntilContextCancel(ctx, 250*time.Millisecond, true, conditionFunc)
}
