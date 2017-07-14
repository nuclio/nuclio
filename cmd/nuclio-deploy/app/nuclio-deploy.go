package app

import (
	"github.com/nuclio/nuclio/pkg/nuclio-deploy"
)

func Run() error {
	cmd := nucliodeploy.NewNuclioDeployCommand()
	return cmd.Execute()
}
