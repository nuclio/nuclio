package common

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

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
	if defaultNamespace == "@nuclio.selfNamespace" {

		// get namespace from within the pod. if found, return that
		if namespacePod, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
			return string(namespacePod)
		}
	}

	if defaultNamespace == "" {
		return "default"
	}

	return defaultNamespace
}

func CompileListFunctionPodsLabelSelector(functionName string) string {
	return fmt.Sprintf("nuclio.io/function-name=%s,nuclio.io/function-cron-job-pod!=true", functionName)
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
