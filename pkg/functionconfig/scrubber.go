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

package functionconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	ReferencePrefix                  = "$ref:"
	ReferenceToEnvVarPrefix          = "NUCLIO_B64_"
	NuclioFlexVolumeSecretNamePrefix = "nuclio-flexvolume"
	SecretTypeFunctionConfig         = "nuclio.io/functionconfig"
	SecretTypeV3ioFuse               = "v3io/fuse"
	SecretContentKey                 = "content"
	FunctionSecretMountPath          = "/etc/nuclio/secrets"
)

type Scrubber struct {
	*common.AbstractScrubber
}

// NewScrubber returns a new scrubber
// If the scrubber is only used for restoring, the arguments can be nil
func NewScrubber(parentLogger logger.Logger, sensitiveFields []*regexp.Regexp, kubeClientSet kubernetes.Interface) *Scrubber {

	filterSecretNameFunction := func(name string) bool {
		// if it is a flex volume secret, skip it
		return strings.HasPrefix(name, NuclioFlexVolumeSecretNamePrefix)
	}
	abstractScrubber := common.NewAbstractScrubber(
		sensitiveFields,
		kubeClientSet,
		ReferencePrefix,
		common.NuclioResourceLabelKeyFunctionName,
		SecretTypeFunctionConfig,
		parentLogger, filterSecretNameFunction)
	scrubber := &Scrubber{
		AbstractScrubber: abstractScrubber,
	}
	abstractScrubber.Scrubber = scrubber
	return scrubber
}

// RestoreFunctionConfig restores a function config from a secret, in case we're running in a kube platform
func (s *Scrubber) RestoreFunctionConfig(ctx context.Context,
	functionConfig *Config,
	platformName string) (*Config, error) {

	// if we're in kube platform, we need to restore the function config's
	// sensitive data from the function's secret
	if platformName == common.KubePlatformName {
		secretMap, err := s.GetObjectSecretMap(ctx,
			functionConfig.Meta.Name,
			functionConfig.Meta.Namespace)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get function secret")
		}
		if len(secretMap) > 0 {

			// restore the function config
			restoredFunctionConfig, err := s.Restore(functionConfig, secretMap)
			if err != nil {
				return nil, errors.Wrap(err, "Failed to restore function config")
			}
			return restoredFunctionConfig.(*Config), nil
		}
	}

	// if we're not in kube platform, or the function doesn't have a secret, just return the function config
	return functionConfig, nil
}

func (s *Scrubber) ValidateReference(objectToScrub interface{},
	existingSecretMap map[string]string,
	fieldPath,
	secretKey,
	stringValue string) error {

	// if KubeClientSet is empty, it means that platform is not Kube, so we skip scrubbing
	if s.KubeClientSet == nil {
		return nil
	}
	functionConfig := objectToScrub.(*Config)
	// for flex volume access keys, we need to check if the volume secret exists
	if strings.Contains(stringValue, "flexvolume") {

		// get the volume name
		volumeIndexStr := strings.Split(strings.Split(stringValue, "[")[1], "]")[0]
		volumeIndex, err := strconv.Atoi(volumeIndexStr)
		if err != nil {
			return errors.Wrap(err, "Failed to parse volume index")
		}
		volumeName := functionConfig.Spec.Volumes[volumeIndex].Volume.Name

		// list secrets with the volume name label selector
		volumeSecrets, err := s.KubeClientSet.CoreV1().Secrets(functionConfig.Meta.Namespace).List(context.Background(),
			metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", common.NuclioResourceLabelKeyVolumeName, volumeName),
			})
		if err != nil {
			return errors.Wrap(err, "Failed to list volume secrets")
		}

		// if no secret exists, return an error
		if len(volumeSecrets.Items) == 0 {
			return errors.New(fmt.Sprintf("No secret exists for volume %s", volumeName))
		}

		return nil
	}

	// for other fields, we need to check if the secret exists in the secret map
	if existingSecretMap != nil {
		trimmedSecretKey := strings.ToLower(strings.TrimSpace(secretKey))
		if _, exists := existingSecretMap[trimmedSecretKey]; !exists {
			return errors.New(fmt.Sprintf("Config data in path %s is already scrubbed, but original value does not exist in secret", fieldPath))
		}
		return nil
	}

	return errors.New(fmt.Sprintf("Config data in path %s is already masked, but secret does not exist.", fieldPath))
}

func (s *Scrubber) ConvertMapToConfig(mapConfig interface{}) (interface{}, error) {

	// marshal and unmarshal the map object back to function config
	functionConfig := &Config{}
	masrhalledFunctionConfig, err := json.Marshal(mapConfig.(map[string]interface{}))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to marshal scrubbed function config")
	}
	if err := json.Unmarshal(masrhalledFunctionConfig, functionConfig); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal scrubbed function config")
	}

	return functionConfig, nil
}

// GenerateFlexVolumeSecretName generates a secret name for a flex volume, in the form of:
// `nuclio-flex-volume-<volume-name>-<unique-id>`
func (s *Scrubber) GenerateFlexVolumeSecretName(functionName, volumeName string) string {
	secretName := fmt.Sprintf("%s-%s-%s", NuclioFlexVolumeSecretNamePrefix, functionName, volumeName)

	// if the secret name is too long, drop the function name
	if len(secretName) > common.KubernetesDomainLevelMaxLength {
		secretName = fmt.Sprintf("%s-%s", NuclioFlexVolumeSecretNamePrefix, volumeName)

	}

	// if the secret name is still too long, trim it and keep space for the unique id
	if len(secretName) > common.KubernetesDomainLevelMaxLength-8 {
		secretName = secretName[:common.KubernetesDomainLevelMaxLength-8]
	}

	// remove trailing non-alphanumeric characters
	secretName = strings.TrimRight(secretName, "-_")

	// add a unique id to the end of the name
	secretName = fmt.Sprintf("%s-%s", secretName, common.GenerateRandomString(8, common.SmallLettersAndNumbers))

	return secretName
}

func (s *Scrubber) GetObjectSecretName(ctx context.Context, name, namespace string) (string, error) {

	secrets, err := s.GetObjectSecrets(ctx, name, namespace)
	if err != nil {
		return "", errors.Wrap(err, "Failed to get function secrets")
	}

	for _, secret := range secrets {
		if !strings.HasPrefix(secret.Kubernetes.Name, NuclioFlexVolumeSecretNamePrefix) {
			return secret.Kubernetes.Name, nil
		}
	}

	return "", nil
}

func (s *Scrubber) ScrubFunctionConfig(ctx context.Context,
	functionConfig *Config) (*Config, error) {
	var err error

	existingSecretName, err := s.GetObjectSecretName(ctx, functionConfig.Meta.Name, functionConfig.Meta.Namespace)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function config secret name")
	}
	scrubbedFunctionConfig, existingSecretName, secretsMap, err := s.GetExistingSecretAndScrub(ctx, functionConfig,
		functionConfig.Meta.Name, functionConfig.Meta.Namespace, existingSecretName)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get existing secret and scrub function config")
	}

	// create flex volume secrets if needed
	if err := s.createFlexVolumeSecrets(ctx,
		functionConfig.Spec.Volumes,
		functionConfig.Meta.Name,
		functionConfig.Meta.Namespace,
		functionConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName],
		secretsMap); err != nil {
		return nil, errors.Wrap(err, "Failed to create flex volume secrets")
	}

	// encode secrets map
	encodedSecretsMap, err := s.EncodeSecretsMap(secretsMap)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to encode secrets map")
	}

	// create or update a secret for the function
	if err := s.CreateOrUpdateObjectSecret(ctx,
		encodedSecretsMap,
		existingSecretName,
		functionConfig.Meta.Name,
		functionConfig.Meta.Namespace,
		functionConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName]); err != nil {
		return nil, errors.Wrap(err, "Failed to create or update function secret")
	}

	return GetFunctionConfigFromInterface(scrubbedFunctionConfig), nil
}

func (s *Scrubber) createFlexVolumeSecrets(ctx context.Context,
	volumes []Volume,
	functionName,
	functionNamespace,
	projectName string,
	secretsMap map[string]string) error {

	for volumeIndex, volume := range volumes {
		if volume.Volume.FlexVolume != nil && volume.Volume.FlexVolume.Driver == SecretTypeV3ioFuse {

			// if the volume doesn't have an access key, skip it
			if _, exists := volume.Volume.FlexVolume.Options["accessKey"]; !exists {
				continue
			}

			if err := s.createOrUpdateFlexVolumeSecret(ctx,
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

func (s *Scrubber) createOrUpdateFlexVolumeSecret(ctx context.Context,
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
	flexVolumeSecretName := s.GenerateFlexVolumeSecretName(functionName, volumeName)

	// check if a secret with the same access key reference already exists
	existingFlexVolumeSecrets, err := s.KubeClientSet.CoreV1().Secrets(functionNamespace).List(ctx, metav1.ListOptions{
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
		Type: SecretTypeV3ioFuse,
		StringData: map[string]string{
			"accessKey": accessKey,
		},
	}

	s.Logger.DebugWithCtx(ctx,
		"Creating/updating flex volume secret",
		"volumeName", volumeName,
		"functionName", functionName,
		"functionNamespace", functionNamespace)
	if err := s.CreateOrUpdateSecret(ctx, functionNamespace, secretConfig); err != nil {
		return errors.Wrap(err, "Failed to create flex volume secret")
	}

	return nil
}
