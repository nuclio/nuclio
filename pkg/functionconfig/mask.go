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
	"encoding/base64"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/nuclio/errors"
	"github.com/nuclio/gosecretive"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	referencePrefix         = "$ref:"
	referenceToEnvVarPrefix = "NUCLIO_B64_"
	NuclioSecretNamePrefix  = "nuclio-secret-"
	NuclioSecretType        = "nuclio.io/functionconfig"
)

// Scrub scrubs sensitive data from a function config
func Scrub(functionConfig *Config,
	existingSecretMap map[string]string,
	sensitiveFields []*regexp.Regexp) (*Config, map[string]string, error) {

	var err error

	// scrub the function config
	scrubbedFunctionConfig, secretsMap := gosecretive.Scrub(functionConfig, func(fieldPath string, valueToScrub interface{}) *string {

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
					if strings.HasPrefix(stringValue, referencePrefix) {
						if existingSecretMap != nil {
							if _, exists := existingSecretMap[secretKey]; !exists {
								err = errors.New(fmt.Sprintf("Config data in path %s is already masked, but original value does not exist in secret", fieldPath))
							}
						} else {
							err = errors.New(fmt.Sprintf("Config data in path %s is already masked, but secret does not exist.", fieldPath))
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
	if existingSecretMap != nil {
		secretsMap = labels.Merge(secretsMap, existingSecretMap)
	}

	return scrubbedFunctionConfig.(*Config), secretsMap, err
}

// Restore restores sensitive data in a function config from a secrets map
func Restore(scrubbedFunctionConfig *Config, secretsMap map[string]string) (*Config, error) {
	restored := gosecretive.Restore(scrubbedFunctionConfig, secretsMap)
	return restored.(*Config), nil
}

// EncodeSecretsMap encodes the keys of a secrets map
func EncodeSecretsMap(secretsMap map[string]string) map[string]string {
	encodedSecretsMap := map[string]string{}
	for secretKey, secretValue := range secretsMap {
		encodedSecretsMap[EncodeSecretKey(secretKey)] = secretValue
	}
	return encodedSecretsMap
}

// DecodeSecretData decodes the keys of a secrets map
func DecodeSecretData(secretData map[string][]byte) (map[string]string, error) {
	decodedSecretsMap := map[string]string{}
	for secretKey, secretValue := range secretData {
		decodedSecretKey, err := DecodeSecretKey(secretKey)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to decode secret key")
		}
		decodedSecretValue, err := base64.StdEncoding.DecodeString(string(secretValue))
		if err != nil {
			return nil, errors.Wrap(err, "Failed to decode secret key")
		}
		decodedSecretsMap[decodedSecretKey] = string(decodedSecretValue)
	}
	return decodedSecretsMap, nil
}

// EncodeSecretKey encodes a secret key
func EncodeSecretKey(fieldPath string) string {
	fieldPath = strings.TrimPrefix(fieldPath, referencePrefix)
	encodedFieldPath := base64.StdEncoding.EncodeToString([]byte(fieldPath))
	encodedFieldPath = strings.ReplaceAll(encodedFieldPath, "=", "_")
	return fmt.Sprintf("%s%s", referenceToEnvVarPrefix, encodedFieldPath)
}

// DecodeSecretKey decodes a secret key and returns the original field
func DecodeSecretKey(secretKey string) (string, error) {
	encodedFieldPath := strings.TrimPrefix(secretKey, referenceToEnvVarPrefix)
	encodedFieldPath = strings.ReplaceAll(encodedFieldPath, "_", "=")
	decodedFieldPath, err := base64.StdEncoding.DecodeString(encodedFieldPath)
	if err != nil {
		return "", errors.Wrap(err, "Failed to decode secret key")
	}
	return string(decodedFieldPath), nil
}

func ResolveEnvVarNameFromReference(reference string) string {
	fieldPath := strings.TrimPrefix(reference, referencePrefix)
	return EncodeSecretKey(fieldPath)
}

func generateSecretKey(fieldPath string) string {
	return fmt.Sprintf("%s%s", referencePrefix, fieldPath)
}
