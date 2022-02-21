package factory

import (
	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/auth/iguazio"
	"github.com/nuclio/nuclio/pkg/auth/nop"

	"github.com/nuclio/logger"
)

func NewAuth(logger logger.Logger, authConfig *auth.Config) auth.Auth {
	switch authConfig.Kind {
	case auth.KindIguazio:
		return iguazio.NewAuth(logger, authConfig)
	case auth.KindNop:
		return nop.NewAuth(logger, authConfig)
	default:
		return nop.NewAuth(logger, authConfig)
	}
}
