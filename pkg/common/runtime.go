package common

import (
	"os/exec"
	"strings"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

func GetPythonExePath(logger logger.Logger, runtimeVersion string) (string, error) {
	baseName := "python3"

	if strings.HasPrefix(runtimeVersion, "2") {
		baseName = "python2"
	}

	exePath, err := exec.LookPath(baseName)
	if err == nil {
		return exePath, nil
	}

	logger.WarnWith("Can't find specific python exe", "name", baseName)

	// Try just "python"
	exePath, err = exec.LookPath("python")
	if err == nil {
		return exePath, nil
	}

	return "", errors.Wrap(err, "Can't find python executable")
}
