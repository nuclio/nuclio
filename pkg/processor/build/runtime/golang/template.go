package golang

const registryFileTemplate = `// Auto generated code by Nuclio
package main

import (
	"github.com/nuclio/nuclio/pkg/processor/runtime/golang/event_handler"
	"github.com/nuclio/nuclio/cmd/processor/user_functions/{{functionName}}"
)

func init() {
     golangruntimeeventhandler.EventHandlers.Register("{{functionName}}", golangruntimeeventhandler.EventHandler({{functionPackage}}.{{functionHandler}}))
}
// Auto generated code by Nuclio
`

const  processorBuilderDockerfileTemplate = `FROM nuclio/processor-builder-golang-onbuild:latest`