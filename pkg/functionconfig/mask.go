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
	"strings"

	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/errors"
	"github.com/nuclio/gosecretive"
	"k8s.io/apimachinery/pkg/labels"
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

// Scrub scrubs sensitive data from a function config
func Scrub(functionConfig *Config,
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

				secretKey := generateSecretKey(fieldPath)

				// if the value to scrub is a string, make sure that we need to scrub it
				if kind := reflect.ValueOf(valueToScrub).Kind(); kind == reflect.String {
					stringValue := reflect.ValueOf(valueToScrub).String()

					// if it's an empty string, don't scrub it
					if stringValue == "" {
						return nil
					}

					// if it's already a reference, validate that it a previous secret map exists,
					// and contains the reference
					if strings.HasPrefix(stringValue, ReferencePrefix) {
						if existingSecretMap != nil {
							trimmedSecretKey := strings.ToLower(strings.TrimSpace(secretKey))
							if _, exists := existingSecretMap[trimmedSecretKey]; !exists {
								scrubErr = errors.New(fmt.Sprintf("Config data in path %s is already masked, but original value does not exist in secret", fieldPath))
							}
						} else {
							scrubErr = errors.New(fmt.Sprintf("Config data in path %s is already masked, but secret does not exist.", fieldPath))
						}
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
func Restore(scrubbedFunctionConfig *Config, secretsMap map[string]string) (*Config, error) {
	restored := gosecretive.Restore(scrubbedFunctionConfig, secretsMap)
	return restored.(*Config), nil
}

// RestoreFunctionConfig restores a function config from a secret, in case we're running in a kube platform
func RestoreFunctionConfig(ctx context.Context,
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
		if secretMap != nil {

			// restore the function config
			restoredFunctionConfig, err := Restore(functionConfig, secretMap)
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
func EncodeSecretsMap(secretsMap map[string]string) (map[string]string, error) {
	encodedSecretsMap := map[string]string{}

	// encode secret map keys
	for secretKey, secretValue := range secretsMap {
		encodedSecretsMap[encodeSecretKey(secretKey)] = secretValue
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
func DecodeSecretsMapContent(secretsMapContent string) (map[string]string, error) {

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
	secretMap, err := DecodeSecretData(common.MapStringStringToMapStringBytesArray(encodedSecretMap))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to decode function secret data")
	}

	return secretMap, nil
}

// DecodeSecretData decodes the keys of a secrets map
func DecodeSecretData(secretData map[string][]byte) (map[string]string, error) {
	decodedSecretsMap := map[string]string{}
	for secretKey, secretValue := range secretData {
		if secretKey == SecretContentKey {

			// when the secret is created, the entire map is encoded into a single string under the "content" key
			// which we don't care about when decoding
			continue
		}
		decodedSecretKey, err := decodeSecretKey(secretKey)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to decode secret key")
		}
		decodedSecretsMap[decodedSecretKey] = string(secretValue)
	}
	return decodedSecretsMap, nil
}

func ResolveEnvVarNameFromReference(reference string) string {
	fieldPath := strings.TrimPrefix(reference, ReferencePrefix)
	return encodeSecretKey(fieldPath)
}

func GenerateFunctionSecretName(functionName, secretPrefix string) string {
	secretName := fmt.Sprintf("%s%s", secretPrefix, functionName)
	if len(secretName) > common.KubernetesDomainLevelMaxLength {
		secretName = secretName[:common.KubernetesDomainLevelMaxLength]
	}

	// remove trailing non-alphanumeric characters
	secretName = strings.TrimRight(secretName, "-_")

	return secretName
}

// HasScrubbedConfig checks if a function config has scrubbed data, using the Scrub function
func HasScrubbedConfig(functionConfig *Config, sensitiveFields []*regexp.Regexp) (bool, error) {
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
func encodeSecretKey(fieldPath string) string {
	encodedFieldPath := base64.StdEncoding.EncodeToString([]byte(fieldPath))
	encodedFieldPath = strings.ReplaceAll(encodedFieldPath, "=", "_")
	return fmt.Sprintf("%s%s", ReferenceToEnvVarPrefix, encodedFieldPath)
}

// decodeSecretKey decodes a secret key and returns the original field
func decodeSecretKey(secretKey string) (string, error) {
	encodedFieldPath := strings.TrimPrefix(secretKey, ReferenceToEnvVarPrefix)
	encodedFieldPath = strings.ReplaceAll(encodedFieldPath, "_", "=")
	decodedFieldPath, err := base64.StdEncoding.DecodeString(encodedFieldPath)
	if err != nil {
		return "", errors.Wrap(err, "Failed to decode secret key")
	}
	return string(decodedFieldPath), nil
}

func generateSecretKey(fieldPath string) string {
	return fmt.Sprintf("%s%s", ReferencePrefix, strings.ToLower(fieldPath))
}
