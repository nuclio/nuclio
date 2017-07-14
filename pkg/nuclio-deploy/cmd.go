package nucliodeploy

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/nuclio/nuclio/pkg/nuclio-deploy/deploy"
	"github.com/nuclio/nuclio/pkg/zap"
	"github.com/pkg/errors"
)

func NewNuclioDeployCommand() *cobra.Command {
	var options deploy.Options

	cmd := &cobra.Command{
		Use:   "nuclio-deploy",
		Short: "Deploy a nuclio function",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("Missing image")
			}

			options.ImageName = args[0]

			zap, err := nucliozap.NewNuclioZap("cmd")
			if err != nil {
				return errors.Wrap(err, "Failed to create logger")
			}

			if options.Verbose {
				// TODO
			}

			return deploy.NewDeployer(zap, &options).Deploy()
		},
	}

	cmd.PersistentFlags().BoolVarP(&options.Verbose, "verbose", "", false, "verbose output")
	cmd.Flags().StringVarP(&options.KubeconfigPath, "kubeconfig", "k", "", "Path to the kubectl configuration of the target cluster")
	cmd.Flags().StringVarP(&options.RegistryURL, "registry-url", "r", "", "URL of registry")
	cmd.Flags().IntVarP(&options.HTTPPort, "http-port", "p", 0, "Port on which HTTP requests will be served")

	return cmd
}
