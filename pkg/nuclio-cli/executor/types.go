package executor

import "github.com/nuclio/nuclio/pkg/nuclio-cli"

type Options struct {
	Common      *nucliocli.CommonOptions
	ClusterIP   string
	ContentType string
	Url         string
	Method      string
	Body        string
	Headers     string
}
