package python

import (
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	"path"
	"fmt"
)

type python struct {
	runtime.AbstractRuntime
}

// returns the image name of the default processor base image
func (p *python) GetDefaultProcessorBaseImage() string {
	return "python:3-alpine"
}

// given a path holding a function (or functions) returns a list of all the handlers
// in that directory
func (p *python) DetectFunctionHandlers(functionPath string) ([]string, error) {
	return []string{p.getFunctionHandler()}, nil
}

func (p *python) GetProcessorConfigFileContents() string {
	processorConfigFileContentsFormat := `
function:
  kind: "python"
  python_version: "3"
  handler: %s
`
	return fmt.Sprintf(processorConfigFileContentsFormat, p.getFunctionHandler())
}

func (p *python) GetStagingDirObjectPaths() []string {
	return []string{
		p.Configuration.GetFunctionPath(),
	}
}

func (p *python) getFunctionHandler() string {

	// use the function path: /some/path/func.py -> func
	functionFileName := path.Base(p.Configuration.GetFunctionPath())
	functionFileName = functionFileName[:len(functionFileName)-len(path.Ext(functionFileName))]

	// take that file name without extension and add a default "handler"
	// TODO: parse the python sources for this
	return fmt.Sprintf("%s:%s", functionFileName, "handler")
}
