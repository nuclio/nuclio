package v4

import (
	"net/http"

	authpkg "github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/auth/iguazio"
	"github.com/nuclio/nuclio/pkg/auth/iguazio/v1"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

type AuthV4 struct {
	*iguazio.AbstractAuth
}

func NewAuth(logger logger.Logger, config *authpkg.Config) authpkg.Auth {
	return &v1.Auth{
		AbstractAuth: iguazio.NewAbstractAuth(logger, config),
	}
}

func (a *AuthV4) Authenticate(request *http.Request, options *authpkg.Options) (authpkg.Session, error) {
	return nil, nuclio.ErrNotImplemented
}

func (a *AuthV4) Middleware(options *authpkg.Options) func(next http.Handler) http.Handler {
	return a.AbstractAuth.Middleware(a.Authenticate, options)
}
