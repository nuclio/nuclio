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

package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"k8s.io/client-go/kubernetes"
)

const SecretTypeAPIGatewayConfig = "nuclio.io/apigatewayconfig"

type APIGateway interface {

	// GetConfig returns the api gateway config
	GetConfig() *APIGatewayConfig
}

type AbstractAPIGateway struct {
	Logger           logger.Logger
	Platform         Platform
	APIGatewayConfig APIGatewayConfig
}

func NewAbstractAPIGateway(parentLogger logger.Logger,
	parentPlatform Platform,
	apiGatewayConfig APIGatewayConfig) (*AbstractAPIGateway, error) {

	return &AbstractAPIGateway{
		Logger:           parentLogger.GetChild("api gateway"),
		Platform:         parentPlatform,
		APIGatewayConfig: apiGatewayConfig,
	}, nil
}

// GetConfig returns the api gateway config
func (ap *AbstractAPIGateway) GetConfig() *APIGatewayConfig {
	return &ap.APIGatewayConfig
}

type APIGatewayScrubber struct {
	*common.AbstractScrubber
}

func NewAPIGatewayScrubber(parentLogger logger.Logger,
	sensitiveFields []*regexp.Regexp,
	kubeClientSet kubernetes.Interface) *APIGatewayScrubber {
	abstractScrubber := common.NewAbstractScrubber(sensitiveFields, kubeClientSet, common.ReferencePrefix, common.NuclioResourceLabelKeyApiGatewayName, SecretTypeAPIGatewayConfig, parentLogger, func(name string) bool {
		return false
	})
	scrubber := &APIGatewayScrubber{abstractScrubber}
	abstractScrubber.Scrubber = scrubber
	return scrubber
}

// RestoreAPIGatewayConfig restores an API Gateway config from a secret, in case we're running in a kube platform
func (s *APIGatewayScrubber) RestoreAPIGatewayConfig(ctx context.Context,
	config *APIGatewayConfig) (*APIGatewayConfig, error) {

	secretMap, err := s.GetObjectSecretMap(ctx,
		config.Meta.Name,
		config.Meta.Namespace)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get api gateway secret")
	}
	if len(secretMap) > 0 {

		// restore the api gateway config
		restoredConfig, err := s.Restore(config, secretMap)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to restore api gateway config")
		}
		return restoredConfig.(*APIGatewayConfig), nil
	}

	// if we're not in kube platform, or the api gateway doesn't have a secret, just return the api gateway config
	return config, nil
}

func (s *APIGatewayScrubber) ValidateReference(objectToScrub interface{},
	existingSecretMap map[string]string,
	fieldPath,
	secretKey,
	stringValue string) error {
	// we need to check if the secret exists in the secret map
	if existingSecretMap != nil {
		trimmedSecretKey := strings.ToLower(strings.TrimSpace(secretKey))
		if _, exists := existingSecretMap[trimmedSecretKey]; !exists {
			return errors.New(fmt.Sprintf("Config data in path %s is already scrubbed, but original value does not exist in secret", fieldPath))
		}
		return nil
	}

	return errors.New(fmt.Sprintf("Config data in path %s is already masked, but secret does not exist.", fieldPath))
}

func (s *APIGatewayScrubber) ConvertMapToConfig(mapConfig interface{}) (interface{}, error) {
	// marshal and unmarshal the map object back to api gateway config
	apiGatewayConfig := &APIGatewayConfig{}

	masrhalledAPIGatewayConfig, err := json.Marshal(mapConfig.(map[string]interface{}))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to marshal scrubbed API Gateway config")
	}
	if err := json.Unmarshal(masrhalledAPIGatewayConfig, apiGatewayConfig); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal scrubbed API Gateway config")
	}

	return apiGatewayConfig, nil
}

func GetAPIGatewaySensitiveField() []*regexp.Regexp {
	var regexpList []*regexp.Regexp
	for _, sensitiveFieldPath := range []string{
		// Path nested in a map
		"^/spec/authentication/basicAuth/password$",
	} {
		regexpList = append(regexpList, regexp.MustCompile("(?i)"+sensitiveFieldPath))
	}
	return regexpList
}

func (s *APIGatewayScrubber) ScrubAPIGatewayConfig(ctx context.Context,
	apiGatewayConfig *APIGatewayConfig) (*APIGatewayConfig, error) {
	var err error

	existingSecretName, err := s.GetObjectSecretName(
		ctx, apiGatewayConfig.Meta.Name,
		apiGatewayConfig.Meta.Namespace)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get api gateway config secret name")
	}

	scrubbedAPIGatewayConfig, existingSecretName, secretsMap, err := s.GetExistingSecretAndScrub(ctx, apiGatewayConfig,
		apiGatewayConfig.Meta.Name, apiGatewayConfig.Meta.Namespace, existingSecretName)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get existing secret and scrub api gateway config")
	}

	// encode secrets map
	encodedSecretsMap, err := s.EncodeSecretsMap(secretsMap)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to encode secrets map")
	}

	// create or update a secret for the api gateway
	if err := s.CreateOrUpdateObjectSecret(ctx,
		encodedSecretsMap,
		existingSecretName,
		apiGatewayConfig.Meta.Name,
		apiGatewayConfig.Meta.Namespace,
		apiGatewayConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName]); err != nil {
		return nil, errors.Wrap(err, "Failed to create or update api gateway secret")
	}

	return GetAPIGatewayConfigFromInterface(scrubbedAPIGatewayConfig), nil
}
