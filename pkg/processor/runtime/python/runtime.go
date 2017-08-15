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

	configuration *Configuration
	entryPoint    string
	scriptPath    string
	env           []string
	ctx           context.Context
}

// NewRuntime returns a new Python runtime
func NewRuntime(parentLogger nuclio.Logger, configuration *Configuration) (runtime.Runtime, error) {

	// create the command string
	newPythonRuntime := &python{
		AbstractRuntime: *runtime.NewAbstractRuntime(parentLogger.GetChild("python").(nuclio.Logger), &configuration.Configuration),
		ctx:             context.Background(),
		configuration:   configuration,
	}

	// update it with some stuff so that we don't have to do this each invocation
	newPythonRuntime.entryPoint = newPythonRuntime.getEntryPoint()
	newPythonRuntime.scriptPath = newPythonRuntime.getScriptPath()
	newPythonRuntime.env = newPythonRuntime.getEnvFromConfiguration()

	envPath := fmt.Sprintf("PYTHONPATH=%s", newPythonRuntime.getPythonPath())
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

	// create a command
	cmd := exec.CommandContext(ctx, "/usr/bin/python3", py.scriptPath, py.entryPoint)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, errors.Wrap(err, "Can't create stdin pipe")
	}
	// set the command env
	cmd.Env = py.env
	// add event stuff to env
	cmd.Env = append(cmd.Env, py.getEnvFromEvent(event)...)

	enc := NewEventJSONEncoder(py.Logger, stdin)
	if err := enc.Encode(event); err != nil {
		return nil, errors.Wrap(err, "Can't encode event")
	}
	stdin.Close()

	// run the command
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
func (py *python) getScriptPath() string {
	scriptPath := os.Getenv("NUCLIO_PYTHON_WRAPPER")
	if len(scriptPath) == 0 {
		return "pkg/processor/runtime/python/wrapper.py"
	}

	return scriptPath
}

func (py *python) getPythonPath() string {
	pythonPath := os.Getenv("NUCLIO_PYTHON_PATH")
	if len(pythonPath) == 0 {
		return "test/e2e/python"
	}

	return pythonPath
}
