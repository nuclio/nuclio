package nop

import (
	"context"
	"net/http"

	authpkg "github.com/nuclio/nuclio/pkg/auth"

	"github.com/nuclio/logger"
)

type Auth struct {
	logger     logger.Logger
	config     *authpkg.Config
	nopSession authpkg.Session
}

func NewAuth(logger logger.Logger, authConfig *authpkg.Config) authpkg.Auth {
	return &Auth{
		logger:     logger.GetChild("nop-auth"),
		config:     authConfig,
		nopSession: &authpkg.NopSession{},
	}
}

// Authenticate will do nothing
func (a *Auth) Authenticate(request *http.Request, options *authpkg.Options) (authpkg.Session, error) {
	return a.nopSession, nil
}

func (a *Auth) Middleware(options *authpkg.Options) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, _ := a.Authenticate(r, options)

			// nothing to do here
			ctx := context.WithValue(r.Context(), authpkg.NopContextKey, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (a *Auth) Kind() authpkg.Kind {
	return a.config.Kind
}
