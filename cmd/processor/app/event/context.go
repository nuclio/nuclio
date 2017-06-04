package event

import (
	"github.com/nuclio/nuclio/pkg/logger"
)

type Context struct {
	FunctionName    string
	FunctionVersion string
	Logger          logger.Logger
	EventID         ID
}
