/*
Copyright 2025 The Nuclio Authors.

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

package utils

import (
	"context"
	"fmt"
	"strings"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/k8s"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
	"k8s.io/api/core/v1"
	kubeapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation"
)

// ValidateLabels validates the given labels according to k8s label constraints
func ValidateLabels(labels map[string]string) error {
	if labels == nil {
		return nil
	}
	for labelKey, labelValue := range labels {
		if errs := validation.IsValidLabelValue(labelValue); len(errs) > 0 {
			errs = append([]string{fmt.Sprintf("Invalid value: %s", labelValue)}, errs...)
			return nuclio.NewErrBadRequest(strings.Join(errs, ", "))
		}

		// Valid label keys have two segments: an optional prefix and name, separated by a slash (/).
		// The name segment is required and must conform to the rules of a valid label value.
		// The prefix is optional. If specified, the prefix must be a DNS subdomain.
		if errs := validation.IsQualifiedName(labelKey); len(errs) > 0 {
			errs = append([]string{fmt.Sprintf("Invalid key: %s", labelKey)}, errs...)
			return nuclio.NewErrBadRequest(strings.Join(errs, ", "))
		}
	}
	return nil
}

// FilterInvalidLabels filters out invalid kubernetes labels from a map of labels
func FilterInvalidLabels(labels map[string]string) map[string]string {

	// From k8s docs:
	//   a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.',
	//   and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345',
	//   regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')
	filteredLabels := map[string]string{}
	for key, value := range labels {
		if len(validation.IsQualifiedName(key)) != 0 || len(validation.IsValidLabelValue(value)) != 0 {
			continue
		}
		filteredLabels[key] = value
	}
	return filteredLabels
}

// MergeNodeSelector merges function, project and platform NodeSelectors
// where function values take precedence over project values, and project values take precedence over platform values
func MergeNodeSelector(functionNodeSelector,
	projectNodeSelector,
	platformNodeSelector map[string]string) map[string]string {

	if functionNodeSelector == nil {
		if projectNodeSelector == nil &&
			platformNodeSelector == nil {
			return nil
		}
	}

	defaultNodeSelector := labels.Merge(platformNodeSelector, projectNodeSelector)
	mergedNodeSelector := labels.Merge(defaultNodeSelector, functionNodeSelector)

	// Remove keys with empty values
	for key, value := range mergedNodeSelector {
		if value == "" {
			delete(mergedNodeSelector, key)
		}
	}

	if len(mergedNodeSelector) == 0 {
		return nil
	}

	return mergedNodeSelector
}

// EnrichProbe sets default values for the probe if they are not already set
// It uses the default probe as a reference for the default values
// probe is being passed with ** lets reassign it to the defaultProbe if it is nil
func EnrichProbe(probe **v1.Probe, defaultProbe *v1.Probe) {
	if *probe == nil {
		*probe = defaultProbe
		return
	}

	// InitialDelaySeconds can technically be 0, but only allow setting it to greater than 0 so that there will always be a delay before the first probe check
	if (*probe).InitialDelaySeconds == 0 {
		(*probe).InitialDelaySeconds = defaultProbe.InitialDelaySeconds
	}

	if (*probe).TimeoutSeconds == 0 {
		(*probe).TimeoutSeconds = defaultProbe.TimeoutSeconds
	}

	if (*probe).PeriodSeconds == 0 {
		(*probe).PeriodSeconds = defaultProbe.PeriodSeconds
	}

	if (*probe).FailureThreshold == 0 {
		(*probe).FailureThreshold = defaultProbe.FailureThreshold
	}
}

// GetStringValueFromSecret returns the string value from the secret by the given key and true if the key exists
func GetStringValueFromSecret(secret *v1.Secret, key string) (string, bool) {
	if secret == nil || key == "" {
		return "", false
	}
	value, ok := secret.Data[key]
	return string(value), ok
}

// EnrichAndValidateServiceAccount enriches the service account with the default platform service account if it is not set
// and validates that the service account is allowed for the project by checking the project secret
// It returns the enriched service account and an error if the service account is not allowed or if there was an error fetching the project secret
func EnrichAndValidateServiceAccount(ctx context.Context,
	kubeClient k8s.Client,
	defaultPlatformServiceAccount,
	projectSecretTemplate,
	projectSecretDefaultServiceAccountKey,
	projectSecretAllowedServiceAccountsKey,
	serviceAccount,
	projectName,
	namespace string,
	shouldEnrich bool) (string, error) {

	// fetch the secret from Kubernetes
	secret, err := GetProjectSecret(ctx,
		kubeClient,
		projectSecretTemplate,
		projectName,
		namespace)

	if err != nil {
		return "", errors.Wrapf(err, "Failed to get project secret. Project: %s", projectName)
	}

	if shouldEnrich {
		serviceAccount = EnrichServiceAccount(
			secret,
			projectSecretDefaultServiceAccountKey,
			serviceAccount,
			defaultPlatformServiceAccount)
	}
	if err = IsServiceAccountAllowed(secret, projectSecretAllowedServiceAccountsKey, serviceAccount); err != nil {
		return "", errors.Wrapf(err, "Service account %s is not allowed for project %s", serviceAccount, projectName)
	}

	return serviceAccount, nil
}

func GetProjectSecret(ctx context.Context, kubeClient k8s.Client, projectSecretTemplate, projectName, namespace string) (*v1.Secret, error) {
	// if project secret template is not specified, return empty data
	if projectSecretTemplate == "" {
		return nil, nil
	}

	// render the project secret name using the template
	templateData := map[string]interface{}{
		"ProjectName": projectName,
		"Namespace":   namespace,
	}
	secretName, err := renderProjectSecretName(projectSecretTemplate, templateData)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to render project secret name")
	}

	// fetch the secret from Kubernetes
	projectSecret, err := kubeClient.GetSecret(ctx, namespace, secretName)
	if err != nil {
		// if not found, skip validation
		if kubeapierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, errors.Wrapf(err, "Failed to get secret %s", secretName)
	}
	return projectSecret, nil
}

func IsServiceAccountAllowed(secret *v1.Secret, secretAllowedServiceAccountsKey string, serviceAccount string) error {

	allowedServiceAccounts, found := getAllowedServiceAccountsFromSecret(secret, secretAllowedServiceAccountsKey)

	// if the key is found, but is empty, treat it as no allowed service accounts configured
	if len(allowedServiceAccounts) == 0 && found {
		return errors.Errorf("No service accounts are allowed")
	}
	if len(allowedServiceAccounts) == 0 {
		return nil
	}
	// trim spaces and check membership
	requestedSA := strings.ToLower(strings.TrimSpace(serviceAccount))
	for _, sa := range allowedServiceAccounts {
		if sa == requestedSA {
			return nil
		}
	}

	return errors.Errorf("Service account %q is not allowed", requestedSA)
}

func EnrichServiceAccount(secret *v1.Secret, secretDefaultServiceAccountsKey, serviceAccount, defaultPlatformServiceAccount string) string {
	if serviceAccount != "" {
		return serviceAccount
	}
	defaultProjectSa, _ := GetStringValueFromSecret(secret, secretDefaultServiceAccountsKey)
	if defaultProjectSa == "" {
		return defaultPlatformServiceAccount
	}
	return defaultProjectSa
}

// getAllowedServiceAccountsFromSecret retrieves the allowed service accounts from the secret
// It returns a slice of allowed service accounts and a boolean indicating if the key was found in the secret
func getAllowedServiceAccountsFromSecret(secret *v1.Secret, secretAllowedServiceAccountsKey string) (allowedServiceAccounts []string, found bool) {
	if secret == nil || secretAllowedServiceAccountsKey == "" {
		return
	}

	allowed, ok := GetStringValueFromSecret(secret, secretAllowedServiceAccountsKey)
	if !ok {
		// if the key is not found, return empty slice
		return
	}

	// if the key is found, set found to true and split the string by comma
	found = true
	rawAccounts := strings.Split(allowed, ",")
	for _, sa := range rawAccounts {
		trimmedLowered := strings.ToLower(strings.TrimSpace(sa))
		if trimmedLowered != "" {
			allowedServiceAccounts = append(allowedServiceAccounts, trimmedLowered)
		}
	}
	return
}

func renderProjectSecretName(projectSecretTemplate string, templateData map[string]interface{}) (string, error) {
	renderedIngressHost, err := common.RenderTemplate(projectSecretTemplate, templateData)
	if err != nil {
		return "", err
	}
	return renderedIngressHost, nil
}
