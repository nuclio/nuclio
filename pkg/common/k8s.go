package common

import (
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

func getKubeconfigFromHomeDir() string {
	homeDir, err := homedir.Dir()
	if err != nil {
		return ""
	}

	homeKubeConfigPath := filepath.Join(homeDir, ".kube", "config")

	// if the file exists @ home, use it
	_, err = os.Stat(homeKubeConfigPath)
	if err == nil {
		return homeKubeConfigPath
	}

	return ""
}
