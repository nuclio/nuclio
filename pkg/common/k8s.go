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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
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
	return fmt.Sprintf("nuclio.io/function-name=%s,nuclio.io/function-cron-job-pod!=true", functionName)
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

func ValidateNodeSelector(nodeSelector map[string]string) error {
	if nodeSelector == nil {
		return nil
	}
	for labelKey, labelValue := range nodeSelector {
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
