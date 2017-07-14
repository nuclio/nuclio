package cmdutil

import (
	"os/exec"
	"fmt"
	"strings"

	"github.com/nuclio/nuclio-sdk/logger"
)

func RunCommand(loggerInstance logger.Logger, format string, vars ...interface{}) error {

	// format the command
	command := fmt.Sprintf(format, vars...)

	loggerInstance.DebugWith("Executing", "command", command)

	// split the command at spaces
	splitCommand := strings.Split(command, " ")

	// get the name of the command (first word)
	name := splitCommand[0]

	// get args, if they were passed
	args := []string{}

	if len(splitCommand) > 1 {
		args = splitCommand[1:]
	}

	// execute and return
	return exec.Command(name, args...).Run()
}
