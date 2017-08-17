package python

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/pkg/errors"
)

type python struct {
	runtime.AbstractRuntime

	configuration     *Configuration
	entryPoint        string
	wrapperScriptPath string
	pythonExe         string
	env               []string
	ctx               context.Context
}

// NewRuntime returns a new Python runtime
func NewRuntime(parentLogger nuclio.Logger, configuration *Configuration) (runtime.Runtime, error) {
	logger := parentLogger.GetChild("python").(nuclio.Logger)

	// create the command string
	newPythonRuntime := &python{
		AbstractRuntime: *runtime.NewAbstractRuntime(logger, &configuration.Configuration),
		ctx:             context.Background(),
		configuration:   configuration,
	}

	// update it with some stuff so that we don't have to do this each invocation
	newPythonRuntime.entryPoint = newPythonRuntime.getEntryPoint()
	logger.InfoWith("Python entry point", "entry_point", newPythonRuntime.entryPoint)
	newPythonRuntime.wrapperScriptPath = newPythonRuntime.getWrapperScriptPath()
	logger.InfoWith("Python wrapper script path", "path", newPythonRuntime.wrapperScriptPath)

	var err error
	newPythonRuntime.pythonExe, err = newPythonRuntime.getPythonExe()
	if err != nil {
		logger.ErrorWith("Can't find Python exe", "error", err)
		return nil, err
	}
	logger.InfoWith("Python executable", "path", newPythonRuntime.pythonExe)

	newPythonRuntime.env = newPythonRuntime.getEnvFromConfiguration()
	envPath := fmt.Sprintf("PYTHONPATH=%s", newPythonRuntime.getPythonPath())
	logger.InfoWith("PYTHONPATH", "value", envPath)
	newPythonRuntime.env = append(newPythonRuntime.env, envPath)

	return newPythonRuntime, nil
}

func (py *python) ProcessEvent(event nuclio.Event) (interface{}, error) {
	py.Logger.DebugWith("Executing python",
		"name", py.configuration.Name,
		"version", py.configuration.Version,
		"eventID", event.GetID())

	// create a timeout context
	ctx, cancel := context.WithTimeout(py.ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, py.pythonExe, py.wrapperScriptPath, py.entryPoint)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, errors.Wrap(err, "Can't create stdin pipe")
	}
	cmd.Env = py.env
	cmd.Env = append(cmd.Env, py.getEnvFromEvent(event)...)

	enc := NewEventJSONEncoder(py.Logger, stdin)
	if err := enc.Encode(event); err != nil {
		return nil, errors.Wrap(err, "Can't encode event")
	}
	stdin.Close()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to run python command")
	}

	py.Logger.DebugWith("python executed",
		"out", string(out),
		"eventID", event.GetID())

	return out, nil
}

func (py *python) getEnvFromConfiguration() []string {
	return []string{
		fmt.Sprintf("NUCLIO_FUNCTION_NAME=%s", py.configuration.Name),
		fmt.Sprintf("NUCLIO_FUNCTION_DESCRIPTION=%s", py.configuration.Description),
		fmt.Sprintf("NUCLIO_FUNCTION_VERSION=%s", py.configuration.Version),
	}
}

func (py *python) getEnvFromEvent(event nuclio.Event) []string {
	return []string{
		fmt.Sprintf("NUCLIO_EVENT_ID=%s", event.GetID()),
		fmt.Sprintf("NUCLIO_EVENT_SOURCE_CLASS=%s", event.GetSource().GetClass()),
		fmt.Sprintf("NUCLIO_EVENT_SOURCE_KIND=%s", event.GetSource().GetKind()),
	}
}

func (py *python) getEntryPoint() string {
	return py.configuration.EntryPoint
}

// TODO: Global processor configuration, where should this go?
func (py *python) getWrapperScriptPath() string {
	scriptPath := os.Getenv("NUCLIO_PYTHON_WRAPPER")
	if len(scriptPath) == 0 {
		return "/opt/nuclio/wrapper.py"
	}

	return scriptPath
}

func (py *python) getPythonPath() string {
	pythonPath := os.Getenv("NUCLIO_PYTHON_PATH")
	if len(pythonPath) == 0 {
		return "/opt/nuclio"
	}

	return pythonPath
}

func (py *python) getPythonExe() (string, error) {
	baseName := "python3"
	if py.configuration.PythonVersion == "2" {
		baseName = "python2"
	}

	exePath, err := exec.LookPath(baseName)
	if err == nil {
		return exePath, nil
	}

	py.Logger.WarnWith("Can't find specific python exe", "name", baseName)
	// Try just "python"
	exePath, err = exec.LookPath("python")
	if err == nil {
		return exePath, nil
	}

	return "", errors.Wrap(err, "Can't find python executable")
}
