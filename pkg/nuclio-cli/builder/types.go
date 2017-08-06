package builder

import "github.com/nuclio/nuclio/pkg/nuclio-cli"

// if there's ever another resource that requires building, move this to FunctionOptions and
// have Options contain `function FunctionOptions`
type Options struct {
	Common          *nucliocli.CommonOptions
	Path            string
	OutputType      string
	NuclioSourceDir string
	NuclioSourceURL string
	PushRegistry    string
	ImageName       string
}
