package nucliodeploy

import (
	"fmt"
	"strings"
	"os/exec"

	"github.com/sirupsen/logrus"
	"github.com/pkg/errors"
)

func deploy(options *deployOptions) error {
	logrus.Debugf("Deploying %s", options)

	// push the image to the registry
	taggedImage, err := pushImageToRegistry(options.image, options.registryURL)
	if err != nil {
		return errors.Wrap(err, "Failed to push image to registry")
	}

	// create function custom resource
	err = createFunctionCR(options.kubeconfigPath, options.functionName, taggedImage, options.httpPort)
	if err != nil {
		return errors.Wrap(err, "Failed to created function custom resource")
	}

	return nil
}

func createFunctionCR(kubeconfigPath, functionName, taggedImage string, httpPort int) error {
	return nil
}

func pushImageToRegistry(processorImageName, registryURL string) (string, error) {
	taggedImage := fmt.Sprintf("%s/%s", registryURL, processorImageName)

	err := runCommand("docker tag %s %s", processorImageName, taggedImage)
	if err != nil {
		return "", errors.Wrap(err, "Unable to tag image")
	}

	// untag at the end, ignore errors
	defer runCommand("docker rmi %s", taggedImage)

	err = runCommand("docker push %s", taggedImage)
	if err != nil {
		return "", errors.Wrap(err, "Unable to push image")
	}

	return taggedImage, nil
}

func runCommand(format string, vars ...interface{}) error {

	// format the command
	command := fmt.Sprintf(format, vars...)

	logrus.Debugf("Executing: %s", command)

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
