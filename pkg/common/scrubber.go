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

package common

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/nuclio/errors"
	"github.com/nuclio/gosecretive"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

const (
	ReferencePrefix         = "$ref:"
	ReferenceToEnvVarPrefix = "NUCLIO_B64_"
	SecretContentKey        = "content"
)

type Scrubber interface {
	// Scrub scrubs sensitive data from an object
	Scrub(objectToScrub interface{}, existingSecretMap map[string]string, sensitiveFields []*regexp.Regexp) (interface{}, map[string]string, error)

	// Restore restores sensitive data in an object from a secrets map
	Restore(scrubbedObject interface{}, secretsMap map[string]string) (interface{}, error)

	// ValidateReference validates references in a scrubbed object
	ValidateReference(objectToScrub interface{},
		existingSecretMap map[string]string,
		fieldPath,
		secretKey,
		stringValue string) error

	// ConvertMapToConfig converts map back to an object entity
	ConvertMapToConfig(mapConfig interface{}) (interface{}, error)
}

type AbstractScrubber struct {
	SensitiveFields []*regexp.Regexp
	KubeClientSet   kubernetes.Interface
	ReferencePrefix string
	Scrubber        Scrubber
}

// NewAbstractScrubber returns a new AbstractScrubber
// If the scrubber is only used for restoring, the arguments can be nil
func NewAbstractScrubber(sensitiveFields []*regexp.Regexp, kubeClientSet kubernetes.Interface) *AbstractScrubber {
	return &AbstractScrubber{
		SensitiveFields: sensitiveFields,
		KubeClientSet:   kubeClientSet,
		ReferencePrefix: ReferencePrefix,
	}
}

// Scrub scrubs sensitive data from an object
func (s *AbstractScrubber) Scrub(objectToScrub interface{},
	existingSecretMap map[string]string,
	sensitiveFields []*regexp.Regexp) (interface{}, map[string]string, error) {

	var scrubErr error

	// hack to support avoid losing unexported fields while scrubbing.
	// scrub the object to map[string]interface{} and revert it back to an object later
	objectAsMap := StructureToMap(objectToScrub)
	if len(objectAsMap) == 0 {
		return nil, nil, errors.New("Failed to convert object to map")
	}

	// scrub the object
	scrubbedObjectAsMap, secretsMap := gosecretive.Scrub(objectAsMap, func(fieldPath string, valueToScrub interface{}) *string {

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
						scrubErr = s.ValidateReference(objectToScrub, existingSecretMap, fieldPath, secretKey, stringValue)
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

	scrubbedObjectConfig, err := s.ConvertMapToConfig(scrubbedObjectAsMap)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to convert scrubbed object map to object entity")
	}

	return scrubbedObjectConfig, secretsMap, scrubErr
}

// Restore restores sensitive data in an object from a secrets map
func (s *AbstractScrubber) Restore(scrubbedObject interface{}, secretsMap map[string]string) (interface{}, error) {

	// hack to avoid changing complex objects in the object.
	// convert the object to map[string]interface{} and revert it back to an object entity later
	scrubbedObjectAsMap := StructureToMap(scrubbedObject)
	if len(scrubbedObjectAsMap) == 0 {
		return nil, errors.New("Failed to convert object to map")
	}

	restoredObjectMap := gosecretive.Restore(scrubbedObjectAsMap, secretsMap)

	restoredObject, err := s.ConvertMapToConfig(restoredObjectMap)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to convert restored object map to an object entity")
	}

	return restoredObject, nil
}

// HasScrubbedConfig checks if a object has scrubbed data, using the Scrub function
func (s *AbstractScrubber) HasScrubbedConfig(object interface{}, sensitiveFields []*regexp.Regexp) (bool, error) {
	var hasScrubbed bool

	// hack to support avoid losing unexported fields while scrubbing.
	// scrub the object to map[string]interface{} and revert it back to an object entity later
	objectAsMap := StructureToMap(object)
	if len(objectAsMap) == 0 {
		return false, errors.New("Failed to convert object to map")
	}

	// scrub the object
	_, _ = gosecretive.Scrub(objectAsMap, func(fieldPath string, valueToScrub interface{}) *string {

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

// EncodeSecretsMap encodes the keys of a secrets map
func (s *AbstractScrubber) EncodeSecretsMap(secretsMap map[string]string) (map[string]string, error) {
	encodedSecretsMap := map[string]string{}

	// encode secret map keys
	for secretKey, secretValue := range secretsMap {
		encodedSecretsMap[s.EncodeSecretKey(secretKey)] = secretValue
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
func (s *AbstractScrubber) DecodeSecretsMapContent(secretsMapContent string) (map[string]string, error) {

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
	secretMap, err := s.DecodeSecretData(MapStringStringToMapStringBytesArray(encodedSecretMap))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to decode function secret data")
	}

	return secretMap, nil
}

// DecodeSecretData decodes the keys of a secrets map
func (s *AbstractScrubber) DecodeSecretData(secretData map[string][]byte) (map[string]string, error) {
	decodedSecretsMap := map[string]string{}
	for secretKey, secretValue := range secretData {
		if secretKey == SecretContentKey {

			// when the secret is created, the entire map is encoded into a single string under the "content" key
			// which we don't care about when decoding
			continue
		}
		decodedSecretKey, err := s.DecodeSecretKey(secretKey)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to decode secret key")
		}
		decodedSecretsMap[decodedSecretKey] = string(secretValue)
	}
	return decodedSecretsMap, nil
}

// GenerateObjectSecretName e generates a secret name for a function, in the form of:
// `nuclio-secret-<project-name>-<object-name>-<unique-id>`
func (s *AbstractScrubber) GenerateObjectSecretName(objectName string) string {
	secretName := fmt.Sprintf("%s-%s", "nuclio", objectName)
	if len(secretName) > KubernetesDomainLevelMaxLength-8 {
		secretName = secretName[:KubernetesDomainLevelMaxLength-8]
	}

	// remove trailing non-alphanumeric characters
	secretName = strings.TrimRight(secretName, "-_")

	// add a unique id to the end of the name
	secretName = fmt.Sprintf("%s-%s", secretName, GenerateRandomString(8, SmallLettersAndNumbers))

	return secretName
}

// EncodeSecretKey encodes a secret key
func (s *AbstractScrubber) EncodeSecretKey(fieldPath string) string {
	encodedFieldPath := base64.StdEncoding.EncodeToString([]byte(fieldPath))
	encodedFieldPath = strings.ReplaceAll(encodedFieldPath, "=", "_")
	return fmt.Sprintf("%s%s", ReferenceToEnvVarPrefix, encodedFieldPath)
}

// DecodeSecretKey decodes a secret key and returns the original field
func (s *AbstractScrubber) DecodeSecretKey(secretKey string) (string, error) {
	encodedFieldPath := strings.TrimPrefix(secretKey, ReferenceToEnvVarPrefix)
	encodedFieldPath = strings.ReplaceAll(encodedFieldPath, "_", "=")
	decodedFieldPath, err := base64.StdEncoding.DecodeString(encodedFieldPath)
	if err != nil {
		return "", errors.Wrap(err, "Failed to decode secret key")
	}
	return string(decodedFieldPath), nil
}

func (s *AbstractScrubber) ConvertMapToConfig(mapConfig interface{}) (interface{}, error) {
	return s.Scrubber.ConvertMapToConfig(mapConfig)
}

func (s *AbstractScrubber) ValidateReference(objectToScrub interface{},
	existingSecretMap map[string]string,
	fieldPath,
	secretKey,
	stringValue string) error {
	return s.Scrubber.ValidateReference(objectToScrub, existingSecretMap, fieldPath, secretKey, stringValue)
}

func (s *AbstractScrubber) generateSecretKey(fieldPath string) string {
	return fmt.Sprintf("%s%s", ReferencePrefix, strings.ToLower(fieldPath))
}
