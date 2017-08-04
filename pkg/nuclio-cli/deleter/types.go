package deleter

import "github.com/nuclio/nuclio/pkg/nuclio-cli"

type Options struct {
	Common             *nucliocli.CommonOptions
	ResourceIdentifier string
}
