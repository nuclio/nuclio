package kube

import (
	"os"
	"path/filepath"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/mitchellh/go-homedir"
)

type CommonOptions struct {
	Namespace      string
	KubeconfigPath string
	KubeHost       string
	SpecFilePath   string
}

func (co *CommonOptions) GetDefaultKubeconfigPath() (string, error) {
	envKubeconfig := os.Getenv("KUBECONFIG")
	if envKubeconfig != "" {
		return envKubeconfig, nil
	}

	homeDir, err := homedir.Dir()
	if err != nil {
		return "", errors.Wrap(err, "Failed to get home directory")
	}

	homeKubeConfigPath := filepath.Join(homeDir, ".kube", "config")

	// if the file exists @ home, use it
	_, err = os.Stat(homeKubeConfigPath)
	if err == nil {
		return homeKubeConfigPath, nil
	}

	return "", nil
}

type DeployOptions struct {
}
