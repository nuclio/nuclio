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
	NuclioSecretNamePrefix           = "nuclio-secret-"
	NuclioFlexVolumeSecretNamePrefix = "nuclio-flexvolume-"
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
// If the scrubber is only used for restoring, the arguments and logger can be nil
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

	// marshal and unmarshal the scrubbed object back to function config
	scrubbedFunctionConfig := &Config{}
	masrhalledScrubbedFunctionConfig, err := json.Marshal(scrubbedFunctionConfigAsMap.(map[string]interface{}))
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to marshal scrubbed function config")
	}
	if err := json.Unmarshal(masrhalledScrubbedFunctionConfig, scrubbedFunctionConfig); err != nil {
		return nil, nil, errors.Wrap(err, "Failed to unmarshal scrubbed function config")
	}

	return scrubbedFunctionConfig, secretsMap, scrubErr
}

// Restore restores sensitive data in a function config from a secrets map
func (s *Scrubber) Restore(scrubbedFunctionConfig *Config, secretsMap map[string]string) (*Config, error) {
	restored := gosecretive.Restore(scrubbedFunctionConfig, secretsMap)
	return restored.(*Config), nil
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

func (s *Scrubber) GenerateFunctionSecretName(functionName, secretPrefix string) string {
	secretName := fmt.Sprintf("%s%s", secretPrefix, functionName)
	if len(secretName) > common.KubernetesDomainLevelMaxLength {
		secretName = secretName[:common.KubernetesDomainLevelMaxLength]
	}

	// remove trailing non-alphanumeric characters
	secretName = strings.TrimRight(secretName, "-_")

	return secretName
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
