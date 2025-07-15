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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func IsInKubernetesCluster() bool {
	return len(os.Getenv("KUBERNETES_SERVICE_HOST")) != 0 && len(os.Getenv("KUBERNETES_SERVICE_PORT")) != 0
}

func GetClientConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}

	return rest.InClusterConfig()
}

func GetKubeconfigPath(kubeconfigPath string) string {

	// do we still not have a kubeconfig path?
	if kubeconfigPath == "" {
		return GetEnvOrDefaultString("KUBECONFIG", getKubeconfigFromHomeDir())
	}
	return kubeconfigPath
}

func GetKubeConfigClientCmdByKubeconfigPath(kubeconfigPath string) (*clientcmdapi.Config, error) {
	configLoadRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configLoadRules.ExplicitPath = GetKubeconfigPath(kubeconfigPath)
	clientCmd, err := configLoadRules.Load()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to load kubeconfig")
	}
	return clientCmd, nil
}

// ResolveNamespace returns the namespace by the following order:
// 1. If namespace is passed as an argument, use that
// 2. If namespace is passed as an environment variable, use that
// 3. Alternatively, use "this" namespace (where the pod is running)
func ResolveNamespace(namespaceArgument string, defaultEnvVarKey string) string {
	// if the namespace was passed in the arguments, use that
	if namespaceArgument != "" {
		return namespaceArgument
	}

	// if the namespace exists in env, use that, else, assume "this" namespace
	return ResolveDefaultNamespace(GetEnvOrDefaultString(defaultEnvVarKey, "@nuclio.selfNamespace"))
}

// ResolveDefaultNamespace returns the proper default resource namespace, given the current default namespace
func ResolveDefaultNamespace(namespace string) string {

	defaultNamespace := "default"
	switch namespace {
	case "@nuclio.selfNamespace":

		// for k8s
		if IsInKubernetesCluster() {
			// get namespace from within the pod. if found, return that
			if namespacePod, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
				return string(namespacePod)
			}
			return defaultNamespace
		} else if RunningInContainer() {
			// for local platform
			return "nuclio"
		}

		// for development
		return defaultNamespace
	case "":
		return defaultNamespace
	default:
		return namespace
	}
}

func CompileListFunctionPodsLabelSelector(functionName string) string {
	return fmt.Sprintf("%s=%s,%s!=true", NuclioResourceLabelKeyFunctionName, functionName, NuclioLabelKeyFunctionCronJobPod)
}

type KubernetesClientWarningHandler struct {
	logger logger.Logger
}

func NewKubernetesClientWarningHandler(logger logger.Logger) *KubernetesClientWarningHandler {
	return &KubernetesClientWarningHandler{
		logger: logger,
	}
}

// CompileStalePodsFieldSelector creates a field selector(string) for stale pods
func CompileStalePodsFieldSelector() string {
	var fieldSelectors []string

	// filter out non-stale pods by their phase
	nonStalePodPhases := []v1.PodPhase{v1.PodPending, v1.PodRunning}
	for _, nonStalePodPhase := range nonStalePodPhases {
		selector := fmt.Sprintf("status.phase!=%s", string(nonStalePodPhase))
		fieldSelectors = append(fieldSelectors, selector)
	}

	return strings.Join(fieldSelectors, ",")
}

// HandleWarningHeader handles miscellaneous warning messages yielded by Kubernetes api server
// e.g.: "autoscaling/v2beta1 HorizontalPodAutoscaler is deprecated in v1.22+, unavailable in v1.25+; use autoscaling/v2beta2 HorizontalPodAutoscaler"
// Note: code is determined by the Kubernetes server
func (kcl *KubernetesClientWarningHandler) HandleWarningHeader(code int, agent string, message string) {
	if code != 299 || len(message) == 0 {
		return
	}

	// special handling for deprecation warnings
	if strings.Contains(message, "is deprecated") {
		kcl.logger.WarnWith("Kubernetes deprecation alert", "message", message, "agent", agent)
		return
	}
	kcl.logger.WarnWith(message, "agent", agent)
}

func getKubeconfigFromHomeDir() string {
	homeDir, err := homedir.Dir()
	if err != nil {
		return ""
	}

	homeKubeConfigPath := filepath.Join(homeDir, ".kube", "config")

	// if the file exists @ home, use it
	if _, err := os.Stat(homeKubeConfigPath); err == nil {
		return homeKubeConfigPath
	}

	return ""
}

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
	if secret == nil {
		return "", false
	}
	value, ok := secret.Data[key]
	return string(value), ok
}

func EnrichAndValidateServiceAccount(ctx context.Context,
	kubeClient kubernetes.Interface,
	defaultPlatformServiceAccount,
	projectSecretTemplate,
	projectSecretDefaultServiceAccountKey,
	projectSecretAllowedServiceAccountsKey,
	serviceAccount, projectName, namespace string, shouldEnrich bool) (string, error) {

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

func GetProjectSecret(ctx context.Context, kubeClient kubernetes.Interface, projectSecretTemplate, projectName, namespace string) (*v1.Secret, error) {
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
	projectSecret, err := kubeClient.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		// if not found, skip validation
		if apierrors.IsNotFound(err) {
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
	renderedIngressHost, err := RenderTemplate(projectSecretTemplate, templateData)
	if err != nil {
		return "", err
	}
	return renderedIngressHost, nil
}
