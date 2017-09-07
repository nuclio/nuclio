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

package command

import (
	"os"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/nuctl"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"path/filepath"
)

type RootCommandeer struct {
	cmd           *cobra.Command
	commonOptions nucliocli.CommonOptions
}

func NewRootCommandeer() *RootCommandeer {
	commandeer := &RootCommandeer{}

	cmd := &cobra.Command{
		Use:          "nuctl [command]",
		Short:        "nuclio command line interface",
		SilenceUsage: true,
	}

	kubeconfigPathDefault, err := commandeer.getDefaultKubeconfigPath()
	if err != nil {
		kubeconfigPathDefault = ""
	}

	cmd.PersistentFlags().BoolVarP(&commandeer.commonOptions.Verbose, "verbose", "v", false, "verbose output")
	cmd.PersistentFlags().StringVarP(&commandeer.commonOptions.KubeconfigPath, "kubeconfig", "k", kubeconfigPathDefault,
		"Path to Kubernetes config (admin.conf)")
	cmd.PersistentFlags().StringVarP(&commandeer.commonOptions.Namespace, "namespace", "n", "default", "Kubernetes namespace")

	// add children
	cmd.AddCommand(
		newGetCommandeer(commandeer).cmd,
		newDeleteCommandeer(commandeer).cmd,
		newBuildCommandeer(commandeer).cmd,
		newRunCommandeer(commandeer).cmd,
		newExecuteCommandeer(commandeer).cmd,
		newUpdateCommandeer(commandeer).cmd,
	)

	commandeer.cmd = cmd

	return commandeer
}

func (rc *RootCommandeer) Execute() error {
	return rc.cmd.Execute()
}

func (rc *RootCommandeer) getDefaultKubeconfigPath() (string, error) {
	envKubeconfig := os.Getenv("KUBECONFIG")
	if envKubeconfig != "" {
		return envKubeconfig, nil
	}

	homeDir, err := homedir.Dir()
	if err != nil {
		return "", errors.Wrap(err, "Failed to get home directory")
	}

	return filepath.Join(homeDir, ".kube", "config"), nil
}

func (rc *RootCommandeer) createLogger() (nuclio.Logger, error) {
	var loggerLevel nucliozap.Level

	if rc.commonOptions.Verbose {
		loggerLevel = nucliozap.DebugLevel
	} else {
		loggerLevel = nucliozap.InfoLevel
	}

	logger, err := nucliozap.NewNuclioZapCmd("nuctl", loggerLevel)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create logger")
	}

	return logger, nil
}
