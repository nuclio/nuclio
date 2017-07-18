package app

import (
	"github.com/nuclio/nuclio/pkg/nuclio-build"
)

func Run() error {
	cmd := nucliobuild.NewNuclioBuildCommand()
	return cmd.Execute()
}
