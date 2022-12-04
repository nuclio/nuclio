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
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
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
	HasSecretAnnotation              = "nuclio.io/has-secret"
	FunctionSecretMountPath          = "/etc/nuclio/secrets"
	AccessKeyLabel                   = "nuclio.io/access-key"
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
							trimmedSecretKey := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(secretKey, ReferencePrefix)))
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
	if existingSecretMap != nil {
		secretsMap = labels.Merge(secretsMap, existingSecretMap)
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

// EncodeSecretsMap encodes the keys of a secrets map
func EncodeSecretsMap(secretsMap map[string]string) (map[string]string, error) {
	encodedSecretsMap := map[string]string{}

	// encode secret map keys
	for secretKey, secretValue := range secretsMap {
		encodedSecretsMap[encodeSecretKey(secretKey)] = secretValue
	}

	// encode the entire map into a single string
	secretsMapContent, err := json.Marshal(encodedSecretsMap)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to marshal secrets map")
	}
	encodedSecretsMap[SecretContentKey] = base64.StdEncoding.EncodeToString(secretsMapContent)

	return encodedSecretsMap, nil
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

func GenerateAccessKeyRefHashString(accessKeyRef string) string {

	// using md5 for a shorter hash string (vs sha256) as k8s has a 63 character limit on secret keys
	hash := md5.Sum([]byte(strings.TrimPrefix(accessKeyRef, ReferencePrefix)))
	return hex.EncodeToString(hash[:])
}

// encodeSecretKey encodes a secret key
func encodeSecretKey(fieldPath string) string {
	fieldPath = strings.TrimPrefix(fieldPath, ReferencePrefix)
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
