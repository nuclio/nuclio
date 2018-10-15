package functiontemplates

import "github.com/nuclio/nuclio/pkg/functionconfig"

type FunctionTemplate struct {
	Name                   string
	DisplayName            string
	SourceCode             string
	FunctionConfigTemplate string
	FunctionConfigValues   string
	FunctionConfig         functionconfig.Config
}
