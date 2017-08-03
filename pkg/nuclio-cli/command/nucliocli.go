package command

import (
	"os"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/nuclio-cli"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
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

	cmd.PersistentFlags().BoolVarP(&commandeer.commonOptions.Verbose, "verbose", "v", false, "verbose output")
	cmd.PersistentFlags().StringVarP(&commandeer.commonOptions.KubeconfigPath, "kubeconfig", "k", os.Getenv("KUBECONFIG"),
		"Path to Kubernetes config (admin.conf)")
	cmd.PersistentFlags().StringVarP(&commandeer.commonOptions.Namespace, "namespace", "n", "default", "Kubernetes namespace")

	// add children
	cmd.AddCommand(
		newGetCommandeer(commandeer).cmd,
		newDeleteCommandeer(commandeer).cmd,
	)

	commandeer.cmd = cmd

	return commandeer
}

func (ncc *RootCommandeer) Execute() error {
	return ncc.cmd.Execute()
}

func (ncc *RootCommandeer) createLogger() (nuclio.Logger, error) {
	var loggerLevel nucliozap.Level

	if ncc.commonOptions.Verbose {
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
