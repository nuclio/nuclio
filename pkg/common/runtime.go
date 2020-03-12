package common

import (
	"os/exec"
	"strings"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

func GetPythonExePath(logger logger.Logger, runtimeVersion string) (string, error) {

	// allow user to provide with default python exe path
	defaultPythonExePath := GetEnvOrDefaultString("NUCLIO_PYTHON_EXE_PATH", "")
	if defaultPythonExePath != "" {
		return defaultPythonExePath, nil
	}

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
