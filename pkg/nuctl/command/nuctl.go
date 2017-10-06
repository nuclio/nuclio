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
	"fmt"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube"
	"github.com/spf13/cobra"
	"github.com/nuclio/nuclio/pkg/platform/local"
)

type RootCommandeer struct {
	logger            nuclio.Logger
	cmd               *cobra.Command
	platformName      string
	platform          platform.Platform
	commonOptions     platform.CommonOptions
	kubeCommonOptions kube.CommonOptions
}

func NewRootCommandeer() *RootCommandeer {
	commandeer := &RootCommandeer{}

	cmd := &cobra.Command{
		Use:           "nuctl [command]",
		Short:         "nuclio command line interface",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// init defaults for common options
	commandeer.commonOptions.InitDefaults()

	cmd.PersistentFlags().BoolVarP(&commandeer.commonOptions.Verbose, "verbose", "v", false, "verbose output")
	cmd.PersistentFlags().StringVarP(&commandeer.platformName, "platform", "", "k8s", "One of k8s/local")
	cmd.PersistentFlags().StringVarP(&commandeer.kubeCommonOptions.KubeconfigPath, "kubeconfig", "k", commandeer.kubeCommonOptions.KubeconfigPath,
		"Path to Kubernetes config (admin.conf)")
	cmd.PersistentFlags().StringVarP(&commandeer.kubeCommonOptions.Namespace, "namespace", "n", "default", "Kubernetes namespace")

	// add children
	cmd.AddCommand(
		newBuildCommandeer(commandeer).cmd,
		newDeployCommandeer(commandeer).cmd,
		newInvokeCommandeer(commandeer).cmd,
		newGetCommandeer(commandeer).cmd,
		newDeleteCommandeer(commandeer).cmd,
		newUpdateCommandeer(commandeer).cmd,
	)

	commandeer.cmd = cmd

	return commandeer
}

func (rc *RootCommandeer) Execute() error {
	return rc.cmd.Execute()
}

func (rc *RootCommandeer) initialize() error {
	var err error

	rc.logger, err = rc.createLogger()
	if err != nil {
		return errors.Wrap(err, "Failed to create logger")
	}

	rc.platform, err = rc.createPlatform(rc.logger)
	if err != nil {
		return errors.Wrap(err, "Failed to create logger")
	}

	return nil
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

func (rc *RootCommandeer) createPlatform(logger nuclio.Logger) (platform.Platform, error) {
	switch rc.platformName {
	case "k8s":
		kubeconfigPath, err := rc.kubeCommonOptions.GetDefaultKubeconfigPath()
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get kubeconfig path")
		}

		// set platform specific common
		rc.commonOptions.Platform = &rc.kubeCommonOptions

		return kube.NewPlatform(logger, kubeconfigPath)

	case "local":
		return local.NewPlatform(logger)
	}

	return nil, fmt.Errorf("Can't create platform - unsupported: %s", rc.platformName)
}
