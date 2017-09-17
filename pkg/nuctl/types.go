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

package nuctl

import (
	"os"
	"path/filepath"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/mitchellh/go-homedir"
)

type CommonOptions struct {
	Verbose        bool
	Identifier     string
	Namespace      string
	KubeconfigPath string
	KubeHost       string
	SpecFilePath   string
}

func (co *CommonOptions) InitDefaults() {
	co.KubeconfigPath, _ = co.getDefaultKubeconfigPath()
	co.Namespace = "default"
}

func (co *CommonOptions) getDefaultKubeconfigPath() (string, error) {
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
