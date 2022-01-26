package nop

import (
	"context"
	"net/http"

	"github.com/nuclio/nuclio/pkg/dashboard/auth"

	"github.com/nuclio/logger"
)

type Auth struct {
	logger     logger.Logger
	config     *auth.Config
	nopSession auth.Session
}

func NewAuth(logger logger.Logger, authConfig *auth.Config) auth.Auth {
	return &Auth{
		logger:     logger.GetChild("nop-auth"),
		config:     authConfig,
		nopSession: &auth.NopSession{},
	}
}

// Authenticate will do nothing
func (a *Auth) Authenticate(request *http.Request, options auth.Options) (auth.Session, error) {
	return a.nopSession, nil
}

func (a *Auth) Middleware(options auth.Options) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, _ := a.Authenticate(r, options)

			// nothing to do here
			ctx := context.WithValue(r.Context(), auth.NopContextKey, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (a *Auth) Kind() auth.Kind {
	return a.config.Kind
}
