package cmdutil

import (
	"os/exec"
	"fmt"
	"strings"

	"github.com/nuclio/nuclio-sdk"
)

type Options struct {
	WorkingDir string
	Env        map[string]string
}

func RunCommand(loggerInstance nuclio.Logger, options *Options, format string, vars ...interface{}) error {

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

	cmd := exec.Command(name, args...)

	// if working directory set, set it
	if options != nil && options.WorkingDir != "" {
		cmd.Dir = options.WorkingDir
	}

	return cmd.Run()
}
