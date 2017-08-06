package updater

import (
	"github.com/nuclio/nuclio/pkg/nuclio-cli"
	"github.com/nuclio/nuclio/pkg/nuclio-cli/runner"
)

type Options struct {
	Common *nucliocli.CommonOptions
	Run    runner.Options
	Alias  string
}
