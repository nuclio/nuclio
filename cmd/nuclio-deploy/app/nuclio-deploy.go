package app

import (
	"os"

	"github.com/nuclio/nuclio/pkg/nuclio-deploy"
	"github.com/spf13/cobra/cobra/cmd"
)

func Run() error {
	cmd := nucliodeploy.NewNuclioDeployCommand()
	return cmd.Execute()
}
