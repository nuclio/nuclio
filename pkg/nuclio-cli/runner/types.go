package runner

import (
	"github.com/nuclio/nuclio/pkg/nuclio-cli"
	"github.com/nuclio/nuclio/pkg/nuclio-cli/builder"
)

// if there's ever another resource that requires building, move this to FunctionOptions and
// have Options contain `function FunctionOptions`
type Options struct {
	Common      *nucliocli.CommonOptions
	Build       builder.Options
	SpecPath    string
	Description string
	Image       string
	Env         string
	Labels      string
	CPU         string
	Memory      string
	WorkDir     string
	Role        string
	Secret      string
	Events      string
	Data        string
	Disabled    bool
	Publish     bool
	HTTPPort    int32
	Scale       string
	MinReplicas int32
	MaxReplicas int32
}
