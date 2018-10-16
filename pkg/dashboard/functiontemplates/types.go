package functiontemplates

import (
	"github.com/nuclio/nuclio/pkg/functionconfig"
)

type functionTemplate struct {
	Name                   string
	DisplayName            string
	SourceCode             string
	FunctionConfigTemplate string
	FunctionConfigValues   string
	FunctionConfig         *functionconfig.Config
}
