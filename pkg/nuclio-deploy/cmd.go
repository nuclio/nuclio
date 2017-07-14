package nucliodeploy

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/sirupsen/logrus"
)

type deployOptions struct {
	verbose      bool
	kubeconfigPath   string
	registryURL  string
	functionName string
	httpPort     int
	image        string
}

func NewNuclioDeployCommand() *cobra.Command {
	var options deployOptions

	cmd := &cobra.Command{
		Use:   "nuclio-deploy",
		Short: "Deploy a nuclio function",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("Missing image")
			}

			options.image = args[0]

			if options.verbose {
				logrus.SetLevel(logrus.DebugLevel)
			}

			deploy(&options)
		},
	}

	cmd.PersistentFlags().BoolVarP(&options.verbose, "verbose", "", false, "verbose output")
	cmd.Flags().StringVarP(&options.kubeconfigPath, "kubeconfig", "k", "", "Path to the kubectl configuration of the target cluster")
	cmd.Flags().StringVarP(&options.registryURL, "registry-url", "r", "", "URL of registry")
	cmd.Flags().StringVarP(&options.functionName, "function-name", "n", "", "Name of function")
	cmd.Flags().IntVarP(&options.httpPort, "http-port", "p", 0, "Port on which HTTP requests will be served")
}
