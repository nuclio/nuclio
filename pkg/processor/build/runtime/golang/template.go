package golang

const registryFileTemplate = `// Auto generated code by Nuclio
package main

import (
	golangruntimeeventhandler "github.com/nuclio/nuclio/pkg/processor/runtime/golang/event_handler"
	handler "{{functionPackage}}"
)

func init() {
     golangruntimeeventhandler.EventHandlers.Register("{{functionName}}", golangruntimeeventhandler.EventHandler({{functionPackage}}.{{functionHandler}}))
}
// Auto generated code by Nuclio
`

const processorBuilderDockerfileTemplate = `FROM nuclio/processor-builder-golang-onbuild:latest`
