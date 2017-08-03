package app

import (
	"github.com/nuclio/nuclio/pkg/nuclio-cli/command"
)

func Run() error {
	return command.NewRootCommandeer().Execute()
}
