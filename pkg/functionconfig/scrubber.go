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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/errors"
	"github.com/nuclio/gosecretive"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	SensitiveFields []*regexp.Regexp
	KubeClientSet   kubernetes.Interface
}

// NewScrubber returns a new scrubber
// If the scrubber is only used for restoring, the arguments can be nil
func NewScrubber(sensitiveFields []*regexp.Regexp, kubeClientSet kubernetes.Interface) *Scrubber {
	return &Scrubber{
		SensitiveFields: sensitiveFields,
		KubeClientSet:   kubeClientSet,
	}
}

// Scrub scrubs sensitive data from a function config
func (s *Scrubber) Scrub(functionConfig *Config,
	existingSecretMap map[string]string,
	sensitiveFields []*regexp.Regexp) (*Config, map[string]string, error) {

	var scrubErr error

	// hack to support avoid losing unexported fields while scrubbing.
	// scrub the function config to map[string]interface{} and revert it back to a function config later
	functionConfigAsMap := common.StructureToMap(functionConfig)
	if len(functionConfigAsMap) == 0 {
		return nil, nil, errors.New("Failed to convert function config to map")
	}

	// scrub the function config
	scrubbedFunctionConfigAsMap, secretsMap := gosecretive.Scrub(functionConfigAsMap, func(fieldPath string, valueToScrub interface{}) *string {

		for _, fieldPathRegexToScrub := range sensitiveFields {

			// if the field path matches the field path to scrub, scrub it
			if fieldPathRegexToScrub.MatchString(fieldPath) {

				secretKey := s.generateSecretKey(fieldPath)

				// if the value to scrub is a string, make sure that we need to scrub it
				if kind := reflect.ValueOf(valueToScrub).Kind(); kind == reflect.String {
					stringValue := reflect.ValueOf(valueToScrub).String()

					// if it's an empty string, don't scrub it
					if stringValue == "" {
						return nil
					}

					// if it's already a reference, validate that the value exists
					if strings.HasPrefix(stringValue, ReferencePrefix) {
						scrubErr = s.validateReference(functionConfig, existingSecretMap, fieldPath, secretKey, stringValue)
						return nil
					}
				}

				// scrub the value, and leave a $ref placeholder
				return &secretKey
			}
		}

		// do not scrub
		return nil
	})

	// merge the new secrets map with the existing one
	// In case of a conflict, the new secrets map will override the existing value
	if existingSecretMap != nil {
		secretsMap = labels.Merge(existingSecretMap, secretsMap)
	}

	scrubbedFunctionConfig, err := s.convertMapToConfig(scrubbedFunctionConfigAsMap)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to convert scrubbed function config map to function config")
	}

	return scrubbedFunctionConfig, secretsMap, scrubErr
}

// Restore restores sensitive data in a function config from a secrets map
func (s *Scrubber) Restore(scrubbedFunctionConfig *Config, secretsMap map[string]string) (*Config, error) {

	// hack to avoid changing complex objects in the function config.
	// convert the function config to map[string]interface{} and revert it back to a function config later
	scrubbedFunctionConfigAsMap := common.StructureToMap(scrubbedFunctionConfig)
	if len(scrubbedFunctionConfigAsMap) == 0 {
		return nil, errors.New("Failed to convert function config to map")
	}

	restoredFunctionConfigMap := gosecretive.Restore(scrubbedFunctionConfigAsMap, secretsMap)

	restoredFunctionConfig, err := s.convertMapToConfig(restoredFunctionConfigMap)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to convert restored function config map to function config")
	}

	return restoredFunctionConfig, nil
}

// RestoreFunctionConfig restores a function config from a secret, in case we're running in a kube platform
func (s *Scrubber) RestoreFunctionConfig(ctx context.Context,
	functionConfig *Config,
	platformName string,
	getSecretMapCallback func(ctx context.Context, functionName, functionNamespace string) (map[string]string, error)) (*Config, error) {

	// if we're in kube platform, we need to restore the function config's
	// sensitive data from the function's secret
	if platformName == common.KubePlatformName {
		secretMap, err := getSecretMapCallback(ctx,
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
			return restoredFunctionConfig, nil
		}
	}

	// if we're not in kube platform, or the function doesn't have a secret, just return the function config
	return functionConfig, nil
}

// EncodeSecretsMap encodes the keys of a secrets map
func (s *Scrubber) EncodeSecretsMap(secretsMap map[string]string) (map[string]string, error) {
	encodedSecretsMap := map[string]string{}

	// encode secret map keys
	for secretKey, secretValue := range secretsMap {
		encodedSecretsMap[s.encodeSecretKey(secretKey)] = secretValue
	}

	if len(encodedSecretsMap) > 0 {

		// encode the entire map into a single string
		secretsMapContent, err := json.Marshal(encodedSecretsMap)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to marshal secrets map")
		}
		encodedSecretsMap[SecretContentKey] = base64.StdEncoding.EncodeToString(secretsMapContent)
	} else {

		// if the map is empty, set the content to an empty string anyway,
		// so that the secret will be created and mounted
		encodedSecretsMap[SecretContentKey] = ""
	}

	return encodedSecretsMap, nil
}

// DecodeSecretsMapContent decodes the secrets map content
func (s *Scrubber) DecodeSecretsMapContent(secretsMapContent string) (map[string]string, error) {

	// decode secret
	secretContentStr, err := base64.StdEncoding.DecodeString(secretsMapContent)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to decode function secret")
	}
	if len(secretContentStr) == 0 {

		// secret is empty, return empty map
		return map[string]string{}, nil
	}

	// unmarshal secret into map
	encodedSecretMap := map[string]string{}
	if err := json.Unmarshal(secretContentStr, &encodedSecretMap); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal function secret")
	}

	// decode secret keys and values
	// convert values to byte array for decoding purposes
	secretMap, err := s.DecodeSecretData(common.MapStringStringToMapStringBytesArray(encodedSecretMap))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to decode function secret data")
	}

	return secretMap, nil
}

// DecodeSecretData decodes the keys of a secrets map
func (s *Scrubber) DecodeSecretData(secretData map[string][]byte) (map[string]string, error) {
	decodedSecretsMap := map[string]string{}
	for secretKey, secretValue := range secretData {
		if secretKey == SecretContentKey {

			// when the secret is created, the entire map is encoded into a single string under the "content" key
			// which we don't care about when decoding
			continue
		}
		decodedSecretKey, err := s.decodeSecretKey(secretKey)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to decode secret key")
		}
		decodedSecretsMap[decodedSecretKey] = string(secretValue)
	}
	return decodedSecretsMap, nil
}

// GenerateFunctionSecretName generates a secret name for a function, in the form of:
// `nuclio-secret-<project-name>-<function-name>-<unique-id>`
func (s *Scrubber) GenerateFunctionSecretName(functionName string) string {
	secretName := fmt.Sprintf("%s-%s", "nuclio", functionName)
	if len(secretName) > common.KubernetesDomainLevelMaxLength-8 {
		secretName = secretName[:common.KubernetesDomainLevelMaxLength-8]
	}

	// remove trailing non-alphanumeric characters
	secretName = strings.TrimRight(secretName, "-_")

	// add a unique id to the end of the name
	secretName = fmt.Sprintf("%s-%s", secretName, common.GenerateRandomString(8, common.SmallLettersAndNumbers))

	return secretName
}

// GenerateFlexVolumeSecretName generates a secret name for a flex volume, in the form of:
// `nuclio-flex-volume-<volume-name>-<unique-id>`
func (s *Scrubber) GenerateFlexVolumeSecretName(functionName, volumeName string) string {
	secretName := fmt.Sprintf("%s-%s-%s", NuclioFlexVolumeSecretNamePrefix, functionName, volumeName)

	// if the secret name is too long, drop the function and project name
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

// HasScrubbedConfig checks if a function config has scrubbed data, using the Scrub function
func (s *Scrubber) HasScrubbedConfig(functionConfig *Config, sensitiveFields []*regexp.Regexp) (bool, error) {
	var hasScrubbed bool

	// hack to support avoid losing unexported fields while scrubbing.
	// scrub the function config to map[string]interface{} and revert it back to a function config later
	functionConfigAsMap := common.StructureToMap(functionConfig)
	if len(functionConfigAsMap) == 0 {
		return false, errors.New("Failed to convert function config to map")
	}

	// scrub the function config
	_, _ = gosecretive.Scrub(functionConfigAsMap, func(fieldPath string, valueToScrub interface{}) *string {

		for _, fieldPathRegexToScrub := range sensitiveFields {

			// if the field path matches the field path to scrub, scrub it
			if fieldPathRegexToScrub.MatchString(fieldPath) {

				// if the value to is a string, check if it's a reference
				if kind := reflect.ValueOf(valueToScrub).Kind(); kind == reflect.String {
					stringValue := reflect.ValueOf(valueToScrub).String()

					if strings.HasPrefix(stringValue, ReferencePrefix) {
						hasScrubbed = true
					}
				}

				// we never actually scrub the value
				return nil
			}
		}

		return nil
	})

	return hasScrubbed, nil
}

// encodeSecretKey encodes a secret key
func (s *Scrubber) encodeSecretKey(fieldPath string) string {
	encodedFieldPath := base64.StdEncoding.EncodeToString([]byte(fieldPath))
	encodedFieldPath = strings.ReplaceAll(encodedFieldPath, "=", "_")
	return fmt.Sprintf("%s%s", ReferenceToEnvVarPrefix, encodedFieldPath)
}

// decodeSecretKey decodes a secret key and returns the original field
func (s *Scrubber) decodeSecretKey(secretKey string) (string, error) {
	encodedFieldPath := strings.TrimPrefix(secretKey, ReferenceToEnvVarPrefix)
	encodedFieldPath = strings.ReplaceAll(encodedFieldPath, "_", "=")
	decodedFieldPath, err := base64.StdEncoding.DecodeString(encodedFieldPath)
	if err != nil {
		return "", errors.Wrap(err, "Failed to decode secret key")
	}
	return string(decodedFieldPath), nil
}

func (s *Scrubber) generateSecretKey(fieldPath string) string {
	return fmt.Sprintf("%s%s", ReferencePrefix, strings.ToLower(fieldPath))
}

func (s *Scrubber) validateReference(functionConfig *Config,
	existingSecretMap map[string]string,
	fieldPath,
	secretKey,
	stringValue string) error {

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

func (s *Scrubber) convertMapToConfig(mapConfig interface{}) (*Config, error) {

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
