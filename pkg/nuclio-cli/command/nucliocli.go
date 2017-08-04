package command

import (
	"os"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/nuclio-cli"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/mitchellh/go-homedir"
	"path/filepath"
)

type RootCommandeer struct {
	cmd           *cobra.Command
	commonOptions nucliocli.CommonOptions
}

func NewRootCommandeer() *RootCommandeer {
	commandeer := &RootCommandeer{}

	cmd := &cobra.Command{
		Use:   "nuclio-cli [blah]",
		Short: "nuclio command line interface",
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

	logger, err := nucliozap.NewNuclioZap("nuclio-cli", loggerLevel)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create logger")
	}

	return logger, nil
}
