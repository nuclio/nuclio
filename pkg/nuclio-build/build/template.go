package build

const registryFileTemplate = `// Auto generated code by Nuclio
package main

import (
	"github.com/nuclio/nuclio/pkg/processor/runtime/golang/event_handler"
	"github.com/nuclio/nuclio/cmd/processor/user_functions/{{.Name}}"
)

func init() {
     golangruntimeeventhandler.EventHandlers.Register("{{.Name}}", golangruntimeeventhandler.EventHandler({{.Name}}.{{.Handler}}))
}
// Auto generated code by Nuclio
`
