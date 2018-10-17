package functiontemplates

import (
	"github.com/nuclio/nuclio/pkg/functionconfig"
)

type FunctionTemplate struct {
	Name                   string
	DisplayName            string
	SourceCode             string
	FunctionConfigTemplate string
	FunctionConfigValues   string
	FunctionConfig         *functionconfig.Config
	serializedTemplate     []byte
}

type generatedFunctionTemplate struct {
	Name               string
	DisplayName        string
	Configuration      functionconfig.Config
	SourceCode         string
	serializedTemplate []byte
}
