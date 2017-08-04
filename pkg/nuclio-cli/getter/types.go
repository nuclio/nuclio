package getter

import "github.com/nuclio/nuclio/pkg/nuclio-cli"

type Options struct {
	Common             *nucliocli.CommonOptions
	AllNamespaces      bool
	NotList            bool
	Watch              bool
	Labels             string
	Format             string
	ResourceIdentifier string
}
