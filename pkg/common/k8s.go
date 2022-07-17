package common

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/nuclio/logger"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
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

// ResolveDefaultNamespace returns the proper default resource namespace, given the current default namespace
func ResolveDefaultNamespace(defaultNamespace string) string {
	switch defaultNamespace {
	case "@nuclio.selfNamespace":

		// get namespace from within the pod. if found, return that
		if namespacePod, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
			return string(namespacePod)
		}
		return "default"
	case "":
		return "default"
	default:
		return defaultNamespace
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
